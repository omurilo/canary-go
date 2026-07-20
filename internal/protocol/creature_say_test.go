package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// TestAppendSayLocus locks in the per-talk-type 0xAA wire tail: local speech
// carries a 5-byte position, channel speech a 2-byte channel id, and private/
// NPC speech (e.g. TALKTYPE_PRIVATE_PN=12, PRIVATE_NP=10) nothing at all.
// Sending a spurious position for a private type desyncs the CipSoft client's
// parser and crashes it (the "not enough bytes at position 27" bug).
func TestAppendSayLocus(t *testing.T) {
	pos := game.Position{X: 100, Y: 200, Z: 7}
	cases := []struct {
		talkType byte
		wantLen  int // bytes appended after the talk-type byte
	}{
		{1, 5},    // SAY → position
		{2, 5},    // WHISPER
		{3, 5},    // YELL
		{36, 5},   // MONSTER_SAY
		{37, 5},   // MONSTER_YELL
		{7, 2},    // CHANNEL_Y → channel id
		{8, 2},    // CHANNEL_O
		{14, 2},   // CHANNEL_R1
		{0xFF, 2}, // CHANNEL_R2
		{10, 0},   // PRIVATE_NP → nothing (NPC → player)
		{12, 0},   // PRIVATE_PN → nothing (player → NPC)  ← the crash case
		{4, 0},    // PRIVATE_FROM
		{5, 0},    // PRIVATE_TO
		{13, 0},   // BROADCAST
	}
	for _, c := range cases {
		w := netmsg.NewWriter()
		appendSayLocus(w, c.talkType, pos, 0)
		if got := w.Len(); got != c.wantLen {
			t.Errorf("talkType %d: appended %d bytes, want %d", c.talkType, got, c.wantLen)
		}
	}
}
