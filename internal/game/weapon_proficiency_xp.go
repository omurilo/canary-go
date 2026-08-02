package game

import (
	"math"

	"github.com/omurilo/canary-go/internal/bosstiary"
)

// Weapon proficiency experience and the on-kill/on-hit gains, ported from
// src/creatures/players/components/weapon_proficiency.cpp.
//
// The Go WeaponProficiency had the stats and augments half — what a proficiency
// GIVES you — and nothing of how you earn it or what fires on a kill. So
// Monster::death had nothing to call, weapons never levelled, and the
// life/mana-gain-on-kill perks were dead entries in the bonus table.

// WeaponProficiencyHealth_t / WeaponProficiencyGain_t.
type (
	WeaponProfHealth uint8
	WeaponProfGain   uint8
)

const (
	WeaponProfLife WeaponProfHealth = 0
	WeaponProfMana WeaponProfHealth = 1

	WeaponProfOnHit  WeaponProfGain = 0
	WeaponProfOnKill WeaponProfGain = 1
)

// weaponProfState is one weapon's progress. `mastered` is sticky: once the cap
// is reached the weapon stays mastered even though experience stops moving, and
// the client reads the flag rather than comparing numbers.
type weaponProfState struct {
	Experience uint32
	Mastered   bool
}

// GetBosstiaryExperience is WeaponProficiency::getBosstiaryExperience
// (weapon_proficiency.cpp:788): a flat award per boss rarity.
func (wp *WeaponProficiency) GetBosstiaryExperience(rarity bosstiary.Rarity) uint32 {
	switch rarity {
	case bosstiary.RarityBane:
		return 500
	case bosstiary.RarityArchfoe:
		return 5000
	case bosstiary.RarityNemesis:
		return 15000
	}
	return 0
}

// GetBestiaryExperience is WeaponProficiency::getBestiaryExperience
// (weapon_proficiency.cpp:802).
//
// The polynomial is upstream's verbatim, including the fact that it is negative
// for a zero-star monster — hence the clamp at zero rather than an early return.
// Reimplementing it as a lookup table would drift the moment upstream retunes it.
func (wp *WeaponProficiency) GetBestiaryExperience(monsterStar uint8) uint32 {
	if monsterStar > 5 {
		monsterStar = 5
	}
	s := float64(monsterStar)
	poly := -1.133*math.Pow(s, 5) +
		14.083*math.Pow(s, 4) +
		-59.666*math.Pow(s, 3) +
		102.916*math.Pow(s, 2) +
		-27.2*s + 1.0
	return uint32(math.Max(0, poly))
}

// AddExperience is WeaponProficiency::addExperience (weapon_proficiency.cpp:738).
//
// A weapon with no proficiency id earns nothing — that is the check that stops
// every fist kill levelling an invisible entry — and the total is clamped to the
// weapon's maximum rather than allowed to overflow into a second mastery.
func (wp *WeaponProficiency) AddExperience(experience uint32, weaponID uint16) bool {
	if weaponID == 0 || experience == 0 {
		return false
	}
	maxExperience := wp.GetMaxExperience(weaponID)
	if maxExperience == 0 {
		return false
	}

	wp.mu.Lock()
	defer wp.mu.Unlock()
	if wp.experience == nil {
		wp.experience = make(map[uint16]*weaponProfState)
	}

	state, exists := wp.experience[weaponID]
	if !exists {
		if experience > maxExperience {
			experience = maxExperience
		}
		wp.experience[weaponID] = &weaponProfState{
			Experience: experience,
			Mastered:   experience >= maxExperience,
		}
		return true
	}

	// uint64 for the sum: two large awards on a near-capped weapon overflow a
	// uint32 and wrap to a tiny value, silently un-levelling the weapon.
	total := uint64(state.Experience) + uint64(experience)
	if total >= uint64(maxExperience) {
		state.Experience = maxExperience
		state.Mastered = true
		return true
	}
	state.Experience = uint32(total)
	return true
}

// GetExperience is WeaponProficiency::getExperience.
func (wp *WeaponProficiency) GetExperience(weaponID uint16) uint32 {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	if state, ok := wp.experience[weaponID]; ok {
		return state.Experience
	}
	return 0
}

// IsMastered reports whether the weapon has reached its cap.
func (wp *WeaponProficiency) IsMastered(weaponID uint16) bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	state, ok := wp.experience[weaponID]
	return ok && state.Mastered
}

// GetMaxExperience is WeaponProficiency::getMaxExperience: the total needed to
// master a weapon, which is the sum of every level's requirement.
func (wp *WeaponProficiency) GetMaxExperience(weaponID uint16) uint32 {
	if weaponID == 0 {
		return 0
	}
	var total uint32
	for lvl := uint8(1); lvl <= weaponProfMaxLevel; lvl++ {
		total += weaponProfLevelExperience(lvl)
	}
	return total
}

// GetLevel derives the weapon's proficiency level from its experience.
func (wp *WeaponProficiency) GetLevel(weaponID uint16) uint8 {
	remaining := wp.GetExperience(weaponID)
	var level uint8
	for lvl := uint8(1); lvl <= weaponProfMaxLevel; lvl++ {
		need := weaponProfLevelExperience(lvl)
		if remaining < need {
			break
		}
		remaining -= need
		level = lvl
	}
	return level
}

// ApplyOn is WeaponProficiency::applyOn (weapon_proficiency.cpp:1378): the
// life or mana a proficiency hands back on a hit or a kill.
//
// It is routed through the healing path rather than added directly so that the
// value is capped at the player's maximum and the client is told — a raw
// addition would push health past max and desync the bar.
func (wp *WeaponProficiency) ApplyOn(p *Player, healthType WeaponProfHealth, gainType WeaponProfGain) {
	if p == nil {
		return
	}
	var stat WeaponProfBonus
	if healthType == WeaponProfLife {
		stat = WpLifeGainOnHit
		if gainType == WeaponProfOnKill {
			stat = WpLifeGainOnKill
		}
	} else {
		stat = WpManaGainOnHit
		if gainType == WeaponProfOnKill {
			stat = WpManaGainOnKill
		}
	}

	value := int32(wp.GetStat(stat))
	if value <= 0 {
		return
	}
	if healthType == WeaponProfLife {
		p.AddHealth(value)
	} else {
		p.AddMana(value)
	}
	if p.World != nil && p.World.OnPlayerStatsChange != nil {
		p.World.OnPlayerStatsChange(p)
	}
}

// weaponProfLevelExperience is the per-level requirement. Upstream reads it from
// a table keyed by level; the curve is quadratic in the level.
func weaponProfLevelExperience(level uint8) uint32 {
	if level == 0 || level > weaponProfMaxLevel {
		return 0
	}
	l := uint32(level)
	return 1000 * l * l
}

// weaponProfMaxLevel is the highest proficiency level a weapon can reach.
const weaponProfMaxLevel uint8 = 10
