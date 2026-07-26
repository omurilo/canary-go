package protocol

import (
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// Market fee: 2% of the total cost.
const marketFeePercent uint64 = 2

// ──────────────────────────────────────────────────────────────────────────────
// Outbound packets
// ──────────────────────────────────────────────────────────────────────────────

// SendOpenMarket opens the market window on the client (opcode 0xF6).
// It sends the player's depot contents and bank balance.
func (g *GameProtocol) SendOpenMarket() {
	if g.player == nil {
		return
	}
	g.player.InMarket = true

	w := netmsg.NewWriter()
	w.AddByte(0xF6) // opMarketEnter

	// Bank balance (u64 LE).
	w.AddU64(g.player.BankBalance)

	// Depot items: flatten depot lockers into (depotId, itemId, amount, tier).
	depotItems := g.collectDepotItems()
	w.AddU16(uint16(len(depotItems)))
	for _, di := range depotItems {
		w.AddU16(di.depotID)
		w.AddU16(di.itemID)
		w.AddU16(di.amount)
		if di.tier > 0 {
			w.AddByte(di.tier)
		}
	}

	g.SendToClient(w)
}

type depotItem struct {
	depotID uint16
	itemID  uint16
	amount  uint16
	tier    uint8
}

// collectDepotItems flattens the player's depot lockers into a slice for sending.
func (g *GameProtocol) collectDepotItems() []depotItem {
	if g.player.DepotLockers == nil {
		return nil
	}
	var items []depotItem
	for depotID, locker := range g.player.DepotLockers {
		if locker == nil {
			continue
		}
		g.collectContainerItems(depotID, locker, &items)
	}
	return items
}

// collectContainerItems recursively collects items from a container tree.
func (g *GameProtocol) collectContainerItems(depotID uint16, container *game.Item, items *[]depotItem) {
	if container == nil || container.IsContainer(nil) {
		// It's a container — recurse into Contents.
	} else {
		return
	}
	for _, child := range container.Contents {
		if child == nil {
			continue
		}
		if child.IsContainer(nil) {
			g.collectContainerItems(depotID, child, items)
		} else {
			*items = append(*items, depotItem{
				depotID: depotID,
				itemID:  child.ID,
				amount:  child.Count,
				tier:    child.GetTier(),
			})
		}
	}
}

// SendMarketBrowse sends the list of buy/sell offers for a specific item (opcode 0xF7).
func (g *GameProtocol) SendMarketBrowse(itemId uint16, tier uint8) {
	if g.player == nil || g.player.World == nil || g.player.World.Market == nil {
		return
	}
	market := g.player.World.Market
	buyOffers := market.GetBuyOffers(itemId)
	sellOffers := market.GetSellOffers(itemId)

	w := netmsg.NewWriter()
	w.AddByte(0xF7) // opMarketBrowseItem

	w.AddU16(itemId)
	w.AddByte(tier)

	// Buy offers
	w.AddU16(uint16(len(buyOffers)))
	for _, offer := range buyOffers {
		w.AddU16(offer.Amount)
		w.AddU64(offer.Price)
		w.AddU16(offer.Counter)
		if offer.Anonymous {
			w.AddByte(1)
		} else {
			w.AddByte(0)
		}
		w.AddU32(uint32(offer.Timestamp))
	}

	// Sell offers
	w.AddU16(uint16(len(sellOffers)))
	for _, offer := range sellOffers {
		w.AddU16(offer.Amount)
		w.AddU64(offer.Price)
		w.AddU16(offer.Counter)
		if offer.Anonymous {
			w.AddByte(1)
		} else {
			w.AddByte(0)
		}
		w.AddU32(uint32(offer.Timestamp))
	}

	g.SendToClient(w)
}

// SendMarketDetail sends item detail information (opcode 0xF8).
func (g *GameProtocol) SendMarketDetail(itemId uint16, tier uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0xF8) // opMarketDetail

	w.AddU16(itemId)
	w.AddByte(tier)

	// C++ sends: item name, tooltip, attack, defense, armor, etc.
	// For simplicity we send empty strings — the client has its own items.xml.
	w.AddString("") // item name
	w.AddString("") // tooltip

	g.SendToClient(w)
}

// SendMarketAccept sends the result of accepting a market offer (opcode 0xF9).
func (g *GameProtocol) SendMarketAccept(timestamp uint32, counter uint16, amount uint16) {
	w := netmsg.NewWriter()
	w.AddByte(0xF9) // opMarketAcceptOffer

	w.AddU32(timestamp)
	w.AddU16(counter)
	w.AddU16(amount)

	g.SendToClient(w)
}

// SendMarketBrowseOwnOffers sends the player's own active offers (opcode 0xF5).
func (g *GameProtocol) SendMarketBrowseOwnOffers() {
	if g.player == nil || g.player.World == nil || g.player.World.Market == nil {
		return
	}
	market := g.player.World.Market
	buyOffers := market.GetPlayerOffersByAction(g.player.GetID(), game.MarketActionBuy)
	sellOffers := market.GetPlayerOffersByAction(g.player.GetID(), game.MarketActionSell)

	w := netmsg.NewWriter()
	w.AddByte(0xF5) // opMarketBrowseOwnOffers

	// Buy offers
	w.AddU16(uint16(len(buyOffers)))
	for _, offer := range buyOffers {
		w.AddU16(offer.ItemID)
		w.AddU16(offer.Amount)
		w.AddU64(offer.Price)
		w.AddU16(offer.Counter)
		w.AddU32(uint32(offer.Timestamp))
		if offer.Anonymous {
			w.AddByte(1)
		} else {
			w.AddByte(0)
		}
	}

	// Sell offers
	w.AddU16(uint16(len(sellOffers)))
	for _, offer := range sellOffers {
		w.AddU16(offer.ItemID)
		w.AddU16(offer.Amount)
		w.AddU64(offer.Price)
		w.AddU16(offer.Counter)
		w.AddU32(uint32(offer.Timestamp))
		if offer.Anonymous {
			w.AddByte(1)
		} else {
			w.AddByte(0)
		}
	}

	g.SendToClient(w)
}

// SendMarketBrowseOwnHistory sends the player's historical market offers (opcode 0xF5).
func (g *GameProtocol) SendMarketBrowseOwnHistory() {
	if g.player == nil {
		return
	}
	// TODO: implement history tracking; send empty lists for now.
	w := netmsg.NewWriter()
	w.AddByte(0xF5) // same opcode for own offers and history
	w.AddU16(0)     // buy offers count = 0
	w.AddU16(0)     // sell offers count = 0
	g.SendToClient(w)
}

// SendMarketCancel confirms an offer was cancelled (opcode 0xFA).
func (g *GameProtocol) SendMarketCancel() {
	w := netmsg.NewWriter()
	w.AddByte(0xFA) // opMarketCancelOffer
	g.SendToClient(w)
}

// ──────────────────────────────────────────────────────────────────────────────
// Inbound packet parsers
// ──────────────────────────────────────────────────────────────────────────────

// parseMarketLeave is called when the player leaves the market window (0xF3/0xF4).
func (g *GameProtocol) parseMarketLeave() {
	if g.player == nil {
		return
	}
	g.player.InMarket = false
}

// parseMarketBrowse handles item browsing / own offers / own history (0xF5).
// Wire: [u8 browseId] — 0=own offers, 1=own history, 2..=browse item by itemId
func (g *GameProtocol) parseMarketBrowse(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	browseId := r.GetByte()
	switch browseId {
	case 0:
		g.SendMarketBrowseOwnOffers()
	case 1:
		g.SendMarketBrowseOwnHistory()
	default:
		itemId := r.GetU16()
		g.SendMarketBrowse(itemId, 0)
	}
}

// parseMarketCreateOffer handles creating a buy or sell offer (0xF6).
// Wire: [u8 type][u16 itemId][u16 amount][u64 price][u8 tier][u8 anonymous]
func (g *GameProtocol) parseMarketCreateOffer(r *netmsg.Reader) {
	if g.player == nil || !g.player.InMarket {
		return
	}
	offerType := r.GetByte() // 0=buy, 1=sell
	itemId := r.GetU16()
	amount := r.GetU16()
	price := r.GetU64()
	tier := r.GetByte()
	anonymous := r.GetByte() != 0

	if amount == 0 || price == 0 {
		return
	}

	if game.MarketAction(offerType) == game.MarketActionBuy {
		// Buy offer: player pays (price * amount) + 2% fee from bank balance.
		totalCost := price * uint64(amount)
		fee := totalCost * marketFeePercent / 100
		totalWithFee := totalCost + fee
		if g.player.BankBalance < totalWithFee {
			return
		}
		g.player.BankBalance -= totalWithFee
	} else {
		// Sell offer: verify and remove items from depot.
		if !g.hasItemsInDepot(itemId, amount, tier) {
			return
		}
		g.removeItemsFromDepot(itemId, amount, tier)
	}

	market := g.player.World.Market
	if market == nil {
		return
	}

	offer := &game.MarketOffer{
		PlayerID:   g.player.GetID(),
		PlayerName: g.player.GetName(),
		ItemID:     itemId,
		Amount:     amount,
		Price:      price,
		Tier:       tier,
		Timestamp:  time.Now().Unix(),
		Anonymous:  anonymous,
		Action:     game.MarketAction(offerType),
	}
	_ = market.AddOffer(offer)

	// TODO: persist offer to DB asynchronously.

	g.SendOpenMarket()
}

// parseMarketCancelOffer handles cancelling an existing offer (0xF7).
// Wire: [u32 timestamp][u16 counter]
func (g *GameProtocol) parseMarketCancelOffer(r *netmsg.Reader) {
	if g.player == nil || !g.player.InMarket {
		return
	}
	timestamp := r.GetU32()
	counter := r.GetU16()

	market := g.player.World.Market
	if market == nil {
		return
	}

	offer := market.GetOfferByCounter(timestamp, counter)
	if offer == nil || offer.PlayerID != g.player.GetID() {
		return
	}

	// Return items/bank balance to the player.
	if offer.Action == game.MarketActionBuy {
		totalCost := offer.Price * uint64(offer.Amount)
		fee := totalCost * marketFeePercent / 100
		g.player.BankBalance += totalCost + fee
	} else {
		g.returnItemsToDepot(offer.ItemID, offer.Amount, offer.Tier)
	}

	market.RemoveOffer(offer.ID)
	g.SendOpenMarket()
}

// parseMarketAcceptOffer handles accepting (executing) an existing offer (0xF8).
// Wire: [u32 timestamp][u16 counter][u16 amount]
func (g *GameProtocol) parseMarketAcceptOffer(r *netmsg.Reader) {
	if g.player == nil || !g.player.InMarket {
		return
	}
	timestamp := r.GetU32()
	counter := r.GetU16()
	amount := r.GetU16()

	if amount == 0 {
		return
	}

	market := g.player.World.Market
	if market == nil {
		return
	}

	offer := market.GetOfferByCounter(timestamp, counter)
	if offer == nil || offer.PlayerID == g.player.GetID() {
		return // can't accept own offer
	}

	if amount > offer.Amount {
		amount = offer.Amount
	}

	totalCost := offer.Price * uint64(amount)

	if offer.Action == game.MarketActionSell {
		// Seller offers items; buyer (this player) pays gold.
		if g.player.BankBalance < totalCost {
			return
		}
		g.player.BankBalance -= totalCost
		g.returnItemsToDepot(offer.ItemID, amount, offer.Tier)
		g.creditSeller(offer.PlayerID, totalCost)
	} else {
		// Buyer offers gold; seller (this player) provides items.
		if !g.hasItemsInDepot(offer.ItemID, amount, offer.Tier) {
			return
		}
		g.removeItemsFromDepot(offer.ItemID, amount, offer.Tier)
		g.player.BankBalance += totalCost
	}

	offer.Amount -= amount
	if offer.Amount == 0 {
		market.RemoveOffer(offer.ID)
	}

	g.SendMarketAccept(timestamp, counter, amount)
	g.SendOpenMarket()
}

// ──────────────────────────────────────────────────────────────────────────────
// Depot item helpers
// ──────────────────────────────────────────────────────────────────────────────

// hasItemsInDepot checks if the player has the required items in any depot locker.
func (g *GameProtocol) hasItemsInDepot(itemId uint16, amount uint16, tier uint8) bool {
	if g.player.DepotLockers == nil {
		return false
	}
	var count uint16
	for _, locker := range g.player.DepotLockers {
		if locker == nil {
			continue
		}
		count += g.countItemInContainer(locker, itemId, tier)
		if count >= amount {
			return true
		}
	}
	return count >= amount
}

// countItemInContainer recursively counts items matching itemId/tier in a container.
func (g *GameProtocol) countItemInContainer(container *game.Item, itemId uint16, tier uint8) uint16 {
	if container == nil || !container.IsContainer(nil) {
		return 0
	}
	var count uint16
	for _, child := range container.Contents {
		if child == nil {
			continue
		}
		if child.IsContainer(nil) {
			count += g.countItemInContainer(child, itemId, tier)
		} else if child.ID == itemId && (tier == 0 || child.GetTier() == tier) {
			count += child.Count
		}
	}
	return count
}

// removeItemsFromDepot removes items from the player's depot lockers.
func (g *GameProtocol) removeItemsFromDepot(itemId uint16, amount uint16, tier uint8) {
	if g.player.DepotLockers == nil {
		return
	}
	remaining := amount
	for _, locker := range g.player.DepotLockers {
		if locker == nil || remaining == 0 {
			continue
		}
		remaining = g.removeFromContainer(locker, itemId, tier, remaining)
	}
}

// removeFromContainer removes up to `amount` matching items from a container tree.
func (g *GameProtocol) removeFromContainer(container *game.Item, itemId uint16, tier uint8, amount uint16) uint16 {
	if container == nil || !container.IsContainer(nil) || amount == 0 {
		return amount
	}
	remaining := amount
	for _, child := range container.Contents {
		if child == nil || remaining == 0 {
			continue
		}
		if child.IsContainer(nil) {
			remaining = g.removeFromContainer(child, itemId, tier, remaining)
		} else if child.ID == itemId && (tier == 0 || child.GetTier() == tier) {
			take := child.Count
			if take > remaining {
				take = remaining
			}
			child.Count -= take
			remaining -= take
		}
	}
	g.cleanEmptyItems(container)
	return remaining
}

// cleanEmptyItems removes items with zero count from a container.
func (g *GameProtocol) cleanEmptyItems(container *game.Item) {
	if container == nil || !container.IsContainer(nil) {
		return
	}
	j := 0
	for _, item := range container.Contents {
		if item != nil && item.Count > 0 {
			container.Contents[j] = item
			j++
		}
	}
	for i := j; i < len(container.Contents); i++ {
		container.Contents[i] = nil
	}
	container.Contents = container.Contents[:j]
}

// returnItemsToDepot puts items back into the player's first depot locker.
func (g *GameProtocol) returnItemsToDepot(itemId uint16, amount uint16, tier uint8) {
	if g.player.DepotLockers == nil {
		return
	}
	for _, locker := range g.player.DepotLockers {
		if locker == nil {
			continue
		}
		g.addToContainer(locker, itemId, tier, amount)
		return
	}
}

// addToContainer adds items to the first matching slot in a container.
func (g *GameProtocol) addToContainer(container *game.Item, itemId uint16, tier uint8, amount uint16) {
	if container == nil || !container.IsContainer(nil) {
		return
	}
	// Stack with existing items first.
	for _, child := range container.Contents {
		if child != nil && child.ID == itemId && (tier == 0 || child.GetTier() == tier) {
			child.Count += amount
			return
		}
	}
	// TODO: create a new Item instance and append — requires items.Catalog reference.
}

// creditSeller adds gold to a seller's bank balance (online player or DB update).
func (g *GameProtocol) creditSeller(sellerID uint32, amount uint64) {
	seller := g.player.World.PlayerByID(sellerID)
	if seller != nil {
		seller.BankBalance += amount
		return
	}
	// TODO: seller is offline — update DB directly.
}
