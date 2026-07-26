package protocol

import (
	"context"
	"log/slog"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// Market fee: 2% of the total cost.
const marketFeePercent uint64 = 2

// ──────────────────────────────────────────────────────────────────────────────
// Outbound packets
// ──────────────────────────────────────────────────────────────────────────────

// depotEntry aggregates depot items by (itemId, tier) → total count.
type depotEntry struct {
	itemID uint16
	tier   uint8
	count  uint16
}

// SendOpenMarket opens the market window on the client (opcode 0xF6).
// Format mirrors C++ ProtocolGame::sendMarketEnter:
//
//	[0xF6][u8 offerCount][u16 itemCount] then per-item { [u16 itemId][u8 tier*][u16 count] }
//	* tier byte only sent when tier > 0.
func (g *GameProtocol) SendOpenMarket() {
	if g.player == nil || g.player.World == nil || g.player.World.Market == nil {
		return
	}
	g.player.InMarket = true

	w := netmsg.NewWriter()
	w.AddByte(0xF6) // opMarketEnter

	// Number of active offers the player currently has (capped at u8).
	offerCount := g.player.World.Market.GetPlayerOfferCount(g.player.DBID)
	if offerCount > 255 {
		offerCount = 255
	}
	w.AddByte(uint8(offerCount))

	// Depot items: aggregate by (itemId, tier) → total count.
	entries := g.collectDepotItems()
	w.AddU16(uint16(len(entries)))
	for _, e := range entries {
		w.AddU16(e.itemID)
		t := g.deps.Items.Get(e.itemID)
		if t != nil && t.UpgradeClassification > 0 {
			w.AddByte(e.tier)
		}
		w.AddU16(e.count)
	}

	g.SendToClient(w)

	// C++ also sends updateCoinBalance and sendResourcesBalance after market enter.
	g.sendCoinBalance()
	g.sendResourcesBalance()
}

// collectDepotItems aggregates items from all depot chests by (itemId, tier).
func (g *GameProtocol) collectDepotItems() []depotEntry {
	if g.player.DepotManager == nil {
		slog.Default().Info("collectDepotItems: DepotManager nil")
		return nil
	}
	slog.Default().Info("collectDepotItems", "chestsCount", len(g.player.DepotManager.Chests))
	agg := make(map[uint32]uint16) // key = (itemId<<8 | tier), value = count
	for _, chest := range g.player.DepotManager.Chests {
		if chest == nil {
			continue
		}
		g.aggregateContainerItems(chest, agg)
	}
		// Add Tibia Coins from the account balance as virtual depot items.
	if g.player.CoinTransferable > 0 {
		coinKey := (uint32(game.ItemStoreCoin) << 8) | 0
		if existing, ok := agg[coinKey]; ok {
			agg[coinKey] = existing + uint16(g.player.CoinTransferable)
		} else {
			agg[coinKey] = uint16(g.player.CoinTransferable)
		}
	}
	if len(agg) == 0 {
		return nil
	}
	entries := make([]depotEntry, 0, len(agg))
	for key, count := range agg {
		itemID := uint16(key >> 8)
		tier := uint8(key & 0xFF)
		entries = append(entries, depotEntry{itemID: itemID, tier: tier, count: count})
	}
	return entries
}

// aggregateContainerItems recursively aggregates items in a container tree.
func (g *GameProtocol) aggregateContainerItems(container *game.Item, agg map[uint32]uint16) {
	if container == nil || !container.IsContainer(g.deps.Items) {
		return
	}
	for _, child := range container.Contents {
		if child == nil {
			continue
		}
		if child.IsContainer(g.deps.Items) {
			g.aggregateContainerItems(child, agg)
		} else {
			key := (uint32(child.ID) << 8) | uint32(child.GetTier())
			agg[key] += child.Count
		}
	}
}

// SendMarketBrowse sends the list of buy/sell offers for a specific item (opcode 0xF9).
// Mirrors C++ ProtocolGame::sendMarketBrowseItem:
//
//	[0xF9][u8 MARKETREQUEST_ITEM_BROWSE=2][u16 itemId][u8 tier?]
//	[u32 buyCount] per-offer: [u32 timestamp][u16 counter][u16 amount][u64 price][string playerName]
//	[u32 sellCount] per-offer: [u32 timestamp][u16 counter][u16 amount][u64 price][string playerName]
//
// Tier byte is only sent when the item has forge tier > 0.
func (g *GameProtocol) SendMarketBrowse(itemId uint16, tier uint8) {
	if g.player == nil || g.player.World == nil || g.player.World.Market == nil {
		return
	}
	market := g.player.World.Market
	buyOffers := market.GetBuyOffers(itemId)
	sellOffers := market.GetSellOffers(itemId)

	w := netmsg.NewWriter()
	w.AddByte(0xF9) // opcode for market browse response
	w.AddByte(3)    // MARKETREQUEST_ITEM_BROWSE

	w.AddU16(itemId)
	t := g.deps.Items.Get(itemId)
	if t != nil && t.UpgradeClassification > 0 {
		w.AddByte(tier)
	}

	w.AddU32(uint32(len(buyOffers)))
	for _, offer := range buyOffers {
		w.AddU32(uint32(offer.Timestamp))
		w.AddU16(offer.Counter)
		w.AddU16(offer.Amount)
		w.AddU64(offer.Price)
		if offer.Anonymous {
			w.AddString("Anonymous")
		} else {
			w.AddString(offer.PlayerName)
		}
	}

	w.AddU32(uint32(len(sellOffers)))
	for _, offer := range sellOffers {
		w.AddU32(uint32(offer.Timestamp))
		w.AddU16(offer.Counter)
		w.AddU16(offer.Amount)
		w.AddU64(offer.Price)
		if offer.Anonymous {
			w.AddString("Anonymous")
		} else {
			w.AddString(offer.PlayerName)
		}
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

// SendMarketBrowseOwnOffers sends the player's own active offers (opcode 0xF9).
// Mirrors C++ ProtocolGame::sendMarketBrowseOwnOffers:
//   [0xF9][u8 MARKETREQUEST_OWN_OFFERS=1]
//   [u32 buyCount] per-offer: [u32 timestamp][u16 counter][u16 itemId][u8 tier][u16 amount][u64 price]
//   [u32 sellCount] per-offer: [u32 timestamp][u16 counter][u16 itemId][u8 tier][u16 amount][u64 price]
// Tier byte is only sent when the item has forge tier > 0.
func (g *GameProtocol) SendMarketBrowseOwnOffers() {
	if g.player == nil || g.player.World == nil || g.player.World.Market == nil {
		slog.Default().Info("SendMarketBrowseOwnOffers: nil check failed", "player", g.player != nil, "world", g.player.World != nil, "market", g.player.World.Market != nil)
		return
	}
	market := g.player.World.Market
	buyOffers := market.GetPlayerOffersByAction(g.player.DBID, game.MarketActionBuy)
	sellOffers := market.GetPlayerOffersByAction(g.player.DBID, game.MarketActionSell)
	slog.Default().Info("SendMarketBrowseOwnOffers", "buyCount", len(buyOffers), "sellCount", len(sellOffers))

	w := netmsg.NewWriter()
	w.AddByte(0xF9) // opcode for own offers response
	w.AddByte(2)    // MARKETREQUEST_OWN_OFFERS

	w.AddU32(uint32(len(buyOffers)))
	for _, offer := range buyOffers {
		w.AddU32(uint32(offer.Timestamp))
		w.AddU16(offer.Counter)
		w.AddU16(offer.ItemID)
		t := g.deps.Items.Get(offer.ItemID)
		if t != nil && t.UpgradeClassification > 0 {
			w.AddByte(offer.Tier)
		}
		w.AddU16(offer.Amount)
		w.AddU64(offer.Price)
	}

	w.AddU32(uint32(len(sellOffers)))
	for _, offer := range sellOffers {
		w.AddU32(uint32(offer.Timestamp))
		w.AddU16(offer.Counter)
		w.AddU16(offer.ItemID)
		t := g.deps.Items.Get(offer.ItemID)
		if t != nil && t.UpgradeClassification > 0 {
			w.AddByte(offer.Tier)
		}
		w.AddU16(offer.Amount)
		w.AddU64(offer.Price)
	}

	g.SendToClient(w)
}

// SendMarketBrowseOwnHistory sends the player's historical market offers (opcode 0xF5).
func (g *GameProtocol) SendMarketBrowseOwnHistory() {
	if g.player == nil || g.player.World == nil || g.player.World.Market == nil {
		return
	}
	// TODO: implement history tracking; send empty lists for now.
	w := netmsg.NewWriter()
	w.AddByte(0xF5) // same opcode for own offers and history
	w.AddByte(1)    // MARKETREQUEST_OWN_HISTORY
	w.AddU32(0)     // buy history count
	w.AddU32(0)     // sell history count
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
	if g.player == nil || g.player.World == nil || g.player.World.Market == nil {
		return
	}
	g.player.InMarket = false
}

// parseMarketBrowse handles item browsing / own offers / own history (0xF5).
// Mirrors C++ ProtocolGame::parseMarketBrowse:
//   [u8 browseId] — 1=own offers, 2=own history, 3+=browse item by itemId
// If the player is not yet in the market, SendOpenMarket is sent first (mirrors
// C++ Game::playerBrowseMarket calling sendMarketEnter when !player->isInMarket()).
func (g *GameProtocol) parseMarketBrowse(r *netmsg.Reader) {
	if g.player == nil || g.player.World == nil || g.player.World.Market == nil {
		return
	}
	browseId := r.GetByte()

	// Auto-open the market window on first browse (client sends 0xF5 on open).
	if !g.player.InMarket {
		g.SendOpenMarket()
	}

	const marketRequestOwnHistory = 1
	const marketRequestOwnOffers = 2

	switch browseId {
	case marketRequestOwnOffers:
		g.SendMarketBrowseOwnOffers()
	case marketRequestOwnHistory:
		g.SendMarketBrowseOwnHistory()
	default:
		// C++ calls sendMarketEnter before browse for items.
		g.SendOpenMarket()
		itemId := r.GetU16()
		tier := uint8(0)
		t := g.deps.Items.Get(itemId)
		if t != nil && t.UpgradeClassification > 0 {
			tier = r.GetByte()
		}
		g.SendMarketBrowse(itemId, tier)
	}
}

// parseMarketCreateOffer handles creating a buy or sell offer (0xF6).
// Wire: [u8 type][u16 itemId][u16 amount][u64 price][u8 tier][u8 anonymous]
func (g *GameProtocol) parseMarketCreateOffer(r *netmsg.Reader) {
	if g.player == nil || !g.player.InMarket {
		slog.Default().Info("parseMarketCreateOffer: player nil or not in market", "player", g.player != nil, "inMarket", g.player != nil && g.player.InMarket)
		return
	}
	offerType := r.GetByte() // 0=buy, 1=sell
	itemId := r.GetU16()

	t := g.deps.Items.Get(itemId)
	var tier uint8
	if t != nil && t.UpgradeClassification > 0 {
		tier = r.GetByte()
	}

	amount := r.GetU16()
	price := r.GetU64()
	anonymous := r.GetByte() != 0
	slog.Default().Info("parseMarketCreateOffer: packet data", "offerType", offerType, "itemId", itemId, "amount", amount, "price", price, "tier", tier, "anonymous", anonymous)

	if amount == 0 || price == 0 {
		slog.Default().Info("parseMarketCreateOffer: amount or price zero", "amount", amount, "price", price)
		return
	}
	if game.MarketAction(offerType) == game.MarketActionBuy {
		// Buy offer: verify bank balance first.
		totalCost := price * uint64(amount)
		fee := totalCost * marketFeePercent / 100
		totalWithFee := totalCost + fee
		if g.player.BankBalance < totalWithFee {
			slog.Default().Info("parseMarketCreateOffer: insufficient bank balance", "balance", g.player.BankBalance, "needed", totalWithFee)
			return
		}
	} else {
		// Sell offer: verify items exist (deduct only after DB persist).
		if itemId == game.ItemStoreCoin {
			if uint32(amount) > g.player.CoinTransferable {
				slog.Default().Info("parseMarketCreateOffer: not enough coins", "amount", amount, "balance", g.player.CoinTransferable)
				return
			}
		} else {
			// Client may send tier > 0 for items that support forging, but
			// the actual items in the depot may have tier=0 (no forge applied).
			// Try the requested tier first, then fall back to tier=0.
			if tier > 0 && !g.hasItemsInDepot(itemId, amount, tier) && g.hasItemsInDepot(itemId, amount, 0) {
				tier = 0
			}
			if !g.hasItemsInDepot(itemId, amount, tier) {
				slog.Default().Info("parseMarketCreateOffer: not enough items in depot", "itemId", itemId, "amount", amount, "tier", tier)
				return
			}
		}
	}

	market := g.player.World.Market
	if market == nil {
		slog.Default().Info("parseMarketCreateOffer: market is nil")
		return
	}

	// Persist offer to DB FIRST (before modifying game state).
	offer := &game.MarketOffer{
		PlayerID:   g.player.DBID,
		PlayerName: g.player.GetName(),
		ItemID:     itemId,
		Amount:     amount,
		Price:      price,
		Tier:       tier,
		Timestamp:  time.Now().Unix(),
		Anonymous:  anonymous,
		Action:     game.MarketAction(offerType),
	}
	sale := uint8(offer.Action)
	slog.Default().Info("CreateMarketOffer", "playerID", offer.PlayerID, "itemID", offer.ItemID, "amount", offer.Amount)
	dbID, dbErr := g.deps.DB.CreateMarketOffer(context.Background(), offer, sale)
	if dbErr != nil {
		slog.Default().Info("parseMarketCreateOffer: failed to persist offer, no changes made", "err", dbErr)
		return
	}
	offer.ID = dbID

	// DB succeeded — now update in-memory state.
	if game.MarketAction(offerType) == game.MarketActionBuy {
		totalCost := price * uint64(amount)
		fee := totalCost * marketFeePercent / 100
		g.player.BankBalance -= totalCost + fee
	} else {
		if itemId == game.ItemStoreCoin {
			g.player.CoinTransferable -= uint32(amount)
		} else {
			g.removeItemsFromDepot(itemId, amount, tier)
		}
	}
	_ = market.AddOffer(offer)
	g.SendOpenMarket()
	g.SendMarketBrowse(itemId, tier)
	g.SendMarketBrowseOwnOffers()
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
	if offer == nil || offer.PlayerID != g.player.DBID {
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
		if _, err := g.deps.DB.RemoveMarketOffer(context.Background(), offer.ID); err != nil {
			slog.Default().Info("failed to remove market offer from DB", "err", err)
		}
	g.SendOpenMarket()
	g.SendMarketCancel()
	g.SendMarketBrowse(offer.ItemID, offer.Tier)
	g.SendMarketBrowseOwnOffers()
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
	if offer == nil || offer.PlayerID == g.player.DBID {
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
		if offer.ItemID == game.ItemStoreCoin {
			g.player.CoinTransferable += uint32(amount)
		} else {
			g.returnItemsToDepot(offer.ItemID, amount, offer.Tier)
		}
		g.creditSeller(offer.PlayerID, totalCost)
	} else {
		// Buyer offers gold; seller (this player) provides items.
		if offer.ItemID == game.ItemStoreCoin {
			if uint32(amount) > g.player.CoinTransferable {
				return
			}
			g.player.CoinTransferable -= uint32(amount)
		} else {
			if !g.hasItemsInDepot(offer.ItemID, amount, offer.Tier) {
				return
			}
			g.removeItemsFromDepot(offer.ItemID, amount, offer.Tier)
		}
		g.player.BankBalance += totalCost
	}

	offer.Amount -= amount
	if offer.Amount == 0 {
		market.RemoveOffer(offer.ID)
		if _, err := g.deps.DB.RemoveMarketOffer(context.Background(), offer.ID); err != nil {
			slog.Default().Info("failed to remove market offer from DB", "err", err)
		}
	}

	g.SendMarketAccept(timestamp, counter, amount)
	g.SendOpenMarket()
	g.SendMarketBrowse(offer.ItemID, offer.Tier)
	g.SendMarketBrowseOwnOffers()
}

// ──────────────────────────────────────────────────────────────────────────────
// Depot item helpers
// ──────────────────────────────────────────────────────────────────────────────

// hasItemsInDepot checks if the player has the required items in any depot chest.
func (g *GameProtocol) hasItemsInDepot(itemId uint16, amount uint16, tier uint8) bool {
	if g.player.DepotManager == nil {
		slog.Default().Info("hasItemsInDepot: DepotManager nil", "itemId", itemId, "amount", amount)
		return false
	}
	slog.Default().Info("hasItemsInDepot", "chestsCount", len(g.player.DepotManager.Chests), "itemId", itemId, "amount", amount)
	var count uint16
	for _, chest := range g.player.DepotManager.Chests {
		if chest == nil {
			continue
		}
		count += g.countItemInContainer(chest, itemId, tier)
		if count >= amount {
			return true
		}
	}
	return count >= amount
}

// countItemInContainer recursively counts items matching itemId/tier in a container.
func (g *GameProtocol) countItemInContainer(container *game.Item, itemId uint16, tier uint8) uint16 {
	if container == nil || !container.IsContainer(g.deps.Items) {
		return 0
	}
	var count uint16
	for _, child := range container.Contents {
		if child == nil {
			continue
		}
		if child.IsContainer(g.deps.Items) {
			count += g.countItemInContainer(child, itemId, tier)
		} else if child.ID == itemId && (tier == 0 || child.GetTier() == tier) {
			count += child.Count
		}
	}
	return count
}

// removeItemsFromDepot removes items from the player's depot chests.
func (g *GameProtocol) removeItemsFromDepot(itemId uint16, amount uint16, tier uint8) {
	if g.player.DepotManager == nil {
		return
	}
	remaining := amount
	for _, chest := range g.player.DepotManager.Chests {
		if chest == nil || remaining == 0 {
			continue
		}
		remaining = g.removeFromContainer(chest, itemId, tier, remaining)
	}
}

// removeFromContainer removes up to `amount` matching items from a container tree.
func (g *GameProtocol) removeFromContainer(container *game.Item, itemId uint16, tier uint8, amount uint16) uint16 {
	if container == nil || !container.IsContainer(g.deps.Items) || amount == 0 {
		return amount
	}
	remaining := amount
	for _, child := range container.Contents {
		if child == nil || remaining == 0 {
			continue
		}
		if child.IsContainer(g.deps.Items) {
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
	if container == nil || !container.IsContainer(g.deps.Items) {
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

// returnItemsToDepot puts items back into the player's first depot chest.
func (g *GameProtocol) returnItemsToDepot(itemId uint16, amount uint16, tier uint8) {
	if g.player.DepotManager == nil {
		return
	}
	for _, chest := range g.player.DepotManager.Chests {
		if chest == nil {
			continue
		}
		g.addToContainer(chest, itemId, tier, amount)
		return
	}
}

// addToContainer adds items to the first matching slot in a container.
func (g *GameProtocol) addToContainer(container *game.Item, itemId uint16, tier uint8, amount uint16) {
	if container == nil || !container.IsContainer(g.deps.Items) {
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

// sendResourcesBalance sends all resource balances (bank, money, prey cards, forge)
// after market enter, matching C++ ProtocolGame::sendResourcesBalance (opcode 0xEE).
func (g *GameProtocol) sendResourcesBalance() {
	if g.player == nil {
		return
	}
	// RESOURCE_BANK = 0
	w := netmsg.NewWriter()
	w.AddByte(0xEE)
	w.AddByte(0x00)
	w.AddU64(g.player.BankBalance)
	g.SendToClient(w)

	// RESOURCE_INVENTORY_MONEY = 1
	w2 := netmsg.NewWriter()
	w2.AddByte(0xEE)
	w2.AddByte(0x01)
	w2.AddU64(g.player.GetMoney())
	g.SendToClient(w2)

	// RESOURCE_PREY_CARDS = 2
	w3 := netmsg.NewWriter()
	w3.AddByte(0xEE)
	w3.AddByte(0x02)
	w3.AddU64(uint64(g.player.PreyCards))
	g.SendToClient(w3)

	// RESOURCE_FORGE_DUST = 4
	w4 := netmsg.NewWriter()
	w4.AddByte(0xEE)
	w4.AddByte(0x04)
	w4.AddU64(g.player.ForgeDusts)
	g.SendToClient(w4)
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
