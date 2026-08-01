package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/tibcrypto"
	"github.com/opentibiabr/canary-go/internal/transport"
)

// buildClientLoginPacket assembles a first login packet the way OTC does in
// Protocol::send (otclient/src/framework/net/protocol.cpp:122-156), for a given
// client version. The payload is opaque here — only the framing is under test.
func buildClientLoginPacket(payload []byte, modern bool) []byte {
	body := payload

	if modern {
		// writePaddingAmount (outputmessage.cpp:151-156): pad so that the count
		// byte plus the body is a multiple of 8, then prepend the count.
		pad := 8 - (len(body) % 8) - 1
		if pad < 0 {
			pad += 8
		}
		padded := make([]byte, 0, 1+len(body)+pad)
		padded = append(padded, byte(pad))
		padded = append(padded, body...)
		padded = append(padded, make([]byte, pad)...)
		body = padded
	}

	// writeChecksum: Adler-32 over everything after it.
	sum := tibcrypto.Adler32(body)
	withSum := make([]byte, 0, 4+len(body))
	withSum = append(withSum, byte(sum), byte(sum>>8), byte(sum>>16), byte(sum>>24))
	withSum = append(withSum, body...)

	// The outer u16: a block count on modern clients, a byte length on old ones.
	var hdr uint16
	if modern {
		hdr = uint16((len(withSum) - 4) / 8)
	} else {
		hdr = uint16(len(withSum))
	}
	return append([]byte{byte(hdr), byte(hdr >> 8)}, withSum...)
}

// unwrap runs the server's first-packet path over a wire frame.
func unwrap(t *testing.T, wire []byte) []byte {
	t.Helper()
	body, modernPad := stripLoginFraming(wire, nil)
	if modernPad {
		var ok bool
		if body, ok = stripModernPadding(body); !ok {
			t.Fatal("modern padding rejected")
		}
	} else {
		body = transport.StripFirstPacketChecksum(body)
	}
	return body
}

// A 13.x client writes (messageSize-4)/8 in the outer header, not a byte count,
// and pads the body. Only the byte-count form was recognised, so the two header
// bytes stayed in front of the payload, every field read two bytes early, and
// the RSA block decrypted to noise — "rsa leading byte non-zero", logged at
// debug with the connection dropped in silence.
func TestModernLoginFramingIsUnwrapped(t *testing.T) {
	// A payload the length of a real login packet: 22 bytes of metadata plus a
	// 128-byte RSA block.
	payload := make([]byte, 22+tibcrypto.BlockSize)
	for i := range payload {
		payload[i] = byte(i)
	}

	got := unwrap(t, buildClientLoginPacket(payload, true))
	if len(got) != len(payload) {
		t.Fatalf("unwrapped %d bytes, want %d", len(got), len(payload))
	}
	for i := range payload {
		if got[i] != payload[i] {
			t.Fatalf("byte %d = %d, want %d — the payload is misaligned", i, got[i], payload[i])
		}
	}
}

// The legacy byte-length header still works; the fix must not trade one client
// for another.
func TestLegacyLoginFramingStillWorks(t *testing.T) {
	payload := []byte("legacy login payload")
	got := unwrap(t, buildClientLoginPacket(payload, false))
	if string(got) != string(payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

// The two header forms can never both match: declared == declared*8+4 has no
// solution, so there is no length at which one is mistaken for the other.
func TestHeaderFormsAreUnambiguous(t *testing.T) {
	for size := 8; size <= 2048; size += 8 {
		payload := make([]byte, size)

		modern := buildClientLoginPacket(payload, true)
		if _, isModern := stripLoginFraming(modern, nil); !isModern {
			t.Errorf("size %d: modern frame read as legacy", size)
		}
		legacy := buildClientLoginPacket(payload, false)
		if _, isModern := stripLoginFraming(legacy, nil); isModern {
			t.Errorf("size %d: legacy frame read as modern", size)
		}
	}
}

// A frame whose header matches neither form is left untouched rather than
// guessed at, so a probe or a malformed packet cannot be silently mangled.
func TestUnrecognisedHeaderIsLeftAlone(t *testing.T) {
	junk := []byte{0xAA, 0xBB, 1, 2, 3, 4, 5}
	got, modern := stripLoginFraming(junk, nil)
	if modern {
		t.Error("junk must not be treated as modern")
	}
	if len(got) != len(junk) {
		t.Errorf("junk was modified: %d bytes, want %d", len(got), len(junk))
	}
}

// Padding that would consume the whole payload is rejected instead of producing
// an empty read.
func TestModernPaddingBoundsAreChecked(t *testing.T) {
	if _, ok := stripModernPadding([]byte{9, 1, 2}); ok {
		t.Error("a padding count larger than the body must be rejected")
	}
	if _, ok := stripModernPadding(nil); ok {
		t.Error("an empty body must be rejected")
	}
	got, ok := stripModernPadding([]byte{2, 'h', 'i', 0, 0})
	if !ok || string(got) != "hi" {
		t.Errorf("got %q ok=%v, want \"hi\" true", got, ok)
	}
}

// An OTC client at version 1200+ writes `getWorldName() + "\n"` raw before its
// first real packet (otclient/src/framework/net/protocol.cpp:408-418), then
// enables sequenced packets. Reached by IP and port rather than through a
// server list it has no world name, so it sends a bare "\n" — the stray 0x0A
// that made the login frame unparseable.
func TestBareWorldNameLineIsConsumed(t *testing.T) {
	payload := make([]byte, 377)
	frame := buildClientLoginPacket(payload, true)

	got := unwrap(t, append([]byte{'\n'}, frame...))
	if len(got) != len(payload) {
		t.Fatalf("unwrapped %d bytes, want %d", len(got), len(payload))
	}
}

// A named world sends the name too, and the whole line has to go.
func TestNamedWorldLineIsConsumed(t *testing.T) {
	payload := []byte("some login payload")
	frame := buildClientLoginPacket(payload, false)

	got := unwrap(t, append([]byte("Canary-Go\n"), frame...))
	if string(got) != string(payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

// The line is only removed when doing so turns a frame that parses as nothing
// into one that parses exactly — so a frame that already works is never
// touched, whatever its first byte happens to be.
func TestValidFrameIsNeverStripped(t *testing.T) {
	payload := make([]byte, 64)
	for _, modern := range []bool{true, false} {
		frame := buildClientLoginPacket(payload, modern)
		got := unwrap(t, frame)
		if len(got) != len(payload) {
			t.Errorf("modern=%v: unwrapped %d bytes, want %d", modern, len(got), len(payload))
		}
	}
}

// Protocol::send writes EITHER a checksum or a sequence number into the four
// bytes after the header (protocol.cpp:144-149), and writeHeaderSize subtracts
// a flat 4 for that slot either way. An OTC client that called
// enabledSequencedPackets — which Protocol::onConnect does, right after sending
// the world-name line — sends writeSequence(0): four zeros, which Adler-32 can
// never be, since its sum starts at 1.
//
// Detecting the slot by verifying it as a checksum therefore left it in place,
// and the payload read four bytes early:
//
//	protoId=0 clientOS=0 version=262 clientVersion=99942410
func TestModernFrameWithSequenceInsteadOfChecksum(t *testing.T) {
	payload := []byte{0x01, 0x0A, 0x00, 0xF5, 0x05, 0xF5, 0x05, 0x00, 0x00}

	pad := 8 - (len(payload) % 8) - 1
	if pad < 0 {
		pad += 8
	}
	body := append([]byte{byte(pad)}, payload...)
	body = append(body, make([]byte, pad)...)

	// Four zeros where a checksum would go: writeSequence(0).
	wire := append([]byte{0, 0, 0, 0}, body...)
	blocks := uint16(len(body) / 8)
	wire = append([]byte{byte(blocks), byte(blocks >> 8)}, wire...)

	got := unwrap(t, wire)
	if len(got) != len(payload) {
		t.Fatalf("unwrapped %d bytes, want %d", len(got), len(payload))
	}
	if got[0] != 0x01 {
		t.Errorf("protocol id = %d, want 1 — the sequence slot was not stripped", got[0])
	}
	if os := uint16(got[1]) | uint16(got[2])<<8; os != 10 {
		t.Errorf("clientOS = %d, want 10", os)
	}
	if v := uint16(got[3]) | uint16(got[4])<<8; v != 1525 {
		t.Errorf("version = %d, want 1525", v)
	}
}
