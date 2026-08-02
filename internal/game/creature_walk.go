package game

import "math"

// Walk pacing, ported from Creature::getStepDuration and
// updateCalculatedStepSpeed (src/creatures/creature.cpp:1607,
// creature.hpp:1061).
//
// Monsters had none of this. The AI loop ran once a second and took one step
// per pass, so every monster in the game walked at exactly one tile per second:
// a rat and a dragon moved identically, ground speed was ignored, diagonals
// cost nothing extra, and a monster with its target adjacent did not slow down.
// That last one is what Monster::isTargetNearby exists for, which is why it had
// no caller.

const (
	// speedA/B/C are the log curve that turns a creature's speed stat into the
	// step speed used by the duration formula (creature.hpp:76).
	speedA = 857.36
	speedB = 261.29
	speedC = -4795.01

	// serverBeat is SERVER_BEAT (game.hpp:64): step durations are rounded up to
	// a multiple of it, so every creature lands on the same tick grid.
	serverBeat = 50

	// walkDiagonalExtraCost and walkTargetNearbyExtraCost are creature.hpp:43,45.
	walkDiagonalExtraCost     = 3
	walkTargetNearbyExtraCost = 2

	// defaultGroundSpeed is the speed of a tile whose ground item declares none.
	defaultGroundSpeed = 150
)

// calculatedStepSpeed is Creature::updateCalculatedStepSpeed.
//
// The guard is upstream's: below -speedB the logarithm is undefined, and the
// answer is clamped to 1 rather than allowed to be zero — a zero would divide
// by zero in the duration formula.
func calculatedStepSpeed(stepSpeed int) uint16 {
	if float64(stepSpeed) <= -speedB {
		return 1
	}
	formula := math.Floor(speedA*math.Log(float64(stepSpeed)+speedB) + speedC + 0.5)
	if formula < 1 {
		return 1
	}
	return uint16(formula)
}

// groundSpeedAt reads the tile's ground speed, Tile::getGroundSpeed.
func groundSpeedAt(w *World, pos Position) uint16 {
	if w == nil || w.Map == nil || w.Items == nil {
		return defaultGroundSpeed
	}
	tile := w.Map.GetTile(pos)
	if tile == nil || tile.Ground == nil {
		return defaultGroundSpeed
	}
	if t := w.Items.Get(tile.Ground.ID); t != nil && t.GroundSpeed > 0 {
		return t.GroundSpeed
	}
	return defaultGroundSpeed
}

// GetStepDuration is Creature::getStepDuration (creature.cpp:1607) for a
// monster: how long the next step in dir takes, in milliseconds.
//
// The two multipliers do not compound. Upstream is an if/else — a diagonal step
// costs 3x and is NOT also slowed for a nearby target, because the diagonal
// already gives the player the opening the nearby-target penalty exists to give.
func (m *Monster) GetStepDuration(w *World, dir Direction) uint32 {
	speed := int(m.GetSpeed())
	stepSpeed := calculatedStepSpeed(speed)
	ground := groundSpeedAt(w, m.GetPosition())

	d := math.Floor(1000 * float64(ground) / float64(stepSpeed))
	duration := int(math.Ceil(d/serverBeat) * serverBeat)

	switch {
	case dir == DirNE || dir == DirNW || dir == DirSE || dir == DirSW:
		duration *= walkDiagonalExtraCost
	case m.IsTargetNearby() && !m.IsFleeing() && m.Master == nil:
		// A monster already standing next to what it is fighting steps at half
		// pace, which is what lets a player kite it at all.
		duration *= walkTargetNearbyExtraCost
	}
	return uint32(duration)
}
