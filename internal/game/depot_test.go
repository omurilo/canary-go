package game

import "testing"

// Player::getDepotChest (src/creatures/players/player.cpp), branch for branch. The
// ids are only contiguous up to box XVII; XVIII, XIX and XX are in unrelated
// ranges, so deriving them arithmetically produced 22814/22815/22816 — and
// 22815/22816 are not item ids at all. An unknown id inside the locker frame makes
// the client read a zero appearance and lose the rest of the packet.
func TestGetDepotChestIDs(t *testing.T) {
	dm := NewPlayerDepotManager(&Player{})

	tests := []struct {
		depotID uint16
		want    uint16
	}{
		{1, 22797},  // ITEM_DEPOT_I
		{17, 22813}, // ITEM_DEPOT_XVII — last of the contiguous run
		{18, 31915}, // ITEM_DEPOT_XVIII — the sequence breaks here
		{19, 39723}, // ITEM_DEPOT_XIX
		{20, 39724}, // ITEM_DEPOT_XX
		{21, 39724}, // past XX C++ hands out XX as well
		// 0 falls to the same last branch in C++; it must NOT become ITEM_DEPOT_NULL
		// (22796), which is an internal-use id and not a container.
		{0, 39724},
	}
	for _, tc := range tests {
		got := dm.GetDepotChest(tc.depotID, true)
		if got == nil {
			t.Fatalf("GetDepotChest(%d) = nil", tc.depotID)
		}
		if got.ID != tc.want {
			t.Errorf("GetDepotChest(%d).ID = %d, want %d", tc.depotID, got.ID, tc.want)
		}
	}

	// The arithmetic shortcut this replaces, spelled out so it cannot creep back.
	for _, bad := range []uint16{22814, 22815, 22816, ItemDepotNull} {
		for id := uint16(0); id <= 21; id++ {
			if chest := dm.GetDepotChest(id, true); chest.ID == bad {
				t.Errorf("GetDepotChest(%d) produced %d, which is not a depot box id", id, bad)
			}
		}
	}
}

// autoCreate=false must not conjure a box, and repeated calls must return the same
// one — the locker rebuilds its parent links from the cached boxes.
func TestGetDepotChestCaching(t *testing.T) {
	dm := NewPlayerDepotManager(&Player{})
	if got := dm.GetDepotChest(3, false); got != nil {
		t.Errorf("GetDepotChest(3, false) on an empty manager = %v, want nil", got)
	}
	first := dm.GetDepotChest(3, true)
	if second := dm.GetDepotChest(3, true); second != first {
		t.Errorf("GetDepotChest(3) returned a different box on the second call")
	}
	if got := dm.GetDepotChest(3, false); got != first {
		t.Errorf("GetDepotChest(3, false) did not return the cached box")
	}
}

// The locker holds exactly what Player::getDepotLocker puts in it, in the order the
// client sees, with the capacities the C++ constructs them with.
func TestGetDepotLockerContents(t *testing.T) {
	dm := NewPlayerDepotManager(&Player{})
	locker := dm.GetDepotLocker(1)

	if locker.ID != ItemLocker {
		t.Errorf("locker id = %d, want %d", locker.ID, ItemLocker)
	}
	// DepotLocker(ITEM_LOCKER, 4): depot container, stash, inbox, market.
	if locker.Container == nil || locker.Container.MaxSize != 4 {
		t.Errorf("locker capacity = %d, want 4", locker.Container.MaxSize)
	}
	wantOrder := []uint16{ItemDepot, ItemStash, ItemInbox, ItemMarket}
	if len(locker.Container.Contents) != len(wantOrder) {
		t.Fatalf("locker holds %d things, want %d", len(locker.Container.Contents), len(wantOrder))
	}
	for i, want := range wantOrder {
		if locker.Container.Contents[i].ID != want {
			t.Errorf("locker slot %d = %d, want %d", i, locker.Container.Contents[i].ID, want)
		}
	}

	// CreateItemAsContainer(ITEM_DEPOT, DEPOT_BOXES): one box per configured slot,
	// numbered 1..n, and the container's capacity is that same count.
	depot := locker.Container.Contents[0]
	boxes := depotBoxes()
	if depot.Container == nil || depot.Container.MaxSize != boxes {
		t.Errorf("depot container capacity = %d, want %d", depot.Container.MaxSize, boxes)
	}
	if len(depot.Container.Contents) != int(boxes) {
		t.Fatalf("depot container holds %d boxes, want %d", len(depot.Container.Contents), boxes)
	}
	for i, box := range depot.Container.Contents {
		want := dm.GetDepotChest(uint16(i+1), true).ID
		if box.ID != want {
			t.Errorf("box slot %d = %d, want %d (box %d)", i, box.ID, want, i+1)
		}
		if box.Container == nil || box.Container.Parent != depot {
			t.Errorf("box %d has no parent link to the depot container", i+1)
		}
	}
}
