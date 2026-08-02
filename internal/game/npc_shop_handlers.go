package game

import (
	"fmt"

	"github.com/omurilo/canary-go/internal/creatures"
)

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

// getSellPriceForItem is the free function of the same name (npc.cpp:144).
//
// It returns the FIRST non-zero match, where the buy-price loop above returns
// the LAST. That asymmetry is upstream's and it is visible: an NPC whose list
// names the same item twice sells at the later price and buys at the earlier
// one.
func getSellPriceForItem(shopVector []creatures.ShopItem, itemID uint16) uint32 {
	for _, block := range shopVector {
		if block.ID == itemID && block.SellPrice != 0 {
			return block.SellPrice
		}
	}
	return 0
}

// OnPlayerSellItem is Npc::onPlayerSellItem (npc.cpp:793). Upstream splits it in
// two: the public entry point starts a zero total and hands off to the version
// carrying a running total, which is what lets sell-all accumulate across items.
func (n *Npc) OnPlayerSellItem(w *World, p *Player, itemID uint16, subType uint8, amount uint32, ignore bool) uint64 {
	var totalPrice uint64
	n.sellItemInto(w, p, itemID, subType, amount, ignore, &totalPrice, false)
	return totalPrice
}

// sellItemInto is the SellItemContext overload (npc.cpp:910): one item type,
// adding to a caller-owned running total.
//
// fromLootPouch stands in for context.lootPouch. Upstream uses the pointer both
// as the container to remove from and as the "this is part of a sell-all" flag;
// here the sweep removes its own items, so only the flag survives — and it is
// load-bearing three times over: it suppresses the per-item "no items to sell"
// message, suppresses the bank-transfer notice, and suppresses the onSellItem
// callback, so a sell-all fires one message rather than forty.
func (n *Npc) sellItemInto(w *World, p *Player, itemID uint16, subType uint8, amount uint32, ignore bool, totalPrice *uint64, fromLootPouch bool) {
	if p == nil || w == nil {
		return
	}

	// Selling the gold pouch itself is the sell-all-loot button. Upstream
	// schedules it onto the player-action lane to get out of the current call;
	// running it inline here reaches the same state, and the comment is the note
	// that the deferral was dropped deliberately.
	if itemID == ItemGoldPouch && !fromLootPouch {
		n.OnPlayerSellAllLoot(w, p, ignore)
		return
	}

	sellPrice := getSellPriceForItem(n.GetShopItemVector(p.DBID), itemID)
	if sellPrice == 0 {
		return
	}

	currency := n.GetCurrency()
	if currency != GoldCoinID {
		// applyCustomSaleProceeds pays in a shop-defined currency item. Paying in
		// gold instead would be a silent exchange-rate bug, so the sale is refused.
		p.SendTextMessage(msgEventAdvance, "An error occurred while completing the sale. Your items were not exchanged.")
		return
	}

	sub := -1
	if subType != 0 {
		sub = int(subType)
	}
	soldAmount := p.RemoveForSale(w.Items, itemID, amount, sub)
	if soldAmount == 0 {
		if !fromLootPouch {
			p.SendTextMessage(msgEventAdvance, "You have no items to sell.")
		}
		return
	}

	totalCost := uint64(sellPrice) * uint64(soldAmount)
	*totalPrice += totalCost
	applyGoldSaleProceeds(w, p, totalCost, !fromLootPouch)

	if fromLootPouch {
		return
	}
	if w.OnNpcSellItem != nil {
		w.OnNpcSellItem(n, p, itemID, subType, soldAmount, ignore, totalCost)
	}
}

// applyGoldSaleProceeds is the free function of the same name (npc.cpp:268).
//
// With AUTOBANK on the proceeds go to the bank and never touch the inventory;
// crediting inventory coins regardless — which is what the protocol layer did —
// hands the player carryable gold a server with AUTOBANK expects to be banked.
func applyGoldSaleProceeds(w *World, p *Player, totalCost uint64, notifyBankTransfer bool) {
	if w.AutoBank {
		p.BankBalance += totalCost
		if notifyBankTransfer {
			p.SendTextMessage(msgEventAdvance, fmt.Sprintf("%d gold coins transferred to your bank.", totalCost))
		}
		return
	}
	p.AddMoney(totalCost)
}

// OnPlayerSellAllLoot is Npc::onPlayerSellAllLoot (npc.cpp:798): sweep the
// player's loot pouch and sell everything the NPC buys, accumulating one total.
//
// The total is threaded through rather than summed per call because the player
// gets one "you sold X items for Y gold", not one line per stack.
func (n *Npc) OnPlayerSellAllLoot(w *World, p *Player, ignore bool) uint64 {
	if p == nil || w == nil {
		return 0
	}
	// The gold pouch, not the backpack. Sweeping the backpack would sell the
	// player's equipment along with their loot.
	pouch := p.GetLootPouch()
	if pouch == nil {
		return 0
	}

	// Only what this NPC actually buys is a candidate. Upstream seeds the map
	// from the shop vector first and then counts the pouch against it, so an item
	// the NPC does not buy is never even looked at.
	shopVector := n.GetShopItemVector(p.DBID)
	prices := make(map[uint16]uint32, len(shopVector))
	for _, block := range shopVector {
		if block.SellPrice == 0 {
			continue
		}
		prices[block.ID] = block.SellPrice
	}
	if len(prices) == 0 {
		p.SendTextMessage(msgTransaction, "You have no items in your loot pouch.")
		return 0
	}

	amounts := make(map[uint16]uint32, len(prices))
	collectSellable(pouch, prices, amounts)

	var pendingTotal uint64
	var totalItemsSold uint32
	for itemID, amount := range amounts {
		cost := uint64(prices[itemID]) * uint64(amount)
		if cost == 0 {
			continue
		}
		pendingTotal += cost
		totalItemsSold += amount
	}

	var totalPrice uint64
	if pendingTotal > 0 {
		applyGoldSaleProceeds(w, p, pendingTotal, false)
		for itemID, amount := range amounts {
			removeItemsFromLootPouch(w, pouch, itemID, amount)
		}
		totalPrice += pendingTotal
	}

	if pendingTotal == 0 {
		p.SendTextMessage(msgTransaction, "You have no items in your loot pouch.")
	} else {
		plural := "s"
		if totalItemsSold == 1 {
			plural = ""
		}
		// Upstream also mails a per-item breakdown to the store inbox
		// (sendSaleLetterIfNeeded). That needs the store inbox container, which the
		// port does not model yet, so the letter half is missing and the summary
		// line is not.
		p.SendTextMessage(msgTransaction, fmt.Sprintf(
			"You sold %d item%s from your loot pouch for %d gold.", totalItemsSold, plural, pendingTotal))
	}
	return totalPrice
}

// removeItemsFromLootPouch is the free function of the same name (npc.cpp:196):
// take up to amount of itemID out of the pouch, descending into nested bags.
func removeItemsFromLootPouch(w *World, pouch *Item, itemID uint16, amount uint32) uint32 {
	remaining := amount
	var walk func(container *Item)
	walk = func(container *Item) {
		out := container.Contents[:0]
		for _, item := range container.Contents {
			if item == nil {
				continue
			}
			if len(item.Contents) > 0 {
				walk(item)
				out = append(out, item)
				continue
			}
			if remaining > 0 && item.ID == itemID && itemIsSellable(item) {
				removed := consumeItem(w.Items, item, remaining)
				remaining -= removed
				if item.Count == 0 {
					continue
				}
			}
			out = append(out, item)
		}
		container.Contents = out
	}
	walk(pouch)
	return amount - remaining
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

// MessageClasses values used by the shop (utils_definitions.hpp:378, :408).
const (
	msgEventAdvance = 19
	msgTransaction  = 51
)

// GetLootPouch is Player::getLootPouch: the gold pouch, wherever it sits in the
// inventory tree. It is a container the player cannot drop, which is why
// onPlayerSellAllLoot sweeps it and not the backpack.
func (p *Player) GetLootPouch() *Item {
	for _, item := range p.Inventory {
		if item == nil {
			continue
		}
		if item.ID == ItemGoldPouch {
			return item
		}
		if found := findItemInContainerTree(item, ItemGoldPouch); found != nil {
			return found
		}
	}
	return nil
}

// findItemInContainerTree is getInventoryItemsFromId's recursive half.
func findItemInContainerTree(parent *Item, id uint16) *Item {
	if parent == nil {
		return nil
	}
	for _, child := range parent.Contents {
		if child == nil {
			continue
		}
		if child.ID == id {
			return child
		}
		if found := findItemInContainerTree(child, id); found != nil {
			return found
		}
	}
	return nil
}

// collectSellable walks the pouch recursively, counting only what the NPC buys.
// Upstream descends into nested containers — a bag of loot inside the pouch is
// sold too, which is the whole point of the sell-all button.
//
// Tiered and imbued items are skipped. They carry state worth more than the
// shop price and selling them by accident is unrecoverable, which is why
// upstream guards it here rather than trusting the player to unload first.
func collectSellable(container *Item, prices map[uint16]uint32, out map[uint16]uint32) {
	for _, item := range container.Contents {
		if item == nil {
			continue
		}
		if len(item.Contents) > 0 {
			collectSellable(item, prices, out)
			continue
		}
		if _, wanted := prices[item.ID]; !wanted {
			continue
		}
		if item.GetTier() > 0 || item.HasImbuements() {
			continue
		}
		count := uint32(item.Count)
		if count == 0 {
			count = 1
		}
		out[item.ID] += count
	}
}
