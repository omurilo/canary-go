package game

const (
	MaxDepotItems = 2000

	ItemDepot      = 3502
	ItemLocker     = 3497
	ItemInbox      = 12902
	ItemMarket     = 12903
	ItemStoreInbox = 23396
	ItemStash      = 28750
	ItemStoreCoin  = 22118
	ItemGoldPouch  = 23721

	// Mail item IDs.
	ItemParcel        = 3503
	ItemParcelStamped = 3504
	ItemLetter        = 3505
	ItemLetterStamped = 3506
	ItemLabel         = 3507
)

// PlayerDepotManager manages all depot lockers and chests for a player.
type PlayerDepotManager struct {
	player  *Player
	Lockers map[uint16]*Item // keyed by town ID
	Chests  map[uint16]*Item // keyed by depot chest ID (1-17)
}

func NewPlayerDepotManager(p *Player) *PlayerDepotManager {
	return &PlayerDepotManager{
		player:  p,
		Lockers: make(map[uint16]*Item),
		Chests:  make(map[uint16]*Item),
	}
}

func (dm *PlayerDepotManager) GetDepotChest(depotId uint16, autoCreate bool) *Item {
	if chest, ok := dm.Chests[depotId]; ok {
		return chest
	}
	if !autoCreate {
		return nil
	}

	chestID := uint16(22796 + depotId) // ITEM_DEPOT_NULL + depotId
	if depotId == 18 {
		chestID = 22814 // ITEM_DEPOT_XVIII
	} else if depotId == 19 {
		chestID = 22815 // ITEM_DEPOT_XIX
	} else if depotId > 19 {
		chestID = 22816 // ITEM_DEPOT_XX
	}

	chest := &Item{ID: chestID, Contents: make([]*Item, 0), Pagination: true, MaxSize: 36}
	dm.Chests[depotId] = chest
	return chest
}

func (dm *PlayerDepotManager) GetDepotLocker(depotId uint16) *Item {
	if locker, ok := dm.Lockers[depotId]; ok {
		// In C++, the inbox parent is updated here, and the depotBoxes parents are updated.
		return locker
	}

	locker := &Item{ID: ItemLocker, Contents: make([]*Item, 0), Pagination: true, MaxSize: 36}

	// Depot Box container that holds the 17 nested boxes
	depotChestContainer := &Item{ID: ItemDepot, Contents: make([]*Item, 0), Pagination: true}
	for i := uint16(1); i <= 17; i++ {
		chest := dm.GetDepotChest(i, true)
		depotChestContainer.Contents = append(depotChestContainer.Contents, chest)
		if chest.Parent == nil {
			chest.Parent = depotChestContainer
		}
	}
	locker.Contents = append(locker.Contents, depotChestContainer)
	depotChestContainer.Parent = locker

	stash := &Item{ID: ItemStash}
	locker.Contents = append(locker.Contents, stash)

	if dm.player.Inbox == nil {
		dm.player.Inbox = &Item{ID: ItemInbox, Contents: make([]*Item, 0), Pagination: true}
	}
	locker.Contents = append(locker.Contents, dm.player.Inbox)

	market := &Item{ID: ItemMarket}
	locker.Contents = append(locker.Contents, market)

	dm.Lockers[depotId] = locker
	return locker
}

// GetDepotItemCount returns the total number of items in the player's depot
// across all chests.
func (p *Player) GetDepotItemCount() int {
	if p.DepotManager == nil {
		return 0
	}

	total := 0
	for _, chest := range p.DepotManager.Chests {
		total += countItemsRecursive(chest)
	}
	return total
}

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
