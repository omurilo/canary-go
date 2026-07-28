// Package transport implements the framing/codec layer that wraps and unwraps
// Tibia packets: outer length header, checksum/sequence, XTEA encryption and the
// modern padding-byte payload. It mirrors the C++ TransportCodec.
//
// Framing rules used here (self-consistent and matching the reference server):
//   - Before encryption is enabled the outer length is the RAW body byte count
//     and the payload is plaintext (login challenge, login request).
//   - Once encryption is enabled the CurrentModern profile is used: outer length
//     is the XTEA block count, payload carries a leading padding-count byte, and
//     the 4-byte checksum is either Adler-32 (login server) or a monotonic
//     sequence number (modern game server).
package transport

import (
	"errors"
	"fmt"

	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/tibcrypto"
)

// ChecksumMethod selects how the 4-byte checksum field is produced/validated.
type ChecksumMethod uint8

const (
	ChecksumNone ChecksumMethod = iota
	ChecksumAdler32
	ChecksumSequence
)

// CryptoMethod selects the XTEA + checksum combination for encrypted frames.
type CryptoMethod uint8

const (
	CryptoNone   CryptoMethod = iota
	CryptoAdler32              // Adler-32 checksum with legacy inner-length payload
	CryptoSequence             // monotonic sequence checksum with modern padding payload
)

// ProfileID selects a complete transport framing profile matching the C++
// TransportProfileId. Each profile bundles outer-length encoding, payload
// layout, checksum method, and compression.
type ProfileID uint8

const (
	ProfileCurrentModern ProfileID = iota // CurrentModern (13.x)
	ProfileLegacyLogin                    // LegacyRawWithLoginHeader (login + 11.00 game)
	ProfileLegacyClassic                  // LegacyClassic (8.60 game — no +1 header)
)

// PayloadLayout selects how the encrypted region is framed internally.
type PayloadLayout uint8

const (
	PayloadModernPad   PayloadLayout = iota // leading padding-count byte (13.x)
	PayloadLegacyInner                      // leading u16 inner length (8.6)
)

// Codec holds the per-connection framing state. Not safe for concurrent use by
// multiple readers/writers; callers serialize writes.
type Codec struct {
	Encryption bool
	Key        tibcrypto.XTEAKey
	Checksum   ChecksumMethod
	Payload    PayloadLayout

	// ModernOuterLength selects the CurrentModern transport's outer-length
	// encoding (OuterLengthEncoding::ModernBlockCount): the 2-byte length header
	// is the XTEA block count and the body carries a leading 4-byte checksum slot,
	// so wire size = header*8 + 4. Unlike the login path this holds even before
	// XTEA is enabled — the pre-encryption challenge and the inbound login packet
	// use block-count framing too. When false the pre-encryption length is the raw
	// body byte count (legacy login handshake).
	ModernOuterLength bool

	serverSeq uint32 // outbound sequence counter
	clientSeq uint32 // inbound sequence counter
}

// New returns a codec in the initial (pre-encryption) state.
func New() *Codec { return &Codec{Payload: PayloadModernPad} }

// ApplyProfile configures the codec for the given transport profile. This sets
// the initial pre-encryption framing; callers must also call EnableEncryption
// (or the version-specific method) once the XTEA key is exchanged.
func (c *Codec) ApplyProfile(profile ProfileID) {
	switch profile {
	case ProfileCurrentModern:
		c.ModernOuterLength = true
		c.Payload = PayloadModernPad
	case ProfileLegacyLogin:
		// 11.00: raw body length pre-encryption, then LegacyInner + Adler32
		c.ModernOuterLength = false
		c.Payload = PayloadLegacyInner
	case ProfileLegacyClassic:
		// 8.60: raw body length pre-encryption, then LegacyInner + Adler32,
		// same as LegacyLogin but with no extra header byte.
		c.ModernOuterLength = false
		c.Payload = PayloadLegacyInner
	}
}

// EnableLegacyGame flips the codec into legacy encrypted game framing with an
// Adler-32 checksum and legacy inner-length payload (used by 11.00 and 8.60).
func (c *Codec) EnableLegacyGame(key tibcrypto.XTEAKey) {
	c.Key = key
	c.Encryption = true
	c.Checksum = ChecksumAdler32
	c.Payload = PayloadLegacyInner
	// After encryption, all profiles use block-count outer length.
	c.ModernOuterLength = true
}

// EnableModernFraming switches the codec to the CurrentModern block-count outer
// length for every packet (see ModernOuterLength). The game connection sets this
// at accept time so the challenge and login handshake frame like the reference
// server before the XTEA key is exchanged.
func (c *Codec) EnableModernFraming() { c.ModernOuterLength = true }

// EnableModernGame flips the codec into the modern encrypted game framing with an
// Adler-32 checksum once the XTEA key has been exchanged.
func (c *Codec) EnableModernGame(key tibcrypto.XTEAKey) {
	c.Key = key
	c.Encryption = true
	c.Checksum = ChecksumAdler32
	c.Payload = PayloadModernPad
}

// EnableModernLogin flips the codec into modern encrypted framing with an Adler
// checksum (used by the login server response path).
func (c *Codec) EnableModernLogin(key tibcrypto.XTEAKey) {
	c.Key = key
	c.Encryption = true
	c.Checksum = ChecksumAdler32
	c.Payload = PayloadModernPad
}

// DecodeBodySize converts the 2-byte outer length header into the number of body
// bytes to read next.
func (c *Codec) DecodeBodySize(header uint16) int {
	if c.Encryption || c.ModernOuterLength {
		return int(header)*8 + 4 // block count * 8 + checksum
	}
	return int(header)
}

// Wrap finalizes an outbound message and returns the wire bytes (headers+body).
func (c *Codec) Wrap(w *netmsg.Writer) []byte {
	if !c.Encryption {
		if c.ModernOuterLength {
			// Block-count length; the body already carries its 4-byte checksum
			// slot (e.g. the challenge's Adler-32), so subtract it before dividing.
			w.PrependU16(uint16((w.Len() - 4) / 8))
		} else {
			w.PrependU16(uint16(w.Len()))
		}
		return w.Bytes()
	}

	switch c.Payload {
	case PayloadModernPad:
		pad := 8 - (w.Len() % 8) - 1
		if pad < 0 {
			pad += 8
		}
		for i := 0; i < pad; i++ {
			w.AddByte(0x33)
		}
		w.PrependByte(byte(pad))
	case PayloadLegacyInner:
		inner := uint16(w.Len())
		w.PrependU16(inner)
		w.PadTo(8)
	}

	c.Key.Encrypt(w.Bytes())
	enc := w.Bytes() // encrypted region, multiple of 8

	switch c.Checksum {
	case ChecksumAdler32:
		w.PrependU32(tibcrypto.Adler32(enc))
	case ChecksumSequence:
		c.serverSeq++
		if c.serverSeq >= 0x7FFFFFFF {
			c.serverSeq = 0
		}
		w.PrependU32(c.serverSeq)
	}

	// Outer length: modern block count = encrypted length / 8.
	w.PrependU16(uint16(len(enc) / 8))
	return w.Bytes()
}

// Unwrap validates and decrypts a received body (bytes after the outer length
// header) and returns a reader positioned at the plaintext message.
func (c *Codec) Unwrap(body []byte) (*netmsg.Reader, error) {
	off := 0
	if c.Checksum != ChecksumNone {
		if len(body) < 4 {
			return nil, errors.New("transport: body too short for checksum")
		}
		recv := uint32(body[0]) | uint32(body[1])<<8 | uint32(body[2])<<16 | uint32(body[3])<<24
		off = 4
		switch c.Checksum {
		case ChecksumAdler32:
			if got := tibcrypto.Adler32(body[4:]); got != recv {
				return nil, fmt.Errorf("transport: adler mismatch got=%08x want=%08x", got, recv)
			}
		case ChecksumSequence:
			c.clientSeq++
			if c.clientSeq >= 0x7FFFFFFF {
				c.clientSeq = 0
			}
			// The high bit signals compression, which we do not negotiate here.
			if recv&0x7FFFFFFF != c.clientSeq {
				// Be lenient: accept and resync rather than drop the packet.
				c.clientSeq = recv & 0x7FFFFFFF
			}
		}
	}

	enc := body[off:]
	if !c.Encryption {
		return netmsg.NewReader(enc), nil
	}
	if len(enc)%8 != 0 {
		return nil, fmt.Errorf("transport: encrypted length %d not multiple of 8", len(enc))
	}
	dec := make([]byte, len(enc))
	copy(dec, enc)
	c.Key.Decrypt(dec)

	switch c.Payload {
	case PayloadModernPad:
		if len(dec) == 0 {
			return nil, errors.New("transport: empty modern payload")
		}
		pad := int(dec[0])
		end := len(dec) - pad
		if end < 1 {
			return nil, errors.New("transport: invalid modern padding")
		}
		return netmsg.NewReader(dec[1:end]), nil
	case PayloadLegacyInner:
		if len(dec) < 2 {
			return nil, errors.New("transport: short legacy payload")
		}
		inner := int(dec[0]) | int(dec[1])<<8
		if 2+inner > len(dec) {
			return nil, errors.New("transport: legacy inner length overflow")
		}
		return netmsg.NewReader(dec[2 : 2+inner]), nil
	}
	return netmsg.NewReader(dec), nil
}

// StripFirstPacketChecksum inspects a first (pre-encryption) packet body and, if
// its leading 4 bytes are a valid Adler-32 of the remainder, strips them. This
// mirrors the reference server's checksum auto-detection on the first packet.
func StripFirstPacketChecksum(body []byte) []byte {
	if len(body) >= 4 {
		recv := uint32(body[0]) | uint32(body[1])<<8 | uint32(body[2])<<16 | uint32(body[3])<<24
		if tibcrypto.Adler32(body[4:]) == recv {
			return body[4:]
		}
	}
	return body
}
