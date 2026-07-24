package protocol

import (
	"github.com/opentibiabr/canary-go/internal/bosstiary"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// SendBosstiaryEntryChanged notifies the client that a boss's cyclopedia entry
// changed (new level/points) so it refreshes that entry. Mirrors
// ProtocolGame::sendBosstiaryEntryChanged (0xE6 + u32 bossId).
func (g *GameProtocol) SendBosstiaryEntryChanged(bossID uint32) {
	w := netmsg.NewWriter()
	w.AddByte(0xE6)
	w.AddU32(bossID)
	g.SendToClient(w)
}

// SendBosstiaryData sends the static Boss Cyclopedia rules: the kill thresholds
// and points for each rarity (Bane/Archfoe/Nemesis) at each level
// (Prowess/Expertise/Mastery). Mirrors ProtocolGame::sendBosstiaryData
// (0x61 + 9 u16 kill thresholds + 9 u16 point rewards); layout matches the
// otclient parseBosstiaryData reader exactly.
func (g *GameProtocol) SendBosstiaryData() {
	g.SendToClient(buildBosstiaryData())
}

// buildBosstiaryData builds the 0x61 Boss Cyclopedia rules packet.
func buildBosstiaryData() *netmsg.Writer {
	w := netmsg.NewWriter()
	w.AddByte(0x61)
	order := []bosstiary.Rarity{bosstiary.RarityBane, bosstiary.RarityArchfoe, bosstiary.RarityNemesis}
	// First all kill thresholds (rarity-major, level-minor), then all points.
	for _, r := range order {
		s, _ := bosstiary.Stages(r)
		for _, stage := range s {
			w.AddU16(uint16(stage.Kills))
		}
	}
	for _, r := range order {
		s, _ := bosstiary.Stages(r)
		for _, stage := range s {
			w.AddU16(stage.Points)
		}
	}
	return w
}
