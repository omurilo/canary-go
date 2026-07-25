package game

// Depot system implementation, mirroring the C++ DepotLocker and DepotChest
// classes (src/items/containers/depot/). A depot is a per-town storage that
// persists in the player_depotitems table.
//
// Structure:
// - Each town has a DepotLocker (item ID 2589-2598, one per town)
// - Inside each locker are DepotChest boxes (17 boxes, item ID 2590-2606)
// - Each chest can hold items (max 2000 items per depot)
//
// The depot system uses a hierarchical SID (slot ID) scheme:
// - SID 0-99: reserved for depot lockers (one per town)
// - SID 100+: items inside depot chests

const (
	// MaxDepotItems is the maximum number of items allowed in a depot chest.
	MaxDepotItems = 2000

	// DepotChestCount is the number of depot chests (boxes) per locker.
	DepotChestCount = 17

	// Item IDs for depot lockers (one per town, ID 1-11 correspond to town IDs)
	ItemDepotLocker = 2589

	// Item IDs for depot chests (boxes inside lockers)
	ItemDepotChestFirst = 2590
	ItemDepotChestLast  = 2606
)

// DepotLocker represents a town-specific depot locker container. It's a special
// container that can only contain DepotChest boxes and cannot be moved or removed.
// Mirrors src/items/containers/depot/depotlocker.hpp.
type DepotLocker struct {
	*Item
	DepotID uint16 // town ID this locker belongs to
}

// NewDepotLocker creates a new depot locker for a specific town.
func NewDepotLocker(townID uint16) *DepotLocker {
	return &DepotLocker{
		Item: &Item{
			ID:       ItemDepotLocker,
			Contents: make([]*Item, 0, DepotChestCount),
		},
		DepotID: townID,
	}
}

// IsDepotLocker returns true if this item is a depot locker.
func (it *Item) IsDepotLocker() bool {
	return it != nil && it.ID == ItemDepotLocker
}

// IsDepotChest returns true if this item is a depot chest (box).
func (it *Item) IsDepotChest() bool {
	return it != nil && it.ID >= ItemDepotChestFirst && it.ID <= ItemDepotChestLast
}

// GetOrCreateDepotChest returns the depot chest at the given index (0-16),
// creating it if it doesn't exist. This mirrors the lazy initialization
// behavior in the C++ server.
func (dl *DepotLocker) GetOrCreateDepotChest(index int) *Item {
	if index < 0 || index >= DepotChestCount {
		return nil
	}

	// Ensure the locker has enough capacity
	for len(dl.Contents) <= index {
		dl.Contents = append(dl.Contents, nil)
	}

	// Create the chest if it doesn't exist
	if dl.Contents[index] == nil {
		chestID := uint16(ItemDepotChestFirst + index)
		dl.Contents[index] = &Item{
			ID:       chestID,
			Contents: make([]*Item, 0),
		}
	}

	return dl.Contents[index]
}

// GetDepotItemCount returns the total number of items stored in this depot
// locker (across all chests), counting recursively.
func (dl *DepotLocker) GetDepotItemCount() int {
	count := 0
	for _, chest := range dl.Contents {
		if chest != nil && chest.IsDepotChest() {
			count += countItemsRecursive(chest)
		}
	}
	return count
}

// countItemsRecursive counts all items in a container and its sub-containers.
func countItemsRecursive(container *Item) int {
	if container == nil {
		return 0
	}

	count := len(container.Contents)
	for _, item := range container.Contents {
		if item != nil && len(item.Contents) > 0 {
			count += countItemsRecursive(item)
		}
	}
	return count
}

// CanAddToDepot checks if an item can be added to the depot (respects the 2000 item limit).
func (dl *DepotLocker) CanAddToDepot(itemCount int) bool {
	currentCount := dl.GetDepotItemCount()
	return currentCount+itemCount <= MaxDepotItems
}

// PlayerDepotManager manages all depot lockers for a player across all towns.
type PlayerDepotManager struct {
	player  *Player
	Lockers map[uint16]*DepotLocker // keyed by town ID
}

// NewPlayerDepotManager creates a new depot manager for a player.
func NewPlayerDepotManager(p *Player) *PlayerDepotManager {
	return &PlayerDepotManager{
		player:  p,
		Lockers: make(map[uint16]*DepotLocker),
	}
}

// GetDepotLocker returns the depot locker for a specific town, creating it if needed.
func (dm *PlayerDepotManager) GetDepotLocker(townID uint16) *DepotLocker {
	if locker, exists := dm.Lockers[townID]; exists {
		return locker
	}

	// Create new locker for this town
	locker := NewDepotLocker(townID)
	dm.Lockers[townID] = locker
	return locker
}

// GetDepotChest returns a specific depot chest (box) for a town and index.
func (dm *PlayerDepotManager) GetDepotChest(townID uint16, chestIndex int) *Item {
	locker := dm.GetDepotLocker(townID)
	return locker.GetOrCreateDepotChest(chestIndex)
}

// OpenDepot opens the depot for the player's current town. This is called when
// the player clicks on a depot chest in the game world.
func (p *Player) OpenDepot() *DepotLocker {
	if p.TownID == 0 {
		return nil
	}

	if p.DepotManager == nil {
		p.DepotManager = NewPlayerDepotManager(p)
	}

	return p.DepotManager.GetDepotLocker(p.TownID)
}

// GetDepotItemCount returns the total number of items in the player's depot
// across all towns.
func (p *Player) GetDepotItemCount() int {
	if p.DepotManager == nil {
		return 0
	}

	total := 0
	for _, locker := range p.DepotManager.Lockers {
		total += locker.GetDepotItemCount()
	}
	return total
}
