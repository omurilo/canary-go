package game

import (
	"math"
	"strings"
)

// This file ports a pragmatic subset of Player::death (src/creatures/players/
// player.cpp:3982). Full parity needs the vocation registry (per-level HP/Mana/
// Cap downgrade) and the blessing/skull systems, which are not modelled yet; we
// apply the experience/level penalty (the visible, persisted part) and refill
// vitals. The protocol layer teleports the player to their temple.

// GetLostPercent returns the fraction (0..1) of total experience lost on death,
// mirroring the spirit of Player::getLostPercent for the default
// DEATH_LOSE_PERCENT (-1) path: a base 10% reduced by held blessings. The full
// high-level formula and promotion/retro factors are simplified here.
func (p *Player) GetLostPercent() float64 {
	blessings := 0
	for _, b := range p.Blessings {
		if b > 0 {
			blessings++
		}
	}
	// Each blessing shaves ~8% off the loss (C++ subtracts blessingCount from
	// the base percent points; with no blessings modelled this is just 10%).
	lost := 10 - blessings
	if lost < 0 {
		lost = 0
	}
	return float64(lost) / 100.0
}

// GetDeathPenalty returns the same value GetLostPercent does, for the Lua
// getDeathPenalty binding.
func (p *Player) GetDeathPenalty() float64 { return p.GetLostPercent() }

// RemoveExperience subtracts experience and recomputes the level downward,
// clamping at level 1. Mirrors the level-loss loop in Player::removeExperience.
func (p *Player) RemoveExperience(amount uint64) {
	if amount >= p.Experience {
		p.Experience = 0
	} else {
		p.Experience -= amount
	}
	for p.Level > 1 && p.Experience < ExpForLevel(uint64(p.Level)) {
		p.Level--
	}
}

// ApplyDeathPenalty applies the experience/level loss, strips conditions, and
// refills vitals, mirroring the skill-loss branch of Player::death (minus the
// unmodelled per-vocation stat downgrade and skull/blessing effects). Players
// at or below level 7, or with no vocation, take no experience loss (C++ guard).
func (p *Player) ApplyDeathPenalty() { p.ApplyDeathPenaltyWith(0) }

// ApplyDeathPenaltyWith is ApplyDeathPenalty with an experience/skill loss
// reduction (0..1), used by the Bless charm (mirrors the deathLossPercent
// reduction in Player::death).
func (p *Player) ApplyDeathPenaltyWith(lossReduction float64) {
	if lossReduction < 0 {
		lossReduction = 0
	} else if lossReduction > 1 {
		lossReduction = 1
	}
	if p.SkillLoss && p.Level > 7 && p.Vocation != 0 {
		lostPercent := p.GetLostPercent() * (1 - lossReduction)
		
		lost := uint64(math.Ceil(float64(p.Experience) * lostPercent))
		if lost > 0 {
			p.RemoveExperience(lost)
		}
		
		// Magic level loss (ManaSpent)
		lostMana := uint64(math.Ceil(float64(p.ManaSpent) * lostPercent))
		if lostMana >= p.ManaSpent {
			p.ManaSpent = 0
		} else {
			p.ManaSpent -= lostMana
		}
		
		// Skill loss (SkillTries)
		for i := range p.SkillTries {
			lostTries := uint64(math.Ceil(float64(p.SkillTries[i]) * lostPercent))
			if lostTries >= p.SkillTries[i] {
				p.SkillTries[i] = 0
			} else {
				p.SkillTries[i] -= lostTries
			}
		}
	}
	// Strip every active condition (C++ removes persistent + removableOnDeath;
	// clearing all is a safe superset for the current condition set).
	p.ClearConditions()
	// Refill vitals. Black-skull respawn (40 HP / 0 mana) is not modelled.
	p.Health = p.GetMaxHealth()
	p.Mana = p.GetMaxMana()
	p.Dead = false
}

// Staff group ids that carry the cannotbeattacked flag (data/XML/groups.xml):
// 4 = gamemaster, 5 = community manager, 6 = god.
const firstProtectedGroup = 4

// CannotBeAttacked reports whether monsters (and hostile combat) must ignore
// this player, mirroring PlayerFlags_t::CannotBeAttacked. True for the staff
// groups (gamemaster/community-manager/god), god-type accounts, and ghost mode.
func (p *Player) CannotBeAttacked() bool {
	if p.Ghost {
		return true
	}
	if p.GroupID >= firstProtectedGroup {
		return true
	}
	// accounts.type ACCOUNT_TYPE_GOD (5) as a fallback signal.
	return p.AccountType >= 5
}

// TemplePosition returns where the player respawns after death: their stored
// login/temple position, falling back to their current position when unset.
func (p *Player) TemplePosition() Position {
	if p.LoginPosition.X != 0 || p.LoginPosition.Y != 0 {
		return p.LoginPosition
	}
	return p.Pos
}

// DropBlessings removes blessings on death based on death type.
func (p *Player) DropBlessings(deathType uint8) {
	// In PvP, lose 1 blessing. In PvE, keep them.
	if deathType == 1 { // PvP death
		for i := range p.Blessings {
			if p.Blessings[i] > 0 {
				p.Blessings[i]--
				break
			}
		}
	}
}

// GetBlessingsName returns comma-separated blessing names.
func (p *Player) GetBlessingsName() string {
	names := []string{
		"Wisdom of the Elderly", "Spark of the Phoenix",
		"", "", "", "", "", "",
	}
	var active []string
	for i, b := range p.Blessings {
		if b > 0 && i < len(names) && names[i] != "" {
			active = append(active, names[i])
		}
	}
	return strings.Join(active, ", ")
}
