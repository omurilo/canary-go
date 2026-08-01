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
