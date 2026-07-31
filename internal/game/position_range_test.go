package game

import "testing"

// InRangeOf is the spectator range — Creature::canSee with MAP_MAX_VIEW_PORT_X/Y
// (src/creatures/creature.cpp:90). It is symmetric ±11/±11, deliberately wider than
// the client's own 8x6 window, and it spans floors. It used to be abs(dx) <= 9 &&
// abs(dy) <= 7 with an early return on any floor difference.
func TestInRangeOfSpectatorBounds(t *testing.T) {
	me := Position{X: 1000, Y: 1000, Z: 7}

	tests := []struct {
		name string
		x, y int
		want bool
	}{
		{"same tile", 1000, 1000, true},
		{"west edge", 1000 - 11, 1000, true},
		{"one past west", 1000 - 12, 1000, false},
		{"east edge", 1000 + 11, 1000, true},
		{"one past east", 1000 + 12, 1000, false},
		{"north edge", 1000, 1000 - 11, true},
		{"one past north", 1000, 1000 - 12, false},
		{"south edge", 1000, 1000 + 11, true},
		{"one past south", 1000, 1000 + 12, false},
		// The rows the old ±7 bound wrongly excluded: a creature 8 to 11 tiles north
		// or south was invisible to every spectator loop.
		{"8 north, was excluded", 1000, 992, true},
		{"11 south, was excluded", 1000, 1011, true},
		// And the columns the old ±9 bound excluded.
		{"10 west, was excluded", 990, 1000, true},
		{"11 east, was excluded", 1011, 1000, true},
	}
	for _, tc := range tests {
		got := me.InRangeOf(Position{X: uint16(tc.x), Y: uint16(tc.y), Z: 7})
		if got != tc.want {
			t.Errorf("%s: InRangeOf(%d,%d) = %v, want %v", tc.name, tc.x, tc.y, got, tc.want)
		}
	}
}

// Cross-floor was the bigger hole: the old predicate returned false for ANY floor
// difference, so a creature one floor down never appeared, spoke or showed an
// effect to anyone.
func TestInRangeOfSpansFloors(t *testing.T) {
	// On or above the surface the view covers 7 -> 0 and stops there.
	surface := Position{X: 1000, Y: 1000, Z: 7}
	if !surface.InRangeOf(Position{X: 1000, Y: 1000, Z: 6}) {
		t.Errorf("a surface creature must see one floor up")
	}
	if !surface.InRangeOf(Position{X: 1000, Y: 1000, Z: 0}) {
		t.Errorf("the view above ground spans 7 -> 0")
	}
	if surface.InRangeOf(Position{X: 1000, Y: 1000, Z: 8}) {
		t.Errorf("a surface creature must not see underground")
	}

	// Underground it is +/- MAP_LAYER_VIEW_LIMIT floors.
	under := Position{X: 1000, Y: 1000, Z: 10}
	for _, z := range []uint8{8, 9, 10, 11, 12} {
		if !under.InRangeOf(Position{X: 1000, Y: 1000, Z: z}) {
			t.Errorf("z=10 must see z=%d (within +/- 2)", z)
		}
	}
	for _, z := range []uint8{7, 13} {
		if under.InRangeOf(Position{X: 1000, Y: 1000, Z: z}) {
			t.Errorf("z=10 must not see z=%d (past +/- 2)", z)
		}
	}

	// The window shifts diagonally by the floor delta, which is how a tile one
	// floor down is drawn: offsetz = myZ - z, so looking DOWN (z greater) shifts the
	// window toward smaller coordinates.
	if !under.InRangeOf(Position{X: 1000 - 12, Y: 1000, Z: 11}) {
		t.Errorf("one floor down, the window reaches one tile further west")
	}
	if under.InRangeOf(Position{X: 1000 + 11, Y: 1000, Z: 11}) {
		t.Errorf("one floor down, the window gives up one tile in the east")
	}
}
