package db

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/io/propstream"
	"github.com/omurilo/canary-go/internal/items"
)

func tileStoreCatalog() *items.Catalog {
	return items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},                  // fixed: not saved
		&items.ItemType{ID: 1650, Name: "table", Movable: true}, // furniture
		&items.ItemType{ID: 1987, Name: "bag", Movable: true},   // container
		&items.ItemType{ID: 3031, Name: "gold coin", Movable: true, Stackable: true},
		&items.ItemType{ID: 1209, Name: "door", IsDoor: true}, // doors are saved
		&items.ItemType{ID: 407, Name: "wall lamp"},           // not movable: skipped
	)
}

// The whole point of the table: a tile's items must survive encode → decode byte for
// byte, including a container's nested contents, which tile_store writes inline
// (IOMapSerialize::saveItem) rather than as separate rows.
func TestTileStoreRoundTrip(t *testing.T) {
	cat := tileStoreCatalog()
	pos := game.Position{X: 1000, Y: 1001, Z: 7}

	count := uint16(37)
	actionID := uint16(4242)
	bag := &game.Item{ID: 1987, Container: &game.Container{Contents: []*game.Item{
		{ID: 3031, Count: count, Attr: &game.ItemAttributes{HasCount: true}},
		{ID: 1650},
	}}}
	tile := &game.Tile{
		Ground: &game.Item{ID: 1},
		Items: []*game.Item{
			{ID: 1},   // fixed ground-ish: filtered out
			{ID: 407}, // not movable: filtered out
			{ID: 1650, Attr: &game.ItemAttributes{ActionID: &actionID}}, // furniture with an attribute
			bag, // container with contents
		},
	}

	blob := encodeTile(tile, pos, cat)
	if blob == nil {
		t.Fatal("encodeTile returned nil for a tile with saveable items")
	}

	// Header: position then the count of saveable items (2 of the 4).
	ps := propstream.NewPropStream(blob)
	x, _ := ps.ReadUint16()
	y, _ := ps.ReadUint16()
	z, _ := ps.ReadUint8()
	if (game.Position{X: x, Y: y, Z: z}) != pos {
		t.Errorf("position round trip = %d,%d,%d, want %v", x, y, z, pos)
	}
	n, _ := ps.ReadUint32()
	if n != 2 {
		t.Fatalf("saved %d items, want 2 (the fixed ground and the wall lamp are not saved)", n)
	}

	var got []*game.Item
	for i := uint32(0); i < n; i++ {
		it, err := readItem(ps)
		if err != nil {
			t.Fatalf("readItem %d: %v", i, err)
		}
		got = append(got, it)
	}

	// C++ builds the list with push_front, so the tile's last saveable item is
	// written first: the bag comes back before the table.
	if got[0].ID != 1987 {
		t.Errorf("first decoded item = %d, want the bag (1987) — the order is reversed on save", got[0].ID)
	}
	if got[1].ID != 1650 {
		t.Errorf("second decoded item = %d, want the table (1650)", got[1].ID)
	}
	if got[1].Attr == nil || got[1].Attr.ActionID == nil || *got[1].Attr.ActionID != actionID {
		t.Errorf("the table's action id did not survive: %+v", got[1].Attr)
	}

	// The bag's contents, also reversed.
	bagBack := got[0]
	if bagBack.Container == nil || len(bagBack.Container.Contents) != 2 {
		t.Fatalf("bag came back with %d items, want 2", len(bagBack.Container.Contents))
	}
	if bagBack.Container.Contents[0].ID != 1650 || bagBack.Container.Contents[1].ID != 3031 {
		t.Errorf("bag contents = %d,%d, want 1650,3031 (reversed)",
			bagBack.Container.Contents[0].ID, bagBack.Container.Contents[1].ID)
	}
	coin := bagBack.Container.Contents[1]
	if coin.Count != count {
		t.Errorf("the coin stack count did not survive: got %d, want %d", coin.Count, count)
	}
	if bagBack.Container.Contents[0].Container != nil && bagBack.Container.Contents[0].Container.Parent != bagBack {
		t.Errorf("a restored child must point at its container")
	}
}

// A tile with nothing worth saving must produce no row at all, so tile_store does
// not grow a row per map tile of every house.
func TestTileStoreSkipsTilesWithNothingToSave(t *testing.T) {
	cat := tileStoreCatalog()
	pos := game.Position{X: 5, Y: 5, Z: 7}

	for _, tc := range []struct {
		name string
		tile *game.Tile
	}{
		{"empty tile", &game.Tile{Ground: &game.Item{ID: 1}}},
		{"only fixed decoration", &game.Tile{Ground: &game.Item{ID: 1}, Items: []*game.Item{{ID: 407}}}},
	} {
		if blob := encodeTile(tc.tile, pos, cat); blob != nil {
			t.Errorf("%s produced a %d-byte row, want none", tc.name, len(blob))
		}
	}

	// An EMPTY container is not saved, but a container with contents is — that is
	// how a bag inside fixed furniture keeps its items (Item::isSavedToHouses).
	withContents := &game.Tile{Items: []*game.Item{
		{ID: 407, Container: &game.Container{Contents: []*game.Item{{ID: 3031, Count: 1}}}},
	}}
	if encodeTile(withContents, pos, cat) == nil {
		t.Errorf("a non-empty container must be saved even when its own type is fixed")
	}
}

// savedToHouses is the filter that decides what belongs in the table.
func TestSavedToHouses(t *testing.T) {
	cat := tileStoreCatalog()
	tests := []struct {
		id   uint16
		want bool
		why  string
	}{
		{1650, true, "movable furniture"},
		{1209, true, "a door"},
		{1, false, "fixed ground"},
		{407, false, "a fixed wall lamp"},
		{60000, false, "an id the catalog does not know"},
	}
	for _, tc := range tests {
		if got := savedToHouses(&game.Item{ID: tc.id}, cat); got != tc.want {
			t.Errorf("savedToHouses(%d) = %v, want %v (%s)", tc.id, got, tc.want, tc.why)
		}
	}
	// Without a catalog nothing can be classified, so nothing is saved rather than
	// everything.
	if savedToHouses(&game.Item{ID: 1650}, nil) {
		t.Errorf("with no catalog savedToHouses must be false")
	}
}

// The house-door bug: a door is saved to tile_store on purpose, because the row
// records whether it is open or shut, but the door already exists on the tile from
// the map. The loader appended every row, so each server start left another closed
// door on the tile — the tile went on blocking, walking into it answered "Sorry,
// not possible", and only house doors were affected because only house tiles pass
// through tile_store.
func TestPlaceHouseItemDoesNotDuplicateDoors(t *testing.T) {
	cat := items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 20446, Name: "closed door", IsDoor: true, Type: items.ItemTypeDoor},
		&items.ItemType{ID: 20447, Name: "open door", IsDoor: true, Type: items.ItemTypeDoor},
		&items.ItemType{ID: 1650, Name: "table", Movable: true},
	)
	mapDoor := &game.Item{ID: 20446}
	tile := &game.Tile{Ground: &game.Item{ID: 1}, Items: []*game.Item{mapDoor}}

	// The save says the door was left open.
	if !placeHouseItem(cat, tile, &game.Item{ID: 20447}) {
		t.Fatalf("a door present on the map must be matched, not dropped")
	}
	if len(tile.Items) != 1 {
		t.Fatalf("the tile has %d items, want 1 — the door must be transformed, never appended", len(tile.Items))
	}
	if tile.Items[0] != mapDoor {
		t.Errorf("the map's own door object must be the one kept")
	}
	if mapDoor.ID != 20447 {
		t.Errorf("the door is %d, want the saved open state 20447", mapDoor.ID)
	}

	// Booting again must not grow the tile.
	placeHouseItem(cat, tile, &game.Item{ID: 20447})
	placeHouseItem(cat, tile, &game.Item{ID: 20447})
	if len(tile.Items) != 1 {
		t.Errorf("after three loads the tile has %d items, want 1", len(tile.Items))
	}
}

// Furniture a player carried in is a genuinely new object and must still be added,
// or the fix above would silently delete everyone's belongings.
func TestPlaceHouseItemAddsMovables(t *testing.T) {
	cat := items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 1650, Name: "table", Movable: true},
	)
	tile := &game.Tile{Ground: &game.Item{ID: 1}}

	for i := 0; i < 3; i++ {
		if !placeHouseItem(cat, tile, &game.Item{ID: 1650}) {
			t.Fatalf("a movable must be added")
		}
	}
	if len(tile.Items) != 3 {
		t.Errorf("three tables were saved, the tile has %d", len(tile.Items))
	}
}

// A stationary item the map no longer has is dropped, not resurrected.
func TestPlaceHouseItemDropsVanishedFurniture(t *testing.T) {
	cat := items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 2000, Name: "bookcase"},
	)
	tile := &game.Tile{Ground: &game.Item{ID: 1}}

	if placeHouseItem(cat, tile, &game.Item{ID: 2000}) {
		t.Errorf("a stationary item with nothing to match must be dropped")
	}
	if len(tile.Items) != 0 {
		t.Errorf("nothing must be added, got %d items", len(tile.Items))
	}
}
