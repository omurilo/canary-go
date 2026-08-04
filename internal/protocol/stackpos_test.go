package protocol

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
)

// stackposCatalog gives id 1 a plain ground/item type and id 2 an always-on-top
// one, so a tile can hold both kinds and only the second may count as a top item.
func stackposCatalog() *items.Catalog {
	return items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 2, Name: "border", AlwaysOnTopOrder: 1},
	)
}

func stackposSetup(t *testing.T, viewer *game.Player) (*GameProtocol, *game.World) {
	t.Helper()
	world := game.NewWorld()
	world.Items = stackposCatalog()
	return &GameProtocol{player: viewer, deps: &Deps{World: world, Items: world.Items}}, world
}

// ClientIndexOfCreature must count the tile's ground and always-on-top items, then
// the creatures that piled up BELOW the target — walking the slice FORWARD, the
// order the client draws them and the one C++ uses to compute the index
// (Tile::getClientIndexOfCreature, tile.cpp:1433). The first creature in the slice
// is the one sitting lowest, directly on the item stack.
func TestClientIndexOfCreatureCountsVisibleCreaturesBelow(t *testing.T) {
	viewer := &game.Player{ID: 1, Name: "Viewer", GroupID: 1}
	g, world := stackposSetup(t, viewer)

	pos := game.Position{X: 100, Y: 100, Z: 7}
	tile := &game.Tile{
		Ground: &game.Item{ID: 1},
		Items: []*game.Item{
			{ID: 2}, // always-on-top: counts
			{ID: 1}, // down item: must NOT count
		},
	}
	world.Map.SetTile(pos, tile)

	first := game.NewMonster(10, "Rat", nil)
	second := game.NewMonster(11, "Cave Rat", nil)
	third := game.NewMonster(12, "Bug", nil)
	tile.Creatures = []game.Creature{first, second, third}

	// first is the lowest creature, so it sits directly on top of the item stack
	// (ground=1 + one top item=1 → index 2); second and third are piled above it.
	tests := []struct {
		id   uint32
		want int
	}{
		{10, 2},
		{11, 3},
		{12, 4},
	}
	for _, tc := range tests {
		if got := g.ClientIndexOfCreature(pos, tc.id); got != tc.want {
			t.Errorf("ClientIndexOfCreature(%d) = %d, want %d", tc.id, got, tc.want)
		}
	}
}

// A creature the viewer cannot see must not be counted. Before the port the
// stackpos was reconstructed as len(Creatures)-index, which counted every
// creature on the tile, so one ghost inflated the value for everyone.
func TestClientIndexOfCreatureSkipsInvisibleCreatures(t *testing.T) {
	viewer := &game.Player{ID: 1, Name: "Viewer", GroupID: 1}
	g, world := stackposSetup(t, viewer)

	pos := game.Position{X: 100, Y: 100, Z: 7}
	tile := &game.Tile{Ground: &game.Item{ID: 1}}
	world.Map.SetTile(pos, tile)

	ghost := &game.Player{ID: 2, Name: "Ghost", GroupID: 1, Ghost: true}
	target := game.NewMonster(10, "Rat", nil)
	// ghost is appended first, so it piles up below the target and occupies the
	// slot right on top of the ground for everyone who can see it.
	tile.Creatures = []game.Creature{ghost, target}

	// A plain player cannot see the ghost: only the ground counts below the target.
	if got := g.ClientIndexOfCreature(pos, 10); got != 1 {
		t.Errorf("with an unseen ghost below: got %d, want 1", got)
	}

	// A gamemaster can see it, so it occupies a slot and pushes the target up.
	gm := &game.Player{ID: 3, Name: "GM", GroupID: 3}
	gmProto := &GameProtocol{player: gm, deps: g.deps}
	if got := gmProto.ClientIndexOfCreature(pos, 10); got != 2 {
		t.Errorf("with a visible ghost below: got %d, want 2", got)
	}
}

// Not being on the tile is -1, not a stack position. Callers treat -1 as "send no
// packet"; returning 0 instead would tell the client to delete its ground.
func TestClientIndexOfCreatureReturnsMinusOneWhenAbsent(t *testing.T) {
	viewer := &game.Player{ID: 1, Name: "Viewer", GroupID: 1}
	g, world := stackposSetup(t, viewer)

	pos := game.Position{X: 100, Y: 100, Z: 7}
	world.Map.SetTile(pos, &game.Tile{Ground: &game.Item{ID: 1}})

	if got := g.ClientIndexOfCreature(pos, 999); got != -1 {
		t.Errorf("absent creature: got %d, want -1", got)
	}
	if got := g.ClientIndexOfCreature(game.Position{X: 1, Y: 1, Z: 7}, 999); got != -1 {
		t.Errorf("absent tile: got %d, want -1", got)
	}
}

// The whole point of the change: the value handed to the 0x6D packet must be the
// one the creature had while it was still on the tile. Capturing it afterwards
// cannot work, so World takes the snapshot inside its own critical section.
func TestCaptureStackPositionsSnapshotsBeforeRemoval(t *testing.T) {
	viewer := &game.Player{ID: 1, Name: "Viewer", GroupID: 1}
	g, world := stackposSetup(t, viewer)

	pos := game.Position{X: 100, Y: 100, Z: 7}
	dest := game.Position{X: 101, Y: 100, Z: 7}
	world.Map.SetTile(pos, &game.Tile{Ground: &game.Item{ID: 1}})
	world.Map.SetTile(dest, &game.Tile{Ground: &game.Item{ID: 1}})

	viewer.SetPosition(pos)
	world.AddPlayer(viewer, g) // AddPlayer assigns the creature id and the session

	// Two monsters land on the tile after the viewer, so the walker (added first
	// among them) ends up with a visible creature above it.
	walker := game.NewMonster(10, "Rat", nil)
	walker.SetPosition(pos)
	world.AddCreature(walker)
	blocker := game.NewMonster(11, "Cave Rat", nil)
	blocker.SetPosition(pos)
	world.AddCreature(blocker)

	// ground(1) + blocker above(1) = 2. The viewer itself was appended before the
	// walker, so it is below and does not count.
	want := 2

	world.CaptureStackPositions = func(p game.Position, c game.Creature) map[uint32]int {
		return CaptureStackPositions(world, p, c)
	}

	var captured map[uint32]int
	world.OnCreatureMove = func(c game.Creature, oldPos, newPos game.Position, oldStackPos map[uint32]int) {
		captured = oldStackPos
	}

	if _, ok := world.TryMoveCreature(walker, game.DirEast); !ok {
		t.Fatalf("the walker could not step east")
	}
	if got, ok := captured[viewer.ID]; !ok || got != want {
		t.Fatalf("captured stackpos for the viewer = %d (present=%v), want %d", got, ok, want)
	}

	// And it is genuinely unrecoverable after the fact: recomputing now, with the
	// walker already gone from the tile, yields -1.
	if got := g.ClientIndexOfCreature(pos, walker.GetID()); got != -1 {
		t.Errorf("post-move recompute = %d, want -1 (the creature has left)", got)
	}
}

// The branch table of ProtocolGame::sendMoveCreature (protocolgame.cpp:8700) for
// the non-self case. The first two rows are the whole point: an uncaptured or
// unseen spectator must get NOTHING. Reconstructing a stackpos for them is what
// removed the wrong thing from their tile and logged "no thing at pos".
func TestCreatureMoveAction(t *testing.T) {
	tests := []struct {
		name                                  string
		oldStack                              int
		captured, seesOld, seesNew, tp, known bool
		want                                  moveAction
	}{
		{"never captured", 0, false, true, true, false, true, moveActionNone},
		{"captured as unseen", -1, true, true, true, false, true, moveActionNone},
		{"plain adjacent step", 2, true, true, true, false, true, moveActionShift},
		{"walks out of view", 2, true, true, false, false, true, moveActionRemove},
		{"walks into view", 2, true, false, true, false, true, moveActionAdd},
		{"passes by outside the view entirely", 2, true, false, false, false, true, moveActionNone},
		{"teleport", 2, true, true, true, true, true, moveActionRemoveAdd},
		{"past the 10-thing window", 10, true, true, true, false, true, moveActionRemoveAdd},
		{"unknown to this client", 2, true, true, true, false, false, moveActionRemoveAdd},
		// Leaving view wins over teleport: there is no new position to add to.
		{"unseen new pos while teleporting", 2, true, true, false, true, true, moveActionRemove},
	}
	for _, tc := range tests {
		got := creatureMoveAction(tc.oldStack, tc.captured, tc.seesOld, tc.seesNew, tc.tp, tc.known)
		if got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

// The exact packet from crashes/36493cfa-b63f-4ac0-8cc2-6c6d51d566c7.dmp.json,
// which the stock client rejected with "target field not existing":
//
//	6D 5E 7E DC 7D 07 01 5D 7E DC 7D 07
//	0x6D  oldPos (32350,32220,7)  stackpos 1  newPos (32349,32220,7)
//
// The player stood at (32358,32224,7), so newPos.x is myX-9 — one column left of
// the description the client holds, which starts at myX-8. Position.InRangeOf is
// symmetric (±9) and called it visible; ProtocolGame::canSee does not.
func TestCanSeeMatchesTheAsymmetricClientWindow(t *testing.T) {
	viewer := &game.Player{Name: "Gm Test", GroupID: 1}
	g, world := stackposSetup(t, viewer)
	_ = world
	viewer.Pos = game.Position{X: 32358, Y: 32224, Z: 7}

	crashPos := game.Position{X: 32349, Y: 32220, Z: 7}
	if g.canSee(crashPos) {
		t.Errorf("canSee(%v) = true; the client's window starts at x=%d", crashPos, 32358-mapMaxClientViewPortX)
	}
	if !viewer.Pos.InRangeOf(crashPos) {
		t.Errorf("InRangeOf(%v) = false; the test no longer reproduces the divergence it guards", crashPos)
	}

	// The bounds themselves: 8 left, 9 right, 6 up, 7 down.
	for _, tc := range []struct {
		name string
		x, y int
		want bool
	}{
		{"left edge", 32358 - 8, 32224, true},
		{"one past the left edge", 32358 - 9, 32224, false},
		{"right edge", 32358 + 9, 32224, true},
		{"one past the right edge", 32358 + 10, 32224, false},
		{"top edge", 32358, 32224 - 6, true},
		{"one past the top edge", 32358, 32224 - 7, false},
		{"bottom edge", 32358, 32224 + 7, true},
		{"one past the bottom edge", 32358, 32224 + 8, false},
	} {
		pos := game.Position{X: uint16(tc.x), Y: uint16(tc.y), Z: 7}
		if got := g.canSee(pos); got != tc.want {
			t.Errorf("%s: canSee(%d,%d) = %v, want %v", tc.name, tc.x, tc.y, got, tc.want)
		}
	}

	// Above ground the view spans 7 -> 0 and never reaches underground.
	if g.canSee(game.Position{X: 32358, Y: 32224, Z: 8}) {
		t.Errorf("a surface player must not see z=8")
	}
	// Underground it is +/- 2 floors, and the window shifts diagonally per floor.
	viewer.Pos = game.Position{X: 32358, Y: 32224, Z: 10}
	if g.canSee(game.Position{X: 32358, Y: 32224, Z: 13}) {
		t.Errorf("z=13 is 3 floors below z=10, past MAP_LAYER_VIEW_LIMIT")
	}
	if !g.canSee(game.Position{X: 32358, Y: 32224, Z: 12}) {
		t.Errorf("z=12 is 2 floors below z=10 and must be visible")
	}
}

// A spectator who cannot see the creature is recorded as -1 so the broadcast skips
// them entirely, mirroring the `if (stackpos != -1)` guard at src/map/map.cpp:783.
func TestCaptureStackPositionsMarksUnseenSpectators(t *testing.T) {
	viewer := &game.Player{ID: 1, Name: "Viewer", GroupID: 1}
	g, world := stackposSetup(t, viewer)

	pos := game.Position{X: 100, Y: 100, Z: 7}
	world.Map.SetTile(pos, &game.Tile{Ground: &game.Item{ID: 1}})

	viewer.SetPosition(pos)
	world.AddPlayer(viewer, g)

	ghost := &game.Player{Name: "Ghost", GroupID: 1, Ghost: true}
	ghost.SetPosition(pos)
	world.AddPlayer(ghost, &GameProtocol{player: ghost, deps: g.deps})

	got := CaptureStackPositions(world, pos, ghost)
	if v, ok := got[viewer.ID]; !ok || v != -1 {
		t.Errorf("viewer entry for a ghost mover = %d (present=%v), want -1", v, ok)
	}
}
