package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/items"
)

func sightWorld(t *testing.T) *World {
	t.Helper()
	w := NewWorld()
	w.Items = items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 2, Name: "wall", BlockSolid: true, BlockProjectile: true},
	)
	for z := 5; z <= 8; z++ {
		for x := 90; x <= 130; x++ {
			for y := 90; y <= 120; y++ {
				w.Map.SetTile(Position{X: uint16(x), Y: uint16(y), Z: uint8(z)}, &Tile{Ground: &Item{ID: 1}})
			}
		}
	}
	return w
}

func wall(w *World, x, y uint16, z uint8) {
	tile := w.Map.GetTile(Position{X: x, Y: y, Z: z})
	tile.Items = append(tile.Items, &Item{ID: 2})
}

func TestCheckSightLineStraight(t *testing.T) {
	w := sightWorld(t)
	a := Position{X: 100, Y: 100, Z: 7}

	if !w.CheckSightLine(a, a) {
		t.Errorf("a tile always sees itself")
	}
	if !w.CheckSightLine(a, Position{X: 105, Y: 100, Z: 7}) {
		t.Errorf("open ground must be clear")
	}

	wall(w, 103, 100, 7)
	if w.CheckSightLine(a, Position{X: 105, Y: 100, Z: 7}) {
		t.Errorf("a wall on the horizontal must block")
	}
	if w.CheckSightLine(Position{X: 105, Y: 100, Z: 7}, a) {
		t.Errorf("blocking must be symmetric")
	}

	// Neither endpoint blocks its own line.
	wall(w, 100, 110, 7)
	wall(w, 100, 114, 7)
	if !w.CheckSightLine(Position{X: 100, Y: 110, Z: 7}, Position{X: 100, Y: 114, Z: 7}) {
		t.Errorf("the shooter and the target must not block themselves")
	}

	wall(w, 100, 112, 7)
	if w.CheckSightLine(Position{X: 100, Y: 110, Z: 7}, Position{X: 100, Y: 114, Z: 7}) {
		t.Errorf("a wall on the vertical must block")
	}
}

// Which tiles a shallow diagonal clips is the whole reason to transcribe Wu's
// algorithm instead of approximating it, and this line is where the two
// candidates disagree most.
//
// Hand-derived from map.cpp:914-943 for (100,100) -> (106,102): distanceX 6,
// distanceY 2, so eAdj = (2 << 16) / 6 = 21845, deltaX = 1, no swap. eAcc runs
// 21845, 43690, 65535, then wraps to 21844 — and the wrap (eAcc <= eAccTemp) is
// the single step where y advances:
//
//	(101,100) (102,100) (103,100) (104,101) (105,101)
//
// Bresenham's midpoint rounding walks (101,100) (102,101) (103,101) (104,101)
// (105,102) instead — three of the five tiles differ. That was the previous
// implementation, so a wall on any of these six distinguishing tiles made the
// two servers answer differently about the same shot.
func TestCheckSightLineShallowDiagonalMatchesUpstream(t *testing.T) {
	from := Position{X: 100, Y: 100, Z: 7}
	to := Position{X: 106, Y: 102, Z: 7}

	onTheLine := []Position{
		{X: 101, Y: 100, Z: 7},
		{X: 102, Y: 100, Z: 7},
		{X: 103, Y: 100, Z: 7},
		{X: 104, Y: 101, Z: 7},
		{X: 105, Y: 101, Z: 7},
	}
	for _, p := range onTheLine {
		w := sightWorld(t)
		wall(w, p.X, p.Y, p.Z)
		if w.CheckSightLine(from, to) {
			t.Errorf("a wall at %v is on the Wu line and must block", p)
		}
	}

	// The tiles Bresenham would have walked but Wu does not. Each of these
	// blocking would mean the old approximation had come back.
	offTheLine := []Position{
		{X: 102, Y: 101, Z: 7},
		{X: 103, Y: 101, Z: 7},
		{X: 105, Y: 102, Z: 7},
	}
	for _, p := range offTheLine {
		w := sightWorld(t)
		wall(w, p.X, p.Y, p.Z)
		if !w.CheckSightLine(from, to) {
			t.Errorf("a wall at %v is off the Wu line and must not block", p)
		}
	}
}

// A blocker on the very last leg still lets the shot through — upstream returns
// true when the walk has reached a tile adjacent to the destination.
func TestCheckSightLineAdjacentBlockerPasses(t *testing.T) {
	w := sightWorld(t)
	from := Position{X: 100, Y: 100, Z: 7}
	to := Position{X: 103, Y: 101, Z: 7}
	wall(w, 102, 101, 7)
	if !w.CheckSightLine(from, to) {
		t.Errorf("a blocker one tile from the target must not stop the shot")
	}
}

func TestIsSightClearFloors(t *testing.T) {
	w := sightWorld(t)
	a := Position{X: 100, Y: 100, Z: 7}

	// floorCheck on: nothing crosses floors, ever.
	if w.IsSightClearFloors(a, Position{X: 100, Y: 100, Z: 6}, true) {
		t.Errorf("with floorCheck a different floor is never clear")
	}
	// And the convenience wrapper used by combat is exactly that.
	if w.IsSightClear(a, Position{X: 100, Y: 100, Z: 6}) {
		t.Errorf("IsSightClear must keep the floor check on")
	}

	// Adjacent on the same floor short-circuits before any line walk, so even a
	// wall on the target tile itself does not matter.
	wall(w, 101, 100, 7)
	if !w.IsSightClearFloors(a, Position{X: 101, Y: 100, Z: 7}, true) {
		t.Errorf("an adjacent tile is always in sight")
	}
}

func TestCanThrowObjectTo(t *testing.T) {
	w := sightWorld(t)
	a := Position{X: 100, Y: 100, Z: 7}

	if !w.CanThrowObjectTo(a, Position{X: 107, Y: 105, Z: 7}, true, true, MaxClientViewportX, MaxClientViewportY) {
		t.Errorf("the far corner of the viewport must be throwable")
	}
	if w.CanThrowObjectTo(a, Position{X: 108, Y: 100, Z: 7}, true, true, MaxClientViewportX, MaxClientViewportY) {
		t.Errorf("8 tiles east is past the 7-wide viewport")
	}
	if w.CanThrowObjectTo(a, Position{X: 100, Y: 106, Z: 7}, true, true, MaxClientViewportX, MaxClientViewportY) {
		t.Errorf("6 tiles south is past the 5-tall viewport")
	}

	// Surface and underground never see each other, whatever the distance.
	if w.CanThrowObjectTo(Position{X: 100, Y: 100, Z: 8}, Position{X: 100, Y: 100, Z: 7}, false, false,
		MaxClientViewportX, MaxClientViewportY) {
		t.Errorf("underground must not see the surface")
	}
}
