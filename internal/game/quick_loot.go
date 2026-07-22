package game

import (
	"slices"
)

const (
	QuickLootFilterAccepted uint8 = 0 // Whitelist: loot only items in list
	QuickLootFilterSkipped  uint8 = 1 // Blacklist: loot all items EXCEPT items in list
)

// IsQuickLootAllowed checks if an item is allowed by player's Quick Loot filter.
func (p *Player) IsQuickLootAllowed(itemID uint16) bool {
	inList := slices.Contains(p.QuickLootList, itemID)
	if p.QuickLootFilter == QuickLootFilterAccepted {
		return inList
	}
	return !inList
}

// PlayerQuickLoot loots items from a corpse or nearby corpses into the player's containers.
func (w *World) PlayerQuickLoot(playerID uint32, pos Position, itemID uint16, stackpos uint8, lootAllCorpses bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}

	tile := w.Map.GetTile(pos)
	if tile == nil {
		return
	}

	// Helper to loot a single corpse container
	lootCorpse := func(corpseContainer *Item) {
		if corpseContainer == nil || !corpseContainer.IsContainer(w.Items) {
			return
		}

		// Find target container (backpack slot)
		var mainContainer *Item
		if p.Inventory[ConstSlotBackpack] != nil && p.Inventory[ConstSlotBackpack].IsContainer(w.Items) {
			mainContainer = p.Inventory[ConstSlotBackpack]
		}

		// Iterate over corpse items in reverse order to safely remove items
		for i := len(corpseContainer.Contents) - 1; i >= 0; i-- {
			item := corpseContainer.Contents[i]
			if item == nil {
				continue
			}

			// Check filter
			inList := slices.Contains(p.QuickLootList, item.ID)
			if p.QuickLootFilter == QuickLootFilterAccepted && !inList {
				continue
			}
			if p.QuickLootFilter == QuickLootFilterSkipped && inList {
				continue
			}

			// Target container selection
			targetContainer := mainContainer
			if catPos, exists := p.ManagedContainers[uint8(item.ID)]; exists {
				if tTile := w.Map.GetTile(catPos); tTile != nil && len(tTile.Items) > 0 {
					topIt := tTile.Items[len(tTile.Items)-1]
					if topIt.IsContainer(w.Items) {
						targetContainer = topIt
					}
				}
			}

			if targetContainer == nil || !targetContainer.IsContainer(w.Items) {
				if !p.QuickLootFallbackToMain {
					continue
				}
				targetContainer = mainContainer
			}

			if targetContainer == nil || !targetContainer.IsContainer(w.Items) {
				continue
			}

			// Perform item move from corpse container to target container
			corpseContainer.Contents = append(corpseContainer.Contents[:i], corpseContainer.Contents[i+1:]...)
			targetContainer.Contents = append(targetContainer.Contents, item)

			// Notify client callback if present
			if w.OnContainerAddItem != nil {
				w.OnContainerAddItem(p, targetContainer, item)
			}
		}
	}

	if len(tile.Items) > 0 {
		for i := len(tile.Items) - 1; i >= 0; i-- {
			it := tile.Items[i]
			if it.IsContainer(w.Items) {
				lootCorpse(it)
				if !lootAllCorpses {
					break
				}
			}
		}
	}
}

// PlayerSetQuickLootFilter updates the player's quick loot filter and item list.
func (w *World) PlayerSetQuickLootFilter(playerID uint32, filter uint8, listedItems []uint16) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}

	p.QuickLootFilter = filter
	p.QuickLootList = listedItems
}

// PlayerSetQuickLootFallback toggles fallback to main container.
func (w *World) PlayerSetQuickLootFallback(playerID uint32, fallback bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}

	p.QuickLootFallbackToMain = fallback
}

// PlayerSetLootContainer sets managed container for a category.
func (w *World) PlayerSetLootContainer(playerID uint32, category uint8, pos Position, itemID uint16, stackpos uint8, isLootContainer bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}

	if p.ManagedContainers == nil {
		p.ManagedContainers = make(map[uint8]Position)
	}

	if isLootContainer {
		p.ManagedContainers[category] = pos
	} else {
		delete(p.ManagedContainers, category)
	}
}
