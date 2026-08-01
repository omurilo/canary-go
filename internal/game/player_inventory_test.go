package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/items"
)

// testCatalog builds a catalog with a stackable gold coin (stack 100), a
// non-stackable sword, and a backpack container (capacity 20).
func testCatalog() *items.Catalog {
	return items.NewCatalog(
		&items.ItemType{ID: 3031, Name: "gold coin", Stackable: true, StackSize: 100, Weight: 0},
		&items.ItemType{ID: 3264, Name: "sword", Weight: 3500},
		&items.ItemType{ID: 1988, Name: "backpack", Group: items.GroupContainer, Capacity: 20, Weight: 1800},
	)
}

func playerWithBackpack() *Player {
	p := &Player{Capacity: 100000}
	p.Inventory[ConstSlotBackpack] = &Item{ID: 1988}
	return p
}

func TestInternalAddItemSplitsStacks(t *testing.T) {
	cat := testCatalog()
	p := playerWithBackpack()

	placed, ok := p.InternalAddItem(cat, 3031, 250, 1, ConstSlotWhereever)
	if !ok {
		t.Fatalf("expected add to succeed")
	}
	// 250 gold with stack size 100 -> 100 + 100 + 50 across three items.
	if len(placed) != 3 {
		t.Fatalf("expected 3 stacks, got %d", len(placed))
	}
	want := []uint16{100, 100, 50}
	for i, it := range placed {
		if it.Count != want[i] {
			t.Errorf("stack %d: got %d want %d", i, it.Count, want[i])
		}
	}
	if got := p.GetItemTypeCount(cat, 3031, -1); got != 250 {
		t.Errorf("GetItemTypeCount = %d, want 250", got)
	}
}

func TestRemoveItemOfTypeTwoPhase(t *testing.T) {
	cat := testCatalog()
	p := playerWithBackpack()
	p.InternalAddItem(cat, 3031, 150, 1, ConstSlotWhereever)

	// Asking to remove more than we have must mutate nothing and return false.
	if p.RemoveItemOfType(cat, 3031, 200, -1, false) {
		t.Fatalf("expected removal of 200 to fail (only 150 present)")
	}
	if got := p.GetItemTypeCount(cat, 3031, -1); got != 150 {
		t.Fatalf("count changed after failed removal: %d", got)
	}

	// Removing an affordable amount succeeds and leaves the remainder.
	if !p.RemoveItemOfType(cat, 3031, 120, -1, false) {
		t.Fatalf("expected removal of 120 to succeed")
	}
	if got := p.GetItemTypeCount(cat, 3031, -1); got != 30 {
		t.Errorf("remaining count = %d, want 30", got)
	}
}

func TestGetFreeBackpackSlots(t *testing.T) {
	cat := testCatalog()
	p := playerWithBackpack()
	if got := p.GetFreeBackpackSlots(cat); got != 20 {
		t.Fatalf("empty backpack free slots = %d, want 20", got)
	}
	// A non-stackable sword occupies one slot.
	p.InternalAddItem(cat, 3264, 1, 1, ConstSlotWhereever)
	if got := p.GetFreeBackpackSlots(cat); got != 19 {
		t.Errorf("after one item free slots = %d, want 19", got)
	}
}

func TestFreeCapacityTracksWeight(t *testing.T) {
	cat := testCatalog()
	p := playerWithBackpack()
	p.UpdateInventoryWeight(cat)
	base := p.GetFreeCapacity()
	// Add a 3500-weight sword; free capacity should drop by exactly that.
	p.InternalAddItem(cat, 3264, 1, 1, ConstSlotWhereever)
	if got := base - p.GetFreeCapacity(); got != 3500 {
		t.Errorf("free capacity dropped by %d, want 3500", got)
	}
}

func TestRemoveForSaleSkipsTiered(t *testing.T) {
	cat := testCatalog()
	p := playerWithBackpack()
	tier := uint8(2)
	// One plain sword and one tiered sword in the backpack.
	bp := p.Inventory[ConstSlotBackpack]
	bp.Contents = append(bp.Contents,
		&Item{ID: 3264, Count: 1},
		&Item{ID: 3264, Count: 1, Attr: &ItemAttributes{Tier: &tier}},
	)

	// Only the plain sword is sellable.
	if n := p.CountSellable(cat, 3264, -1); n != 1 {
		t.Fatalf("CountSellable = %d, want 1 (tiered excluded)", n)
	}
	sold := p.RemoveForSale(cat, 3264, 5, -1)
	if sold != 1 {
		t.Errorf("RemoveForSale sold %d, want 1", sold)
	}
	// The tiered sword must still be present.
	remaining := 0
	for _, it := range bp.Contents {
		if it != nil && it.ID == 3264 {
			remaining++
		}
	}
	if remaining != 1 {
		t.Errorf("tiered sword count after sale = %d, want 1 (untouched)", remaining)
	}
}
