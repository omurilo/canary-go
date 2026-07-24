package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/charms"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// charmTestSetup builds a protocol with two charms (one Major, one Minor) and a
// monster the player has fully unlocked in the bestiary.
func charmTestSetup(t *testing.T) (*GameProtocol, *game.Player) {
	t.Helper()
	world := game.NewWorld()
	world.Charms.Add(&charms.Charm{ID: charms.Wound, Name: "Wound", Category: charms.CategoryMajor, Percent: 5, Points: [3]uint16{240, 360, 1200}})
	world.Charms.Add(&charms.Charm{ID: charms.Adrenaline, Name: "Adrenaline", Category: charms.CategoryMinor, Points: [3]uint16{100, 200, 400}})
	world.TypeRegistry.Monsters["rat"] = &creatures.MonsterType{
		Name: "Rat", RaceID: 21, BestiaryToKill: 25, BestiarySecondUnlock: 5,
	}
	p := &game.Player{Level: 50}
	return &GameProtocol{player: p, deps: &Deps{World: world}}, p
}

func TestBestiaryCharmsPacket(t *testing.T) {
	g, p := charmTestSetup(t)
	// Unlock Wound at tier 1 and assign it to the rat (race 21).
	p.SetCharmTier(charms.Wound, 1)
	p.UnlockedRunesBit = uint32(charms.SetBit(0, charms.Wound))
	p.SetCharmRace(charms.Wound, 21)
	p.UsedRunesBit = uint32(charms.SetBit(0, charms.Wound))
	p.AddBestiaryKillCount(21, 10) // >= secondUnlock(5), so it's a finished monster

	b := g.buildBestiaryCharms().Bytes()
	if b[0] != 0xD8 {
		t.Fatalf("opcode = 0x%02X, want 0xD8", b[0])
	}
	r := netmsg.NewReader(b[1:])
	if got := r.GetU64(); got != 100000 { // level 50 <= 100, no expansion
		t.Fatalf("resetAllCost = %d, want 100000", got)
	}
	if got := r.GetByte(); got != 2 {
		t.Fatalf("charm count = %d, want 2", got)
	}
	// charm 0: Wound, unlocked (tier 1), assigned to race 21 with remove cost
	if id := r.GetByte(); id != charms.Wound {
		t.Fatalf("charm[0] id = %d, want %d", id, charms.Wound)
	}
	if tier := r.GetByte(); tier != 1 {
		t.Fatalf("charm[0] tier = %d, want 1", tier)
	}
	if flag := r.GetByte(); flag != 0x01 {
		t.Fatalf("charm[0] assigned flag = %d, want 1", flag)
	}
	if race := r.GetU16(); race != 21 {
		t.Fatalf("charm[0] race = %d, want 21", race)
	}
	if cost := r.GetU32(); cost != 5000 { // level 50 * 100
		t.Fatalf("charm[0] remove cost = %d, want 5000", cost)
	}
	// charm 1: Adrenaline, locked -> two zero bytes
	if id := r.GetByte(); id != charms.Adrenaline {
		t.Fatalf("charm[1] id = %d, want %d", id, charms.Adrenaline)
	}
	if a, b2 := r.GetByte(), r.GetByte(); a != 0 || b2 != 0 {
		t.Fatalf("charm[1] locked bytes = %d,%d, want 0,0", a, b2)
	}
	// available slots: non-premium, 1 used -> 2-1 = 1
	if slots := r.GetByte(); slots != 1 {
		t.Fatalf("available slots = %d, want 1", slots)
	}
	// finished monsters: race 21 assigned only once (<2), so still listed
	if n := r.GetU16(); n != 1 {
		t.Fatalf("finished count = %d, want 1", n)
	}
	if race := r.GetU32(); race != 21 {
		t.Fatalf("finished[0] = %d, want 21", race)
	}
	if r.Remaining() != 0 {
		t.Fatalf("leftover %d bytes (client would desync)", r.Remaining())
	}
}

func TestBuyCharmRuneUnlockAndAssign(t *testing.T) {
	g, p := charmTestSetup(t)
	p.AddCharmPoints(1000)   // spendable major currency
	p.AddBestiaryKillCount(21, 30) // >= toKill(25), so assignment is allowed

	// action 0: unlock Wound (Major) at tier 0 -> spends 240 points, +50 echoes.
	unlock := netmsg.NewReader([]byte{0x00, charms.Wound, 0x00, 0x00})
	g.parseSendBuyCharmRune(unlock)
	if p.GetCharmTier(charms.Wound) != 1 {
		t.Fatalf("tier after unlock = %d, want 1", p.GetCharmTier(charms.Wound))
	}
	if p.GetCharmPoints() != 760 {
		t.Fatalf("charm points after unlock = %d, want 760 (1000-240)", p.GetCharmPoints())
	}
	if p.GetMinorCharmEchoes() != 50 {
		t.Fatalf("minor echoes = %d, want 50", p.GetMinorCharmEchoes())
	}
	if !charms.HasBit(int32(p.UnlockedRunesBit), charms.Wound) {
		t.Fatal("unlocked bit not set")
	}

	// action 1: assign Wound to race 21 (rat).
	assign := netmsg.NewReader([]byte{0x01, charms.Wound, 21, 0})
	g.parseSendBuyCharmRune(assign)
	if p.GetCharmRace(charms.Wound) != 21 {
		t.Fatalf("assigned race = %d, want 21", p.GetCharmRace(charms.Wound))
	}
	if !charms.HasBit(int32(p.UsedRunesBit), charms.Wound) {
		t.Fatal("used bit not set after assign")
	}
}

func TestBuyCharmRuneAssignBlockedWithoutKills(t *testing.T) {
	g, p := charmTestSetup(t)
	p.SetCharmTier(charms.Wound, 1)
	p.UnlockedRunesBit = uint32(charms.SetBit(0, charms.Wound))
	// only 1 kill, below toKill(25): assignment must be refused.
	p.AddBestiaryKillCount(21, 1)
	assign := netmsg.NewReader([]byte{0x01, charms.Wound, 21, 0})
	g.parseSendBuyCharmRune(assign)
	if p.GetCharmRace(charms.Wound) != 0 {
		t.Fatalf("race assigned despite too few kills: %d", p.GetCharmRace(charms.Wound))
	}
}

func TestBuyCharmRuneResetAll(t *testing.T) {
	g, p := charmTestSetup(t)
	p.AddCharmPoints(500) // max becomes 500, spend some
	p.SpendCharmPoints(300)
	p.SetCharmTier(charms.Wound, 2)
	p.SetCharmRace(charms.Wound, 21)
	p.UsedRunesBit = uint32(charms.SetBit(0, charms.Wound))
	p.UnlockedRunesBit = uint32(charms.SetBit(0, charms.Wound))
	p.AddMoney(200000) // enough gold for the 100000 reset fee

	reset := netmsg.NewReader([]byte{0x03, 0x00, 0x00, 0x00})
	g.parseSendBuyCharmRune(reset)

	if p.GetCharmTier(charms.Wound) != 0 || p.GetCharmRace(charms.Wound) != 0 {
		t.Fatal("charm not reset")
	}
	if p.UsedRunesBit != 0 || p.UnlockedRunesBit != 0 {
		t.Fatal("rune bits not cleared")
	}
	if p.GetCharmPoints() != 500 {
		t.Fatalf("points not refunded to max: %d, want 500", p.GetCharmPoints())
	}
}
