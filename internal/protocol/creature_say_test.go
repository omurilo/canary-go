package protocol

import (
	"testing"

	"github.com/omurilo/canary-go/internal/netmsg"
)

// TestCreatureSayFormat documents the 0xAA creature-say wire layout the client
// requires: opcode, statementId u32, name, show byte, level u16, talkType byte,
// POSITION (5 bytes), then the text. Creature/NPC speech (incl. NPC PRIVATE_NP
// replies) always carries the position — omitting it makes the client read the
// text's bytes as a huge string length and crash ("not enough bytes at N").
func TestCreatureSayFormat(t *testing.T) {
	w := netmsg.NewWriter()
	w.AddByte(0xAA)
	w.AddU32(1)
	w.AddString("Gorn")
	w.AddByte(0)
	w.AddU16(0)
	w.AddByte(10) // TALKTYPE_PRIVATE_NP
	w.AddPosition(netmsg.Position{X: 100, Y: 200, Z: 7})
	w.AddString("hi")

	b := w.Bytes()
	// 1 + 4 + (2+4) + 1 + 2 + 1 + 5 + (2+2) = 29
	const want = 1 + 4 + 6 + 1 + 2 + 1 + 5 + 4
	if len(b) != want {
		t.Fatalf("packet len = %d, want %d", len(b), want)
	}
	if b[0] != 0xAA {
		t.Fatalf("opcode = 0x%02X, want 0xAA", b[0])
	}
	// talkType byte sits at offset 1+4+6+1+2 = 14, position follows immediately.
	if b[14] != 10 {
		t.Fatalf("talkType at offset 14 = %d, want 10", b[14])
	}
}
