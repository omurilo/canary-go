package transport

import (
	"bytes"
	"testing"

	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/tibcrypto"
)

// wrapUnwrap simulates one side wrapping a message and the peer unwrapping it,
// including the 2-byte outer length header the TCP framing carries.
func wrapUnwrap(t *testing.T, sender, receiver *Codec, build func(*netmsg.Writer)) *netmsg.Reader {
	t.Helper()
	w := netmsg.NewWriter()
	build(w)
	wire := sender.Wrap(w)

	// Split header + body as the read loop does.
	if len(wire) < 2 {
		t.Fatal("wire too short")
	}
	header := uint16(wire[0]) | uint16(wire[1])<<8
	body := wire[2:]
	if got := receiver.DecodeBodySize(header); got != len(body) {
		t.Fatalf("decoded body size %d != actual %d", got, len(body))
	}
	r, err := receiver.Unwrap(body)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	return r
}

func TestModernGameRoundTrip(t *testing.T) {
	key := tibcrypto.XTEAKey{1, 2, 3, 4}
	sender := New()
	sender.EnableModernGame(key)
	receiver := New()
	receiver.EnableModernGame(key)

	r := wrapUnwrap(t, sender, receiver, func(w *netmsg.Writer) {
		w.AddByte(0xA0)
		w.AddU32(185)
		w.AddString("hello world")
		w.AddU16(1525)
	})
	if op := r.GetByte(); op != 0xA0 {
		t.Fatalf("opcode = 0x%02X", op)
	}
	if v := r.GetU32(); v != 185 {
		t.Fatalf("u32 = %d", v)
	}
	if s := r.GetString(); s != "hello world" {
		t.Fatalf("string = %q", s)
	}
	if v := r.GetU16(); v != 1525 {
		t.Fatalf("u16 = %d", v)
	}
}

func TestModernLoginRoundTrip(t *testing.T) {
	key := tibcrypto.XTEAKey{0xAAAA, 0xBBBB, 0xCCCC, 0xDDDD}
	sender := New()
	sender.EnableModernLogin(key)
	receiver := New()
	receiver.EnableModernLogin(key)

	r := wrapUnwrap(t, sender, receiver, func(w *netmsg.Writer) {
		w.AddByte(0x64)
		w.AddString("Canary-Go")
	})
	if op := r.GetByte(); op != 0x64 {
		t.Fatalf("opcode = 0x%02X", op)
	}
	if s := r.GetString(); s != "Canary-Go" {
		t.Fatalf("string = %q", s)
	}
}

func TestPlaintextRoundTrip(t *testing.T) {
	sender := New() // encryption off (login challenge path)
	receiver := New()
	w := netmsg.NewWriter()
	w.AddByte(0x1F)
	w.AddU32(123456)
	w.AddByte(70)
	wire := sender.Wrap(w)
	body := wire[2:]
	r, err := receiver.Unwrap(body)
	if err != nil {
		t.Fatal(err)
	}
	if op := r.GetByte(); op != 0x1F {
		t.Fatalf("opcode 0x%02X", op)
	}
	if ts := r.GetU32(); ts != 123456 {
		t.Fatalf("ts %d", ts)
	}
}

func TestSequenceIncrements(t *testing.T) {
	key := tibcrypto.XTEAKey{9, 9, 9, 9}
	sender := New()
	sender.EnableModernGame(key)
	receiver := New()
	receiver.EnableModernGame(key)
	for i := 0; i < 5; i++ {
		r := wrapUnwrap(t, sender, receiver, func(w *netmsg.Writer) {
			w.AddByte(0x1E)
		})
		if op := r.GetByte(); op != 0x1E {
			t.Fatalf("iter %d opcode 0x%02X", i, op)
		}
	}
	if sender.serverSeq != 5 {
		t.Fatalf("server seq = %d, want 5", sender.serverSeq)
	}
}

func TestChecksumDetectionStrip(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	sum := tibcrypto.Adler32(payload)
	body := append([]byte{byte(sum), byte(sum >> 8), byte(sum >> 16), byte(sum >> 24)}, payload...)
	if got := StripFirstPacketChecksum(body); !bytes.Equal(got, payload) {
		t.Fatalf("checksum not stripped: %v", got)
	}
	// Without a valid checksum the body is returned unchanged.
	if got := StripFirstPacketChecksum(payload); !bytes.Equal(got, payload) {
		t.Fatalf("payload changed: %v", got)
	}
}
