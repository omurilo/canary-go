package game

import (
	"math"

	"github.com/omurilo/canary-go/internal/items"
)

// Shopping-bag constants from the anonymous namespace at the top of
// src/creatures/npcs/npc.cpp (lines 29-30).
const (
	ShoppingBagPrice = 20
	ShoppingBagSlots = 20
)

// calculateSlotsNeeded ports calculateSlotsNeeded (npc.cpp:45): how many inventory
// slots (or shopping bags) a purchase of `amount` occupies.
func calculateSlotsNeeded(it *items.ItemType, amount uint16, inBackpacks bool) float64 {
	if it != nil && it.Stackable {
		stackSize := float64(it.StackSize)
		if stackSize <= 0 {
			stackSize = 100
		}
		stackSlots := math.Ceil(float64(amount) / stackSize)
		if inBackpacks {
			return math.Ceil(stackSlots / ShoppingBagSlots)
		}
		return stackSlots
	}
	if inBackpacks {
		return math.Ceil(float64(amount) / ShoppingBagSlots)
	}
	return float64(amount)
}

// IsBackpackSlotUnavailable ports isBackpackSlotUnavailable (npc.cpp:32): the
// purchase is refused when the player has no free slot, unless the item being
// bought is itself a backpack-slot container (so a player with a full inventory can
// still buy a new bag).
func (p *Player) IsBackpackSlotUnavailable(catalog *items.Catalog, itemID uint16, ignore bool) bool {
	if p == nil {
		return true
	}
	if ignore || p.GetFreeBackpackSlots(catalog) != 0 {
		return false
	}
	if p.Inventory[ConstSlotBackpack] != nil {
		return true
	}
	it := catalog.Get(itemID)
	if it == nil {
		return true
	}
	// C++ tests `itemType.slotPosition & SLOTP_BACKPACK`; Go models slotPosition as
	// the raw items.xml string, so this compares against "backpack".
	return !it.IsContainer() || it.SlotPosition != "backpack"
}

// ExceedsTileLimit ports exceedsTileLimit (npc.cpp:54). Note the inverted-looking
// guard: upstream only performs this check when `ignore` (ignoreCapacity) is SET,
// because that is the case where the overflow is dropped on the floor instead of
// being refused.
func (w *World) ExceedsTileLimit(p *Player, it *items.ItemType, amount uint16, inBackpacks, ignore bool) bool {
	if !ignore || p == nil {
		return false
	}
	tile := w.Map.GetTile(p.GetPosition())
	if tile == nil {
		return false
	}
	slotsNeeded := calculateSlotsNeeded(it, amount, inBackpacks)
	itemCount := float64(len(tile.Items))
	return itemCount+(slotsNeeded-float64(p.GetFreeBackpackSlots(w.Items))) > 30
}

// CalculateBagsCost ports calculateBagsCost (npc.cpp:70): buying "in backpacks"
// charges for the shopping bags on top of the goods.
func CalculateBagsCost(it *items.ItemType, amount uint16, inBackpacks bool) uint64 {
	if !inBackpacks {
		return 0
	}
	return ShoppingBagPrice * uint64(calculateSlotsNeeded(it, amount, true))
}

// HasInsufficientFunds ports hasInsufficientFunds (npc.cpp:87).
//
// For gold, purse plus bank must cover goods and bags together. For any other
// currency the currency ITEMS must cover the goods, while the bags are still paid
// in gold — which is why the two are checked separately rather than summed.
func (p *Player) HasInsufficientFunds(catalog *items.Catalog, currency uint16, totalCost, bagsCost uint64) bool {
	if p == nil {
		return true
	}
	if currency == GoldCoinID {
		return p.GetMoney()+p.BankBalance < totalCost+bagsCost
	}
	available := uint64(p.GetItemTypeCount(catalog, currency, -1))
	return available < totalCost || (p.GetMoney()+p.BankBalance) < bagsCost
}

// HasShopItemForSale is Player::hasShopItemForSale (player.cpp:5971): does the
// merchant the player has open actually offer this item.
//
// It reads the per-player shop vector, so an item a script installed for this
// player alone passes and one only on the type's list does not — which is the
// gate that stops a crafted packet buying anything the NPC happens to know
// about.
func (p *Player) HasShopItemForSale(npc *Npc, itemID uint16, subType uint8) bool {
	if p == nil || npc == nil || p.World == nil {
		return false
	}
	it := p.World.Items.Get(itemID)
	fluid := it != nil && it.IsFluidContainer()
	for _, block := range npc.GetShopItemVector(p.DBID) {
		if block.ID != itemID || block.BuyPrice == 0 {
			continue
		}
		// Only fluid containers are matched on subtype: for everything else the
		// client's count byte is a stack size, not a discriminator.
		if !fluid || block.SubType == subType {
			return true
		}
	}
	return false
}

// ContainerHoldingCountExceeded is the MAX_CONTAINER guard in Game::playerBuyItem
// (game.cpp:6250): how many containers the main backpack holds, recursively.
//
// A limit of zero means unconfigured, and is treated as no limit rather than as
// "no containers allowed".
func (p *Player) ContainerHoldingCountExceeded(maxContainer uint32) bool {
	if p == nil || maxContainer == 0 {
		return false
	}
	backpack := p.Inventory[ConstSlotBackpack]
	if backpack == nil {
		return false
	}
	return containerHoldingCount(backpack) >= maxContainer
}

// containerHoldingCount is Container::getContainerHoldingCount
// (container.cpp:410): every nested container, not every item.
func containerHoldingCount(container *Item) uint32 {
	var n uint32
	if container == nil || container.Container == nil {
		return 0
	}
	for _, child := range container.Container.Contents {
		if child == nil || child.Container == nil || len(child.Container.Contents) == 0 {
			continue
		}
		n += 1 + containerHoldingCount(child)
	}
	return n
}

// TileItemCount is tile->getItemCount() at pos, used by the buy path's
// twenty-item floor guard.
func (w *World) TileItemCount(pos Position) int {
	if w == nil || w.Map == nil {
		return 0
	}
	tile := w.Map.GetTile(pos)
	if tile == nil {
		return 0
	}
	return len(tile.Items)
}

// ShopBuyPrice returns the NPC's buy price for an item, and whether it sells it at
// all. Mirrors the shop-vector scan in onPlayerBuyItem, which only accepts an entry
// whose itemBuyPrice is non-zero.
func (n *Npc) ShopBuyPrice(itemID uint16, subType uint8) (uint32, bool) {
	if n == nil || n.Type == nil {
		return 0, false
	}
	for _, si := range n.Type.ShopItems {
		if si.ID != itemID {
			continue
		}
		if si.SubType != 0 && si.SubType != subType {
			continue
		}
		if si.BuyPrice == 0 {
			continue
		}
		return si.BuyPrice, true
	}
	return 0, false
}

// ShopSellPrice returns the NPC's sell price for an item (what it pays the player).
func (n *Npc) ShopSellPrice(itemID uint16, subType uint8) (uint32, bool) {
	if n == nil || n.Type == nil {
		return 0, false
	}
	for _, si := range n.Type.ShopItems {
		if si.ID != itemID {
			continue
		}
		if si.SubType != 0 && si.SubType != subType {
			continue
		}
		if si.SellPrice == 0 {
			continue
		}
		return si.SellPrice, true
	}
	return 0, false
}

// ShopPurchase is the outcome of delivering a shop purchase.
type ShopPurchase struct {
	Delivered uint32 // units actually placed in the inventory
	Charged   uint64 // gold taken, goods plus shopping bags
	BagsCost  uint64
}

// SellItemTo delivers a purchase and charges for it, porting the delivery half of
// luaNpcSellItem (npc_functions.cpp:569): create the items, then remove
// itemCost + backpackCost.
//
// It charges only for what was actually delivered, so a partial delivery (the
// inventory filled up) leaves the player paying for the items they received. The
// caller owns the "Bought Nx ..." message, because the wording differs between the
// protocol path and the Lua path.
//
// Returns ok=false when nothing could be delivered; nothing is charged in that case.
func (n *Npc) SellItemTo(p *Player, catalog *items.Catalog, itemID uint16, amount uint16, subType int, inBackpacks bool) (ShopPurchase, bool) {
	var out ShopPurchase
	if n == nil || p == nil || amount == 0 {
		return out, false
	}

	price, found := n.ShopBuyPrice(itemID, uint8(subType))
	if !found {
		return out, false
	}

	placed, _ := p.InternalAddItem(catalog, itemID, uint32(amount), subType, ConstSlotWhereever)
	for _, it := range placed {
		if it == nil {
			continue
		}
		if it.Count == 0 {
			out.Delivered++
			continue
		}
		out.Delivered += uint32(it.Count)
	}
	if out.Delivered == 0 {
		return out, false
	}

	out.BagsCost = CalculateBagsCost(catalog.Get(itemID), uint16(out.Delivered), inBackpacks)
	out.Charged = uint64(price)*uint64(out.Delivered) + out.BagsCost

	// Bank is allowed as a fallback, matching removeMoney(..., useBank = true).
	if !p.RemoveMoney(out.Charged, true) {
		return out, false
	}
	return out, true
}
