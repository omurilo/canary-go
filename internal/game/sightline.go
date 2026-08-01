package game

// Sight lines, ported from src/map/map.cpp.
//
// Three layers, same as upstream:
//
//	CheckSightLine      Map::checkSightLine  — one floor, tile by tile
//	IsSightClearFloors  Map::isSightClear    — adds the between-floors rules
//	CanThrowObjectTo    Map::canThrowObjectTo — adds the range and z-band limits
//
// The monster AI and the action range checks both need these, and they need the
// SAME answer the C++ server would give: a shot the client can see landing but
// the server refuses (or the reverse) is exactly the kind of divergence that
// reads as "the server is broken" rather than as a rule.

// CheckSightLine is Map::checkSightLine (map.cpp:845-944). Both endpoints are
// excluded — the shooter and the target never block their own line.
//
// The diagonal arm is Xiaolin Wu's line algorithm, transcribed rather than
// approximated. An earlier version here walked Bresenham's major axis instead,
// which agrees on straight lines and open ground but picks different tiles on a
// shallow diagonal — so the two servers disagreed on precisely the awkward shots
// a player would notice and call a bug.
//
// The uint16 arithmetic is load-bearing: eAcc detects its own overflow with
// `eAcc <= eAccTemp`, and a delta of 0xFFFF is a step of -1 by wraparound. Widen
// any of it and the line moves.
func (w *World) CheckSightLine(start, destination Position) bool {
	if start.X == destination.X && start.Y == destination.Y {
		return true
	}

	distanceX := int32(abs(int(start.X) - int(destination.X)))
	distanceY := int32(abs(int(start.Y) - int(destination.Y)))

	blocked := func(x, y uint16) bool {
		return w.Map.GetTile(Position{X: x, Y: y, Z: start.Z}).BlocksProjectile(w.Items)
	}

	switch {
	case start.Y == destination.Y:
		// Horizontal line.
		delta := uint16(0x0001)
		if start.X >= destination.X {
			delta = 0xFFFF
		}
		for distanceX--; distanceX > 0; distanceX-- {
			start.X += delta
			if blocked(start.X, start.Y) {
				return false
			}
		}

	case start.X == destination.X:
		// Vertical line.
		delta := uint16(0x0001)
		if start.Y >= destination.Y {
			delta = 0xFFFF
		}
		for distanceY--; distanceY > 0; distanceY-- {
			start.Y += delta
			if blocked(start.X, start.Y) {
				return false
			}
		}

	default:
		var eAdj, eAcc uint16
		deltaX, deltaY := uint16(0x0001), uint16(0x0001)

		if distanceY > distanceX {
			eAdj = uint16((uint32(distanceX) << 16) / uint32(distanceY))

			if start.Y > destination.Y {
				start.X, destination.X = destination.X, start.X
				start.Y, destination.Y = destination.Y, start.Y
			}
			if start.X > destination.X {
				deltaX = 0xFFFF
				eAcc -= eAdj
			}

			for distanceY--; distanceY > 0; distanceY-- {
				var xIncrease uint16
				eAccTemp := eAcc
				eAcc += eAdj
				if eAcc <= eAccTemp {
					xIncrease = deltaX
				}
				if blocked(start.X+xIncrease, start.Y+deltaY) {
					// A blocker on the last leg still lets the shot through: at one
					// tile out there is nothing left of the line to block.
					return inRange11(start, destination)
				}
				start.X += xIncrease
				start.Y += deltaY
			}
		} else {
			eAdj = uint16((uint32(distanceY) << 16) / uint32(distanceX))

			if start.X > destination.X {
				start.X, destination.X = destination.X, start.X
				start.Y, destination.Y = destination.Y, start.Y
			}
			if start.Y > destination.Y {
				deltaY = 0xFFFF
				eAcc -= eAdj
			}

			for distanceX--; distanceX > 0; distanceX-- {
				var yIncrease uint16
				eAccTemp := eAcc
				eAcc += eAdj
				if eAcc <= eAccTemp {
					yIncrease = deltaY
				}
				if blocked(start.X+deltaX, start.Y+yIncrease) {
					return inRange11(start, destination)
				}
				start.X += deltaX
				start.Y += yIncrease
			}
		}
	}
	return true
}

// IsSightClearFloors is Map::isSightClear (map.cpp:950-998): CheckSightLine plus
// the rules for looking between floors. floorCheck true — the default for
// actions and for combat — refuses anything off the shooter's own floor.
func (w *World) IsSightClearFloors(fromPos, toPos Position, floorCheck bool) bool {
	if floorCheck && fromPos.Z != toPos.Z {
		return false
	}

	// Adjacent on the same floor needs no line walk at all.
	if fromPos.Z == toPos.Z && (inRange11(fromPos, toPos) || (!floorCheck && fromPos.Z == 0)) {
		return true
	}

	// Downwards, only one floor.
	if fromPos.Z > toPos.Z && distanceZ(fromPos, toPos) > 1 {
		return false
	}

	sightClear := w.CheckSightLine(fromPos, toPos)
	if floorCheck || (fromPos.Z == toPos.Z && sightClear) {
		return sightClear
	}

	var startZ uint8
	if sightClear && fromPos.Z <= toPos.Z {
		startZ = fromPos.Z
	} else {
		// Blocked on this floor: can we throw over the obstacle, one floor up?
		//
		// fromPos.Z == 0 underflows to 255 here, exactly as it does in C++ — the
		// tile lookups then all miss, startZ wraps back to 0 and the loop below
		// still checks the floors above the destination. Guarding the underflow
		// would read as tidier and answer differently, so it is left alone.
		above := Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z - 1}
		tile := w.Map.GetTile(above)
		if (tile != nil && (tile.Ground != nil || tile.BlocksProjectile(w.Items))) ||
			!w.CheckSightLine(above, Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z - 1}) {
			return false
		}
		if fromPos.Z > toPos.Z {
			return true
		}
		startZ = fromPos.Z - 1
	}

	// Everything between the two floors has to be open above the destination.
	for ; startZ != toPos.Z; startZ++ {
		tile := w.Map.GetTile(Position{X: toPos.X, Y: toPos.Y, Z: startZ})
		if tile != nil && (tile.Ground != nil || tile.BlocksProjectile(w.Items)) {
			return false
		}
	}
	return true
}

// CanThrowObjectTo is Map::canThrowObjectTo (map.cpp:816-843): the range and
// floor-band limits that come before the line check.
//
// rangeX/rangeY are the client viewport, 7 and 5 — deliberately not square,
// because the screen is not.
func (w *World) CanThrowObjectTo(fromPos, toPos Position, checkSightLine, floorCheck bool, rangeX, rangeY int) bool {
	// Underground and above-ground never see each other.
	if (fromPos.Z >= mapInitSurfaceLayer+1 && toPos.Z <= mapInitSurfaceLayer) ||
		(toPos.Z >= mapInitSurfaceLayer+1 && fromPos.Z <= mapInitSurfaceLayer) {
		return false
	}

	deltaZ := distanceZ(fromPos, toPos)
	if deltaZ > mapLayerViewLimit {
		return false
	}

	// The z delta buys back range on both axes: a floor up is drawn shifted, so
	// what is reachable on screen grows with it.
	if abs(int(fromPos.X)-int(toPos.X))-deltaZ > rangeX {
		return false
	}
	if abs(int(fromPos.Y)-int(toPos.Y))-deltaZ > rangeY {
		return false
	}

	if !checkSightLine {
		return true
	}
	return w.IsSightClearFloors(fromPos, toPos, floorCheck)
}

const (
	// MAP_INIT_SURFACE_LAYER / MAP_LAYER_VIEW_LIMIT (src/map/map_const.hpp:18-19).
	mapInitSurfaceLayer = 7
	mapLayerViewLimit   = 2

	// Map::maxClientViewportX / Y — the client's half-screen in tiles.
	MaxClientViewportX = 7
	MaxClientViewportY = 5
)

// inRange11 is Position::areInRange<1, 1>: adjacent or the same tile, ignoring z.
func inRange11(a, b Position) bool {
	return abs(int(a.X)-int(b.X)) <= 1 && abs(int(a.Y)-int(b.Y)) <= 1
}

func distanceZ(a, b Position) int {
	return abs(int(a.Z) - int(b.Z))
}
