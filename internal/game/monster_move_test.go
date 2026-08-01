package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/items"
)

// getDistanceStep is 500 lines of branch. These tests pin the parts whose
// behaviour differs from "step directly away", which is what the port had.

func distanceMonster(w *World, pos Position, targetDistance int) *Monster {
	mt := &creatures.MonsterType{Name: "Archer"}
	mt.Flags.Hostile = true
	mt.Flags.TargetDistance = targetDistance
	m := aiMonster(w, pos, mt)
	m.SpawnPosition = pos
	return m
}

// wall makes a tile unwalkable by dropping a solid item on it.
func blockTile(w *World, pos Position) {
	t := w.Map.GetTile(pos)
	if t == nil {
		t = &Tile{Ground: &Item{ID: 1}}
		w.Map.SetTile(pos, t)
	}
	t.Items = append(t.Items, &Item{ID: 2})
}

// Already at the fighting distance, getDistanceStep reports success without
// choosing anything — the dance step handles moving from there. Returning a
// direction here would make a distance monster drift every tick.
func TestDistanceStepAtTargetDistanceKeepsPosition(t *testing.T) {
	w := aiWorld(t)
	m := distanceMonster(w, Position{X: 110, Y: 105, Z: 7}, 4)

	got, ok := m.GetDistanceStep(w, Position{X: 106, Y: 105, Z: 7}, DirSouth, false)
	if !ok {
		t.Fatal("at exactly targetDistance the step must succeed")
	}
	if got != DirSouth {
		t.Errorf("direction = %v, want the incoming %v left untouched", got, DirSouth)
	}
}

// Further away than it wants to be, it declines and lets A* close the gap.
func TestDistanceStepDefersToPathfindingWhenFar(t *testing.T) {
	w := aiWorld(t)
	m := distanceMonster(w, Position{X: 115, Y: 105, Z: 7}, 4)

	if _, ok := m.GetDistanceStep(w, Position{X: 105, Y: 105, Z: 7}, DirNorth, false); ok {
		t.Error("beyond targetDistance getDistanceStep must decline so A* runs")
	}
}

// Too close, it backs off along the axis the target is on.
func TestDistanceStepBacksAwayWhenTooClose(t *testing.T) {
	w := aiWorld(t)
	m := distanceMonster(w, Position{X: 110, Y: 105, Z: 7}, 4)

	// Target due west and adjacent: the escape is east.
	got, ok := m.GetDistanceStep(w, Position{X: 109, Y: 105, Z: 7}, DirNorth, false)
	if !ok {
		t.Fatal("too close: a step must be chosen")
	}
	if got != DirEast {
		t.Errorf("direction = %v, want %v (straight away from the target)", got, DirEast)
	}
}

// With the straight retreat walled off, it sidesteps rather than giving up. The
// offset gate means only the side that does not close the distance is allowed.
func TestDistanceStepSidestepsWhenRetreatIsBlocked(t *testing.T) {
	w := aiWorld(t)
	m := distanceMonster(w, Position{X: 110, Y: 105, Z: 7}, 4)
	blockTile(w, Position{X: 111, Y: 105, Z: 7}) // east, the straight retreat

	got, ok := m.GetDistanceStep(w, Position{X: 109, Y: 105, Z: 7}, DirNorth, false)
	if !ok {
		t.Fatal("a sidestep must be found")
	}
	if got != DirNorth && got != DirSouth {
		t.Errorf("direction = %v, want a north/south sidestep", got)
	}
}

// Boxed in on three sides and not fleeing, it must not step towards the target.
// A fleeing monster may — that is the branch that lets a cornered one squeeze
// past instead of standing there.
func TestOnlyAFleeingMonsterStepsTowardsTheTarget(t *testing.T) {
	build := func() (*World, *Monster) {
		w := aiWorld(t)
		m := distanceMonster(w, Position{X: 110, Y: 105, Z: 7}, 4)
		blockTile(w, Position{X: 111, Y: 105, Z: 7}) // east
		blockTile(w, Position{X: 110, Y: 104, Z: 7}) // north
		blockTile(w, Position{X: 110, Y: 106, Z: 7}) // south
		blockTile(w, Position{X: 111, Y: 104, Z: 7}) // NE
		blockTile(w, Position{X: 111, Y: 106, Z: 7}) // SE
		return w, m
	}
	targetPos := Position{X: 109, Y: 105, Z: 7}

	w, m := build()
	if got, ok := m.GetDistanceStep(w, targetPos, DirNorth, false); ok && got == DirWest {
		t.Error("a non-fleeing monster stepped into its target")
	}

	w, m = build()
	got, ok := m.GetDistanceStep(w, targetPos, DirNorth, true)
	if !ok || got != DirWest {
		t.Errorf("fleeing and cornered: got %v ok=%v, want %v true", got, ok, DirWest)
	}
}

// A target standing on top of the monster has no direction to flee from, so
// upstream falls back to a random step rather than dividing by zero.
func TestDistanceStepWithTargetOnTopTakesARandomStep(t *testing.T) {
	w := aiWorld(t)
	m := distanceMonster(w, Position{X: 110, Y: 105, Z: 7}, 4)

	if _, ok := m.GetDistanceStep(w, Position{X: 110, Y: 105, Z: 7}, DirNorth, false); !ok {
		t.Error("a target on the monster's own tile must still produce a step")
	}
}

// canWalkTo is three checks, and dropping any of them changes behaviour. The
// creature check is the one the port was missing.
func TestCanWalkToRejectsOccupiedTiles(t *testing.T) {
	w := aiWorld(t)
	m := distanceMonster(w, Position{X: 105, Y: 105, Z: 7}, 1)

	if !m.CanWalkTo(w, m.GetPosition(), DirEast) {
		t.Fatal("an empty walkable tile must be reachable")
	}
	other := aiMonster(w, Position{X: 106, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Blocker"})
	other.ID = 99
	if m.CanWalkTo(w, m.GetPosition(), DirEast) {
		t.Error("a tile with another creature on it must not be walkable")
	}
}

func TestCanWalkToRespectsSpawnRange(t *testing.T) {
	w := aiWorld(t)
	m := distanceMonster(w, Position{X: 105, Y: 105, Z: 7}, 1)
	m.SpawnPosition = Position{X: 105 - monsterDespawnRadius, Y: 105, Z: 7}

	// One more step east leaves the despawn radius.
	if m.CanWalkTo(w, m.GetPosition(), DirEast) {
		t.Error("a step out of the spawn radius must be refused")
	}
}

// getRandomStep never picks a diagonal — a wandering monster moves on the
// cardinals only.
func TestRandomStepIsCardinalOnly(t *testing.T) {
	w := aiWorld(t)
	m := distanceMonster(w, Position{X: 105, Y: 105, Z: 7}, 1)

	for i := 0; i < 50; i++ {
		dir, ok := m.GetRandomStep(w, m.GetPosition())
		if !ok {
			t.Fatal("open ground: a random step must exist")
		}
		switch dir {
		case DirNorth, DirSouth, DirEast, DirWest:
		default:
			t.Fatalf("random step returned the diagonal %v", dir)
		}
	}
}

// The push targets are sideways relative to the movement, never ahead of it —
// pushing an item into the tile you are about to enter would be pointless.
func TestPushItemLocationOptionsAreSideways(t *testing.T) {
	cases := map[Direction][][2]int{
		DirNorth: {{-1, 0}, {1, 0}},
		DirSouth: {{-1, 0}, {1, 0}},
		DirEast:  {{0, -1}, {0, 1}},
		DirWest:  {{0, -1}, {0, 1}},
		DirNE:    {{0, -1}, {1, 0}},
		DirSW:    {{0, 1}, {-1, 0}},
	}
	for dir, want := range cases {
		got := getPushItemLocationOptions(dir)
		if len(got) != len(want) {
			t.Errorf("%v: got %v, want %v", dir, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%v: got %v, want %v", dir, got, want)
				break
			}
		}
	}
}

// pushItems shoves a blocking movable item aside so the monster can pass.
func TestPushItemsMovesABlockingItemAside(t *testing.T) {
	w := aiWorld(t)
	w.Items = items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 2, Name: "wall", BlockSolid: true, BlockProjectile: true},
		&items.ItemType{ID: 3, Name: "crate", BlockSolid: true, Movable: true},
	)
	mt := &creatures.MonsterType{Name: "Bruiser"}
	mt.Flags.CanPushItems = true
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)

	blocked := Position{X: 106, Y: 105, Z: 7}
	crate := &Item{ID: 3}
	w.Map.GetTile(blocked).Items = []*Item{crate}

	m.PushItems(w, w.Map.GetTile(blocked), blocked, DirEast)

	if len(w.Map.GetTile(blocked).Items) != 0 {
		t.Fatal("the crate is still in the way")
	}
	// Pushing east puts it north or south of where it was, never further east.
	north := w.Map.GetTile(Position{X: 106, Y: 104, Z: 7})
	south := w.Map.GetTile(Position{X: 106, Y: 106, Z: 7})
	if len(north.Items)+len(south.Items) != 1 {
		t.Errorf("the crate did not land on a sideways tile")
	}
}

// A monster that cannot push items leaves them where they are.
func TestPushItemsIsSkippedWithoutTheFlag(t *testing.T) {
	w := aiWorld(t)
	mt := &creatures.MonsterType{Name: "Polite"}
	mt.Flags.Hostile = true
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)

	blocked := Position{X: 106, Y: 105, Z: 7}
	w.Map.GetTile(blocked).Items = []*Item{{ID: 2}}

	if _, ok := m.GetNextStep(w); ok {
		// A step may or may not be chosen; what matters is the item is untouched.
		_ = ok
	}
	if len(w.Map.GetTile(blocked).Items) != 1 {
		t.Error("a monster without canPushItems moved an item")
	}
}

// House tiles are exempt: a monster must not rearrange a player's furniture.
func TestPushItemsSkipsHouseTiles(t *testing.T) {
	w := aiWorld(t)
	w.Items = items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 3, Name: "crate", BlockSolid: true, Movable: true},
	)
	mt := &creatures.MonsterType{Name: "Bruiser"}
	mt.Flags.CanPushItems = true
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)

	blocked := Position{X: 106, Y: 105, Z: 7}
	tile := w.Map.GetTile(blocked)
	tile.HouseID = 7
	tile.Items = []*Item{{ID: 3}}

	m.PushItems(w, tile, blocked, DirEast)
	if len(tile.Items) != 1 {
		t.Error("furniture inside a house was pushed")
	}
}

// An idle or dead monster does not move at all.
func TestGetNextStepRefusesWhenIdleOrDead(t *testing.T) {
	w := aiWorld(t)
	m := distanceMonster(w, Position{X: 105, Y: 105, Z: 7}, 1)

	m.Idle = true
	if _, ok := m.GetNextStep(w); ok {
		t.Error("an idle monster must not walk")
	}
	m.Idle = false
	m.Health = 0
	if _, ok := m.GetNextStep(w); ok {
		t.Error("a dead monster must not walk")
	}
}
