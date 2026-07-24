package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// TestBosstiaryDataPacket decodes the 0x61 Boss Cyclopedia rules packet exactly
// as otclient parseBosstiaryData does (9 u16 kill thresholds then 9 u16 points,
// Bane/Archfoe/Nemesis x Prowess/Expertise/Mastery) and checks the values.
func TestBosstiaryDataPacket(t *testing.T) {
	b := buildBosstiaryData().Bytes()
	if b[0] != 0x61 {
		t.Fatalf("opcode = 0x%02X, want 0x61", b[0])
	}
	if len(b) != 1+18*2 {
		t.Fatalf("len = %d, want %d", len(b), 1+18*2)
	}
	r := netmsg.NewReader(b[1:])
	wantKills := []uint16{25, 100, 300, 5, 20, 60, 1, 3, 5}
	wantPoints := []uint16{5, 15, 30, 10, 30, 60, 10, 30, 60}
	for i, want := range wantKills {
		if got := r.GetU16(); got != want {
			t.Errorf("kill threshold[%d] = %d, want %d", i, got, want)
		}
	}
	for i, want := range wantPoints {
		if got := r.GetU16(); got != want {
			t.Errorf("points[%d] = %d, want %d", i, got, want)
		}
	}
	if r.Remaining() != 0 {
		t.Fatalf("leftover %d bytes (client would desync)", r.Remaining())
	}
}
