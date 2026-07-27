package protocol

import (
	"log/slog"
	"strings"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// CyclopediaHouseState mirrors C++ src/enums/player_cyclopedia.hpp
const (
	HouseStateAvailable = 0
	HouseStateRented    = 2
	HouseStateTransfer  = 3
	HouseStateMoveOut   = 4
)

// parseCyclopediaHouseAuction handles opcode 0xAD (Cyclopedia House Auction request).
// Mirrors C++ ProtocolGame::parseCyclopediaHouseAuction.
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
		slog.Default().Info("house: bid request", "houseId", houseID, "bidValue", bidValue,
			"player", g.player.Name)
		// Send bid error response (0xC3) so the client doesn't hang.
			g.processHouseBid(houseID, bidValue)
	case 2:
		houseID := r.GetU32()
		_ = r.GetU32() // timestamp
		slog.Default().Info("house: move out", "houseId", houseID, "player", g.player.Name)
	case 3:
		houseID := r.GetU32()
		_ = r.GetU32() // timestamp
		newOwner := r.GetString()
		_ = r.GetU64() // bidValue
		slog.Default().Info("house: transfer", "houseId", houseID, "newOwner", newOwner, "player", g.player.Name)
	case 4:
		houseID := r.GetU32()
		slog.Default().Info("house: cancel move out", "houseId", houseID, "player", g.player.Name)
	case 5:
		houseID := r.GetU32()
		slog.Default().Info("house: cancel transfer", "houseId", houseID, "player", g.player.Name)
	case 6:
		houseID := r.GetU32()
		slog.Default().Info("house: accept transfer", "houseId", houseID, "player", g.player.Name)
	case 7:
		houseID := r.GetU32()
		slog.Default().Info("house: reject transfer", "houseId", houseID, "player", g.player.Name)
	default:
		slog.Default().Info("house: unknown action", "type", actionType)
	}
}

// sendHouseAuctionMessage sends 0xC3 (auction result message).
// Mirrors C++ ProtocolGame::sendHouseAuctionMessage.
func (g *GameProtocol) sendHouseAuctionMessage(houseID uint32, actionType uint8, index uint8, bidSuccess ...bool) {
	w := netmsg.NewWriter()
	w.AddByte(0xC3)
	w.AddU32(houseID)
	w.AddByte(actionType)
	if actionType == 1 {
		w.AddByte(0x00) // OTClient expects extra 0x00 byte for bid actionType
	}
	w.AddByte(index)
	slog.Default().Info("house: sent 0xC3", "houseId", houseID, "actionType", actionType, "index", index, "bidSuccess", len(bidSuccess) > 0 && bidSuccess[0])
	g.SendToClient(w)
}

// sendCyclopediaHouseList sends the house list for a town (opcode 0xC7).
// Mirrors C++ ProtocolGame::sendCyclopediaHouseList exactly.

func (g *GameProtocol) processHouseBid(clientID uint32, bidValue uint64) {
	if g.player == nil || g.deps.World == nil {
		return
	}
	world := g.deps.World
	house := world.GetHouseByClientID(clientID)
	if house == nil {
		slog.Default().Info("house: bid failed - house not found", "clientId", clientID)
		g.sendHouseAuctionMessage(clientID, 1, 24) // Internal error
		return
	}
	p := g.player
	// Check if player has enough balance (bid + rent)
	if p.BankBalance < uint64(house.Rent) + bidValue {
		slog.Default().Info("house: bid failed - not enough money",
			"balance", p.BankBalance, "rent", house.Rent, "bid", bidValue)
		g.sendHouseAuctionMessage(clientID, 1, 17) // NotEnoughMoney
		return
	}
	// Deduct rent + bid from bank balance
	newBalance := p.BankBalance - (uint64(house.Rent) + bidValue)
	p.BankBalance = newBalance
	// TODO: persist to DB
	// Update house bid info
	house.BidderName = p.Name
	house.HighestBid = 0
	house.InternalBid = bidValue
	house.BidHolderLimit = bidValue
	house.Bidder = p.DBID
	// Send success
	g.sendHouseAuctionMessage(clientID, 1, 0, true) // BidSuccess
	slog.Default().Info("house: bid successful", "clientId", clientID,
		"bidValue", bidValue, "newBalance", newBalance)
	// Refresh house list for the player
	// house list refreshed via client next request
	g.sendResourceBalances()
}

func (g *GameProtocol) sendCyclopediaHouseList(townName string) {
	if g.player == nil || g.deps.World == nil {
		return
	}

	world := g.deps.World
	slog.Default().Info("house: sendCyclopediaHouseList start",
		"worldHouses", len(world.Houses), "townName", townName)

	// Filter houses by the requested town name (case-insensitive).
	var townHouses []*game.House
	for _, h := range world.Houses {
		if h == nil || townName == "" {
			continue
		}
		if name, ok := world.TownNames[h.TownID]; ok && strings.EqualFold(name, townName) {
			townHouses = append(townHouses, h)
		}
	}

	slog.Default().Info("house: filtered by town", "matched", len(townHouses), "total", len(world.Houses))

	w := netmsg.NewWriter()
	w.AddByte(0xC7)
	w.AddU16(uint16(len(townHouses)))

	for _, h := range townHouses {
		// clientId: use the door item client ID from XML, or fall back to house ID.
		clientID := h.ClientID
		if clientID == 0 {
			clientID = h.ID
		}
		w.AddU32(clientID)

		// Category: always 0x01 = Available for rent.
		w.AddByte(0x01)

		if h.OwnerID == 0 {
			w.AddByte(HouseStateAvailable)
			if h.BidderName != "" {
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
				w.AddString("")
				w.AddByte(0)
				w.AddByte(0)
			}
		} else {
			w.AddByte(HouseStateRented)
			w.AddString("") // ownerName
			w.AddU32(0)     // paidUntil
			w.AddByte(0)    // isOwner (false)
		}
	}

	slog.Default().Info("house: sent 0xC7", "count", len(townHouses))
	g.SendToClient(w)
}

// sendHousesInfo sends the 0xC6 packet at login, informing the client about
// houses and enabling the cyclopedia house auction UI.
// Mirrors C++ ProtocolGame::sendHousesInfo exactly.
func (g *GameProtocol) sendHousesInfo() {
	if g.player == nil || g.deps.World == nil {
		slog.Default().Info("house: sendHousesInfo skipped (nil player/world)")
		return
	}
	world := g.deps.World

	slog.Default().Info("house: sendHousesInfo start",
		"player", g.player.Name, "totalHouses", len(world.Houses))

	// Find the player's owned house.
	houseClientID := uint32(0)
	for _, h := range world.Houses {
		if h != nil && h.OwnerID == g.player.DBID {
			houseClientID = h.ClientID
			slog.Default().Info("house: player owns house", "clientId", houseClientID, "houseName", h.Name)
			break
		}
	}

	// Count houses owned by any player on the same account.
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
	w.AddU32(houseClientID)                      // houseDoorClientId (player's owned house)
	w.AddByte(0x00)                               // padding
	w.AddByte(accountHouseCount)                  // accountHouseCount
	w.AddByte(0)                                  // accountHighscoreCount
	w.AddByte(3)                                  // premiumForHouseHighscore
	w.AddByte(3)                                  // maxHouseHighscore
	w.AddByte(0x01)                               // ownedHouseCountSameAccount
	w.AddByte(0x01)                               // ownedHouseCount
	w.AddU32(houseClientID)                       // houseDoorClientId (repeated)
	w.AddU16(uint16(len(world.Houses)))           // totalHouses
	for _, h := range world.Houses {
		if h == nil {
			continue
		}
		clientID := h.ClientID
		if clientID == 0 {
			clientID = h.ID
		}
		w.AddU32(clientID)
	}

	slog.Default().Info("house: sent 0xC6", "player", g.player.Name,
		"houseClientID", houseClientID, "accountCount", accountHouseCount,
		"totalHouses", len(world.Houses))
	g.SendToClient(w)

	// Also proactively send the full house list (0xC7) for ALL houses.
	// This bypasses the need for the client to send 0xAD first.
	// house list refreshed via client next request
	g.sendResourceBalances()
}
