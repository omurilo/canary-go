package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/charms"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game/combat"
)

func TestApplyCharmRune_OffensiveProc(t *testing.T) {
	w := newCombatWorld()
	// Enflame (Major, offensive, fire, 5%) with a guaranteed 100% chance at tier 1.
	w.Charms.Add(&charms.Charm{
		ID: charms.Enflame, Name: "Enflame", Category: charms.CategoryMajor,
		Type: charms.TypeOffensive, DamageType: 1, Percent: 5,
		Chance: [3]float32{100, 100, 100},
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
	e.applyCharmRune(p, monster, 10)
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
		Percent: 5, Chance: [3]float32{100, 100, 100},
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
	e.applyCharmRune(p, monster, 10)
	if monster.GetHealth() != before {
		t.Fatalf("unassigned charm should not damage: %d -> %d", before, monster.GetHealth())
	}
}

// charmMonster builds a rat (race 21) at full health for charm combat tests.
func charmMonster(w *World, maxHP uint32) *Monster {
	mType := &creatures.MonsterType{Name: "Rat", RaceID: 21, MaxHealth: maxHP}
	m := NewMonster(1, "Rat", mType)
	m.MaxHealth, m.Health = maxHP, maxHP
	m.SetPosition(Position{X: 101, Y: 100, Z: 7})
	w.AddCreature(m)
	return m
}

// assignCharm unlocks (tier 1) and assigns a charm to race 21 for the player.
func assignCharm(p *Player, id uint8) {
	p.SetCharmTier(id, 1)
	p.SetCharmRace(id, 21)
	p.UsedRunesBit |= uint32(charms.SetBit(0, id))
}

func TestApplyCharmRune_Overpower(t *testing.T) {
	w := newCombatWorld()
	w.Charms.Add(&charms.Charm{
		ID: charms.Overpower, Category: charms.CategoryMajor, Type: charms.TypeOffensive,
		Percent: 5, Chance: [3]float32{100, 100, 100},
	})
	m := charmMonster(w, 1000)
	e := NewCombatEngine(w)
	p := &Player{Level: 50, MaxHealth: 10000, Health: 10000}
	assignCharm(p, charms.Overpower)

	before := m.GetHealth()
	e.applyCharmRune(p, m, 0)
	// min(8% of 1000 = 80, 5% of 10000 = 500) = 80.
	if got := before - m.GetHealth(); got != 80 {
		t.Fatalf("overpower damage = %d, want 80", got)
	}
}

func TestApplyCharmRune_Cripple(t *testing.T) {
	w := newCombatWorld()
	w.Charms.Add(&charms.Charm{
		ID: charms.Cripple, Category: charms.CategoryMinor, Type: charms.TypeOffensive,
		Chance: [3]float32{100, 100, 100},
	})
	m := charmMonster(w, 1000)
	m.Speed = 220
	e := NewCombatEngine(w)
	p := &Player{Level: 50}
	assignCharm(p, charms.Cripple)

	e.applyCharmRune(p, m, 10)
	if !m.HasCondition(combat.ConditionParalyze) {
		t.Fatal("cripple did not paralyze the monster")
	}
}

func TestApplyCharmRune_VampLeech(t *testing.T) {
	w := newCombatWorld()
	w.Charms.Add(&charms.Charm{
		ID: charms.Vamp, Category: charms.CategoryMinor, Type: charms.TypePassive,
		Chance: [3]float32{2.4, 2.4, 2.4},
	})
	m := charmMonster(w, 1000)
	e := NewCombatEngine(w)
	p := &Player{Level: 50, MaxHealth: 10000, Health: 5000}
	assignCharm(p, charms.Vamp)

	e.applyCharmRune(p, m, 1000) // 2.4% of 1000 = 24 healed
	if got := p.Health - 5000; got != 24 {
		t.Fatalf("vamp heal = %d, want 24", got)
	}
}

func TestApplyDefensiveCharmRune_Parry(t *testing.T) {
	w := newCombatWorld()
	w.Charms.Add(&charms.Charm{
		ID: charms.Parry, Category: charms.CategoryMajor, Type: charms.TypeDefensive,
		Chance: [3]float32{100, 100, 100},
	})
	m := charmMonster(w, 1000) // attacker monster, race 21
	e := NewCombatEngine(w)
	p := &Player{Level: 50, MaxHealth: 10000, Health: 10000}
	assignCharm(p, charms.Parry)

	before := m.GetHealth()
	e.applyDefensiveCharmRune(m, p, 200) // reflect 200 back
	if got := before - m.GetHealth(); got != 200 {
		t.Fatalf("parry reflected %d, want 200", got)
	}
}

func TestApplyDefensiveCharmRune_Dodge(t *testing.T) {
	w := newCombatWorld()
	w.Charms.Add(&charms.Charm{
		ID: charms.Dodge, Category: charms.CategoryMajor, Type: charms.TypeDefensive,
		Chance: [3]float32{100, 100, 100},
	})
	m := charmMonster(w, 1000)
	e := NewCombatEngine(w)
	p := &Player{Level: 50, MaxHealth: 10000, Health: 9800} // just took 200
	assignCharm(p, charms.Dodge)

	e.applyDefensiveCharmRune(m, p, 200) // negate: heal 200 back
	if p.Health != 10000 {
		t.Fatalf("dodge health = %d, want 10000 (damage undone)", p.Health)
	}
}

func TestApplyDefensiveCharmRune_Numb(t *testing.T) {
	w := newCombatWorld()
	w.Charms.Add(&charms.Charm{
		ID: charms.Numb, Category: charms.CategoryMinor, Type: charms.TypeDefensive,
		Chance: [3]float32{100, 100, 100},
	})
	m := charmMonster(w, 1000)
	m.Speed = 220
	e := NewCombatEngine(w)
	p := &Player{Level: 50, MaxHealth: 1000, Health: 1000}
	assignCharm(p, charms.Numb)

	e.applyDefensiveCharmRune(m, p, 100)
	if !m.HasCondition(combat.ConditionParalyze) {
		t.Fatal("numb did not paralyze the attacker")
	}
}

func TestApplyCarnageOnDeath(t *testing.T) {
	w := newCombatWorld()
	w.Charms.Add(&charms.Charm{
		ID: charms.Carnage, Category: charms.CategoryMajor, Type: charms.TypeOffensive,
		Percent: 15, Chance: [3]float32{100, 100, 100},
	})
	// Neighbor monster on the tile west of the victim.
	neighborType := &creatures.MonsterType{Name: "Rat", RaceID: 99, MaxHealth: 1000}
	neighbor := NewMonster(2, "Rat", neighborType)
	neighbor.MaxHealth, neighbor.Health = 1000, 1000
	neighbor.SetPosition(Position{X: 100, Y: 100, Z: 7})
	w.AddCreature(neighbor)

	victim := charmMonster(w, 1000) // at (101,100,7), race 21
	e := NewCombatEngine(w)
	p := &Player{Level: 50}
	p.SetCharmTier(charms.Carnage, 1)
	p.SetCharmRace(charms.Carnage, 21)
	p.UsedRunesBit |= uint32(charms.SetBit(0, charms.Carnage))

	before := neighbor.GetHealth()
	e.applyCarnageOnDeath(victim, p)
	// min(15% of 1000 = 150, 50*6 = 300) = 150.
	if got := before - neighbor.GetHealth(); got != 150 {
		t.Fatalf("carnage splash = %d, want 150", got)
	}
}

func TestBlessDeathReduction(t *testing.T) {
	w := newCombatWorld()
	w.Charms.Add(&charms.Charm{
		ID: charms.Bless, Category: charms.CategoryMinor, Type: charms.TypePassive,
		Chance: [3]float32{6, 9, 12},
	})
	killer := charmMonster(w, 1000) // race 21
	e := NewCombatEngine(w)
	p := &Player{Level: 100, Vocation: 1, SkillLoss: true, Experience: 10_000_000}

	if r := e.blessDeathReduction(p, killer); r != 0 {
		t.Fatalf("no-bless reduction = %v, want 0", r)
	}
	p.SetCharmTier(charms.Bless, 3) // tier 3 -> chance index 2 -> 12%
	p.SetCharmRace(charms.Bless, 21)
	p.UsedRunesBit |= uint32(charms.SetBit(0, charms.Bless))
	if r := e.blessDeathReduction(p, killer); r != 0.12 {
		t.Fatalf("bless reduction = %v, want 0.12", r)
	}
}

func TestApplyDeathPenaltyWith_Reduces(t *testing.T) {
	p1 := &Player{Level: 100, Vocation: 1, SkillLoss: true, Experience: 10_000_000}
	p2 := &Player{Level: 100, Vocation: 1, SkillLoss: true, Experience: 10_000_000}
	p1.ApplyDeathPenaltyWith(0)
	p2.ApplyDeathPenaltyWith(0.5)
	if p2.Experience <= p1.Experience {
		t.Fatalf("bless did not reduce loss: full-loss exp %d, reduced exp %d", p1.Experience, p2.Experience)
	}
}
