package protocol

import (
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// SendBestiaryEntryChanged tells the client a bestiary monster entry changed
// (new unlock stage) so it refreshes it. Mirrors
// ProtocolGame::sendBestiaryEntryChanged (0xD9 + u16 raceid).
func (g *GameProtocol) SendBestiaryEntryChanged(raceID uint16) {
	w := netmsg.NewWriter()
	w.AddByte(0xD9)
	w.AddU16(raceID)
	g.SendToClient(w)
}
