package game

import (
	"slices"

	"github.com/omurilo/canary-go/internal/items"
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

// QuickLootMaxCorpses bounds how many corpses a single loot-all/nearby action
// sweeps, mirroring QUICK_LOOT_MAX_CORPSES.
const QuickLootMaxCorpses = 30

// findManagedContainer returns the inventory container assigned to a loot
// category, honoring the C++ resolution order: the category's own container,
// then the OBJECTCATEGORY_DEFAULT container, then (only if enabled) the main
// backpack. Returns nil when nothing applies. Mirrors Player::getManaged
// container + findManagedContainer.
func (p *Player) findManagedContainer(cat *items.Catalog, category uint8) *Item {
	if id, ok := p.ManagedContainers[category]; ok {
		if c := p.findInventoryContainer(cat, id); c != nil {
			return c
		}
	}
	if id, ok := p.ManagedContainers[ObjectCategoryDefault]; ok {
		if c := p.findInventoryContainer(cat, id); c != nil {
			return c
		}
	}
	if p.QuickLootFallbackToMain {
		if bp := p.Inventory[ConstSlotBackpack]; bp != nil && bp.IsContainer(cat) {
			return bp
		}
	}
	return nil
}

// findInventoryContainer returns the first container of the given id in the
// inventory tree, or nil.
func (p *Player) findInventoryContainer(cat *items.Catalog, id uint16) *Item {
	var found *Item
	p.WalkInventory(func(it *Item) {
		if found == nil && it.ID == id && it.IsContainer(cat) {
			found = it
		}
	})
	return found
}

// PlayerQuickLoot loots a corpse (or corpses) into the player's managed loot
// containers. variant 0 loots the single corpse identified by itemID/stackpos
// on the tile at pos; variant 1 loots every corpse on that tile; variant 2
// ("loot nearby") sweeps the 3x3 area around the player. Both sweeps are capped
// at QuickLootMaxCorpses.
func (w *World) PlayerQuickLoot(playerID uint32, pos Position, itemID uint16, stackpos uint8, lootAllCorpses bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}

	lootCorpse := func(corpseContainer *Item) {
		if corpseContainer == nil || corpseContainer.Container == nil || !corpseContainer.IsContainer(w.Items) {
			return
		}
		for i := len(corpseContainer.Container.Contents) - 1; i >= 0; i-- {
			item := corpseContainer.Container.Contents[i]
			if item == nil {
				continue
			}

			// Loot filter: skip-list (blacklist) skips listed items; accept-list
			// (whitelist) loots only listed items.
			inList := slices.Contains(p.QuickLootList, item.ID)
			if p.QuickLootFilter == QuickLootFilterAccepted && !inList {
				continue
			}
			if p.QuickLootFilter == QuickLootFilterSkipped && inList {
				continue
			}

			cat := item.GetObjectCategory(w.Items)

			// Gold routing straight to the bank when AUTOBANK is on.
			if cat == ObjectCategoryGold && w.AutoBank {
				count := uint64(item.Count)
				if count == 0 {
					count = 1
				}
				p.BankBalance += item.Worth() * count
				corpseContainer.Container.Contents = append(corpseContainer.Container.Contents[:i], corpseContainer.Container.Contents[i+1:]...)
				continue
			}

			// Resolve the destination container (category → default → main).
			targetContainer := p.findManagedContainer(w.Items, cat)
			if targetContainer == nil {
				continue // no container and fallback disabled: leave it in the corpse
			}

			corpseContainer.Container.Contents = append(corpseContainer.Container.Contents[:i], corpseContainer.Container.Contents[i+1:]...)
			AddItemToContainer(w.Items, targetContainer, item)

			if p.Session != nil {
				p.Session.RefreshContainer(targetContainer)
				p.Session.RefreshContainer(corpseContainer)
			}

			if w.OnContainerAddItem != nil {
				w.OnContainerAddItem(p, targetContainer, item)
			}
		}
	}

	// Collect the corpse containers to loot.
	var corpses []*Item
	collectFromTile := func(t *Tile) {
		if t == nil {
			return
		}
		for i := len(t.Items) - 1; i >= 0; i-- {
			if len(corpses) >= QuickLootMaxCorpses {
				return
			}
			it := t.Items[i]
			if it != nil && it.IsContainer(w.Items) {
				corpses = append(corpses, it)
			}
		}
	}

	switch {
	case lootAllCorpses && pos.X == 0xFFFF:
		// "loot nearby": 3x3 around the player.
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				collectFromTile(w.Map.GetTile(Position{X: p.Pos.X + uint16(dx), Y: p.Pos.Y + uint16(dy), Z: p.Pos.Z}))
			}
		}
	case lootAllCorpses:
		// Loot every corpse on the target tile.
		collectFromTile(w.Map.GetTile(pos))
	default:
		// Single corpse: the specific item at itemID/stackpos, else the topmost
		// container on the tile.
		tile := w.Map.GetTile(pos)
		if tile == nil {
			return
		}
		var chosen *Item
		if int(stackpos) < len(tile.Items) {
			if it := tile.Items[stackpos]; it != nil && it.ID == itemID && it.IsContainer(w.Items) {
				chosen = it
			}
		}
		if chosen == nil {
			for i := len(tile.Items) - 1; i >= 0; i-- {
				if it := tile.Items[i]; it != nil && (itemID == 0 || it.ID == itemID) && it.IsContainer(w.Items) {
					chosen = it
					break
				}
			}
		}
		if chosen != nil {
			corpses = append(corpses, chosen)
		}
	}

	for _, c := range corpses {
		lootCorpse(c)
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

// setQuickLootBit sets or clears the category bit in the container's persisted
// loot/obtain bitmask attribute (ATTR_QUICKLOOTCONTAINER / ATTR_OBTAINCONTAINER).
func (i *Item) setQuickLootBit(category uint8, isLoot, on bool) {
	if i == nil {
		return
	}
	if i.Attr == nil {
		i.Attr = &ItemAttributes{}
	}
	field := &i.Attr.QuickLootContainer
	if !isLoot {
		field = &i.Attr.ObtainContainer
	}
	var v uint32
	if *field != nil {
		v = **field
	}
	if on {
		v |= 1 << category
	} else {
		v &^= 1 << category
	}
	if v == 0 {
		*field = nil
	} else {
		*field = &v
	}
}

// PlayerSetManagedContainer assigns the inventory container `itemID` as the
// managed loot (or obtain) container for a category. The assignment is stored
// both in the fast-lookup map and as a persisted bitmask attribute on the
// container item, mirroring C++ ATTR_QUICKLOOTCONTAINER/ATTR_OBTAINCONTAINER.
func (w *World) PlayerSetManagedContainer(playerID uint32, category uint8, pos Position, itemID uint16, stackpos uint8, isLootContainer bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}
	_ = pos
	_ = stackpos

	m := &p.ManagedContainers
	if !isLootContainer {
		m = &p.ManagedObtainContainers
	}
	if *m == nil {
		*m = make(map[uint8]uint16)
	}
	(*m)[category] = itemID

	if c := p.findInventoryContainer(w.Items, itemID); c != nil {
		c.setQuickLootBit(category, isLootContainer, true)
	}
}

// RebuildManagedContainers repopulates the managed-container maps from the
// persisted per-container bitmask attributes. Called on login after the
// inventory is loaded so quick-loot assignments survive relog.
func (p *Player) RebuildManagedContainers() {
	p.ManagedContainers = make(map[uint8]uint16)
	p.ManagedObtainContainers = make(map[uint8]uint16)
	p.WalkInventory(func(it *Item) {
		if it.Attr == nil {
			return
		}
		if it.Attr.QuickLootContainer != nil {
			mask := *it.Attr.QuickLootContainer
			for cat := uint8(0); cat <= 31; cat++ {
				if mask&(1<<cat) != 0 {
					p.ManagedContainers[cat] = it.ID
				}
			}
		}
		if it.Attr.ObtainContainer != nil {
			mask := *it.Attr.ObtainContainer
			for cat := uint8(0); cat <= 31; cat++ {
				if mask&(1<<cat) != 0 {
					p.ManagedObtainContainers[cat] = it.ID
				}
			}
		}
	})
}

// PlayerClearManagedContainer clears a managed container.
func (w *World) PlayerClearManagedContainer(playerID uint32, category uint8, isLootContainer bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, ok := w.players[playerID]
	if !ok {
		return
	}

	m := p.ManagedContainers
	if !isLootContainer {
		m = p.ManagedObtainContainers
	}
	if m == nil {
		return
	}
	if itemID, exists := m[category]; exists {
		if c := p.findInventoryContainer(w.Items, itemID); c != nil {
			c.setQuickLootBit(category, isLootContainer, false)
		}
		delete(m, category)
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
