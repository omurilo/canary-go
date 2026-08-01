package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/actions"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
)

// rangeProto builds a protocol with a player standing at 100,100,7 on open
// ground, which is all these checks look at.
func rangeProto(t *testing.T) *GameProtocol {
	t.Helper()
	w := game.NewWorld()
	w.Items = items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 2, Name: "wall", BlockSolid: true, BlockProjectile: true},
	)
	for x := 90; x <= 130; x++ {
		for y := 90; y <= 120; y++ {
			w.Map.SetTile(game.Position{X: uint16(x), Y: uint16(y), Z: 7}, &game.Tile{Ground: &game.Item{ID: 1}})
		}
	}
	p := &game.Player{Name: "Tester"}
	p.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	return &GameProtocol{player: p, deps: &Deps{World: w, Items: w.Items}}
}

// The bug this whole file exists for: a container slot is NOT a map position.
// The client sends {0xFFFF, containerId|0x40, slot}, and the old guard read that
// slot index as a floor, decided the item was on another floor, and returned
// with no message. Every use of a container item on a target was dead — a
// training weapon on an exercise dummy did precisely nothing.
func TestContainerSlotIsNotAMapPosition(t *testing.T) {
	g := rangeProto(t)

	// Container 4, slot 11 — the shape from the real log that exposed this.
	slot := game.Position{X: 0xFFFF, Y: 0x40 | 4, Z: 11}
	if ret := g.actionCanUse(slot); ret != retNoError {
		t.Fatalf("a container slot must never be range-checked, got %v (%q)", ret, ret.message())
	}
	if ret := g.actionCanUseFar(slot, true, true); ret != retNoError {
		t.Fatalf("canUseFar must exempt a container slot too, got %v", ret)
	}

	// An inventory slot is the same shape with no container bit.
	inv := game.Position{X: 0xFFFF, Y: 0, Z: 0}
	if ret := g.actionCanUse(inv); ret != retNoError {
		t.Fatalf("an inventory slot must not be range-checked, got %v", ret)
	}
}

// canUse is arm's length, and reports WHICH way the floors are wrong. The old
// code answered every one of these with silence.
func TestActionCanUseIsArmsLength(t *testing.T) {
	g := rangeProto(t)

	for _, p := range []game.Position{
		{X: 100, Y: 100, Z: 7}, // same tile
		{X: 101, Y: 101, Z: 7}, // diagonal
		{X: 99, Y: 100, Z: 7},
	} {
		if ret := g.actionCanUse(p); ret != retNoError {
			t.Errorf("%v is within reach, got %v", p, ret)
		}
	}

	if ret := g.actionCanUse(game.Position{X: 102, Y: 100, Z: 7}); ret != retTooFarAway {
		t.Errorf("two tiles away must be too far, got %v", ret)
	}
	// A square-8 check let this through; upstream requires adjacency.
	if ret := g.actionCanUse(game.Position{X: 105, Y: 100, Z: 7}); ret != retTooFarAway {
		t.Errorf("five tiles away must be too far, got %v", ret)
	}

	// Z below the player means a bigger Z value: the player has to go down.
	if ret := g.actionCanUse(game.Position{X: 100, Y: 100, Z: 8}); ret != retFirstGoDownstairs {
		t.Errorf("target one floor below: got %v, want first-go-downstairs", ret)
	}
	if ret := g.actionCanUse(game.Position{X: 100, Y: 100, Z: 6}); ret != retFirstGoUpstairs {
		t.Errorf("target one floor above: got %v, want first-go-upstairs", ret)
	}
}

// canUseFar is the client viewport, 7 wide and 5 tall — deliberately not square,
// and the square-8 guard was wrong in both directions at once.
func TestActionCanUseFarIsTheViewport(t *testing.T) {
	g := rangeProto(t)

	if ret := g.actionCanUseFar(game.Position{X: 107, Y: 100, Z: 7}, false, true); ret != retNoError {
		t.Errorf("7 tiles east is on screen, got %v", ret)
	}
	if ret := g.actionCanUseFar(game.Position{X: 108, Y: 100, Z: 7}, false, true); ret != retTooFarAway {
		t.Errorf("8 tiles east is off screen, got %v", ret)
	}
	// The square-8 guard accepted this; the viewport is only 5 tall.
	if ret := g.actionCanUseFar(game.Position{X: 100, Y: 106, Z: 7}, false, true); ret != retTooFarAway {
		t.Errorf("6 tiles south is off screen, got %v", ret)
	}
	if ret := g.actionCanUseFar(game.Position{X: 100, Y: 105, Z: 7}, false, true); ret != retNoError {
		t.Errorf("5 tiles south is on screen, got %v", ret)
	}
}

// With line-of-sight checking on, a wall in the way is CANNOTTHROW, not
// TOOFARAWAY — the two produce different messages and only one of them should
// make the player walk.
func TestActionCanUseFarChecksLineOfSight(t *testing.T) {
	g := rangeProto(t)
	target := game.Position{X: 104, Y: 100, Z: 7}

	if ret := g.actionCanUseFar(target, true, true); ret != retNoError {
		t.Fatalf("clear ground must be throwable, got %v", ret)
	}

	wall := g.deps.World.Map.GetTile(game.Position{X: 102, Y: 100, Z: 7})
	wall.Items = append(wall.Items, &game.Item{ID: 2})

	if ret := g.actionCanUseFar(target, true, true); ret != retCannotThrow {
		t.Errorf("a blockprojectile tile in the way must be cannot-throw, got %v", ret)
	}
	// The same action with blockWalls(false) shoots straight through it.
	if ret := g.actionCanUseFar(target, false, true); ret != retNoError {
		t.Errorf("with line-of-sight checking off the wall is irrelevant, got %v", ret)
	}
}

// canExecuteAction picks between the two, and an Action built without New()
// would silently default to checkFloor/checkLineOfSight false.
func TestActionCanExecutePicksByAllowFarUse(t *testing.T) {
	g := rangeProto(t)
	far := game.Position{X: 105, Y: 100, Z: 7}

	near := actions.New()
	if ret := g.actionCanExecute(near, far); ret != retTooFarAway {
		t.Errorf("without allowFarUse the target must be too far, got %v", ret)
	}

	ranged := actions.New()
	ranged.AllowFarUse = true
	if ret := g.actionCanExecute(ranged, far); ret != retNoError {
		t.Errorf("with allowFarUse the target is on screen, got %v", ret)
	}

	if !near.CheckFloor || !near.CheckLineOfSight {
		t.Errorf("actions.New must default both checks to true, as C++ does")
	}
}

// Every refusal carries a message. Silence is what made the original bug survive
// three rounds of diagnosis.
func TestEveryRefusalHasAMessage(t *testing.T) {
	for _, r := range []actionReturn{retTooFarAway, retFirstGoUpstairs, retFirstGoDownstairs, retCannotThrow} {
		if r.message() == "" {
			t.Errorf("actionReturn %d has no message", r)
		}
	}
	if retNoError.message() != "" {
		t.Errorf("success must not carry a message")
	}
}
