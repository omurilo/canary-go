package protocol

import (
	"log/slog"
	"sort"
	"strings"
	"time"

	"context"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// CyclopediaHouseState mirrors C++ src/enums/player_cyclopedia.hpp
const (
	HouseStateAvailable = 0
	_                   = 1 // unused
	HouseStateRented    = 2
	HouseStateTransfer  = 3
	HouseStateMoveOut   = 4
)

// BidErrorMessage codes for house auction.
const (
	BidErrNoError           = 0
	BidErrRookgaard         = 3
	BidErrPremium           = 5
	BidErrGuildhall         = 6
	BidErrOnlyOneBid        = 7
	BidErrNotEnoughMoney    = 17
	BidErrInternal          = 24
	BidErrCharacterNotFound = 4
)

// parseCyclopediaHouseAuction handles opcode 0xAD (Cyclopedia House Auction request).
func (g *GameProtocol) parseCyclopediaHouseAuction(r *netmsg.Reader) {
	if g.player == nil {
		slog.Default().Info("house: player nil, skipping")
		return
	}
	if r.Remaining() < 1 {
		slog.Default().Info("house: no data, skipping")
		return
	}
	actionType := r.GetByte()
	slog.Default().Info("house: received 0xAD", "actionType", actionType,
		"player", g.player.Name, "remaining", r.Remaining())

	switch actionType {
	case 0:
		townName := r.GetString()
		slog.Default().Info("house: list request", "town", townName, "player", g.player.Name)
		g.sendCyclopediaHouseList(townName)
	case 1:
		houseID := r.GetU32()
		bidValue := r.GetU64()
		g.processHouseBid(houseID, bidValue)
	case 2:
		houseID := r.GetU32()
		timestamp := r.GetU32()
		g.processHouseMoveOut(houseID, timestamp)
	case 3:
		houseID := r.GetU32()
		timestamp := r.GetU32()
		newOwner := r.GetString()
		bidValue := r.GetU64()
		g.processHouseTransfer(houseID, timestamp, newOwner, bidValue)
	case 4:
		houseID := r.GetU32()
		g.processHouseCancelMoveOut(houseID)
	case 5:
		houseID := r.GetU32()
		g.processHouseCancelTransfer(houseID)
	case 6:
		houseID := r.GetU32()
		g.processHouseAcceptTransfer(houseID)
	case 7:
		houseID := r.GetU32()
		g.processHouseRejectTransfer(houseID)
	default:
		slog.Default().Info("house: unknown action", "type", actionType)
	}
}

// sendHouseAuctionMessage sends 0xC3 (auction result message).
func (g *GameProtocol) sendHouseAuctionMessage(houseID uint32, actionType uint8, index uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0xC3)
	w.AddU32(houseID)
	w.AddByte(actionType)
	if actionType == 1 {
		w.AddByte(0x00) // OTClient expects extra 0x00 byte for bid actionType
	}
	w.AddByte(index)
	slog.Default().Info("house: sent 0xC3", "houseId", houseID, "actionType", actionType, "index", index)
	g.SendToClient(w)
}

// getHouseByClientOrID finds a house by client ID or house ID.
func (g *GameProtocol) getHouseByClientOrID(id uint32) *game.House {
	if g.deps.World == nil {
		return nil
	}
	house := g.deps.World.GetHouseByClientID(id)
	return house
}

// processHouseBid handles action type 1 (bid).
func (g *GameProtocol) processHouseBid(clientID uint32, bidValue uint64) {
	if g.player == nil || g.deps.World == nil {
		return
	}
	house := g.getHouseByClientOrID(clientID)
	if house == nil {
		slog.Default().Info("house: bid failed - house not found", "clientId", clientID)
		g.sendHouseAuctionMessage(clientID, 1, BidErrInternal)
		return
	}
	if house.OwnerID != 0 {
		slog.Default().Info("house: bid failed - already owned", "clientId", clientID)
		g.sendHouseAuctionMessage(clientID, 1, BidErrInternal)
		return
	}
	p := g.player
	if p.BankBalance < uint64(house.Rent)+bidValue {
		slog.Default().Info("house: bid failed - not enough money",
			"balance", p.BankBalance, "rent", house.Rent, "bid", bidValue)
		g.sendHouseAuctionMessage(clientID, 1, BidErrNotEnoughMoney)
		return
	}
	newBalance := p.BankBalance - (uint64(house.Rent) + bidValue)
	p.BankBalance = newBalance

	house.BidderName = p.Name
	house.HighestBid = bidValue
	house.InternalBid = bidValue
	house.BidHolderLimit = bidValue
	house.Bidder = p.DBID
	house.BidEndDate = uint32(time.Now().Add(7 * 24 * time.Hour).Unix())

	if g.deps.DB != nil {
		if err := g.deps.DB.SaveHouseBid(context.Background(), house); err != nil {
			slog.Default().Info("house: failed to persist bid", "error", err)
		}
	}

	g.sendHouseAuctionMessage(clientID, 1, 0)
	slog.Default().Info("house: bid successful", "clientId", clientID,
		"bidValue", bidValue, "newBalance", newBalance)
	g.sendResourceBalances()
}

// processHouseMoveOut handles action type 2 (move out).
func (g *GameProtocol) processHouseMoveOut(houseID uint32, timestamp uint32) {
	if g.player == nil || g.deps.World == nil {
		return
	}
	house := g.getHouseByClientOrID(houseID)
	if house == nil {
		slog.Default().Info("house: moveout failed - house not found", "houseId", houseID)
		g.sendHouseAuctionMessage(houseID, 2, BidErrInternal)
		return
	}
	if house.OwnerID != g.player.DBID {
		slog.Default().Info("house: moveout failed - not owner", "houseId", houseID, "player", g.player.Name)
		g.sendHouseAuctionMessage(houseID, 2, BidErrInternal)
		return
	}

	house.OwnerID = 0
	house.BidderName = ""
	house.Bidder = 0
	house.HighestBid = 0
	house.InternalBid = 0
	house.BidHolderLimit = 0
	house.BidEndDate = 0
	house.TransferToName = ""
	house.TransferPrice = 0
	house.TransferAccept = 0

	if g.deps.DB != nil {
		if err := g.deps.DB.SaveHouseOwner(context.Background(), house.ID, 0); err != nil {
			slog.Default().Info("house: failed to persist moveout owner", "error", err)
		}
	}

	g.sendHouseAuctionMessage(houseID, 2, 0)
	slog.Default().Info("house: moveout successful", "houseId", houseID, "player", g.player.Name)
}

// processHouseTransfer handles action type 3 (initiate transfer).
func (g *GameProtocol) processHouseTransfer(houseID uint32, timestamp uint32, newOwner string, price uint64) {
	if g.player == nil || g.deps.World == nil {
		return
	}
	if newOwner == "" {
		g.sendHouseAuctionMessage(houseID, 3, BidErrCharacterNotFound)
		return
	}
	house := g.getHouseByClientOrID(houseID)
	if house == nil {
		slog.Default().Info("house: transfer failed - house not found", "houseId", houseID)
		g.sendHouseAuctionMessage(houseID, 3, BidErrInternal)
		return
	}
	if house.OwnerID != g.player.DBID {
		slog.Default().Info("house: transfer failed - not owner", "houseId", houseID)
		g.sendHouseAuctionMessage(houseID, 3, BidErrInternal)
		return
	}

	house.TransferToName = newOwner
	house.TransferPrice = price
	house.TransferAccept = 0

	if g.deps.DB != nil {
		if err := g.deps.DB.SaveHouseTransfer(context.Background(), house); err != nil {
			slog.Default().Info("house: failed to persist transfer", "error", err)
		}
	}

	g.sendHouseAuctionMessage(houseID, 3, 0)
	slog.Default().Info("house: transfer initiated", "houseId", houseID,
		"newOwner", newOwner, "price", price, "player", g.player.Name)
}

// processHouseCancelMoveOut handles action type 4 (cancel move out).
func (g *GameProtocol) processHouseCancelMoveOut(houseID uint32) {
	if g.player == nil || g.deps.World == nil {
		return
	}
	house := g.getHouseByClientOrID(houseID)
	if house == nil {
		g.sendHouseAuctionMessage(houseID, 4, BidErrInternal)
		return
	}
	if house.OwnerID != g.player.DBID {
		g.sendHouseAuctionMessage(houseID, 4, BidErrInternal)
		return
	}
	g.sendHouseAuctionMessage(houseID, 4, 0)
	slog.Default().Info("house: cancel moveout", "houseId", houseID, "player", g.player.Name)
}

// processHouseCancelTransfer handles action type 5 (cancel transfer).
func (g *GameProtocol) processHouseCancelTransfer(houseID uint32) {
	if g.player == nil || g.deps.World == nil {
		return
	}
	house := g.getHouseByClientOrID(houseID)
	if house == nil {
		g.sendHouseAuctionMessage(houseID, 5, BidErrInternal)
		return
	}
	if house.OwnerID != g.player.DBID {
		g.sendHouseAuctionMessage(houseID, 5, BidErrInternal)
		return
	}

	house.TransferToName = ""
	house.TransferPrice = 0
	house.TransferAccept = 0

	if g.deps.DB != nil {
		if err := g.deps.DB.ClearHouseTransfer(context.Background(), house.ID); err != nil {
			slog.Default().Info("house: failed to clear transfer in DB", "error", err)
		}
	}

	g.sendHouseAuctionMessage(houseID, 5, 0)
	slog.Default().Info("house: transfer cancelled", "houseId", houseID, "player", g.player.Name)
}

// processHouseAcceptTransfer handles action type 6 (accept transfer).
func (g *GameProtocol) processHouseAcceptTransfer(houseID uint32) {
	if g.player == nil || g.deps.World == nil {
		return
	}
	house := g.getHouseByClientOrID(houseID)
	if house == nil {
		g.sendHouseAuctionMessage(houseID, 6, BidErrInternal)
		return
	}
	if house.TransferToName == "" {
		g.sendHouseAuctionMessage(houseID, 6, BidErrInternal)
		return
	}
	if !strings.EqualFold(house.TransferToName, g.player.Name) {
		slog.Default().Info("house: accept transfer - not the target", "houseId", houseID,
			"transferTo", house.TransferToName, "player", g.player.Name)
		g.sendHouseAuctionMessage(houseID, 6, BidErrInternal)
		return
	}

	// Transfer ownership
	house.OwnerID = g.player.DBID
	house.TransferToName = ""
	house.TransferPrice = 0
	house.TransferAccept = g.player.DBID

	house.BidderName = ""
	house.Bidder = 0
	house.HighestBid = 0
	house.InternalBid = 0
	house.BidHolderLimit = 0
	house.BidEndDate = 0

	if g.deps.DB != nil {
		if err := g.deps.DB.SaveHouseOwner(context.Background(), house.ID, g.player.DBID); err != nil {
			slog.Default().Info("house: failed to persist transfer owner", "error", err)
		}
	}

	g.sendHouseAuctionMessage(houseID, 6, 0)
	slog.Default().Info("house: transfer accepted", "houseId", houseID,
		"newOwner", g.player.Name)
}

// processHouseRejectTransfer handles action type 7 (reject transfer).
func (g *GameProtocol) processHouseRejectTransfer(houseID uint32) {
	if g.player == nil || g.deps.World == nil {
		return
	}
	house := g.getHouseByClientOrID(houseID)
	if house == nil {
		g.sendHouseAuctionMessage(houseID, 7, BidErrInternal)
		return
	}
	if house.TransferToName == "" {
		g.sendHouseAuctionMessage(houseID, 7, BidErrInternal)
		return
	}
	if !strings.EqualFold(house.TransferToName, g.player.Name) {
		g.sendHouseAuctionMessage(houseID, 7, BidErrInternal)
		return
	}

	house.TransferToName = ""
	house.TransferPrice = 0
	house.TransferAccept = 0

	if g.deps.DB != nil {
		if err := g.deps.DB.ClearHouseTransfer(context.Background(), house.ID); err != nil {
			slog.Default().Info("house: failed to clear transfer rejection in DB", "error", err)
		}
	}

	g.sendHouseAuctionMessage(houseID, 7, 0)
	slog.Default().Info("house: transfer rejected", "houseId", houseID, "player", g.player.Name)
}

// sendCyclopediaHouseList sends the house list for a town (opcode 0xC7).
func (g *GameProtocol) sendCyclopediaHouseList(townName string) {
	if g.player == nil || g.deps.World == nil {
		return
	}

	world := g.deps.World
	slog.Default().Info("house: sendCyclopediaHouseList start",
		"worldHouses", len(world.Houses), "townName", townName)

	// An EMPTY town name is not "match no town" — it is the request for the
	// player's OWN houses (Game::playerCyclopediaHousesByTown, game.cpp:12732-12752
	// branches on townName.empty() and calls getAllHousesByPlayerId). Filtering by
	// "" matched nothing, which is why the cyclopedia never listed a single house
	// of yours.
	var townHouses []*game.House
	for _, h := range world.Houses {
		if h == nil {
			continue
		}
		if townName == "" {
			if h.OwnerID == g.player.DBID {
				townHouses = append(townHouses, h)
			}
			continue
		}
		if name, ok := world.TownNames[h.TownID]; ok && strings.EqualFold(name, townName) {
			townHouses = append(townHouses, h)
		}
	}

	slog.Default().Info("house: filtered by town", "matched", len(townHouses), "total", len(world.Houses))

	// C++ collects these into a std::map<clientId, house>
	// (game.cpp:12743), which does three things this loop has to reproduce:
	// key by clientId, DE-DUPLICATE on it, and emit in ascending order.
	//
	// Go was ranging a Go map, so the order was different on every request, and
	// houses sharing a clientId were all sent. otservbr-house.xml gives 102 of its
	// 1029 houses clientid="0" — 69 of them in Thais alone — so that town went out
	// with 69 entries the client cannot tell apart instead of the one upstream
	// sends.
	byClientID := make(map[uint32]*game.House, len(townHouses))
	for _, h := range townHouses {
		if _, seen := byClientID[h.ClientID]; !seen {
			byClientID[h.ClientID] = h
		}
	}
	clientIDs := make([]uint32, 0, len(byClientID))
	for id := range byClientID {
		clientIDs = append(clientIDs, id)
	}
	sort.Slice(clientIDs, func(i, j int) bool { return clientIDs[i] < clientIDs[j] })

	w := netmsg.NewWriter()
	w.AddByte(0xC7)
	w.AddU16(uint16(len(clientIDs)))

	for _, clientID := range clientIDs {
		h := byClientID[clientID]
		// No fallback to the house id. 0xC6 advertises getClientId() with no
		// fallback (protocolgame.cpp:12026), so substituting the house id here sent
		// the client a number it was never told about — it cannot resolve a name,
		// a town or an owner for it, because the client resolves all of those from
		// the clientId against its own house data. The server never sends a house
		// name at all.
		//
		// A clientId of 0 therefore renders as nothing. That is a hole in the
		// datapack, not something the protocol can paper over.
		w.AddU32(clientID)
		w.AddByte(0x01) // Category: Available for rent

		if h.OwnerID == 0 {
			// Available state — may have bids or be open for auction
			if h.BidderName != "" {
				w.AddByte(HouseStateAvailable)
				w.AddString(h.BidderName)
				isBidder := byte(0)
				if h.BidderName == g.player.Name {
					isBidder = 1
				}
				w.AddByte(isBidder)
				w.AddByte(0) // disableIndex
				w.AddU32(h.BidEndDate)
				w.AddU64(h.HighestBid)
				if isBidder == 1 {
					w.AddU64(h.BidHolderLimit)
				}
			} else {
				w.AddByte(HouseStateAvailable)
				w.AddString("")
				w.AddByte(0)
				w.AddByte(0)
			}
		} else if h.TransferToName != "" {
			// Transfer state
			w.AddByte(HouseStateTransfer)
			w.AddString(g.houseOwnerName(h))
			w.AddU32(uint32(h.PaidUntil))
			isOwner := byte(0)
			if h.OwnerID == g.player.DBID {
				isOwner = 1
			}
			w.AddByte(isOwner)
			if isOwner == 1 {
				w.AddByte(0) // disableIndex
				w.AddByte(0) // disableBid
			}
			w.AddU32(h.BidEndDate) // transfer scheduled date
			w.AddString(h.TransferToName)
			w.AddByte(0) // isBidderOfTransfer (not used here)
			w.AddU64(h.TransferPrice)
			isNewOwner := byte(0)
			if strings.EqualFold(h.TransferToName, g.player.Name) {
				isNewOwner = 1
			}
			w.AddByte(isNewOwner)
			if isNewOwner == 1 {
				w.AddByte(0) // disableIndex
				w.AddByte(0) // disableBid
			}
			if isOwner == 1 {
				w.AddByte(0) // cancelAvailable
			}
		} else {
			// Rented state (protocolgame.cpp:11923-11933).
			w.AddByte(HouseStateRented)
			w.AddString(g.houseOwnerName(h))
			w.AddU32(uint32(h.PaidUntil))
			isOwner := byte(0)
			if h.OwnerID == g.player.DBID {
				isOwner = 1
			}
			w.AddByte(isOwner)
			// C++ writes two more bytes when the house is the reader's own. They were
			// missing, so every frame containing a house you rent ended two bytes
			// short and the client ran off the end of it — opening the Thais list,
			// 201 houses including one of mine, dropped the connection.
			if isOwner == 1 {
				w.AddByte(0)
				w.AddByte(0)
			}
		}
	}

	slog.Default().Info("house: sent 0xC7", "count", len(townHouses))
	g.SendToClient(w)
}

// sendHousesInfo sends the 0xC6 packet at login, informing the client about
// houses and enabling the cyclopedia house auction UI.
func (g *GameProtocol) sendHousesInfo() {
	if g.player == nil || g.deps.World == nil {
		slog.Default().Info("house: sendHousesInfo skipped (nil player/world)")
		return
	}
	world := g.deps.World

	slog.Default().Info("house: sendHousesInfo start",
		"player", g.player.Name, "totalHouses", len(world.Houses))

	houseClientID := uint32(0)
	for _, h := range world.Houses {
		if h != nil && h.OwnerID == g.player.DBID {
			houseClientID = h.ClientID
			slog.Default().Info("house: player owns house", "clientId", houseClientID, "houseName", h.Name)
			break
		}
	}

	accountHouseCount := uint8(0)
	for _, p := range world.Players() {
		if p.AccountID == g.player.AccountID {
			for _, h := range world.Houses {
				if h != nil && h.OwnerID == p.DBID {
					accountHouseCount++
				}
			}
		}
	}

	w := netmsg.NewWriter()
	w.AddByte(0xC6)
	w.AddU32(houseClientID)
	w.AddByte(0x00) // padding
	w.AddByte(accountHouseCount)
	w.AddByte(0)    // accountHighscoreCount
	w.AddByte(3)    // premiumForHouseHighscore
	w.AddByte(3)    // maxHouseHighscore
	w.AddByte(0x01) // ownedHouseCountSameAccount
	w.AddByte(0x01) // ownedHouseCount
	w.AddU32(houseClientID)
	// The count and the entries must be built from the SAME list. This wrote
	// len(world.Houses) and then skipped nils while writing, so a single nil left
	// the client reading one entry too many and running off the end of the frame.
	ids := make([]uint32, 0, len(world.Houses))
	for _, h := range world.Houses {
		if h == nil {
			continue
		}
		// getClientId(), with no fallback to the house id (protocolgame.cpp:12026).
		// Substituting the id sent the client a number it then quotes back when
		// asking for that house's details — and no house answers to it.
		ids = append(ids, h.ClientID)
	}
	w.AddU16(uint16(len(ids)))
	for _, id := range ids {
		w.AddU32(id)
	}

	slog.Default().Info("house: sent 0xC6", "player", g.player.Name,
		"houseClientID", houseClientID, "accountCount", accountHouseCount,
		"totalHouses", len(world.Houses))
	g.SendToClient(w)
	g.sendResourceBalances()
}

// houseOwnerName is IOLoginData::getNameByGuid for a house's owner
// (protocolgame.cpp:11924). The list used to send an empty string here, so no
// house ever showed who rented it — including the reader's own.
//
// House.OwnerName is filled at load and by setOwner; the world lookup is the
// fallback for a house whose row was written by something else.
func (g *GameProtocol) houseOwnerName(h *game.House) string {
	if h == nil || h.OwnerID == 0 {
		return ""
	}
	if h.OwnerName != "" {
		return h.OwnerName
	}
	if g.deps.World != nil && g.deps.World.LookupPlayerAccount != nil {
		if name, _, ok := g.deps.World.LookupPlayerAccount(h.OwnerID); ok {
			return name
		}
	}
	return ""
}
