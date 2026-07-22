package protocol

import (
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// getCIPVocation maps server OT vocation ID to CIP client vocation ID used in Wheel UI packets.
// Server OT Vocations: 1=Sorcerer, 2=Druid, 3=Paladin, 4=Knight (and 5..8 for promoted).
// CIP Client Vocations: 1=Knight, 2=Paladin, 3=Sorcerer, 4=Druid.
func getCIPVocation(vocation uint16) byte {
	switch vocation {
	case 1, 5: // Sorcerer / Master Sorcerer
		return 3
	case 2, 6: // Druid / Elder Druid
		return 4
	case 3, 7: // Paladin / Royal Paladin
		return 2
	case 4, 8: // Knight / Elite Knight
		return 1
	default:
		return 1 // Fallback
	}
}

// getSupremeModifiers returns the exact WheelGemSupremeModifier_t enum values per vocation matching C++ modsSupremePositionByVocation.
func getSupremeModifiers(vocation uint16) []byte {
	switch vocation {
	case 1, 5: // Sorcerer / Master Sorcerer
		return []byte{
			0, 1, 2, 3, 4, 5,
			43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59,
		}
	case 2, 6: // Druid / Elder Druid
		return []byte{
			0, 1, 2, 3, 4, 5,
			61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77,
		}
	case 3, 7: // Paladin / Royal Paladin
		return []byte{
			0, 1, 2, 3, 5,
			24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41,
		}
	case 4, 8: // Knight / Elite Knight
		return []byte{
			0, 1, 2, 3, 5,
			6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23,
		}
	default: // Default Knight fallback
		return []byte{
			0, 1, 2, 3, 5,
			6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23,
		}
	}
}

// sendGiftOfLifeCooldown sends opcode 0x5E (Wheel of Destiny Gift of Life cooldown status).
func (g *GameProtocol) sendGiftOfLifeCooldown() {
	w := netmsg.NewWriter()
	w.AddByte(0x5E)
	w.AddByte(0x01) // Gift of life ID
	w.AddByte(0x00) // Cooldown ENUM
	w.AddU32(0)     // Remaining cooldown seconds
	w.AddU32(0)     // Total cooldown seconds
	w.AddByte(0x00) // Infight / cooldown paused flag
	g.SendToClient(w)
}

// parseOpenWheel handles opcode 0x61 (Open Wheel of Destiny window).
func (g *GameProtocol) parseOpenWheel(r *netmsg.Reader) {
	_ = r.GetU32() // ownerID
	g.SendWheelOfDestiny()
}

// parseSaveWheel handles opcode 0x62 (Save Wheel of Destiny allocations).
// In Tibia 13.x protocol, the client sends 36 uint16 values representing allocated points for slots 1..36.
func (g *GameProtocol) parseSaveWheel(r *netmsg.Reader) {
	pointsMap := make(map[uint16]uint16)
	for slotID := uint16(1); slotID <= 36 && r.Remaining() >= 2; slotID++ {
		pts := r.GetU16()
		if pts > 0 {
			pointsMap[slotID] = pts
		}
	}
	wheel := g.player.GetWheel()
	wheel.SaveSlotPoints(pointsMap)
	g.SendWheelOfDestiny()
}

// parseWheelOfDestiny handles opcode 0xEC (Legacy / Alternative Wheel request).
func (g *GameProtocol) parseWheelOfDestiny(r *netmsg.Reader) {
	action := r.GetByte()
	switch action {
	case 0: // Request / Open Wheel Data
		g.SendWheelOfDestiny()
	case 1: // Save Wheel Allocation / Preset
		_ = r.GetByte() // preset
		pointsMap := make(map[uint16]uint16)
		for slotID := uint16(1); slotID <= 36 && r.Remaining() >= 2; slotID++ {
			pts := r.GetU16()
			if pts > 0 {
				pointsMap[slotID] = pts
			}
		}
		wheel := g.player.GetWheel()
		wheel.SaveSlotPoints(pointsMap)
		g.SendWheelOfDestiny()
	}
}

// SendWheelOfDestiny sends the full Wheel of Destiny payload (Opcode 0x5F) to client.
func (g *GameProtocol) SendWheelOfDestiny() {
	wheel := g.player.GetWheel()

	// Vocation check: Vocation 0 (no vocation) cannot use Wheel of Destiny
	canUse := g.player.Vocation > 0

	w := netmsg.NewWriter()
	w.AddByte(0x5F) // Wheel window response opcode
	w.AddU32(g.player.ID)

	if !canUse {
		w.AddByte(0) // canUse = false
		g.SendToClient(w)
		return
	}

	w.AddByte(1) // canUse = true
	w.AddByte(1) // options = 1 (1 = can increase and decrease points)

	// Map OT vocation ID to CIP client vocation ID
	vocationByte := getCIPVocation(g.player.Vocation)
	w.AddByte(vocationByte)

	totalPoints := wheel.GetTotalPoints(g.player.Level)

	w.AddU16(totalPoints)
	w.AddU16(wheel.BonusPoints) // extra points

	// Write slot allocations for 36 slots
	slotPoints := wheel.GetSlotPointsCopy()
	for slotID := uint16(1); slotID <= 36; slotID++ {
		w.AddU16(slotPoints[slotID])
	}

	w.AddU16(0)  // promotion scrolls count (u16 = 2 bytes)
	w.AddByte(0) // monk quest bonus flag (u8 = 1 byte)
	w.AddU16(0)  // monk quest bonus amount (u16 = 2 bytes)

	// Gems section
	w.AddByte(0) // active gems count (u8 = 1 byte)
	w.AddU16(0)  // revealed gems count (u16 = 2 bytes)

	// Grade modifiers section
	// Basic grade modifiers (46 entries: 0x00..0x2D)
	w.AddByte(46)
	for i := byte(0); i < 46; i++ {
		w.AddByte(i)
		w.AddByte(0) // grade 0
	}

	// Supreme grade modifiers (vocation-specific 23 entries matching C++ WheelGemSupremeModifier_t)
	supremeMods := getSupremeModifiers(g.player.Vocation)
	w.AddByte(byte(len(supremeMods)))
	for _, modPos := range supremeMods {
		w.AddByte(modPos)
		w.AddByte(0) // grade 0
	}

	g.SendToClient(w)

	// Send Gift of Life cooldown state (opcode 0x5E)
	g.sendGiftOfLifeCooldown()

	// Send resource balance updates expected by Wheel UI
	g.sendResourceBalance(0, g.player.BankBalance)
	g.sendResourceBalance(1, uint64(g.player.GetMoney()))
	g.sendResourceBalance(27, 0) // RESOURCE_LESSER_GEMS
	g.sendResourceBalance(28, 0) // RESOURCE_REGULAR_GEMS
	g.sendResourceBalance(29, 0) // RESOURCE_GREATER_GEMS
	g.sendResourceBalance(30, 0) // RESOURCE_LESSER_FRAGMENT
	g.sendResourceBalance(31, 0) // RESOURCE_GREATER_FRAGMENT
}
