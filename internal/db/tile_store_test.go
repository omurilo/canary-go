package db

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/io/propstream"
	"github.com/opentibiabr/canary-go/internal/items"
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
	bag := &game.Item{ID: 1987, Contents: []*game.Item{
		{ID: 3031, Count: count, Attr: &game.ItemAttributes{HasCount: true}},
		{ID: 1650},
	}}
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
	if len(bagBack.Contents) != 2 {
		t.Fatalf("bag came back with %d items, want 2", len(bagBack.Contents))
	}
	if bagBack.Contents[0].ID != 1650 || bagBack.Contents[1].ID != 3031 {
		t.Errorf("bag contents = %d,%d, want 1650,3031 (reversed)",
			bagBack.Contents[0].ID, bagBack.Contents[1].ID)
	}
	coin := bagBack.Contents[1]
	if coin.Count != count {
		t.Errorf("the coin stack count did not survive: got %d, want %d", coin.Count, count)
	}
	if bagBack.Contents[0].Parent != bagBack {
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
		{ID: 407, Contents: []*game.Item{{ID: 3031, Count: 1}}},
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
