package protocol

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// itemAmuletOfLoss mirrors ITEM_AMULETOFLOSS.
const itemAmuletOfLoss = 2173

// Skulls_t values (src/utils/utils_definitions.hpp:500).
const (
	skullRed   = 4
	skullBlack = 5
)

// calculateEquipmentLoss ports calculateEquipmentLoss (src/utils/tools.cpp:2251).
// isContainer multiplies the percentage by 10.
func calculateEquipmentLoss(blessingAmount uint8, isContainer bool) float64 {
	var lossPercent float64
	switch blessingAmount {
	case 0:
		lossPercent = 10
	case 1:
		lossPercent = 7
	case 2:
		lossPercent = 4.5
	case 3:
		lossPercent = 2.5
	case 4:
		lossPercent = 1
	default: // >= 5
		lossPercent = 0
	}
	if isContainer {
		return lossPercent * 10
	}
	return lossPercent
}

// calculateMaxPvpReduction ports calculateMaxPvpReduction (src/utils/tools.cpp:2278).
func calculateMaxPvpReduction(blessCount uint8, isPromoted bool) uint8 {
	result := 80 + (2 * int(blessCount)) - (int(blessCount) / 3)
	if blessCount == 5 {
		result--
	}
	if isPromoted {
		result += 6
	}
	return uint8(result)
}

// SendBlessingsDialog sends the blessing window (opcode 0x9B), porting
// ProtocolGame::sendBlessingWindow (protocolgame.cpp:5793).
//
// This used to emit 0x9C — which is sendBlessStatus, a different packet — with an
// invented [u8 id][u8 has][u32 cost] body. The real layout is
// [0x9B][u8 count][per blessing: u16 (1<<value), u8 count, u8 storeCount]
// followed by the death-penalty reduction block.
//
// isRetro is hardcoded false: the C++ TOGGLE_SERVER_IS_RETRO config key is not
// modeled in internal/config yet.
func (g *GameProtocol) SendBlessingsDialog() {
	if g.player == nil {
		return
	}

	const isRetro = false

	w := netmsg.NewWriter()
	w.AddByte(0x9B)

	if isRetro {
		w.AddByte(0x07)
	} else {
		w.AddByte(0x08)
	}

	// Blessings enum order; TwistOfFate is the last one and is skipped on retro.
	for i := 0; i < 8; i++ {
		w.AddU16(uint16(1) << uint(i))
		w.AddByte(g.player.Blessings[i])
		// Store-bought count is not tracked separately yet.
		w.AddByte(0)
	}

	// blessCount counts owned blessings, excluding TwistOfFate (index 7).
	var blessCount uint8
	for i := 0; i < 7; i++ {
		if g.player.Blessings[i] > 0 {
			blessCount++
		}
	}

	// Promotion is not modeled on Player; vocations 5-8 are the promoted tier.
	isPromoted := g.player.Vocation > 4

	factor := 8.0
	if isRetro {
		factor = 6.31
	}
	promotionReduction := 0
	if isPromoted {
		promotionReduction = 30
	}
	minReduction := uint8(factor*float64(blessCount)) + uint8(promotionReduction)
	maxLossPvpDeath := calculateMaxPvpReduction(blessCount, isPromoted)

	w.AddByte(boolToByte(isPromoted))
	w.AddByte(30) // reduction bonus with promotion
	w.AddByte(minReduction)
	if isRetro {
		w.AddByte(minReduction)
	} else {
		w.AddByte(maxLossPvpDeath)
	}
	w.AddByte(minReduction)

	hasSkull := g.player.Skull == skullRed || g.player.Skull == skullBlack
	usingAol := false
	if necklace := g.player.Inventory[game.ConstSlotNecklace]; necklace != nil && necklace.ID == itemAmuletOfLoss {
		usingAol = true
	}

	switch {
	case hasSkull:
		w.AddByte(100)
		w.AddByte(100)
	case usingAol:
		w.AddByte(0)
		w.AddByte(0)
	default:
		loss := uint8(calculateEquipmentLoss(blessCount, true))
		w.AddByte(loss)
		w.AddByte(loss)
	}

	w.AddByte(boolToByte(hasSkull))
	w.AddByte(boolToByte(usingAol))
	w.AddByte(0x00)

	g.SendToClient(w)
}

// parseBuyBlessing handles a request to buy a blessing.
//
// Intentionally NOT dispatched: the docstring used to claim 0xD2, but 0xD2 is
// playerRequestOutfit (protocolgame.cpp). Canary has no client opcode for buying
// blessings — it goes through the NPC/Lua path. Kept for that path to call into.
func (g *GameProtocol) parseBuyBlessing(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	blessingID := r.GetByte()
	
	if blessingID < 1 || blessingID > 8 {
		g.player.SendTextMessage(0x14, "Invalid blessing.")
		return
	}

	// Check if already has blessing
	if g.player.Blessings[blessingID-1] > 0 {
		g.player.SendTextMessage(0x14, "You already have this blessing.")
		return
	}

	// Calculate cost: 2000 * (level / 5)
	baseCost := uint64(2000)
	level := uint64(g.player.Level)
	cost := baseCost * level / 5
	if cost < baseCost {
		cost = baseCost
	}

	// Check if player has enough money
	if !g.player.RemoveMoney(cost, true) {
		g.player.SendTextMessage(0x14, "You don't have enough money.")
		return
	}

	// Add blessing
	g.player.Blessings[blessingID-1] = 1

	// Send success message
	blessingNames := []string{
		"Twist of Fate",
		"The Wisdom of Solitude",
		"The Spark of the Phoenix",
		"The Fire of the Suns",
		"The Spiritual Shielding",
		"The Embrace of Tibia",
		"The Blood of the Mountain",
		"The Heart of the Mountain",
	}
	
	if int(blessingID-1) < len(blessingNames) {
		g.player.SendTextMessage(0x13, "You have been blessed with "+blessingNames[blessingID-1]+"!")
	} else {
		g.player.SendTextMessage(0x13, "You have been blessed!")
	}

	// Refresh blessing dialog
	g.SendBlessingsDialog()
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}
