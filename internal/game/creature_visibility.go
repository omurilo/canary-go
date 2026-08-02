package game

import "github.com/omurilo/canary-go/internal/game/combat"

// Invisibility, ported from Creature::isInvisible / canSeeCreature
// (src/creatures/creature.cpp:1849, :93) and Player::canSeeCreature
// (src/creatures/players/player.cpp:1388).
//
// The port had CONDITION_INVISIBLE in the condition enum and a monster spell
// that applied it, and nothing anywhere consulted it: an invisible player was
// drawn to every client and targeted by every monster. That is also why
// Monster::canSeeInvisibility and Npc::canSeeInvisibility — both ported — had no
// caller. They are the exception to a rule nobody was enforcing.

// IsInvisible is Creature::isInvisible (creature.cpp:1849).
//
// It asks the condition store directly rather than caching a flag, because a
// condition can end from three places (expiry, dispel, death) and a cached flag
// would have to be cleared from all three.
func IsInvisible(c Creature) bool {
	if c == nil {
		return false
	}
	h, ok := c.(conditionHolder)
	return ok && h.HasCondition(combat.ConditionInvisible)
}

// CanSeeInvisibility is the base Creature::canSeeInvisibility: false. Monster
// and Npc override it, and Player answers on its group flags.
//
// Taking the interface rather than switching on the concrete type keeps the
// three overrides where upstream puts them — on the types themselves.
func CanSeeInvisibility(c Creature) bool {
	if c == nil {
		return false
	}
	if s, ok := c.(interface{ CanSeeInvisibility() bool }); ok {
		return s.CanSeeInvisibility()
	}
	return false
}

// CanSeeCreature is Creature::canSeeCreature (creature.cpp:93).
func CanSeeCreature(viewer, target Creature) bool {
	if target == nil {
		return false
	}
	return CanSeeInvisibility(viewer) || !IsInvisible(target)
}

// CanSeeInvisibility is Player::canSeeInvisibility (player.cpp:12767).
//
// Upstream reads PlayerFlags_t::CanSenseInvisibility or group access. The port
// has no per-flag group model, so group access stands in alone — a GM sees
// through invisibility, a player does not.
func (p *Player) CanSeeInvisibility() bool {
	return p != nil && p.GroupID >= 3
}
