package game

// Position is a tile coordinate in the game world.
type Position struct {
	X uint16
	Y uint16
	Z uint8
}

// Direction values match the client protocol.
type Direction uint8

const (
	DirNorth Direction = 0
	DirEast  Direction = 1
	DirSouth Direction = 2
	DirWest  Direction = 3
	DirSW    Direction = 4
	DirSE    Direction = 5
	DirNW    Direction = 6
	DirNE    Direction = 7
)

// Offset returns the position shifted for a cardinal/diagonal direction.
func (p Position) Offset(d Direction) Position {
	switch d {
	case DirNorth:
		p.Y--
	case DirEast:
		p.X++
	case DirSouth:
		p.Y++
	case DirWest:
		p.X--
	case DirNE:
		p.X++
		p.Y--
	case DirSE:
		p.X++
		p.Y++
	case DirSW:
		p.X--
		p.Y++
	case DirNW:
		p.X--
		p.Y--
	}
	return p
}

// Spectator view range (src/map/map_const.hpp:12-19). It is deliberately WIDER
// than the client's own window (8x6): the server tracks events a little beyond
// what the player can draw, so a creature stepping into view is already known.
const (
	MapMaxViewPortX     = 11 // MAP_MAX_CLIENT_VIEW_PORT_X + 3
	MapMaxViewPortY     = 11 // MAP_MAX_CLIENT_VIEW_PORT_Y + 5
	MapInitSurfaceLayer = 7
	MapLayerViewLimit   = 2
)

// InRangeOf reports whether other is within spectator range of p — the port of
// Creature::canSee (src/creatures/creature.cpp:90), which calls the shared canSee
// with MAP_MAX_VIEW_PORT_X/Y. It is symmetric, unlike the client window, and it
// DOES span floors:
//
//   - on or above the surface (z <= 7) the view covers 7 -> 0;
//   - underground it covers +/- 2 floors from the one we stand on;
//   - and the window shifts diagonally by the floor delta, because that is how a
//     tile one floor down is drawn.
//
// This used to be abs(dx) <= 9 && abs(dy) <= 7 with an early return on any floor
// difference, which was wrong three ways: two columns too narrow, four rows too
// short, and it delivered no cross-floor events at all — a creature on the floor
// below was invisible to every spectator loop, so it never appeared, spoke, or
// showed an effect.
//
// This is NOT the predicate for deciding what a client can render; that is
// ProtocolGame::canSee, ported separately in the protocol layer, and it is
// narrower and asymmetric.
func (p Position) InRangeOf(other Position) bool {
	if p.Z <= MapInitSurfaceLayer {
		if other.Z > MapInitSurfaceLayer {
			return false
		}
	} else if abs(int(p.Z)-int(other.Z)) > MapLayerViewLimit {
		return false
	}
	offsetz := int(p.Z) - int(other.Z)
	x, y := int(other.X), int(other.Y)
	px, py := int(p.X), int(p.Y)
	return x >= px-MapMaxViewPortX+offsetz && x <= px+MapMaxViewPortX+offsetz &&
		y >= py-MapMaxViewPortY+offsetz && y <= py+MapMaxViewPortY+offsetz
}

// MaxDistance returns the max Chebyshev distance (max(dx, dy)) to other on same floor.
// If on different floors, returns -1.
func (p Position) MaxDistance(other Position) int {
	if p.Z != other.Z {
		return -1
	}
	dx := abs(int(p.X) - int(other.X))
	dy := abs(int(p.Y) - int(other.Y))
	if dx > dy {
		return dx
	}
	return dy
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// DistanceTo returns the Chebyshev distance to other on the same floor, or -1
// for different floors (convenience alias for MaxDistance).
func (p Position) DistanceTo(other Position) int {
	return p.MaxDistance(other)
}
