package game

import (
	"slices"
)

const (
	QuickLootFilterSkipped  uint8 = 0 // Blacklist: loot all items EXCEPT items in list
	QuickLootFilterAccepted uint8 = 1 // Whitelist: loot only items in list
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
			
			// AutoBank logic could go here, but for now we do ManagedContainers check
			cat := item.GetObjectCategory(w.Items)
			
			// AutoBank logic
			if cat == ObjectCategoryGold && w.AutoBank {
				// C++ adds the worth * count
				worth := item.Worth()
				count := uint64(item.Count)
				if count == 0 {
					count = 1
				}
				p.BankBalance += worth * count
				
				// Remove the item from corpse and don't add to container
				corpseContainer.Contents = append(corpseContainer.Contents[:i], corpseContainer.Contents[i+1:]...)
				
				// Notify the client about the loot if we had a proper system (missing in Go right now)
				continue
			}

			if catPos, exists := p.ManagedContainers[cat]; exists {
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

	tiles := []*Tile{tile}
	if lootAllCorpses {
		// Sweep a 3x3 area around the target pos
		tiles = nil
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				t := w.Map.GetTile(Position{X: pos.X + uint16(dx), Y: pos.Y + uint16(dy), Z: pos.Z})
				if t != nil {
					tiles = append(tiles, t)
				}
			}
		}
	}

	for _, t := range tiles {
		if len(t.Items) > 0 {
			for i := len(t.Items) - 1; i >= 0; i-- {
				it := t.Items[i]
				if it.IsContainer(w.Items) {
					lootCorpse(it)
					// In C++, quickloot loots all containers on the tile if lootAllCorpses is false, wait:
					// "loot nearby" means loot all corpses. If not lootAllCorpses, it's just that single item.
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

// PlayerSetManagedContainer sets managed container for a category.
func (w *World) PlayerSetManagedContainer(playerID uint32, category uint8, pos Position, itemID uint16, stackpos uint8, isLootContainer bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}

	if isLootContainer {
		if p.ManagedContainers == nil {
			p.ManagedContainers = make(map[uint8]Position)
		}
		p.ManagedContainers[category] = pos
	} else {
		if p.ManagedObtainContainers == nil {
			p.ManagedObtainContainers = make(map[uint8]Position)
		}
		p.ManagedObtainContainers[category] = pos
	}
}

// PlayerClearManagedContainer clears a managed container.
func (w *World) PlayerClearManagedContainer(playerID uint32, category uint8, isLootContainer bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}

	if isLootContainer && p.ManagedContainers != nil {
		delete(p.ManagedContainers, category)
	} else if !isLootContainer && p.ManagedObtainContainers != nil {
		delete(p.ManagedObtainContainers, category)
	}
}

// PlayerOpenManagedContainer opens a managed container for the player.
func (w *World) PlayerOpenManagedContainer(playerID uint32, category uint8, isLootContainer bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}
	
	// We don't have container opening implemented fully here yet, so just stub it out.
	// C++ sends the open container packet if the container exists.
	// For now this is a no-op until container windows are fully built in Go.
	_ = p
}
