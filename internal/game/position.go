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

// InRangeOf reports whether other is within the client view distance of p (same
// floor). Uses the 8x6 half-viewport plus a small margin.
func (p Position) InRangeOf(other Position) bool {
	if p.Z != other.Z {
		return false
	}
	dx := int(p.X) - int(other.X)
	dy := int(p.Y) - int(other.Y)
	return abs(dx) <= 9 && abs(dy) <= 7
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
