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

// SendBosstiaryInfo sends the Boss Cyclopedia boss list (0x73): every boss with
// this player's kill count and unlock category. Mirrors the 0x73 block of
// ProtocolGame::parseSendBosstiary; layout matches otclient parseBosstiaryInfo
// (u16 count, then per boss: u32 raceId, u8 race, u32 kills, u8 reserved, u8
// tracker — the tracker flag is read at protocol >= 1320).
// bossListEntry is one boss row in the cyclopedia list (0x73).
type bossListEntry struct {
	RaceID  uint16
	Race    uint8
	Kills   uint32
	Tracked bool
}

func (g *GameProtocol) SendBosstiaryInfo() {
	if g.player == nil || g.deps == nil || g.deps.World == nil || g.deps.World.TypeRegistry == nil {
		return
	}
	bosses := g.deps.World.TypeRegistry.BosstiaryMonsters()
	entries := make([]bossListEntry, 0, len(bosses))
	for raceID, mt := range bosses {
		entries = append(entries, bossListEntry{
			RaceID: raceID,
			Race:   uint8(mt.BosstiaryRace),
			Kills:  g.player.GetBestiaryKillCount(raceID),
		})
	}
	g.SendToClient(buildBosstiaryInfo(entries))
}

// buildBosstiaryInfo builds the 0x73 Boss Cyclopedia list packet.
func buildBosstiaryInfo(entries []bossListEntry) *netmsg.Writer {
	w := netmsg.NewWriter()
	w.AddByte(0x73)
	w.AddU16(uint16(len(entries)))
	for _, e := range entries {
		w.AddU32(uint32(e.RaceID))
		w.AddByte(e.Race)
		w.AddU32(e.Kills)
		w.AddByte(0) // reserved
		if e.Tracked {
			w.AddByte(1)
		} else {
			w.AddByte(0)
		}
	}
	return w
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
