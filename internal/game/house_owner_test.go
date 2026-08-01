package game

import "testing"

// The bug behind "/owner does not work": SetOwner assigned h.OwnerID and nothing
// else. The command reported success, the house answered IsOwner correctly for the
// rest of the session, and the change was gone after a restart because no row was
// ever written.
func TestSetOwnerPersists(t *testing.T) {
	w := NewWorld()
	h := &House{ID: 5}
	w.RegisterHouse(h)

	var gotHouse uint32
	var gotOwner uint32
	calls := 0
	w.OnHouseOwnerChange = func(house *House, ownerID uint32) {
		calls++
		gotHouse, gotOwner = house.ID, ownerID
	}
	w.LookupPlayerAccount = func(guid uint32) (string, uint32, bool) {
		return "Owner", 42, true
	}

	h.SetOwner(w, 77, true, nil)

	if calls != 1 {
		t.Fatalf("the ownership change must be persisted exactly once, got %d writes", calls)
	}
	if gotHouse != 5 || gotOwner != 77 {
		t.Errorf("persisted house %d owner %d, want 5/77", gotHouse, gotOwner)
	}
	if !h.IsOwner(77) {
		t.Errorf("in-memory ownership must be set too")
	}
	if h.OwnerName != "Owner" || h.OwnerAccountID != 42 {
		t.Errorf("owner name/account not resolved: %q/%d", h.OwnerName, h.OwnerAccountID)
	}
	if h.State != HouseStateRented {
		t.Errorf("state = %d, want Rented(%d)", h.State, HouseStateRented)
	}
}

// updateDatabase=false is the in-memory-only form C++ uses on the load path; it
// must still apply the change, just without writing.
func TestSetOwnerWithoutDatabase(t *testing.T) {
	w := NewWorld()
	h := &House{ID: 5}
	w.RegisterHouse(h)
	w.OnHouseOwnerChange = func(*House, uint32) { t.Error("must not write when updateDatabase is false") }
	w.LookupPlayerAccount = func(uint32) (string, uint32, bool) { return "Owner", 42, true }

	h.SetOwner(w, 77, false, nil)
	if !h.IsOwner(77) {
		t.Errorf("the owner must still be applied in memory")
	}
}

// C++ aborts the assignment when the players row is missing, so a typo'd guid
// leaves the house unowned instead of owned by an id that belongs to nobody.
func TestSetOwnerRejectsUnknownGuid(t *testing.T) {
	w := NewWorld()
	h := &House{ID: 5}
	w.RegisterHouse(h)
	w.LookupPlayerAccount = func(uint32) (string, uint32, bool) { return "", 0, false }

	h.SetOwner(w, 999, false, nil)
	if h.IsOwner(999) {
		t.Errorf("an unknown guid must not become the owner")
	}
}

// Re-setting the same owner is a no-op once loaded — otherwise every call would
// run the transfer path and wipe the guest list of the owner's own house.
func TestSetOwnerIsIdempotent(t *testing.T) {
	w := NewWorld()
	h := &House{ID: 5}
	w.RegisterHouse(h)
	writes := 0
	w.OnHouseOwnerChange = func(*House, uint32) { writes++ }
	w.LookupPlayerAccount = func(uint32) (string, uint32, bool) { return "Owner", 42, true }

	h.SetOwner(w, 77, true, nil)
	h.AddGuest("Friend")
	h.SetOwner(w, 77, true, nil)

	if writes != 1 {
		t.Errorf("the same owner must not be written twice, got %d writes", writes)
	}
	if !h.IsGuest("Friend") {
		t.Errorf("re-setting the same owner must not clear the guest list")
	}
}

// Handing the house to someone else runs the transfer path: the previous owner's
// access lists go, and the new owner replaces them.
func TestSetOwnerTransferClearsTheOldAccessLists(t *testing.T) {
	w := NewWorld()
	h := &House{ID: 5}
	w.RegisterHouse(h)
	w.LookupPlayerAccount = func(guid uint32) (string, uint32, bool) {
		if guid == 78 {
			return "Second", 43, true
		}
		return "First", 42, true
	}

	h.SetOwner(w, 77, false, nil)
	h.AddGuest("FriendOfFirst")
	h.AddSubOwner("DeputyOfFirst")

	h.SetOwner(w, 78, false, nil)

	if !h.IsOwner(78) {
		t.Fatalf("the house must belong to the new owner")
	}
	if h.IsGuest("FriendOfFirst") || h.IsSubOwner("DeputyOfFirst") {
		t.Errorf("the previous owner's access lists must not survive the transfer")
	}
	if h.OwnerName != "Second" {
		t.Errorf("owner name = %q, want Second", h.OwnerName)
	}
}

// Looking at a door is the only way in game to find out who owns a house — there
// is no /houseinfo command in the datapack. The description was never built, so
// the door said nothing at all.
func TestDoorDescriptionNamesTheOwner(t *testing.T) {
	w := NewWorld()
	pos := Position{X: 100, Y: 100, Z: 7}
	doorID := uint8(1)
	door := &Item{ID: 1209, Attr: &ItemAttributes{HouseDoorID: &doorID}}
	w.Map.SetTile(pos, &Tile{Ground: &Item{ID: 1}, Items: []*Item{door}})

	h := &House{ID: 5, Name: "Harbour Place 7", HouseTiles: []Position{pos}}
	w.RegisterHouse(h)
	w.LookupPlayerAccount = func(uint32) (string, uint32, bool) { return "Gm Test", 42, true }

	h.UpdateDoorDescription(w)
	if door.Attr.Description == nil {
		t.Fatalf("an unowned house must still describe itself")
	}
	if got := *door.Attr.Description; got != "It belongs to house 'Harbour Place 7'. Nobody owns this house." {
		t.Errorf("unowned door reads %q", got)
	}

	h.SetOwner(w, 77, false, nil)
	if got := *door.Attr.Description; got != "It belongs to house 'Harbour Place 7'. Gm Test owns this house." {
		t.Errorf("owned door reads %q", got)
	}
}

func TestFormatNumber(t *testing.T) {
	for in, want := range map[uint64]string{0: "0", 42: "42", 999: "999", 1000: "1,000", 12345: "12,345", 1234567: "1,234,567"} {
		if got := formatNumber(in); got != want {
			t.Errorf("formatNumber(%d) = %q, want %q", in, got, want)
		}
	}
}
