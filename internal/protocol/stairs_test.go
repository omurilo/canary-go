package protocol

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
)

func stairsCatalog() *items.Catalog {
	return items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},                 // plain walkable ground
		&items.ItemType{ID: 2, Name: "step", HasHeight: true},  // a height item (stair)
	)
}

// step builds a tile with plain ground plus n height items (a "step").
func step(n int) *game.Tile {
	t := &game.Tile{Ground: &game.Item{ID: 1}}
	for i := 0; i < n; i++ {
		t.Items = append(t.Items, &game.Item{ID: 2})
	}
	return t
}

func TestStairDescend(t *testing.T) {
	cat := stairsCatalog()
	m := game.NewMap()
	// Underground: standing at z 8 (z 7<->8 boundary uses holes, not ramps).
	m.SetTile(game.Position{X: 100, Y: 100, Z: 8}, &game.Tile{Ground: &game.Item{ID: 1}})
	// Step onto (north) has NO ground on z 8, but one floor below (z 9) is a step.
	m.SetTile(game.Position{X: 100, Y: 99, Z: 9}, step(3))

	dest, ok := stairDestination(m, cat, game.Position{X: 100, Y: 100, Z: 8}, game.DirNorth)
	if !ok {
		t.Fatalf("expected to descend the stairs")
	}
	if dest != (game.Position{X: 100, Y: 99, Z: 9}) {
		t.Errorf("descend dest = %+v, want (100,99,9)", dest)
	}
}

func TestStairAscendAndFlat(t *testing.T) {
	cat := stairsCatalog()
	m := game.NewMap()
	// Standing on a step (height 3) at surface z 7.
	m.SetTile(game.Position{X: 100, Y: 100, Z: 7}, step(3))
	// Tile directly above (z 6) is open (no tile).
	// Destination one floor up (north, z 6) has walkable ground.
	m.SetTile(game.Position{X: 100, Y: 99, Z: 6}, &game.Tile{Ground: &game.Item{ID: 1}})

	dest, ok := stairDestination(m, cat, game.Position{X: 100, Y: 100, Z: 7}, game.DirNorth)
	if !ok {
		t.Fatalf("expected to ascend the stairs")
	}
	if dest != (game.Position{X: 100, Y: 99, Z: 6}) {
		t.Errorf("ascend dest = %+v, want (100,99,6)", dest)
	}

	// A flat walk (no steps involved) must NOT trigger a floor change.
	m2 := game.NewMap()
	m2.SetTile(game.Position{X: 50, Y: 50, Z: 7}, &game.Tile{Ground: &game.Item{ID: 1}})
	m2.SetTile(game.Position{X: 50, Y: 49, Z: 7}, &game.Tile{Ground: &game.Item{ID: 1}})
	if _, ok := stairDestination(m2, cat, game.Position{X: 50, Y: 50, Z: 7}, game.DirNorth); ok {
		t.Errorf("flat walk should not be a stair move")
	}

	// Diagonal moves never change floor.
	if _, ok := stairDestination(m, cat, game.Position{X: 100, Y: 100, Z: 8}, game.DirNE); ok {
		t.Errorf("diagonal move should not be a stair move")
	}
}

func fcCatalog() *items.Catalog {
	return items.NewCatalog(
		&items.ItemType{ID: 10, Name: "downstair", FloorChange: "down"},
		&items.ItemType{ID: 11, Name: "upstair-north", FloorChange: "north"},
		&items.ItemType{ID: 12, Name: "ground-north", FloorChange: "north"},
	)
}

func TestFloorChangeDownWithOffset(t *testing.T) {
	cat := fcCatalog()
	m := game.NewMap()
	// Step onto a "down" stair at (100,100,7).
	m.SetTile(game.Position{X: 100, Y: 100, Z: 7}, &game.Tile{Ground: &game.Item{ID: 10}})
	// The tile directly below (z 8) carries a "north" floor-change → dy++.
	m.SetTile(game.Position{X: 100, Y: 100, Z: 8}, &game.Tile{Ground: &game.Item{ID: 12}})

	dest, ok := floorChangeDestination(m, cat, game.Position{X: 100, Y: 100, Z: 7})
	if !ok {
		t.Fatalf("expected a down floor change")
	}
	if dest != (game.Position{X: 100, Y: 101, Z: 8}) {
		t.Errorf("down dest = %+v, want (100,101,8) [north offset dy++]", dest)
	}
}

func TestFloorChangeUp(t *testing.T) {
	cat := fcCatalog()
	m := game.NewMap()
	// Step onto an up-ramp with "north" → z-1, dy--.
	m.SetTile(game.Position{X: 100, Y: 100, Z: 7}, &game.Tile{Ground: &game.Item{ID: 11}})

	dest, ok := floorChangeDestination(m, cat, game.Position{X: 100, Y: 100, Z: 7})
	if !ok {
		t.Fatalf("expected an up floor change")
	}
	if dest != (game.Position{X: 100, Y: 99, Z: 6}) {
		t.Errorf("up dest = %+v, want (100,99,6)", dest)
	}
}
