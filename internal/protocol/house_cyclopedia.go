package protocol

import (
	"fmt"
	"log/slog"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseCyclopediaHouseAuction handles opcode 0xAD (Cyclopedia House Auction request).
// Wire: [u8 actionType][string townName]
// actionType 0 = list houses by town, 1 = bid on house
func (g *GameProtocol) parseCyclopediaHouseAuction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	if r.Remaining() < 1 {
		return
	}
	actionType := r.GetByte()
	slog.Default().Info("0xAD actionType=%d remaining=%d\n", actionType, r.Remaining())
	switch actionType {
	case 0:
		townName := r.GetString()
		slog.Default().Info("townName=%q\n", townName)
		g.sendCyclopediaHouseList(townName)
	case 1:
		// bid on house — not implemented
		_ = r.GetU32() // houseId
		_ = r.GetU64() // bidValue
	default:
		slog.Default().Info("parseCyclopediaHouseAuction: unknown action", "type", actionType)
	}
}

// sendCyclopediaHouseList sends the house list for a town (opcode 0xC7).
// Mirrors C++ ProtocolGame::sendCyclopediaHouseList.
func (g *GameProtocol) sendCyclopediaHouseList(townName string) {
	if g.player == nil || g.deps.World == nil {
		return
	}

	// Filter houses by the requested town (lowercase match)
	var townHouses []*game.House
	world := g.deps.World
	slog.Default().Info("world.Houses=%d towns=%d\n", len(world.Houses), len(world.TownNames))
	for _, h := range world.Houses {
		if h == nil {
			continue
		}
		// Check town by name if available
		if name, ok := world.TownNames[h.TownID]; ok && name == townName {
			townHouses = append(townHouses, h)
		}
	}

	w := netmsg.NewWriter()
	w.AddByte(0xC7)
	w.AddU16(uint16(len(townHouses)))

	for _, h := range townHouses {
		// clientId: use the house tile's ground item client ID from the first tile,
		// or fall back to the house ID when no tile data is available.
		clientID := h.ID
		if len(h.HouseTiles) > 0 {
			if tile := world.Map.GetTile(h.HouseTiles[0]); tile != nil && tile.Ground != nil {
				clientID = uint32(tile.Ground.ID)
			}
		}

		w.AddU32(clientID)

		// State: 0x01 = Available (simplified — no auction system).
		// C++ sends CyclopediaHouseState enum: 0=Renovation, 1=Available,
		// 2=Rented, 3=Transfer
		if h.OwnerID == 0 {
			w.AddByte(0x01) // Available
			w.AddByte(0x01) // state value (Available)
			w.AddString("") // bidder name (empty)
			w.AddByte(0)    // isBidder
			w.AddByte(0)    // disableIndex
			w.AddByte(0)    // acceptTransferError
			w.AddByte(0)    // rejectTransferError
			w.AddByte(0)    // cancelTransferError
		} else {
			w.AddByte(0x02) // Rented
			w.AddByte(0x02) // state value (Rented)
			w.AddString("") // owner name (not available without DB lookup)
			w.AddU32(0)     // paidUntil
			w.AddByte(0)    // isOwner
			w.AddByte(0x02) // ?
			w.AddByte(0x02) // ?
		}
	}

	g.SendToClient(w)
}
