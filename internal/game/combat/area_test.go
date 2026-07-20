package combat

import "testing"

// TestAreaCombat_SingleCell verifies a 1x1 area with just a center resolves to
// exactly the target position.
func TestAreaCombat_SingleCell(t *testing.T) {
	area := NewAreaCombat([]uint32{3}, 1)
	target := Position{X: 100, Y: 100, Z: 7}
	list := area.GetList(target, target)
	if len(list) != 1 {
		t.Fatalf("expected 1 tile, got %d: %+v", len(list), list)
	}
	if list[0] != target {
		t.Fatalf("expected center at target %+v, got %+v", target, list[0])
	}
}

// TestAreaCombat_NorthBeam verifies a 3-row vertical beam (center at the bottom)
// resolves, when cast to the north, to three tiles running up to and including
// the target — mirroring how AreaCombat orients the matrix by cast direction.
func TestAreaCombat_NorthBeam(t *testing.T) {
	// {1},{1},{3} : column beam, center on the last (nearest) row.
	area := NewAreaCombat([]uint32{1, 1, 3}, 3)

	caster := Position{X: 100, Y: 100, Z: 7}
	target := Position{X: 100, Y: 98, Z: 7} // two tiles north

	list := area.GetList(caster, target)
	if len(list) != 3 {
		t.Fatalf("expected 3 tiles, got %d: %+v", len(list), list)
	}

	// The target tile must be covered.
	found := false
	for _, p := range list {
		if p == target {
			found = true
		}
		if p.X != target.X {
			t.Errorf("beam tile drifted in X: %+v", p)
		}
	}
	if !found {
		t.Errorf("expected the target tile %+v to be covered, got %+v", target, list)
	}
}

// TestAreaCombat_RowWidth verifies a wide single-row area keeps its width after
// orientation (a 3-wide row centered in the middle).
func TestAreaCombat_RowWidth(t *testing.T) {
	area := NewAreaCombat([]uint32{1, 3, 1}, 1)
	target := Position{X: 100, Y: 100, Z: 7}
	list := area.GetList(target, target)
	if len(list) != 3 {
		t.Fatalf("expected 3 tiles, got %d: %+v", len(list), list)
	}
}
