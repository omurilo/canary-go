package game

import (
	"math"

	"github.com/opentibiabr/canary-go/internal/game/combat"
)

// Monster defensive stats, ported from src/creatures/monsters/monster.cpp.
//
// GetArmor and GetDefense came from BaseCreature and returned a flat 0 for every
// monster, so the shield and armor arms of the damage pipeline reduced nothing.
// That was not visible before this change because the values were not being read
// off the datapack either — monster.defenses.{defense,armor,mitigation} were
// parsed into nothing. Both halves had to land together to matter.

// GetDefense is Monster::getDefense (monster.cpp:241): the type's defense plus
// whatever a script added, scaled by the forge defense multiplier.
func (m *Monster) GetDefense() int32 {
	base := 0
	if m.Type != nil {
		base = m.Type.Defense
	}
	return int32(float64(base+int(m.Defense)) * m.GetDefenseMultiplier())
}

// AddDefense is Monster::addDefense (monster.cpp:253).
func (m *Monster) AddDefense(defense int32) { m.Defense += defense }

// GetArmor is Monster::getArmor (monster.cpp:1397).
func (m *Monster) GetArmor() int32 {
	if m.Type == nil {
		return 0
	}
	return int32(float64(m.Type.Armor) * m.GetDefenseMultiplier())
}

// GetMitigation is Monster::getMitigation (monster.cpp:1389), capped at 30.
//
// The defense+armor term is behind DISABLE_MONSTER_ARMOR upstream: it is the
// compensation applied when armor is switched off, not an addition on top of it.
// Armor is on here, so only the type's own mitigation counts.
func (m *Monster) GetMitigation() float64 {
	if m.Type == nil {
		return 0
	}
	return math.Min(m.Type.Mitigation*m.GetDefenseMultiplier(), 30)
}

// GetAttackMultiplier is Monster::getAttackMultiplier (monster.cpp:3550): the
// forge stack bonus to outgoing damage.
func (m *Monster) GetAttackMultiplier() float64 {
	if m.ForgeStack == 0 {
		return 1
	}
	return 1.35 + float64(m.ForgeStack-1)*0.1
}

// GetDefenseMultiplier is Monster::getDefenseMultiplier (monster.cpp:3558).
func (m *Monster) GetDefenseMultiplier() float64 {
	if m.ForgeStack == 0 {
		return 1
	}
	return 1 + 0.1*float64(m.ForgeStack)
}

// GetReflectPercent is Monster::getReflectPercent (monster.cpp:208): the type's
// reflect map plus whatever a script added at runtime, summed.
func (m *Monster) GetReflectPercent(reflectType combat.CombatType) int16 {
	var result int16
	if m.ReflectElements != nil {
		result += m.ReflectElements[uint32(reflectType)]
	}
	return result
}

// AddReflectElement is Monster::addReflectElement (monster.cpp:236). It
// accumulates rather than replaces, which is what lets two sources stack.
func (m *Monster) AddReflectElement(combatType combat.CombatType, percent int16) {
	if m.ReflectElements == nil {
		m.ReflectElements = make(map[uint32]int16)
	}
	m.ReflectElements[uint32(combatType)] += percent
}

// IsImmune is Monster::isImmune (monster.cpp:3538): a combat type listed in the
// type's immunities does nothing at all, which is distinct from 100% resistance
// because it also suppresses the condition the hit would apply.
func (m *Monster) IsImmune(combatType combat.CombatType) bool {
	if m.Type == nil {
		return false
	}
	for _, imm := range m.Type.Immunities {
		if imm == uint32(combatType) {
			return true
		}
	}
	return false
}
