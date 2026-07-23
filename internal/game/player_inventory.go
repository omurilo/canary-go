package game

import "github.com/opentibiabr/canary-go/internal/items"

// This file mirrors the inventory block of src/creatures/players/player.cpp:
// item counting, type search, type removal, capacity/weight, and placement.
// All traversal walks equipment slots 1..10 plus the recursive container tree.

// GetInventoryItem returns the item equipped in slot (1..10), or nil.
func (p *Player) GetInventoryItem(slot int) *Item {
	if slot < ConstSlotFirst || slot > ConstSlotLast {
		return nil
	}
	return p.Inventory[slot]
}

// GetCapacity returns the player's total capacity (base + bonus). Mirrors
// Player::getCapacity (varStats/wheel bonuses are TODO and folded into
// BonusCapacity when modelled).
func (p *Player) GetCapacity() uint32 {
	cap := p.Capacity + p.BonusCapacity
	if p.Wheel != nil {
		cap += p.Wheel.GetBonusCapacity()
	}
	return cap
}

// GetFreeCapacity returns capacity minus carried weight, clamped at 0. Mirrors
// Player::getFreeCapacity. Relies on InventoryWeight being kept current.
func (p *Player) GetFreeCapacity() uint32 {
	cap := p.GetCapacity()
	if p.InventoryWeight >= cap {
		return 0
	}
	return cap - p.InventoryWeight
}

// UpdateInventoryWeight recomputes the cached total weight of everything the
// player carries. Call after every inventory mutation. Mirrors
// Player::updateInventoryWeight.
func (p *Player) UpdateInventoryWeight(catalog *items.Catalog) {
	var total uint32
	p.WalkInventory(func(it *Item) {
		unitWeight := itemUnitWeight(catalog, it.ID)
		if isStackable(catalog, it.ID) {
			total += unitWeight * uint32(max16(it.Count, 1))
		} else {
			total += unitWeight
		}
	})
	p.InventoryWeight = total
}

// itemUnitWeight returns the per-unit weight of an item id from the catalog.
func itemUnitWeight(catalog *items.Catalog, id uint16) uint32 {
	if catalog == nil {
		return 0
	}
	if t := catalog.Get(id); t != nil {
		return t.Weight
	}
	return 0
}

// WalkInventory invokes fn for every item the player carries: equipped items
// and the recursive contents of any containers. Order is slot 1..10 then
// depth-first container contents.
func (p *Player) WalkInventory(fn func(it *Item)) {
	var walk func(items []*Item)
	walk = func(items []*Item) {
		for _, it := range items {
			if it == nil {
				continue
			}
			fn(it)
			if len(it.Contents) > 0 {
				walk(it.Contents)
			}
		}
	}
	walk(p.Inventory[ConstSlotFirst : ConstSlotLast+1])
}

// countMatch returns how many of it match the itemId/subType filter. subType
// of -1 matches any. Stackable items contribute their Count; others contribute 1.
func countMatch(catalog *items.Catalog, it *Item, itemId uint16, subType int) uint32 {
	if it.ID != itemId {
		return 0
	}
	stackable := isStackable(catalog, it.ID)
	if stackable {
		// For stackables the subType filter is effectively the stack; -1 or an
		// explicit match both count the whole stack.
		if subType == -1 || subType == int(it.Count) {
			return uint32(max16(it.Count, 1))
		}
		return 0
	}
	if subType == -1 || subType == int(it.Count) {
		return 1
	}
	return 0
}

func isStackable(catalog *items.Catalog, id uint16) bool {
	if catalog == nil {
		return false
	}
	if t := catalog.Get(id); t != nil {
		return t.Stackable
	}
	return false
}

func stackSizeOf(catalog *items.Catalog, id uint16) uint16 {
	if catalog != nil {
		if t := catalog.Get(id); t != nil && t.StackSize > 0 {
			return t.StackSize
		}
	}
	return 100
}

// GetItemTypeCount returns the total count of itemId (subType filter, -1 = any)
// across the whole inventory tree. Mirrors Player::getItemTypeCount.
func (p *Player) GetItemTypeCount(catalog *items.Catalog, itemId uint16, subType int) uint32 {
	var total uint32
	p.WalkInventory(func(it *Item) {
		total += countMatch(catalog, it, itemId, subType)
	})
	return total
}

// FindItemOfType returns the first item matching itemId/subType. When deepSearch
// is false only equipment slots are scanned; otherwise container trees too.
// Mirrors g_game().findItemOfType.
func (p *Player) FindItemOfType(catalog *items.Catalog, itemId uint16, deepSearch bool, subType int) *Item {
	for slot := ConstSlotFirst; slot <= ConstSlotLast; slot++ {
		it := p.Inventory[slot]
		if it == nil {
			continue
		}
		if countMatch(catalog, it, itemId, subType) > 0 {
			return it
		}
		if deepSearch && len(it.Contents) > 0 {
			if found := findInContents(catalog, it.Contents, itemId, subType); found != nil {
				return found
			}
		}
	}
	return nil
}

func findInContents(catalog *items.Catalog, items []*Item, itemId uint16, subType int) *Item {
	for _, it := range items {
		if it == nil {
			continue
		}
		if countMatch(catalog, it, itemId, subType) > 0 {
			return it
		}
		if len(it.Contents) > 0 {
			if found := findInContents(catalog, it.Contents, itemId, subType); found != nil {
				return found
			}
		}
	}
	return nil
}

// RemoveItemOfType removes up to `amount` of itemId, two-phase like C++
// Player::removeItemOfType: it first counts matches and only mutates when the
// total is at least `amount`, returning false (mutating nothing) otherwise.
// ignoreEquipped skips the equipment slots (containers only).
func (p *Player) RemoveItemOfType(catalog *items.Catalog, itemId uint16, amount uint32, subType int, ignoreEquipped bool) bool {
	if amount == 0 {
		return true
	}
	if p.GetItemTypeCount(catalog, itemId, subType) < amount {
		return false
	}
	remaining := amount
	for slot := ConstSlotFirst; slot <= ConstSlotLast && remaining > 0; slot++ {
		it := p.Inventory[slot]
		if it == nil {
			continue
		}
		if !ignoreEquipped {
			if m := countMatch(catalog, it, itemId, subType); m > 0 {
				removed := consumeItem(catalog, it, remaining)
				remaining -= removed
				if it.Count == 0 || (!isStackable(catalog, it.ID) && removed > 0) {
					p.Inventory[slot] = nil
				}
				if remaining == 0 {
					break
				}
			}
		}
		if it2 := p.Inventory[slot]; it2 != nil && len(it2.Contents) > 0 {
			it2.Contents = removeFromContents(catalog, it2.Contents, itemId, subType, &remaining)
		}
	}
	return true
}

// consumeItem decrements a matched item's stack (or marks a non-stackable for
// deletion) and returns how many units it removed.
func consumeItem(catalog *items.Catalog, it *Item, want uint32) uint32 {
	if isStackable(catalog, it.ID) {
		have := uint32(max16(it.Count, 1))
		if have <= want {
			it.Count = 0
			return have
		}
		it.Count -= uint16(want)
		return want
	}
	it.Count = 0
	return 1
}

// removeFromContents walks a container's contents removing matches until
// *remaining hits 0, returning the compacted slice.
func removeFromContents(catalog *items.Catalog, contents []*Item, itemId uint16, subType int, remaining *uint32) []*Item {
	out := contents[:0]
	for _, it := range contents {
		if it == nil {
			continue
		}
		if *remaining > 0 && countMatch(catalog, it, itemId, subType) > 0 {
			removed := consumeItem(catalog, it, *remaining)
			*remaining -= removed
			if it.Count == 0 {
				continue // drop non-stackables and emptied stacks
			}
		}
		if len(it.Contents) > 0 {
			it.Contents = removeFromContents(catalog, it.Contents, itemId, subType, remaining)
		}
		out = append(out, it)
	}
	return out
}

// GetFreeBackpackSlots returns the number of free slots in the main backpack
// (slot 3) and its nested containers. Mirrors Player::getFreeBackpackSlots.
func (p *Player) GetFreeBackpackSlots(catalog *items.Catalog) int {
	bp := p.Inventory[ConstSlotBackpack]
	if bp == nil || !bp.IsContainer(catalog) {
		return 0
	}
	return containerFreeSlots(catalog, bp)
}

// containerFreeSlots sums (capacity - used) across a container and every nested
// container, clamped at 0. Guards against runaway depth on a cyclic tree.
func containerFreeSlots(catalog *items.Catalog, c *Item) int {
	free := int(c.ContainerCapacity(catalog)) - len(c.Contents)
	if free < 0 {
		free = 0
	}
	for _, child := range c.Contents {
		if child != nil && child.IsContainer(catalog) {
			free += containerFreeSlots(catalog, child)
		}
	}
	return free
}

// InternalAddItem places `count` of itemId into the player's inventory, mirroring
// the core of Game::internalPlayerAddItem: stackables are split into StackSize
// chunks and merged into existing stacks / dropped into the backpack; a specific
// equipment slot is honored when slot != ConstSlotWhereever. It returns the
// items actually created/placed and whether everything fit.
func (p *Player) InternalAddItem(catalog *items.Catalog, itemId uint16, count uint32, subType, slot int) ([]*Item, bool) {
	if count == 0 {
		count = 1
	}
	stackable := isStackable(catalog, itemId)
	stackSize := uint32(stackSizeOf(catalog, itemId))

	var placed []*Item
	remaining := count
	for remaining > 0 {
		chunk := remaining
		if stackable && chunk > stackSize {
			chunk = stackSize
		} else if !stackable {
			chunk = 1
		}
		it := &Item{ID: itemId, Count: uint16(chunk)}
		if !stackable && subType > 0 {
			it.Count = uint16(subType)
		}
		if !p.placeItem(catalog, it, slot) {
			return placed, false
		}
		placed = append(placed, it)
		remaining -= chunk
		// After the first placement an explicit slot is full; overflow goes to
		// the backpack.
		if slot != ConstSlotWhereever {
			slot = ConstSlotWhereever
		}
	}
	p.UpdateInventoryWeight(catalog)
	return placed, true
}

// placeItem puts a single item into a specific equipment slot (when given and
// free), else merges into an existing stack / drops into the backpack, else the
// first free equipment slot. Returns false when there was no room.
func (p *Player) placeItem(catalog *items.Catalog, it *Item, slot int) bool {
	if slot >= ConstSlotFirst && slot <= ConstSlotLast {
		if p.Inventory[slot] == nil {
			p.Inventory[slot] = it
			return true
		}
	}
	// Merge stackables into an existing matching stack with headroom.
	if isStackable(catalog, it.ID) {
		stackSize := stackSizeOf(catalog, it.ID)
		if merged := p.mergeIntoStack(it, stackSize); merged {
			return true
		}
	}
	// Drop into the backpack container (recursively into nested bags).
	if bp := p.Inventory[ConstSlotBackpack]; bp != nil && bp.IsContainer(catalog) {
		if addToContainerTree(catalog, bp, it) {
			return true
		}
	}
	// Fall back to the first free equipment slot.
	for s := ConstSlotFirst; s <= ConstSlotLast; s++ {
		if p.Inventory[s] == nil {
			p.Inventory[s] = it
			return true
		}
	}
	return false
}

// mergeIntoStack tops up an existing stack of the same id that has headroom,
// consuming it.Count. Returns true when fully merged.
func (p *Player) mergeIntoStack(it *Item, stackSize uint16) bool {
	var try func(items []*Item) bool
	try = func(items []*Item) bool {
		for _, existing := range items {
			if existing == nil {
				continue
			}
			if existing.ID == it.ID && existing.Count < stackSize {
				room := stackSize - existing.Count
				if room > 0 {
					take := it.Count
					if take > room {
						take = room
					}
					existing.Count += take
					it.Count -= take
					if it.Count == 0 {
						return true
					}
				}
			}
			if len(existing.Contents) > 0 && try(existing.Contents) {
				return true
			}
		}
		return false
	}
	return try(p.Inventory[ConstSlotFirst : ConstSlotLast+1])
}

// addToContainerTree inserts it into the first container with a free slot,
// descending into nested containers. Returns false when the whole tree is full.
func addToContainerTree(catalog *items.Catalog, c *Item, it *Item) bool {
	if int(c.ContainerCapacity(catalog)) > len(c.Contents) {
		it.Parent = c
		c.Contents = append(c.Contents, it)
		return true
	}
	for _, child := range c.Contents {
		if child != nil && child.IsContainer(catalog) {
			if addToContainerTree(catalog, child, it) {
				return true
			}
		}
	}
	return false
}

// itemIsSellable reports whether an item may be sold to an NPC. Mirrors the
// core of Npc::isInvalidItemForNpcSell: tiered items are excluded. (Imbuements
// are not modelled on Item yet, so only the tier guard applies.)
func itemIsSellable(it *Item) bool {
	if it.Attr != nil && it.Attr.Tier != nil && *it.Attr.Tier > 0 {
		return false
	}
	return true
}

// CountSellable returns how many sellable units of itemId the player holds
// across the whole inventory tree (subType -1 = any). Mirrors the count pass of
// Npc::getInventoryItemsFromId.
func (p *Player) CountSellable(catalog *items.Catalog, itemId uint16, subType int) uint32 {
	var total uint32
	p.WalkInventory(func(it *Item) {
		if itemIsSellable(it) {
			total += countMatch(catalog, it, itemId, subType)
		}
	})
	return total
}

// RemoveForSale removes up to `amount` sellable units of itemId from the whole
// inventory tree and returns how many were removed. Unlike RemoveItemOfType it
// is not two-phase (it removes what is available up to amount) and it skips
// tiered items. Mirrors Npc::removeItemsFromInventory.
func (p *Player) RemoveForSale(catalog *items.Catalog, itemId uint16, amount uint32, subType int) uint32 {
	remaining := amount
	for slot := ConstSlotFirst; slot <= ConstSlotLast && remaining > 0; slot++ {
		it := p.Inventory[slot]
		if it == nil {
			continue
		}
		if itemIsSellable(it) && countMatch(catalog, it, itemId, subType) > 0 {
			removed := consumeItem(catalog, it, remaining)
			remaining -= removed
			if it.Count == 0 {
				p.Inventory[slot] = nil
				continue
			}
		}
		if it2 := p.Inventory[slot]; it2 != nil && len(it2.Contents) > 0 {
			it2.Contents = removeSellableFromContents(catalog, it2.Contents, itemId, subType, &remaining)
		}
	}
	removed := amount - remaining
	if removed > 0 {
		p.UpdateInventoryWeight(catalog)
	}
	return removed
}

func removeSellableFromContents(catalog *items.Catalog, contents []*Item, itemId uint16, subType int, remaining *uint32) []*Item {
	out := contents[:0]
	for _, it := range contents {
		if it == nil {
			continue
		}
		if *remaining > 0 && itemIsSellable(it) && countMatch(catalog, it, itemId, subType) > 0 {
			removed := consumeItem(catalog, it, *remaining)
			*remaining -= removed
			if it.Count == 0 {
				continue
			}
		}
		if len(it.Contents) > 0 {
			it.Contents = removeSellableFromContents(catalog, it.Contents, itemId, subType, remaining)
		}
		out = append(out, it)
	}
	return out
}

func max16(a, b uint16) uint16 {
	if a > b {
		return a
	}
	return b
}
