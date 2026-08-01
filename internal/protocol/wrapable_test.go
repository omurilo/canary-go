package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

func wrapCatalog() *items.Catalog {
	return items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 1650, Name: "table", WrapableTo: game.ItemDecorationKit, Movable: true},
		&items.ItemType{ID: game.ItemDecorationKit, Name: "decoration kit", WrapKit: true},
		&items.ItemType{ID: 25879, Name: "health cask", WrapableTo: game.ItemDecorationKit},
		&items.ItemType{ID: itemFilledBathTube, Name: "filled bath tub", WrapableTo: game.ItemDecorationKit},
		// Blocking, always-on-top, no height: the shape that takes an auto carpet.
		&items.ItemType{ID: 900, Name: "carpet base", BlockSolid: true, AlwaysOnTopOrder: 1},
	)
}

func wrapSetup(t *testing.T) (*GameProtocol, *game.World, *game.Player, game.Position) {
	t.Helper()
	w := game.NewWorld()
	w.Items = wrapCatalog()
	pos := game.Position{X: 100, Y: 100, Z: 7}
	w.Map.SetTile(pos, &game.Tile{Ground: &game.Item{ID: 1}, Flags: game.TileFlagProtectionZone, HouseID: 5})
	w.Houses = map[uint32]*game.House{5: {ID: 5}}
	w.Houses[5].SetOwner(77)

	p := &game.Player{Name: "Owner", DBID: 77, GroupID: 1}
	p.SetPosition(pos)
	g := &GameProtocol{player: p, deps: &Deps{World: w, Items: w.Items}}
	p.Session = g
	w.AddPlayer(p, g)
	return g, w, p, pos
}

// The point of the whole feature: a kit bought in the store becomes the item it
// was wrapped from. parseWrapableItem used to read its arguments and discard them.
func TestUnwrapRestoresTheItem(t *testing.T) {
	g, w, _, pos := wrapSetup(t)
	tile := w.Map.GetTile(pos)

	kit := &game.Item{ID: game.ItemDecorationKit}
	kit.SetCustomAttribute("unWrapId", int64(1650))
	tile.Items = append(tile.Items, kit)

	g.unwrapItem(pos, kit)

	if kit.ID != 1650 {
		t.Fatalf("kit became %d, want the table (1650)", kit.ID)
	}
	if _, ok := kit.GetCustomAttribute("unWrapId"); ok {
		t.Errorf("unWrapId must be cleared after unwrapping")
	}
	if kit.Attr != nil && kit.Attr.Description != nil {
		t.Errorf("the store description must be cleared: %q", *kit.Attr.Description)
	}
}

// A kit with no unWrapId has nothing to become; it must be refused rather than
// transformed into item id 0.
func TestUnwrapWithoutTheAttributeIsRefused(t *testing.T) {
	g, w, _, pos := wrapSetup(t)
	tile := w.Map.GetTile(pos)
	kit := &game.Item{ID: game.ItemDecorationKit}
	tile.Items = append(tile.Items, kit)

	g.unwrapItem(pos, kit)
	if kit.ID != game.ItemDecorationKit {
		t.Errorf("a kit with no unWrapId must stay a kit, became %d", kit.ID)
	}
}

// Wrapping round-trips: the id comes back, and so do the attributes C++ carries
// across — the amount, the store stamp, and a cask's hidden charges.
func TestWrapUnwrapRoundTrip(t *testing.T) {
	g, _, _, pos := wrapSetup(t)
	stamp := int64(1234567)

	table := &game.Item{ID: 1650, Count: 1, Attr: &game.ItemAttributes{StoreTimestamp: &stamp}}
	g.wrapItem(pos, table, g.deps.Items.Get(1650))

	if table.ID != game.ItemDecorationKit {
		t.Fatalf("wrapping produced %d, want a decoration kit", table.ID)
	}
	if raw, ok := table.GetCustomAttribute("unWrapId"); !ok || customAttrUint16(raw) != 1650 {
		t.Fatalf("unWrapId = %v, want 1650", raw)
	}
	if table.Attr.Description == nil {
		t.Errorf("a wrapped item must carry the store description")
	}
	if table.Attr.StoreTimestamp == nil || *table.Attr.StoreTimestamp != stamp {
		t.Errorf("the store stamp must survive wrapping")
	}

	g.unwrapItem(pos, table)
	if table.ID != 1650 {
		t.Errorf("the round trip lost the item: %d", table.ID)
	}
	if table.Attr.StoreTimestamp == nil || *table.Attr.StoreTimestamp != stamp {
		t.Errorf("the store stamp must survive unwrapping too")
	}
}

// A cask hides its charges in the DATE slot across the wrap. Without that it comes
// back empty, which is the kind of loss a player notices and cannot undo.
func TestCaskChargesSurviveTheWrap(t *testing.T) {
	g, _, _, pos := wrapSetup(t)
	cask := &game.Item{ID: 25879, Count: 37}

	g.wrapItem(pos, cask, g.deps.Items.Get(25879))
	if cask.Attr == nil || cask.Attr.WrittenDate == nil || *cask.Attr.WrittenDate != 37 {
		t.Fatalf("the cask charges were not hidden in the kit: %+v", cask.Attr)
	}

	g.unwrapItem(pos, cask)
	if cask.ID != 25879 {
		t.Fatalf("cask became %d", cask.ID)
	}
	if cask.Count != 37 {
		t.Errorf("cask came back with %d charges, want 37", cask.Count)
	}
}

func TestIsCaskItem(t *testing.T) {
	for _, id := range []uint16{25879, 25883, 25889, 25893, 25899, 25902} {
		if !isCaskItem(id) {
			t.Errorf("%d is inside a cask range", id)
		}
	}
	for _, id := range []uint16{25878, 25884, 25888, 25894, 25898, 25903, 1650} {
		if isCaskItem(id) {
			t.Errorf("%d is outside every cask range", id)
		}
	}
}

// canReceiveAutoCarpet is blocking AND always-on-top AND no height — all three, so
// an ordinary wall or a plain carpet does not qualify.
func TestCanReceiveAutoCarpet(t *testing.T) {
	cat := wrapCatalog()
	if !canReceiveAutoCarpet(&game.Item{ID: 900}, cat) {
		t.Errorf("a blocking always-on-top item with no height must qualify")
	}
	if canReceiveAutoCarpet(&game.Item{ID: 1650}, cat) {
		t.Errorf("a plain table must not qualify")
	}
}

// C++ sends the kit's unWrapId in the wrap-kit field; this sent a constant 0, so
// the client was told every kit unwraps into nothing. It is NOT why the unwrap did
// not work — the option did appear in the menu — but a field that lies about an
// item is worth correcting on its own.
func TestAddItemSendsTheRealUnwrapID(t *testing.T) {
	g, _, _, _ := wrapSetup(t)

	kit := &game.Item{ID: game.ItemDecorationKit}
	kit.SetCustomAttribute("unWrapId", int64(1650))

	w := netmsg.NewWriter()
	g.addItem(w, kit)
	b := w.Bytes()

	// id u16 then the wrap-kit u16.
	if len(b) < 4 {
		t.Fatalf("packet too short: % X", b)
	}
	gotID := uint16(b[0]) | uint16(b[1])<<8
	gotUnwrap := uint16(b[2]) | uint16(b[3])<<8
	if gotID != game.ItemDecorationKit {
		t.Fatalf("item id = %d, want the kit", gotID)
	}
	if gotUnwrap != 1650 {
		t.Errorf("unwrap id = %d, want 1650 — with 0 the client hides the unwrap option", gotUnwrap)
	}

	// A kit with no attribute still writes the field, as zero, so the frame keeps
	// its length.
	bare := &game.Item{ID: game.ItemDecorationKit}
	w2 := netmsg.NewWriter()
	g.addItem(w2, bare)
	if len(w2.Bytes()) != len(b) {
		t.Errorf("the wrap-kit field must always be present: %d vs %d bytes", len(w2.Bytes()), len(b))
	}
}

// House::getHouseAccessLevel grants HOUSE_OWNER to a staff member carrying
// CanEditHouses, not only to the registered owner. Checking ownership alone stopped
// a gamemaster from decorating in someone else's building, which upstream allows.
func TestStaffCountAsHouseOwner(t *testing.T) {
	house := &game.House{ID: 5}
	house.SetOwner(77)

	owner := &game.Player{Name: "Owner", DBID: 77, GroupID: 1}
	stranger := &game.Player{Name: "Stranger", DBID: 78, GroupID: 1}
	staff := &game.Player{Name: "God", DBID: 79, GroupID: 5}

	if !houseAccessIsOwner(house, owner) {
		t.Errorf("the registered owner must have owner access")
	}
	if houseAccessIsOwner(house, stranger) {
		t.Errorf("a stranger must not have owner access")
	}
	if !houseAccessIsOwner(house, staff) {
		t.Errorf("staff must have owner access to any house")
	}
	if houseAccessIsOwner(nil, owner) || houseAccessIsOwner(house, nil) {
		t.Errorf("nil arguments must not grant access")
	}
}

// stackPosOfItem must count creatures the way the client does: only the ones this
// viewer can see, between the top items and the down items. It counted
// len(tile.Creatures), so the value drifted as anything walked over the tile — a
// session log shows the same door reported at stackpos 2, then 3, then 9, while the
// client had it at 7 throughout, so every door transform updated the wrong slot.
func TestStackPosOfItemCountsOnlyVisibleCreatures(t *testing.T) {
	g, w, viewer, pos := wrapSetup(t)
	tile := w.Map.GetTile(pos)

	// A down item (not always-on-top) sits after the creatures.
	door := &game.Item{ID: 1650}
	tile.Items = append(tile.Items, door)

	// ground(1) + the viewer, who stands on this tile and counts as a creature = 2.
	if got := g.stackPosOfItem(pos, door); got != 2 {
		t.Fatalf("with only the viewer present the door is at %d, want 2", got)
	}

	// A visible creature pushes it down by one.
	rat := game.NewMonster(10, "Rat", nil)
	rat.SetPosition(pos)
	tile.Creatures = append(tile.Creatures, rat)
	if got := g.stackPosOfItem(pos, door); got != 3 {
		t.Errorf("with a second visible creature the door is at %d, want 3", got)
	}

	// A ghost the viewer cannot see occupies no slot in this client's stack.
	ghost := &game.Player{Name: "Ghost", DBID: 999, GroupID: 1, Ghost: true}
	ghost.SetPosition(pos)
	tile.Creatures = append(tile.Creatures, ghost)
	if got := g.stackPosOfItem(pos, door); got != 3 {
		t.Errorf("an unseen ghost must not shift the stack: got %d, want 3", got)
	}
	_ = viewer
}
