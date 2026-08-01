// Package network provides the TCP listener and per-connection read loop that
// drive a Protocol implementation, mirroring the C++ ServicePort/Connection.
package network

import (
	"bufio"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/transport"
)

const (
	headerLength    = 2
	inputMessageMax = 24576 // generous cap; C++ uses 4096 for game, larger for content
)

// Protocol is implemented by the login and game handlers.
type Protocol interface {
	// OnConnect runs right after the socket is accepted (before any client
	// bytes). The game protocol sends its challenge here.
	OnConnect(c *Connection)
	// OnFirstPacket receives the raw first packet body (pre-encryption). The
	// checksum has NOT been stripped; use transport.StripFirstPacketChecksum.
	OnFirstPacket(c *Connection, body []byte)
	// OnPacket receives every subsequent decrypted packet.
	OnPacket(c *Connection, r *netmsg.Reader)
	// OnDisconnect runs once when the connection closes.
	OnDisconnect(c *Connection)
}

// Connection wraps a TCP socket with the transport codec and its protocol.
type Connection struct {
	conn       net.Conn
	codec      *transport.Codec
	proto      Protocol
	serverName string
	log        *slog.Logger
	writeMu    sync.Mutex
	closeOne   sync.Once
	gotFirst   bool

	// Data is a per-connection scratch pointer protocols use to stash session
	// state (e.g. the logged-in player).
	Data any
	// RawFirstPacket tells serve() to skip the 2-byte header on the first read.
	RawFirstPacket bool
}

// Codec exposes the transport codec so protocols can flip encryption state.
func (c *Connection) Codec() *transport.Codec { return c.codec }

// Logger returns the connection logger.
func (c *Connection) Logger() *slog.Logger { return c.log }

// RemoteIP returns the peer IPv4 as a uint32 (little-endian octet order, as the
// protocol stores IPs).
func (c *Connection) RemoteIP() uint32 {
	host, _, err := net.SplitHostPort(c.conn.RemoteAddr().String())
	if err != nil {
		return 0
	}
	ip := net.ParseIP(host).To4()
	if ip == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(ip)
}

// RemoteAddr returns the peer address string.
func (c *Connection) RemoteAddr() string { return c.conn.RemoteAddr().String() }

// Send finalizes and writes a message using the current codec state.
func (c *Connection) Send(w *netmsg.Writer) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	wire := c.codec.Wrap(w)
	if len(wire) > 6 {
		_ = uint16(wire[0]) | uint16(wire[1])<<8
		// slog.Debug("wire packet", "len", len(wire), "outer", outer, "first_enc", fmt.Sprintf("0x%02X", wire[6]))
	}
	_, err := c.conn.Write(wire)
	return err
}

// Close shuts the connection down (idempotent).
// WriteRaw sends bytes directly without encryption (used by status protocol).
func (c *Connection) WriteRaw(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.conn.Write(data)
	return err
}

func (c *Connection) Close() {
	c.closeOne.Do(func() {
		_ = c.conn.Close()
		if c.proto != nil {
			c.proto.OnDisconnect(c)
		}
	})
}

// proxyIdentPeekTimeout bounds how long we wait for a client's opening bytes to
// decide whether it leads with a proxy-identification prefix. Real clients send
// it immediately on connect; a challenge-first client (our headless client)
// sends nothing until it has received the challenge, so it hits this timeout.
const proxyIdentPeekTimeout = 500 * time.Millisecond

// consumeProxyIdentification mirrors the C++ Connection::parseProxyIdentification.
// A real client opens the game connection by sending "<serverName>\n" before the
// game protocol proper, and only then reads the server's challenge — so the
// challenge must be sent AFTER this returns. It is detected — rather than a
// normal packet — when the first two bytes case-insensitively match the server
// name and the high length byte is non-zero (a real length header for a small
// first packet is 0x00 there). When matched, the whole "<serverName>\n" string is
// consumed and discarded. Returns true iff a proxy-identification prefix was
// consumed. Clients that wait for the challenge first send no bytes yet, so the
// bounded peek times out and we return false without consuming anything.
func (c *Connection) consumeProxyIdentification(r *bufio.Reader) bool {
	if c.serverName == "" {
		return false
	}
	ident := c.serverName + "\n"

	_ = c.conn.SetReadDeadline(time.Now().Add(proxyIdentPeekTimeout))
	prefix, err := r.Peek(2)
	_ = c.conn.SetReadDeadline(time.Time{})
	if err != nil || len(prefix) < 2 {
		return false // nothing sent yet (challenge-first client) or read error
	}
	if prefix[1] == 0x00 || !strings.EqualFold(string(prefix), ident[:2]) {
		return false // a normal packet, not proxy identification
	}

	buf := make([]byte, len(ident))
	if _, err := io.ReadFull(r, buf); err != nil {
		return false
	}
	if !strings.EqualFold(string(buf), ident) {
		c.log.Debug("proxy identification mismatch", "got", string(buf), "want", ident)
		return false
	}
	c.log.Debug("consumed proxy identification", "server", c.serverName)
	return true
}

// serve runs the read loop.
func (c *Connection) serve() {
	defer c.Close()

	// Real clients open the connection with a proxy-identification prefix
	// ("<serverName>\n") and only then read the server's challenge; consume it
	// first so the challenge (sent by OnConnect) lands after it, matching the
	// reference server's ordering. Challenge-first clients send nothing yet, so
	// the bounded peek returns quickly and the challenge still goes out first.
	r := bufio.NewReader(c.conn)
	c.consumeProxyIdentification(r)
	c.proto.OnConnect(c)

	// Raw first-packet: the login service needs the bytes unframed so it can tell
	// an HTTP request or a MyAAC status probe from a binary Tibia login.
	if c.RawFirstPacket {
		c.RawFirstPacket = false
		if raw := c.readFirstPacketRaw(r); len(raw) > 0 {
			c.gotFirst = true
			c.proto.OnFirstPacket(c, raw)
			// After raw first packet, return to avoid re-entering the framed loop.
			return
		}
		return
	}

	header := make([]byte, headerLength)
	for {
		if _, err := io.ReadFull(r, header); err != nil {
			return
		}
		size := c.codec.DecodeBodySize(binary.LittleEndian.Uint16(header))
		if size <= 0 || size > inputMessageMax {
			c.log.Debug("invalid body size", "size", size)
			return
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}

		if !c.gotFirst {
			c.gotFirst = true
			c.proto.OnFirstPacket(c, body)
			continue
		}
		r, err := c.codec.Unwrap(body)
		if err != nil {
			c.log.Debug("unwrap failed", "err", err)
			return
		}
		c.proto.OnPacket(c, r)
	}
}

// firstPacketQuietPeriod bounds how long readFirstPacketRaw waits for more bytes
// once it has some. A client sends its login packet in one burst, so anything
// still in flight arrives within a round trip.
const firstPacketQuietPeriod = 250 * time.Millisecond

// readFirstPacketRaw reads the whole first packet, not just whatever one Read
// call happened to return.
//
// TCP does not preserve write boundaries: a client's ~500-byte login packet
// (metadata, a 128-byte RSA block, the OGL strings, and a second RSA block for
// the authenticator token) can arrive in several segments. A single Read then
// returns a prefix, and the login parser sees a frame whose outer header does
// not describe the bytes it has —
//
//	login: unrecognised outer header declared=... rest=...
//
// Worse, the remainder is left unread in the socket. Closing a TCP connection
// with unread data in the receive queue sends RST rather than FIN, and an RST
// discards whatever is still in the send buffer — so the reply we just wrote
// can be thrown away before the client reads it.
//
// The framed loop below never had this problem: it reads a header and then
// exactly that many bytes. Only this raw path, which exists for the HTTP and
// MyAAC probes, read opportunistically.
//
// The length is not known up front here — that is the whole point of the raw
// path — so it keeps reading until the peer goes quiet, capped at the same
// buffer size as before.
func (c *Connection) readFirstPacketRaw(r *bufio.Reader) []byte {
	buf := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)

	// The first read blocks: the client may not have sent anything yet.
	n, err := r.Read(chunk)
	if n > 0 {
		buf = append(buf, chunk[:n]...)
	}
	if err != nil {
		return buf
	}

	for len(buf) < cap(buf) {
		if r.Buffered() == 0 {
			// Nothing already buffered: give the network one short window to
			// deliver the rest of the burst before deciding the packet is whole.
			_ = c.conn.SetReadDeadline(time.Now().Add(firstPacketQuietPeriod))
			n, err = r.Read(chunk)
			_ = c.conn.SetReadDeadline(time.Time{})
		} else {
			n, err = r.Read(chunk)
		}
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if err != nil {
			break // timeout (the packet is complete) or a real read error
		}
	}
	return buf
}
