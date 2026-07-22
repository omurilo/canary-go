package protocol

import (
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseWheelOfDestiny handles opcode 0xEC (Wheel of Destiny Open/Save).
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
		for i := 0; i < slotCount; i++ {
			slotID := r.GetU16()
			pts := r.GetU16()
			pointsMap[slotID] = pts
		}
		wheel := g.player.GetWheel()
		wheel.SaveSlotPoints(pointsMap)
		g.SendWheelOfDestiny()
	}
}

// SendWheelOfDestiny sends the Wheel of Destiny payload (Opcode 0xD6) to client.
func (g *GameProtocol) SendWheelOfDestiny() {
	wheel := g.player.GetWheel()
	isPromoted := g.player.Vocation > 0 && (g.player.Vocation > 4)

	totalPoints := wheel.GetTotalPoints(g.player.Level, isPromoted)
	spentPoints := wheel.GetSpentPoints()

	w := netmsg.NewWriter()
	w.AddByte(0xD6) // Wheel payload opcode
	w.AddU16(totalPoints)
	w.AddU16(spentPoints)
	w.AddByte(wheel.ActivePreset)

	slotPoints := wheel.GetSlotPointsCopy()
	w.AddU16(uint16(len(slotPoints)))
	for slotID, pts := range slotPoints {
		w.AddU16(slotID)
		w.AddU16(pts)
	}

	g.SendToClient(w)
}
