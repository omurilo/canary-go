package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/items"
)

func TestContainerCapacityDefaultsAndExplicit(t *testing.T) {
	cat := items.NewCatalog(
		&items.ItemType{ID: 2854, Name: "backpack", Group: items.GroupContainer, Capacity: 20},
		&items.ItemType{ID: 1987, Name: "bag", Group: items.GroupContainer}, // no containersize
		&items.ItemType{ID: 3264, Name: "sword"},                            // not a container
	)

	bp := &Item{ID: 2854}
	if got := bp.ContainerCapacity(cat); got != 20 {
		t.Errorf("backpack capacity = %d, want 20 (explicit containersize)", got)
	}

	bag := &Item{ID: 1987}
	if got := bag.ContainerCapacity(cat); got != DefaultContainerCapacity {
		t.Errorf("bag capacity = %d, want %d (default)", got, DefaultContainerCapacity)
	}

	sword := &Item{ID: 3264}
	if got := sword.ContainerCapacity(cat); got != 0 {
		t.Errorf("non-container capacity = %d, want 0", got)
	}

	// An explicit MaxSize on the instance always wins.
	custom := &Item{ID: 1987, MaxSize: 32}
	if got := custom.ContainerCapacity(cat); got != 32 {
		t.Errorf("instance MaxSize = %d, want 32", got)
	}
}
