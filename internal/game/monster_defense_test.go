package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game/combat"
)

// GetArmor and GetDefense came from BaseCreature and returned 0 for every
// monster, so the armor and shield arms of the damage pipeline reduced nothing.
// The datapack values were not being read either, so both halves had to land
// together before a single point of damage changed.
func TestMonsterDefenseAndArmorComeFromTheType(t *testing.T) {
	mt := &creatures.MonsterType{Name: "Dragon", Defense: 30, Armor: 25}
	m := &Monster{Type: mt}

	if got := m.GetDefense(); got != 30 {
		t.Errorf("GetDefense = %d, want 30", got)
	}
	if got := m.GetArmor(); got != 25 {
		t.Errorf("GetArmor = %d, want 25", got)
	}
}

// A monster with no type must not panic or invent stats.
func TestMonsterWithoutATypeHasNoDefensiveStats(t *testing.T) {
	m := &Monster{}
	if m.GetDefense() != 0 || m.GetArmor() != 0 || m.GetMitigation() != 0 {
		t.Errorf("typeless monster reported defense=%d armor=%d mitigation=%v",
			m.GetDefense(), m.GetArmor(), m.GetMitigation())
	}
}

// addDefense accumulates on top of the type's value rather than replacing it.
func TestAddDefenseStacksOnTheTypeValue(t *testing.T) {
	m := &Monster{Type: &creatures.MonsterType{Name: "Dragon", Defense: 30}}
	m.AddDefense(20)
	if got := m.GetDefense(); got != 50 {
		t.Errorf("GetDefense = %d, want 50", got)
	}
}

// The forge multipliers scale both directions: an influenced monster hits harder
// and takes less. The defense multiplier applies to armor and defense alike.
func TestForgeStackScalesTheDefensiveStats(t *testing.T) {
	m := &Monster{Type: &creatures.MonsterType{Name: "Dragon", Defense: 100, Armor: 100}}
	m.ForgeStack = 5

	if want := 1.5; m.GetDefenseMultiplier() != want {
		t.Errorf("defense multiplier = %v, want %v", m.GetDefenseMultiplier(), want)
	}
	if got := m.GetDefense(); got != 150 {
		t.Errorf("GetDefense = %d, want 150", got)
	}
	if got := m.GetArmor(); got != 150 {
		t.Errorf("GetArmor = %d, want 150", got)
	}
	// getAttackMultiplier is 1.35 + (stacks-1)*0.1.
	if want := 1.75; absFloat(m.GetAttackMultiplier()-want) > 1e-9 {
		t.Errorf("attack multiplier = %v, want %v", m.GetAttackMultiplier(), want)
	}
}

// Mitigation is capped at 30 whatever the type declares.
func TestMitigationIsCappedAtThirty(t *testing.T) {
	m := &Monster{Type: &creatures.MonsterType{Name: "Wall", Mitigation: 99}}
	if got := m.GetMitigation(); got != 30 {
		t.Errorf("GetMitigation = %v, want 30", got)
	}
}

// Reflect accumulates so two sources stack, rather than the second replacing
// the first.
func TestReflectElementsAccumulate(t *testing.T) {
	m := &Monster{}
	m.AddReflectElement(combat.CombatFire, 10)
	m.AddReflectElement(combat.CombatFire, 15)

	if got := m.GetReflectPercent(combat.CombatFire); got != 25 {
		t.Errorf("fire reflect = %d, want 25", got)
	}
	if got := m.GetReflectPercent(combat.CombatIce); got != 0 {
		t.Errorf("ice reflect = %d, want 0", got)
	}
}

// Immunities were parsed off the datapack into MonsterType.Immunities and never
// consulted anywhere — a fire elemental took full damage from fire.
func TestImmunitiesAreReported(t *testing.T) {
	mt := &creatures.MonsterType{
		Name:       "Fire Elemental",
		Immunities: []uint32{uint32(combat.CombatFire)},
	}
	m := &Monster{Type: mt}

	if !m.IsImmune(combat.CombatFire) {
		t.Error("a declared immunity must be reported")
	}
	if m.IsImmune(combat.CombatIce) {
		t.Error("an undeclared combat type must not be immune")
	}
}

// Immunity is not the same as 100% resistance: the hit is refused outright, so
// the condition it carried is never applied either.
func TestImmuneTargetTakesNoDamage(t *testing.T) {
	mt := &creatures.MonsterType{
		Name:       "Fire Elemental",
		MaxHealth:  1000,
		Immunities: []uint32{uint32(combat.CombatFire)},
	}
	m := &Monster{Type: mt}
	m.MaxHealth, m.Health = 1000, 1000

	c := combat.NewCombat()
	c.SetParam(combat.CombatParamType, uint32(combat.CombatFire))
	attacker := &Monster{Type: &creatures.MonsterType{Name: "Attacker"}}
	attacker.MaxHealth, attacker.Health = 100, 100
	applied := c.DoCombatHealth(adaptCreature(attacker), adaptCreature(m), combat.CombatDamage{
		PrimaryType:  combat.CombatFire,
		PrimaryValue: 500,
	})

	if applied {
		t.Error("an immune target must refuse the hit")
	}
	if m.Health != 1000 {
		t.Errorf("health = %d, want 1000 — immunity did not hold", m.Health)
	}
}

// The same monster still takes damage from a type it is not immune to, so the
// short-circuit is not swallowing everything.
func TestNonImmuneDamageStillLands(t *testing.T) {
	mt := &creatures.MonsterType{
		Name:       "Fire Elemental",
		MaxHealth:  1000,
		Immunities: []uint32{uint32(combat.CombatFire)},
	}
	m := &Monster{Type: mt}
	m.MaxHealth, m.Health = 1000, 1000

	c := combat.NewCombat()
	c.SetParam(combat.CombatParamType, uint32(combat.CombatIce))
	attacker := &Monster{Type: &creatures.MonsterType{Name: "Attacker"}}
	attacker.MaxHealth, attacker.Health = 100, 100
	c.DoCombatHealth(adaptCreature(attacker), adaptCreature(m), combat.CombatDamage{
		PrimaryType:  combat.CombatIce,
		PrimaryValue: 500,
	})

	if m.Health == 1000 {
		t.Error("ice damage was blocked by a fire immunity")
	}
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
