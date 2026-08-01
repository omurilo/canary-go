package protocol

import (
	"github.com/omurilo/canary-go/internal/actions"
	"github.com/omurilo/canary-go/internal/game"
)

// The reach rules for using an item, ported from src/lua/creature/actions.cpp
// (Actions::canUse, Actions::canUseFar, Action::canExecuteAction).
//
// What was here before was a pair of ad-hoc `MaxDistance(...) > 8` guards, and
// they had two faults that between them made a whole class of action silently
// impossible:
//
//  1. No 0xFFFF exemption on the SOURCE position. When an item is used out of a
//     container the client does not send a map position — it sends
//     {0xFFFF, containerId|0x40, slot}. Read as a map position that is a
//     different floor (the slot index), MaxDistance returned -1 and the handler
//     returned with no message at all. Using any container item on anything was
//     dead, which is why a training weapon on an exercise dummy did nothing.
//  2. The range was a square 8. Upstream is `areInRange<1,1>` for a near use and
//     the 7x5 client viewport for a far one — the screen is not square and
//     neither is the rule.
//
// Every refusal here carries the message C++ sends. Silence is what made the
// original bug take three rounds to find.

// actionReturn is the slice of ReturnValue_t these checks can produce.
type actionReturn int

const (
	retNoError actionReturn = iota
	retTooFarAway
	retFirstGoUpstairs
	retFirstGoDownstairs
	retCannotThrow
)

// message is the text from getReturnMessage (src/utils/tools.cpp:1388-1420).
func (r actionReturn) message() string {
	switch r {
	case retTooFarAway:
		return "Too far away."
	case retFirstGoUpstairs:
		return "First go upstairs."
	case retFirstGoDownstairs:
		return "First go downstairs."
	case retCannotThrow:
		return "You cannot throw there."
	}
	return ""
}

// isMapPosition reports whether a protocol position refers to the map at all.
// An X of 0xFFFF means inventory slot or container slot, and then Y and Z are
// not coordinates — they must never reach a distance check.
func isMapPosition(pos game.Position) bool {
	return pos.X != 0xFFFF
}

// actionCanUse is Actions::canUse(player, pos) (actions.cpp:179-191): the item
// has to be on the player's own floor and within arm's reach.
func (g *GameProtocol) actionCanUse(pos game.Position) actionReturn {
	if !isMapPosition(pos) {
		return retNoError
	}
	playerPos := g.player.Pos
	if playerPos.Z != pos.Z {
		if playerPos.Z > pos.Z {
			return retFirstGoUpstairs
		}
		return retFirstGoDownstairs
	}
	if !areInRange11(playerPos, pos) {
		return retTooFarAway
	}
	return retNoError
}

// actionCanUseFar is Actions::canUseFar (actions.cpp:201-220): the relaxed check
// for an action that declared allowFarUse — on screen, and in line of sight.
func (g *GameProtocol) actionCanUseFar(toPos game.Position, checkLineOfSight, checkFloor bool) actionReturn {
	if !isMapPosition(toPos) {
		return retNoError
	}
	creaturePos := g.player.Pos
	if checkFloor && creaturePos.Z != toPos.Z {
		if creaturePos.Z > toPos.Z {
			return retFirstGoUpstairs
		}
		return retFirstGoDownstairs
	}
	// areInRange<7, 5>(toPos, creaturePos) — the client viewport, not a square.
	if !areInRangeXY(toPos, creaturePos, game.MaxClientViewportX, game.MaxClientViewportY) {
		return retTooFarAway
	}
	if checkLineOfSight &&
		!g.deps.World.CanThrowObjectTo(creaturePos, toPos, true, checkFloor,
			game.MaxClientViewportX, game.MaxClientViewportY) {
		return retCannotThrow
	}
	return retNoError
}

// actionCanExecute is Action::canExecuteAction (actions.cpp:536-542): which of
// the two checks applies is the action's own choice.
func (g *GameProtocol) actionCanExecute(a *actions.Action, toPos game.Position) actionReturn {
	if a == nil || !a.AllowFarUse {
		return g.actionCanUse(toPos)
	}
	return g.actionCanUseFar(toPos, a.CheckLineOfSight, a.CheckFloor)
}

func areInRange11(a, b game.Position) bool {
	return areInRangeXY(a, b, 1, 1)
}

func areInRangeXY(a, b game.Position, dx, dy int) bool {
	return absInt(int(a.X)-int(b.X)) <= dx && absInt(int(a.Y)-int(b.Y)) <= dy
}
