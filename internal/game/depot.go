package game

import "github.com/omurilo/canary-go/internal/config"

// Depot box item ids (src/utils/utils_definitions.hpp:546-571). Boxes I..XVII are
// contiguous from ITEM_DEPOT_NULL, and then the sequence BREAKS: XVIII, XIX and XX
// live in unrelated ranges. Deriving them by arithmetic yields 22814/22815/22816,
// two of which are not item ids at all, and an unknown id in a container frame
// makes the client read a zero appearance and lose the rest of the packet.
const (
	ItemDepotNull  = 22796 // for internal use — actual item id 168
	ItemDepotI     = 22797
	ItemDepotXVII  = 22813
	ItemDepotXVIII = 31915
	ItemDepotXIX   = 39723
	ItemDepotXX    = 39724
)

// DepotChestPageSize is DepotChest::maxSize (depotchest.cpp:17): how many slots
// one page of a depot box shows.
const DepotChestPageSize = 32

// defaultDepotBoxes mirrors `depotBoxes = 20` in config.lua.dist.
const defaultDepotBoxes = 20

// depotBoxes is g_configManager().getNumber(DEPOT_BOXES): how many boxes a locker
// holds, and the capacity of the depot container that holds them.
func depotBoxes() uint16 {
	n := config.Number("depotBoxes", defaultDepotBoxes)
	if n < 1 {
		return 1
	}
	if n > defaultDepotBoxes {
		// Beyond box XX there is no item id to give a box, so the C++ hands them all
		// ITEM_DEPOT_XX. Capping keeps the locker from listing duplicates.
		return defaultDepotBoxes
	}
	return uint16(n)
}

const (
	MaxDepotItems = 2000

	ItemDepot      = 3502
	ItemLocker     = 3497
	ItemInbox      = 12902
	ItemMarket     = 12903
	ItemStoreInbox = 23396
	// ItemDecorationKit is ITEM_DECORATION_KIT (utils_definitions.hpp:547): the
	// wrapped form of house furniture, carrying the original id in a custom attribute.
	ItemDecorationKit = 23398
	ItemStash         = 28750
	ItemStoreCoin     = 22118
	ItemGoldPouch     = 23721
	ItemBrowseField   = 470

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
	Chests  map[uint16]*Item // keyed by depot box number (1..depotBoxes)
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

	// Player::getDepotChest (src/creatures/players/player.cpp), branch for branch.
	// Note 0 falls to the last case in C++ too, not to ITEM_DEPOT_NULL.
	var chestID uint16
	switch {
	case depotId > 0 && depotId < 18:
		chestID = ItemDepotNull + depotId
	case depotId == 18:
		chestID = ItemDepotXVIII
	case depotId == 19:
		chestID = ItemDepotXIX
	default:
		chestID = ItemDepotXX
	}

	// DepotChest::DepotChest sets maxSize = 32, not 36
	// (src/items/containers/depot/depotchest.cpp:14-18). The capacity travels to the
	// client in the 0x6E frame and is what it pages by, so an invented number makes
	// the server and the client disagree about where page two starts.
	//
	// maxSize is the PAGE, not the limit: queryAdd caps a depot chest at
	// maxDepotItems (2000), so holding far more than 32 is correct and pagination is
	// the only way to see the rest.
	// MaxItems is the real limit, MaxSize is only the page. Leaving MaxItems at 0
	// made AddItemToContainer fall back to the page size, so a depot box refused
	// everything past its 32nd item — and the move path had already taken the item
	// off the floor by then, so it was simply gone.
	//
	// DepotChest::maxDepotItems = 2000 (depotchest.cpp:16).
	chest := &Item{
		ID: chestID, Contents: make([]*Item, 0), Pagination: true,
		MaxSize: DepotChestPageSize, MaxItems: MaxDepotItems,
	}
	dm.Chests[depotId] = chest
	return chest
}

func (dm *PlayerDepotManager) GetDepotLocker(depotId uint16) *Item {
	if locker, ok := dm.Lockers[depotId]; ok {
		// In C++, the inbox parent is updated here, and the depotBoxes parents are updated.
		return locker
	}

	// DepotLocker(ITEM_LOCKER, 4): market, inbox, stash and the depot container are
	// all it ever holds. It was 36, which advertises 32 empty slots the locker does
	// not have.
	locker := &Item{ID: ItemLocker, Contents: make([]*Item, 0), Pagination: true, MaxSize: 4}

	// CreateItemAsContainer(ITEM_DEPOT, DEPOT_BOXES): the container's capacity is the
	// box count, not a fixed 17.
	boxes := depotBoxes()
	depotChestContainer := &Item{ID: ItemDepot, Contents: make([]*Item, 0), Pagination: true, MaxSize: boxes}
	for i := uint16(1); i <= boxes; i++ {
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
