package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/items"
)

func TestAddItemToContainerStackable(t *testing.T) {
	cat := items.NewCatalog(&items.ItemType{ID: 2160, Stackable: true, StackSize: 100})
	bp := &Item{ID: 1988}

	// First stack of gold coins (ID 2160, stackable) with 50 coins
	gold1 := &Item{ID: 2160, Count: 50}
	bp.Contents = []*Item{gold1}

	// Add 30 gold coins
	gold2 := &Item{ID: 2160, Count: 30}
	ok := AddItemToContainer(cat, bp, gold2)
	if !ok {
		t.Fatalf("AddItemToContainer failed")
	}

	// Should stack into gold1 -> 80 coins, no new item in Contents
	if len(bp.Contents) != 1 {
		t.Fatalf("expected 1 item in container, got %d", len(bp.Contents))
	}
	if bp.Contents[0].Count != 80 {
		t.Fatalf("expected 80 coins in stack, got %d", bp.Contents[0].Count)
	}
}
