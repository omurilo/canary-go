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

// TestBosstiaryInfoPacket decodes the 0x73 boss-list packet exactly as otclient
// parseBosstiaryInfo does (u16 count, then per boss: u32 raceId, u8 race, u32
// kills, u8 reserved, u8 tracker for >=1320).
func TestBosstiaryInfoPacket(t *testing.T) {
	entries := []bossListEntry{
		{RaceID: 900, Race: 1, Kills: 5, Tracked: false},
		{RaceID: 46, Race: 0, Kills: 123, Tracked: true},
	}
	b := buildBosstiaryInfo(entries).Bytes()
	if b[0] != 0x73 {
		t.Fatalf("opcode = 0x%02X, want 0x73", b[0])
	}
	r := netmsg.NewReader(b[1:])
	if got := r.GetU16(); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	for i, e := range entries {
		if got := uint16(r.GetU32()); got != e.RaceID {
			t.Errorf("entry[%d] raceId = %d, want %d", i, got, e.RaceID)
		}
		if got := r.GetByte(); got != e.Race {
			t.Errorf("entry[%d] race = %d, want %d", i, got, e.Race)
		}
		if got := r.GetU32(); got != e.Kills {
			t.Errorf("entry[%d] kills = %d, want %d", i, got, e.Kills)
		}
		_ = r.GetByte() // reserved
		track := r.GetByte()
		wantTrack := byte(0)
		if e.Tracked {
			wantTrack = 1
		}
		if track != wantTrack {
			t.Errorf("entry[%d] tracker = %d, want %d", i, track, wantTrack)
		}
	}
	if r.Remaining() != 0 {
		t.Fatalf("leftover %d bytes", r.Remaining())
	}
}

// TestBosstiarySlotsPacket decodes 0x62 exactly as otclient parseBosstiarySlots.
func TestBosstiarySlotsPacket(t *testing.T) {
	v := bosstiarySlotsView{
		playerPoints:      300,
		pointsNextBonus:   750,
		currentBonus:      55,
		slotOneUnlocked:   true,
		slotOne:           bosstiarySlotView{filled: true, bossID: 900, race: 1, kills: 60, lootBonus: 80, killBonus: 0, removePrice: 0, inactive: false},
		slotTwoUnlocked:   false,
		slotTwoLockPoints: 1500,
		todayUnlocked:     false,
		boostedBossID:     0,
		unlocked:          []bossListEntry{{RaceID: 46, Race: 0}},
	}
	b := buildBosstiarySlots(v).Bytes()
	if b[0] != 0x62 {
		t.Fatalf("opcode 0x%02X want 0x62", b[0])
	}
	r := netmsg.NewReader(b[1:])
	if r.GetU32() != 300 || r.GetU32() != 750 || r.GetU16() != 55 || r.GetU16() != 56 {
		t.Fatal("header points/bonus mismatch")
	}
	if r.GetByte() != 1 { // slotOneUnlocked
		t.Fatal("slotOneUnlocked != 1")
	}
	if r.GetU32() != 900 { // bossIdSlotOne
		t.Fatal("bossIdSlotOne != 900")
	}
	// slot one bytes: race, kills, lootBonus, killBonus, raceRepeat, removePrice, inactive
	if r.GetByte() != 1 || r.GetU32() != 60 || r.GetU16() != 80 || r.GetByte() != 0 || r.GetByte() != 1 || r.GetU32() != 0 || r.GetByte() != 0 {
		t.Fatal("slot one bytes mismatch")
	}
	if r.GetByte() != 0 { // slotTwoUnlocked
		t.Fatal("slotTwoUnlocked != 0")
	}
	if r.GetU32() != 1500 { // lock points shown
		t.Fatal("slotTwo lock points != 1500")
	}
	if r.GetByte() != 0 || r.GetU32() != 0 { // today slot: locked, id 0
		t.Fatal("today slot mismatch")
	}
	if r.GetByte() != 1 { // has unlocked list
		t.Fatal("hasUnlocked != 1")
	}
	if r.GetU16() != 1 {
		t.Fatal("unlocked count != 1")
	}
	if r.GetU32() != 46 || r.GetByte() != 0 {
		t.Fatal("unlocked entry mismatch")
	}
	if r.Remaining() != 0 {
		t.Fatalf("leftover %d bytes", r.Remaining())
	}
}
