package luaengine

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
)

// eqWorld: one house, one tile in it, a player standing on that tile.
func eqWorld(t *testing.T) (*Engine, *game.House) {
	t.Helper()
	w := game.NewWorld()
	w.Items = items.NewCatalog(&items.ItemType{ID: 1, Name: "ground"})

	pos := game.Position{X: 100, Y: 100, Z: 7}
	w.Map.SetTile(pos, &game.Tile{Ground: &game.Item{ID: 1}})

	h := &game.House{ID: 42, Name: "Test House", HouseTiles: []game.Position{pos}}
	w.RegisterHouse(h)
	w.Map.GetTile(pos).HouseID = h.ID

	e := New(w, nil)
	t.Cleanup(e.Close)
	return e, h
}

// The exercise dummy bug, reduced. Two independent lookups of one house have to
// compare equal; gopher-lua compares the userdata boxes unless __eq says
// otherwise, so this was false and the script's "You must be inside the house"
// branch fired no matter where the player stood.
func TestSameHouseComparesEqual(t *testing.T) {
	e, _ := eqWorld(t)

	err := e.L.DoString(`
		local a = Tile(Position(100, 100, 7)):getHouse()
		local b = Tile(Position(100, 100, 7)):getHouse()
		assert(a ~= nil, "the tile must have a house")
		assert(a == b, "two handles on the same house must be equal")
		assert(not (a ~= b), "and must not be unequal")
	`)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

// Different objects still differ — an __eq that answers true for everything
// would hide the bug rather than fix it.
func TestDifferentHousesCompareUnequal(t *testing.T) {
	e, _ := eqWorld(t)
	other := game.Position{X: 200, Y: 200, Z: 7}
	e.world.Map.SetTile(other, &game.Tile{Ground: &game.Item{ID: 1}})
	h2 := &game.House{ID: 43, Name: "Other House", HouseTiles: []game.Position{other}}
	e.world.RegisterHouse(h2)
	e.world.Map.GetTile(other).HouseID = h2.ID

	err := e.L.DoString(`
		local a = Tile(Position(100, 100, 7)):getHouse()
		local b = Tile(Position(200, 200, 7)):getHouse()
		assert(a ~= nil and b ~= nil, "both tiles must have a house")
		assert(a ~= b, "two different houses must not be equal")
	`)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

// Item handles compare by the item, not by the position they were fetched
// through — luaItem carries both, and comparing the struct would make the same
// item fetched two ways look like two items.
func TestSameItemComparesEqual(t *testing.T) {
	_, _ = eqWorld(t)
	item := &game.Item{ID: 1234}

	if !sameUserdataTarget(luaItem{item: item, pos: game.Position{X: 1}},
		luaItem{item: item, pos: game.Position{X: 2}}) {
		t.Errorf("one item fetched at two positions must still be the same item")
	}
	if sameUserdataTarget(luaItem{item: item}, luaItem{item: &game.Item{ID: 1234}}) {
		t.Errorf("two distinct items with the same id must not be equal")
	}
}

// Position keeps its own value-based __eq: `pos == otherPos` has to be true for
// equal coordinates, which identity comparison would break.
func TestPositionKeepsValueEquality(t *testing.T) {
	e, _ := eqWorld(t)
	err := e.L.DoString(`
		assert(Position(10, 20, 7) == Position(10, 20, 7), "equal coordinates must compare equal")
		assert(Position(10, 20, 7) ~= Position(10, 21, 7), "different coordinates must not")
	`)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

// Mismatched types and nils must answer false rather than panic — reflect
// comparison on an uncomparable type would take the whole server down from a
// datapack script.
func TestUserdataCompareIsTotal(t *testing.T) {
	cases := []struct {
		a, b any
		want bool
	}{
		{nil, nil, true},
		{nil, &game.Item{}, false},
		{&game.Item{ID: 1}, &game.House{ID: 1}, false},
		{[]int{1}, []int{1}, false}, // uncomparable: false, not a panic
	}
	for _, c := range cases {
		if got := sameUserdataTarget(c.a, c.b); got != c.want {
			t.Errorf("sameUserdataTarget(%T, %T) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
