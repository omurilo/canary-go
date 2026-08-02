package game

// The four Npc:: shop entry points, ported from src/creatures/npcs/npc.cpp.
//
// These are the guard layer between the protocol and the datapack's shop
// scripts. The port had the individual guards — backpack space, tile limit,
// funds — sitting in npc_shop.go with no single caller, and the protocol
// reached past them straight to the script. So the checks existed and did
// nothing: a player could buy into a full backpack, and the per-player shop
// list installed by a quest NPC was never consulted for the price.
//
// Upstream's ordering is load-bearing and reproduced exactly: room first, then
// tile limit, then price, then funds, and the script callback last. A callback
// that fires before the funds check has already changed the world by the time
// the purchase is refused.

// OnPlayerBuyItem is Npc::onPlayerBuyItem (npc.cpp:738). It returns the total
// cost and whether the purchase may proceed.
//
// The price is read from getShopItemVector, not from the type — that is the
// whole point of the per-player registry, and reading the type directly is what
// the port did.
func (n *Npc) OnPlayerBuyItem(w *World, p *Player, itemID uint16, subType uint8, amount uint16, ignore, inBackpacks bool) (uint64, bool) {
	if p == nil || w == nil {
		return 0, false
	}
	catalog := w.Items
	if p.IsBackpackSlotUnavailable(catalog, itemID, ignore) {
		p.SendCancelMessage(msgNotEnoughRoom)
		return 0, false
	}
	it := catalog.Get(itemID)
	if it == nil {
		return 0, false
	}
	if w.ExceedsTileLimit(p, it, amount, inBackpacks, ignore) {
		p.SendCancelMessage(msgNotEnoughRoom)
		return 0, false
	}

	var buyPrice uint32
	for _, block := range n.GetShopItemVector(p.DBID) {
		if block.ID == itemID && block.BuyPrice != 0 {
			buyPrice = block.BuyPrice
		}
	}

	totalCost := uint64(buyPrice) * uint64(amount)
	bagsCost := CalculateBagsCost(it, amount, inBackpacks)
	if p.HasInsufficientFunds(catalog, n.GetCurrency(), totalCost, bagsCost) {
		return 0, false
	}

	if w.OnNpcBuyItem != nil {
		w.OnNpcBuyItem(n, p, itemID, subType, amount, ignore, inBackpacks, totalCost)
	}
	return totalCost, true
}

// OnPlayerSellItem is Npc::onPlayerSellItem (npc.cpp:793). Upstream splits it in
// two: the public entry point starts a zero total and hands off to the version
// carrying a running total, which is what lets sell-all accumulate across items.
func (n *Npc) OnPlayerSellItem(w *World, p *Player, itemID uint16, subType uint8, amount uint32, ignore bool) uint64 {
	var totalPrice uint64
	n.sellItemInto(w, p, itemID, subType, amount, ignore, &totalPrice)
	return totalPrice
}

// sellItemInto is the SellItemContext overload (npc.cpp:910): one item type,
// adding to a caller-owned running total.
func (n *Npc) sellItemInto(w *World, p *Player, itemID uint16, subType uint8, amount uint32, ignore bool, totalPrice *uint64) {
	if p == nil || w == nil {
		return
	}
	var sellPrice uint32
	for _, block := range n.GetShopItemVector(p.DBID) {
		if block.ID == itemID && block.SellPrice != 0 {
			sellPrice = block.SellPrice
		}
	}
	if sellPrice == 0 {
		return
	}
	*totalPrice += uint64(sellPrice) * uint64(amount)

	if w.OnNpcSellItem != nil {
		w.OnNpcSellItem(n, p, itemID, subType, amount, ignore, uint64(sellPrice)*uint64(amount))
	}
}

// OnPlayerSellAllLoot is Npc::onPlayerSellAllLoot (npc.cpp:798): sweep the
// player's loot pouch and sell everything the NPC buys, accumulating one total.
//
// The total is threaded through rather than summed per call because the script
// callback receives the running figure — a player selling forty stacks gets one
// "you sold X for Y gold", not forty.
func (n *Npc) OnPlayerSellAllLoot(w *World, p *Player, ignore bool) uint64 {
	if p == nil || w == nil {
		return 0
	}
	var totalPrice uint64
	// Upstream sweeps the loot pouch; this port has no pouch abstraction yet, so
	// the main backpack stands in. Same items in practice, and the sweep is
	// depth-one either way — a bag inside the backpack is not emptied.
	sellable := make(map[uint16]uint32)
	if bp := p.Inventory[ConstSlotBackpack]; bp != nil {
		for _, item := range bp.Contents {
			if item == nil {
				continue
			}
			count := uint32(item.Count)
			if count == 0 {
				count = 1
			}
			sellable[item.ID] += count
		}
	}
	for itemID, amount := range sellable {
		n.sellItemInto(w, p, itemID, 0, amount, ignore, &totalPrice)
	}
	return totalPrice
}

// OnPlayerCheckItem is Npc::onPlayerCheckItem (npc.cpp:1007): the player looked
// at an item in the shop window. Upstream's body is only the script callback.
func (n *Npc) OnPlayerCheckItem(w *World, p *Player, itemID uint16, subType uint8) {
	if p == nil || w == nil || w.OnNpcCheckItem == nil {
		return
	}
	w.OnNpcCheckItem(n, p, itemID, subType)
}

// msgNotEnoughRoom is the text of RETURNVALUE_NOTENOUGHROOM.
const msgNotEnoughRoom = "There is not enough room."
