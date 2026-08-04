// Command canary-client is a headless test client that speaks the same wire
// protocol as canary-go. It performs the full flow — account login, character
// list, game login, enter world, then walk/chat/ping/logout — proving the
// server and a client can connect and play.
package main

import (
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/omurilo/canary-go/internal/netmsg"
	"github.com/omurilo/canary-go/internal/tibcrypto"
	"github.com/omurilo/canary-go/internal/transport"
)

const clientVersion = 1525

func main() {
	host := flag.String("host", "127.0.0.1", "server host")
	loginPort := flag.Int("login", 7171, "login port")
	gamePort := flag.Int("game", 7172, "game port")
	account := flag.String("account", "god", "account name/email")
	password := flag.String("password", "god", "password")
	rsaFile := flag.String("rsa", "key.pem", "RSA key file (public part used to encrypt)")
	flag.Parse()

	rsa, err := tibcrypto.LoadRSAFromPEM(*rsaFile)
	if err != nil {
		log.Fatalf("load rsa: %v", err)
	}

	// Fresh XTEA key for the session.
	var keyBytes [16]byte
	if _, err := rand.Read(keyBytes[:]); err != nil {
		log.Fatal(err)
	}
	key := tibcrypto.KeyFromBytes(keyBytes[:])

	charName, err := doLogin(*host, *loginPort, *account, *password, key, keyBytes, rsa)
	if err != nil {
		log.Fatalf("login failed: %v", err)
	}
	log.Printf("✅ login OK — character selected: %q", charName)

	if err := doGame(*host, *gamePort, *account, *password, charName, key, keyBytes, rsa); err != nil {
		log.Fatalf("game failed: %v", err)
	}
	log.Printf("✅ game session completed successfully")
}

// ---- framed I/O helpers ----

func send(conn net.Conn, codec *transport.Codec, w *netmsg.Writer) error {
	_, err := conn.Write(codec.Wrap(w))
	return err
}

func recv(conn net.Conn, codec *transport.Codec) (*netmsg.Reader, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	size := codec.DecodeBodySize(binary.LittleEndian.Uint16(header))
	if size <= 0 || size > 65535 {
		return nil, fmt.Errorf("bad body size %d", size)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	if !codec.Encryption {
		return netmsg.NewReader(transport.StripFirstPacketChecksum(body)), nil
	}
	return codec.Unwrap(body)
}

// buildRSABlock assembles and encrypts the 128-byte login block.
func buildRSABlock(keyBytes [16]byte, fields func(*netmsg.Writer), rsa *tibcrypto.RSA) []byte {
	inner := netmsg.NewWriter()
	inner.AddByte(0x00) // leading zero
	inner.AddBytes(keyBytes[:])
	fields(inner)
	block := make([]byte, tibcrypto.BlockSize)
	copy(block, inner.Bytes())
	// remaining bytes stay zero (padding)
	_ = rsa.Encrypt(block)
	return block
}

// ---- login flow ----

func doLogin(host string, port int, account, password string, key tibcrypto.XTEAKey, keyBytes [16]byte, rsa *tibcrypto.RSA) (string, error) {
	conn, err := net.Dial("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	codec := transport.New()

	w := netmsg.NewWriter()
	w.AddByte(0x01)              // protocol id: login
	w.AddU16(6)                  // OS: NEW_MAC-ish; any value
	w.AddU16(clientVersion)      // protocol version
	w.AddU32(clientVersion)      // client version
	w.AddBytes(make([]byte, 13)) // 12 asset signatures + 1 preview byte
	w.AddBytes(buildRSABlock(keyBytes, func(b *netmsg.Writer) {
		b.AddString(account)
		b.AddString(password)
	}, rsa))
	if err := send(conn, codec, w); err != nil {
		return "", err
	}
	codec.EnableModernLogin(key)

	// Read responses until we get the character list.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(deadline)
		r, err := recv(conn, codec)
		if err != nil {
			return "", err
		}
		for r.Remaining() > 0 {
			op := r.GetByte()
			switch op {
			case 0x0B: // error
				return "", fmt.Errorf("server error: %s", r.GetString())
			case 0x14: // MOTD
				log.Printf("   MOTD: %s", r.GetString())
			case 0x28: // session key
				log.Printf("   session key received")
			case 0x64: // character list
				return parseCharList(r)
			default:
				log.Printf("   login opcode 0x%02X", op)
			}
		}
	}
	return "", fmt.Errorf("timed out waiting for character list")
}

func parseCharList(r *netmsg.Reader) (string, error) {
	worlds := r.GetByte()
	for i := byte(0); i < worlds; i++ {
		_ = r.GetByte() // world id
		name := r.GetString()
		ip := r.GetString()
		port := r.GetU16()
		_ = r.GetByte() // preview
		log.Printf("   world %q at %s:%d", name, ip, port)
	}
	count := r.GetByte()
	var first string
	for i := byte(0); i < count; i++ {
		_ = r.GetByte() // world id
		name := r.GetString()
		if first == "" {
			first = name
		}
		log.Printf("   character: %s", name)
	}
	if first == "" {
		return "", fmt.Errorf("no characters on account")
	}
	return first, nil
}

// ---- game flow ----

func doGame(host string, port int, account, password, charName string, key tibcrypto.XTEAKey, keyBytes [16]byte, rsa *tibcrypto.RSA) error {
	conn, err := net.Dial("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		return err
	}
	defer conn.Close()
	codec := transport.New()
	// GameProtocol.OnConnect applies this before it writes the challenge, so the
	// outer length is a block count, not a byte count. Without the same profile here
	// DecodeBodySize read the challenge's 12-byte body as 1 byte.
	codec.ApplyProfile(transport.ProfileCurrentModern)

	// 1. Read the challenge (plaintext).
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	r, err := recv(conn, codec)
	if err != nil {
		return fmt.Errorf("read challenge: %w", err)
	}
	op := r.GetByte()
	if op == 0x01 {
		op = r.GetByte() // modern challenge has a leading 0x01 marker before 0x1F
	}
	if op != 0x1F {
		return fmt.Errorf("expected challenge 0x1F, got 0x%02X", op)
	}
	challengeTS := r.GetU32()
	challengeRand := r.GetByte()
	log.Printf("   challenge ts=%d rand=%d", challengeTS, challengeRand)

	// 2. Send game login. The layout has to match ProtocolGame.OnFirstPacket
	// exactly: it skips firstHeaderBytes() (4-byte checksum + 2 marker bytes on the
	// modern profile) and then reads OS, versions and the RSA block — with the
	// challenge echo INSIDE that block, after the character name.
	sessionKey := account + "\n" + password

	payload := netmsg.NewWriter()
	payload.AddByte(0x0A)         // protocol id
	payload.AddByte(0x00)         // second marker byte; the server skips both
	payload.AddU16(6)             // OS
	payload.AddU16(clientVersion) // protocol version
	payload.AddU32(clientVersion) // client version
	payload.AddString("1525")     // client version string
	payload.AddString("")         // asset hash
	payload.AddByte(0)            // preview state
	payload.AddBytes(buildRSABlock(keyBytes, func(b *netmsg.Writer) {
		b.AddByte(0) // gamemaster
		b.AddString(sessionKey)
		b.AddString(charName)
		b.AddU32(challengeTS)    // echo, read from inside the block
		b.AddByte(challengeRand) // echo
	}, rsa))
	// The game codec is on the modern profile, so the outer length is a block count.
	// Wrap sends (total-4)/8 and DecodeBodySize inverts it as header*8+4, so the
	// division is only exact when the payload after the checksum is a multiple of 8.
	// Otherwise the truncated block count makes the server read a short body and
	// bail with "short packet, no RSA block".
	for payload.Len()%8 != 0 {
		payload.AddByte(0x00)
	}

	w := netmsg.NewWriter()
	w.AddU32(tibcrypto.Adler32(payload.Bytes()))
	w.AddBytes(payload.Bytes())
	if err := send(conn, codec, w); err != nil {
		return err
	}
	codec.EnableModernGame(key)

	// 3. Read the enter-world sequence.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	r, err = recv(conn, codec)
	if err != nil {
		return fmt.Errorf("read enter-world: %w", err)
	}
	if err := inspectEnterWorld(r); err != nil {
		return err
	}
	log.Printf("   entered world")

	// 4. Play: walk, say hello, ping, then logout.
	//
	// Try all four directions rather than just north. Whether a given step is legal
	// depends on where the character happens to stand, and the sample characters
	// spawn against a wall — a blocked step answers 0xB5 (walk cancel), which is a
	// correct server response but proves nothing about movement.
	for _, d := range []struct {
		op   byte
		name string
	}{
		{0x65, "north"}, {0x66, "east"}, {0x67, "south"}, {0x68, "west"},
	} {
		walk := netmsg.NewWriter()
		walk.AddByte(d.op)
		if err := send(conn, codec, walk); err != nil {
			return err
		}
		log.Printf("   → sent walk %s", d.name)
	}

	say := netmsg.NewWriter()
	say.AddByte(0x96)
	say.AddByte(0x01) // SAY
	say.AddString("Hello from canary-client!")
	if err := send(conn, codec, say); err != nil {
		return err
	}
	log.Printf("   → sent say")

	// 0x1D is the client keep-alive the server answers with opPingBack 0x1E
	// (game.go: inPing / inPong). Sending 0x1E instead announced OUR pong, which the
	// server only uses to refresh liveness — it never replies, so this always timed
	// out waiting for one.
	ping := netmsg.NewWriter()
	ping.AddByte(0x1D)
	if err := send(conn, codec, ping); err != nil {
		return err
	}
	log.Printf("   → sent ping")

	// Drain responses for a moment.
	gotMove, gotSay, gotPong := false, false, false
	end := time.Now().Add(2 * time.Second)
	for time.Now().Before(end) {
		_ = conn.SetReadDeadline(end)
		r, err := recv(conn, codec)
		if err != nil {
			break
		}
		op := r.GetByte()
		switch op {
		case 0x6D, 0x65, 0x66, 0x67, 0x68, 0x64:
			gotMove = true
			log.Printf("   ← map/move update 0x%02X", op)
		case 0xAA:
			gotSay = true
			_ = r.GetU32()    // statement id
			name := r.GetString()
			_ = r.GetByte()   // show (traded) — this byte was missing here, so every
			_ = r.GetU16()    // field below read one byte early and the text came out empty
			_ = r.GetByte()   // talk type
			_ = r.GetPosition()
			text := r.GetString()
			log.Printf("   ← %s says: %q", name, text)
		case 0x1E: // opPingBack
			gotPong = true
			log.Printf("   ← pong")
		case 0xB5:
			log.Printf("   ← walk cancelled (that direction is blocked)")
		default:
			log.Printf("   ← opcode 0x%02X", op)
		}
	}

	logout := netmsg.NewWriter()
	logout.AddByte(0x14)
	_ = send(conn, codec, logout)
	log.Printf("   → sent logout")

	if !gotMove || !gotSay || !gotPong {
		return fmt.Errorf("missing responses (move=%v say=%v pong=%v)", gotMove, gotSay, gotPong)
	}
	return nil
}

func inspectEnterWorld(r *netmsg.Reader) error {
	op := r.GetByte()
	if op != 0x17 {
		return fmt.Errorf("expected self-appear 0x17, got 0x%02X", op)
	}
	playerID := r.GetU32()
	log.Printf("   self appear: playerID=%d", playerID)
	return nil
}
