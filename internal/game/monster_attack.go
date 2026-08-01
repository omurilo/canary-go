package game

import (
	"time"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game/combat"
)

// Monster attack selection, ported from src/creatures/monsters/monster.cpp.
//
// The combat engine fires a monster's melee on a fixed per-monster interval and
// rolls its spells against their own cooldowns. Upstream does neither: every
// attack block, melee included, is gated by canUseSpell against one shared
// attackTicks counter, with a separate 1500ms floor between melee swings and an
// extra-swing flag that bypasses both.
//
// The consequence of not having it: a monster whose target stepped out of view
// and back lost a full attack interval, and one with several spell blocks could
// fire all of them on the same tick.

// DoAttacking is Monster::doAttacking (monster.cpp:1753). It only advances the
// counter and asks for an attack; which block is chosen is canUseSpell's job.
//
// A summon never attacks itself, and a player inside login protection is not a
// legal target — upstream returns before touching attackTicks in both cases, so
// the monster is ready to swing the moment the situation clears.
func (m *Monster) DoAttacking(w *World, interval uint32) bool {
	target := m.GetTarget()
	if target == nil || target.GetHealth() == 0 {
		return false
	}
	if m.Master != nil && target == Creature(m) {
		return false
	}
	if p, ok := target.(*Player); ok && p.CannotBeAttacked() {
		return false
	}

	m.attackTicks += int(interval)

	pos, targetPos := m.GetPosition(), target.GetPosition()
	attacked := false
	resetTicks := true

	for i := range m.attackBlocks() {
		block := &m.attackBlocks()[i]
		inRange, blockResets := true, true
		if !m.CanUseSpell(pos, targetPos, block, interval, &inRange, &blockResets) {
			if !blockResets {
				resetTicks = false
			}
			continue
		}
		if block.Chance < 100 && randomRange(1, 100) > int32(block.Chance) {
			continue
		}
		if w != nil && w.OnMonsterCastSpell != nil {
			w.OnMonsterCastSpell(m, target, *block)
		}
		if block.IsMelee() {
			m.lastMeleeAttack = time.Now().UnixMilli()
			m.extraMeleeAttack = false
		}
		attacked = true
	}

	if resetTicks {
		m.attackTicks = 0
	}
	return attacked
}

// attackBlocks is the monster's attack spell list. Runtime blocks added by a
// script sit alongside the ones from the type.
func (m *Monster) attackBlocks() []creatures.MonsterAttack {
	if m.Type == nil {
		return nil
	}
	return m.Type.Attacks
}

// CanUseSpell is Monster::canUseSpell (monster.cpp:2108): whether one attack
// block may fire this tick.
//
// Four gates, and each one changes behaviour on its own:
//
//   - a fleeing monster does not melee at all
//   - two melee swings are at least 1500ms apart, independent of the block's
//     own interval, unless the extra-swing flag is up
//   - the shared attackTicks arithmetic, the same `% speed >= interval` trick
//     onThinkDefense uses, which fires a block once per period rather than on
//     every tick after the first
//   - range, which reports inRange = false rather than failing outright, so the
//     caller can tell "not yet" from "too far"
//
// resetTicks is an out-parameter for the same reason it is upstream: a block
// whose interval has not elapsed must hold the shared counter open for the
// slower blocks behind it.
func (m *Monster) CanUseSpell(pos, targetPos Position, sb *creatures.MonsterAttack, interval uint32, inRange, resetTicks *bool) bool {
	*inRange = true

	if sb.IsMelee() && m.IsFleeing() {
		return false
	}

	if m.extraMeleeAttack {
		m.lastMeleeAttack = time.Now().UnixMilli()
	} else if sb.IsMelee() && time.Now().UnixMilli()-m.lastMeleeAttack < meleeAttackFloorMs {
		return false
	}

	if !sb.IsMelee() || !m.extraMeleeAttack {
		speed := sb.Interval
		if speed <= 0 {
			return false
		}
		if speed > m.attackTicks {
			*resetTicks = false
			return false
		}
		if m.attackTicks%speed >= int(interval) {
			return false // already fired this round
		}
	}

	if sb.Range != 0 && chebyshevDistance(pos, targetPos) > sb.Range {
		*inRange = false
		return false
	}
	return true
}

// HasExtraSwing is Monster::hasExtraSwing (monster.cpp:2090): a one-shot flag
// that lets the next melee bypass both the interval and the 1500ms floor. It is
// set when the target vanishes and reappears.
func (m *Monster) HasExtraSwing() bool { return m.extraMeleeAttack }

// GetCombatValues is Monster::getCombatValues (monster.cpp:3345): the damage
// range the currently-executing spell block set. Both being zero means no block
// is mid-cast, which is distinct from a block that deals zero.
func (m *Monster) GetCombatValues() (min, max int32, ok bool) {
	if m.minCombatValue == 0 && m.maxCombatValue == 0 {
		return 0, 0, false
	}
	return m.minCombatValue, m.maxCombatValue, true
}

// BlockHit is Monster::blockHit (monster.cpp:1401): the element modifier on top
// of whatever the generic creature block did.
//
// A modifier that reduces the damage to zero reports BLOCK_ARMOR rather than
// BLOCK_NONE, which is what makes the client draw a block instead of a zero.
func (m *Monster) BlockHit(attacker Creature, combatType combat.CombatType, damage int32) (int32, bool) {
	if damage == 0 || m.Type == nil || m.Type.Elements == nil {
		return damage, false
	}
	elementMod := int32(m.Type.Elements[uint32(combatType)])
	if elementMod == 0 {
		return damage, false
	}
	damage = int32(float64(damage) * (float64(100-elementMod) / 100.0))
	if damage <= 0 {
		return 0, true // blocked by armour
	}
	return damage, false
}

// GetIcons is Monster::getIcons (monster.cpp:3510): the state icons the client
// draws over the monster. Only the challenge icon is answerable here; the
// damage-received and damage-dealt buffs have no counterpart yet, so they are
// absent rather than guessed at.
func (m *Monster) GetIcons() []uint8 {
	var icons []uint8
	if m.IsTurnedMelee() {
		icons = append(icons, creatureIconTurnedMelee)
	}
	return icons
}

// GetPathSearchParams is Monster::getPathSearchParams (monster.cpp:3827): how
// wide a path search this monster is allowed for this target.
//
// The fleeing case is the one that matters and the one a naive version gets
// wrong: a fleeing monster searches out to the full server view rather than the
// client view, keeps its distance, and drops the clear-sight requirement —
// otherwise it runs into a corner because every tile it can see is closer.
func (m *Monster) GetPathSearchParams(target Creature) PathSearchParams {
	fpp := PathSearchParams{
		MinTargetDist: 1,
		MaxTargetDist: m.TargetDistanceOf(),
		ClearSight:    true,
	}
	switch {
	case m.Master != nil && target == m.Master:
		fpp.MaxTargetDist = 2
		fpp.FullPathSearch = true
	case m.IsFleeing():
		fpp.MaxTargetDist = mapMaxViewPortX
		fpp.ClearSight = false
		fpp.KeepDistance = true
		fpp.FullPathSearch = false
	case m.TargetDistanceOf() <= 1:
		fpp.FullPathSearch = true
	default:
		fpp.FullPathSearch = !m.CanUseAttack(m.GetPosition(), target, m.World)
	}
	return fpp
}

// PathSearchParams is FindPathParams (src/map/map.hpp): the bounds a creature
// puts on its own path search.
type PathSearchParams struct {
	MinTargetDist  int
	MaxTargetDist  int
	FullPathSearch bool
	ClearSight     bool
	KeepDistance   bool
}

// CheckCanApplyCharm is Monster::checkCanApplyCharm (monster.cpp:3997): whether
// a player's charm rune is bound to this monster's bestiary race.
func (m *Monster) CheckCanApplyCharm(p *Player, charmRune uint8) bool {
	if p == nil || m.Type == nil || m.Type.RaceID == 0 {
		return false
	}
	return p.GetCharmRace(charmRune) == m.Type.RaceID
}

const (
	// meleeAttackFloorMs is the hard minimum between two melee swings, applied on
	// top of the block's own interval (monster.cpp:2117).
	meleeAttackFloorMs = 1500
	// mapMaxViewPortX is MAP_MAX_VIEW_PORT_X, the server-side view a fleeing
	// monster is allowed to path across.
	mapMaxViewPortX = 11
	// CreatureIconModifications_t::TurnedMelee
	creatureIconTurnedMelee uint8 = 2
)
