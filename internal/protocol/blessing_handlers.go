package protocol

import (
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// SendBlessingsDialog sends the blessing status window to the client (Opcode 0x9C).
// Shows which blessings the player has and their costs.
func (g *GameProtocol) SendBlessingsDialog() {
	if g.player == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0x9C) // OPCODE_BLESS_DIALOG

	// Send blessing count (8 standard blessings)
	w.AddByte(8)

	// Blessing costs and status
	// In Tibia, blessing costs scale with level: basePrice * (level / baseLevelDivisor)
	// Standard formula: 2000 * (level / 5) for each blessing
	baseCost := uint32(2000)
	level := uint32(g.player.Level)
	cost := baseCost * level / 5
	if cost < baseCost {
		cost = baseCost
	}

	for i := uint8(0); i < 8; i++ {
		// Blessing ID (1-8)
		w.AddByte(i + 1)
		
		// Has blessing?
		hasBless := g.player.Blessings[i] > 0
		w.AddByte(boolToByte(hasBless))
		
		// Blessing cost
		w.AddU32(cost)
	}

	g.SendToClient(w)
}

// parseBuyBlessing handles client request to buy a blessing (Opcode 0xD2).
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
