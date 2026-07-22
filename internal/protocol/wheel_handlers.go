package protocol

import (
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseOpenWheel handles opcode 0x61 (Open Wheel of Destiny window).
func (g *GameProtocol) parseOpenWheel(r *netmsg.Reader) {
	_ = r.GetU32() // ownerID
	g.SendWheelOfDestiny()
}

// parseSaveWheel handles opcode 0x62 (Save Wheel of Destiny allocations).
func (g *GameProtocol) parseSaveWheel(r *netmsg.Reader) {
	if r.Remaining() >= 4 {
		_ = r.GetU32() // ownerID
	}
	if r.Remaining() >= 2 {
		slotCount := int(r.GetU16())
		pointsMap := make(map[uint16]uint16)
		for i := 0; i < slotCount && r.Remaining() >= 4; i++ {
			slotID := r.GetU16()
			pts := r.GetU16()
			pointsMap[slotID] = pts
		}
		wheel := g.player.GetWheel()
		wheel.SaveSlotPoints(pointsMap)
	}
	g.SendWheelOfDestiny()
}

// parseWheelOfDestiny handles opcode 0xEC (Legacy / Alternative Wheel request).
func (g *GameProtocol) parseWheelOfDestiny(r *netmsg.Reader) {
	action := r.GetByte()
	switch action {
	case 0: // Request / Open Wheel Data
		g.SendWheelOfDestiny()
	case 1: // Save Wheel Allocation / Preset
		preset := r.GetByte()
		_ = preset
		slotCount := int(r.GetU16())
		pointsMap := make(map[uint16]uint16)
		for i := 0; i < slotCount && r.Remaining() >= 4; i++ {
			slotID := r.GetU16()
			pts := r.GetU16()
			pointsMap[slotID] = pts
		}
		wheel := g.player.GetWheel()
		wheel.SaveSlotPoints(pointsMap)
		g.SendWheelOfDestiny()
	}
}

// SendWheelOfDestiny sends the Wheel of Destiny payload (Opcode 0x5F) to client.
func (g *GameProtocol) SendWheelOfDestiny() {
	wheel := g.player.GetWheel()
	isPromoted := g.player.Vocation > 0 && (g.player.Vocation > 4)

	totalPoints := wheel.GetTotalPoints(g.player.Level, isPromoted)

	w := netmsg.NewWriter()
	w.AddByte(0x5F) // Wheel window response opcode
	w.AddU32(g.player.ID)
	w.AddByte(1) // canUse = true
	w.AddByte(0) // options
	w.AddByte(byte(g.player.Vocation))

	w.AddU16(totalPoints)
	w.AddU16(wheel.BonusPoints) // extra points

	// Write slot allocations for standard 36 slots
	slotPoints := wheel.GetSlotPointsCopy()
	for slotID := uint16(1); slotID <= 36; slotID++ {
		w.AddU16(slotPoints[slotID])
	}

	w.AddByte(0) // promotion scrolls count
	w.AddByte(0) // monk quest bonus flag
	w.AddU16(0)  // monk quest bonus amount

	g.SendToClient(w)
}
