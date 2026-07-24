package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/charms"
	"github.com/opentibiabr/canary-go/internal/creatures"
)

func TestApplyCharmRune_OffensiveProc(t *testing.T) {
	w := newCombatWorld()
	// Enflame (Major, offensive, fire, 5%) with a guaranteed 100% chance at tier 1.
	w.Charms.Add(&charms.Charm{
		ID: charms.Enflame, Name: "Enflame", Category: charms.CategoryMajor,
		Type: charms.TypeOffensive, DamageType: 1, Percent: 5,
		Chance: [3]uint16{100, 100, 100},
	})

	mType := &creatures.MonsterType{Name: "Rat", RaceID: 21, MaxHealth: 1000}
	monster := NewMonster(1, "Rat", mType)
	monster.MaxHealth, monster.Health = 1000, 1000
	monster.SetPosition(Position{X: 101, Y: 100, Z: 7})
	w.AddCreature(monster)

	e := NewCombatEngine(w)

	p := &Player{Level: 100}
	p.SetPosition(Position{X: 100, Y: 100, Z: 7})
	// Unlock + assign Enflame to the rat.
	p.SetCharmTier(charms.Enflame, 1)
	p.SetCharmRace(charms.Enflame, 21)
	p.UsedRunesBit = uint32(charms.SetBit(0, charms.Enflame))

	before := monster.GetHealth()
	e.applyCharmRune(p, monster)
	if monster.GetHealth() >= before {
		t.Fatalf("charm did not damage monster: health %d -> %d", before, monster.GetHealth())
	}
	// 5% of 1000 = 50; capped by 2*level = 200, so exactly 50 damage.
	if got := before - monster.GetHealth(); got != 50 {
		t.Fatalf("charm damage = %d, want 50 (5%% of 1000 maxHealth)", got)
	}
}

func TestApplyCharmRune_NoAssignmentNoProc(t *testing.T) {
	w := newCombatWorld()
	w.Charms.Add(&charms.Charm{
		ID: charms.Wound, Category: charms.CategoryMajor, Type: charms.TypeOffensive,
		Percent: 5, Chance: [3]uint16{100, 100, 100},
	})
	mType := &creatures.MonsterType{Name: "Rat", RaceID: 21, MaxHealth: 1000}
	monster := NewMonster(1, "Rat", mType)
	monster.MaxHealth, monster.Health = 1000, 1000
	w.AddCreature(monster)
	e := NewCombatEngine(w)

	p := &Player{Level: 100}
	// Charm is unlocked but NOT assigned to this race -> no proc.
	p.SetCharmTier(charms.Wound, 1)

	before := monster.GetHealth()
	e.applyCharmRune(p, monster)
	if monster.GetHealth() != before {
		t.Fatalf("unassigned charm should not damage: %d -> %d", before, monster.GetHealth())
	}
}
