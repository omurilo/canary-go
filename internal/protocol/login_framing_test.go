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
	body = transport.StripFirstPacketChecksum(body)
	if modernPad {
		var ok bool
		if body, ok = stripModernPadding(body); !ok {
			t.Fatal("modern padding rejected")
		}
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
