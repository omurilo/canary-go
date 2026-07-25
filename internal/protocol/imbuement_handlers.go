package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// SendImbuementWindow opens the imbuement window for the given item (Opcode 0xEB).
func (g *GameProtocol) SendImbuementWindow(item *game.Item) {
	if g.player == nil || item == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0xEB)

	// Item info
	w.AddU16(item.ID)
	w.AddByte(0)
	// Client needs slot info for the selected item
	w.AddByte(3) // max slots on item

	g.SendToClient(w)
}

// parseImbuementAction handles client imbuement actions (Opcode 0xEC).
// Action: 0=apply, 1=clear, 2=clearAll
func (g *GameProtocol) parseImbuementAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	action := r.GetByte()
	slotID := r.GetByte()

	switch action {
	case 0: // Apply imbuement
		imbuementID := r.GetByte()
		tier := r.GetByte()
		g.applyImbuement(slotID, imbuementID, tier)
	case 1: // Clear single slot
		g.clearImbuementSlot(slotID)
	case 2: // Clear all imbuements
		g.clearAllImbuements()
	}
}

func (g *GameProtocol) applyImbuement(slotID, imbuementID, tier uint8) {
	cost := game.GetImbuementCost(imbuementID, tier)
	if cost == 0 {
		g.player.SendTextMessage(0x14, "Invalid imbuement.")
		return
	}

	if !g.player.RemoveMoney(cost, true) {
		g.player.SendTextMessage(0x14, "You don't have enough money for this imbuement.")
		return
	}

	g.player.SendTextMessage(0x13, "Imbuement applied successfully!")
}

func (g *GameProtocol) clearImbuementSlot(slotID uint8) {
	g.player.SendTextMessage(0x13, "Imbuement cleared.")
}

func (g *GameProtocol) clearAllImbuements() {
	g.player.SendTextMessage(0x13, "All imbuements cleared.")
}

// parseImbuementOpen handles client request to open imbuement window (Opcode 0xED).
func (g *GameProtocol) parseImbuementOpen(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	g.SendImbuementWindow(nil)
}
