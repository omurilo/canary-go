package protocol

import (
	"fmt"
	"log/slog"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/vocations"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

const (
	cyclopediaCharacterInfoInspection   = 9
	cyclopediaCharacterInfoAchievements = 5
)

func (g *GameProtocol) parseInspectPlayer(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	action := r.GetByte()
	if action >= 1 && action <= 5 {
		targetID := r.GetU32()
		target := g.findPlayerByID(targetID)
		if target == nil {
			target = g.player
		}
		g.sendCyclopediaCharacterInspection(target)
	} else {
		g.sendCyclopediaCharacterInspection(g.player)
	}
}

func (g *GameProtocol) parseCyclopediaCharacterInfo(r *netmsg.Reader) {
	if g.player == nil {
		slog.Default().Info("parseCyclopediaCharacterInfo: player nil")
		return
	}
	characterID := r.GetU32()
	characterInfoType := r.GetByte()
	slog.Default().Info("parseCyclopediaCharacterInfo", "charID", characterID, "infoType", characterInfoType)
	if characterID == 0 {
		characterID = g.player.ID
	}
	if characterID != g.player.ID {
		g.sendCyclopediaNoData(characterInfoType, 2)
		return
	}

	switch characterInfoType {
	case cyclopediaCharacterInfoInspection: // 9
		g.sendCyclopediaCharacterInspection(g.player)
	case cyclopediaCharacterInfoAchievements: // 5
		g.sendCyclopediaCharacterAchievements(g.player)
	default:
		g.sendCyclopediaNoData(characterInfoType, 1)
	}
}

// sendCyclopediaCharacterAchievements sends the player's achievements (cyclopedia tab).
func (g *GameProtocol) sendCyclopediaCharacterAchievements(p *game.Player) {
	if p == nil {
		return
	}
	reg := g.deps.World.Achievements
	if reg == nil {
		g.sendCyclopediaNoData(cyclopediaCharacterInfoAchievements, 1)
		return
	}

	type entry struct {
		ach       *game.Achievement
		timestamp uint32
	}
	entries := make([]entry, 0, len(p.Achievements))
	var totalPoints uint16
	var secretsUnlocked uint16

	for id, ts := range p.Achievements {
		ach := reg.GetByID(uint16(id))
		if ach == nil {
			continue
		}
		entries = append(entries, entry{ach: ach, timestamp: uint32(ts)})
		totalPoints += uint16(ach.Points)
		if ach.Secret {
			secretsUnlocked++
		}
	}

	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoAchievements) // 5
	w.AddByte(0x00)                                 // no error
	w.AddU16(totalPoints)
	w.AddU16(secretsUnlocked)
	w.AddU16(uint16(len(entries)))
	for _, e := range entries {
		w.AddU16(e.ach.ID)
		w.AddU32(e.timestamp)
		if e.ach.Secret {
			w.AddByte(1)
			w.AddString(e.ach.Name)
			w.AddString(e.ach.Description)
			w.AddByte(1) // grade (stars), default 1
		} else {
			w.AddByte(0)
		}
	}
	g.SendToClient(w)
}

// sendCyclopediaNoData sends [0xDA][u8 type][u8 errorCode] — a "no data available" response.
func (g *GameProtocol) sendCyclopediaNoData(infoType uint8, errorCode uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(infoType)
	w.AddByte(errorCode)
	g.SendToClient(w)
}

// sendCyclopediaCharacterInspection sends the 0xDA inspection packet with
// the full item serialization (matching C++ ProtocolGame::sendCyclopediaCharacterInspection).
// Mount is NOT sent in the outfit, matching the C++ addMount=false call.
func (g *GameProtocol) sendCyclopediaCharacterInspection(p *game.Player) {
	if p == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoInspection) // 9
	w.AddByte(0x00)                              // no error

	// Count inventory items first, then write.
	var invItems []*game.Item
	var invSlots []uint8
	for slot := uint8(game.ConstSlotFirst); slot <= uint8(game.ConstSlotLast); slot++ {
		if int(slot) < len(p.Inventory) && p.Inventory[slot] != nil {
			invItems = append(invItems, p.Inventory[slot])
			invSlots = append(invSlots, slot)
		}
	}
	w.AddByte(byte(len(invItems)))
	for i, item := range invItems {
		w.AddByte(invSlots[i])
		name := "Item"
		if t := g.deps.Items.Get(item.ID); t != nil && t.Name != "" {
			name = t.Name
		}
		w.AddString(name)
		g.addItem(w, item)
		w.AddByte(0) // imbuements count
		w.AddByte(0) // descriptions count
	}

	w.AddString(p.Name)
	addOutfitNoMount(w, p.Outfit) // C++ calls AddOutfit(msg, outfit, false)

	// Player descriptions (key-value pairs matching C++ sendCyclopediaCharacterInspection).
	vocName := "unknown"
	if voc := vocations.GetVocation(uint32(p.Vocation)); voc != nil {
		vocName = voc.Name
	}
	descriptions := []struct{ Key, Val string }{
		{"Level", fmt.Sprintf("%d", p.Level)},
		{"Vocation", vocName},
	}
	w.AddByte(byte(len(descriptions)))
	for _, d := range descriptions {
		w.AddString(d.Key)
		w.AddString(d.Val)
	}
	g.SendToClient(w)
}

func (g *GameProtocol) findPlayerByID(id uint32) *game.Player {
	if g.deps == nil || g.deps.World == nil {
		return nil
	}
	if p := g.deps.World.PlayerByID(id); p != nil {
		return p
	}
	for _, p := range g.deps.World.Players() {
		if p != nil && p.ID == id {
			return p
		}
	}
	return nil
}

func (g *GameProtocol) parseInspectionObject(r *netmsg.Reader) {
	inspectionType := r.GetByte()
	switch inspectionType {
	case 0: // INSPECT_NORMALOBJECT — inspect a thing on the map
		r.GetU16() // x
		r.GetU16() // y
		r.GetByte() // z
	case 1, 2: // INSPECT_NPCTRADE / INSPECT_CYCLOPEDIA — inspect item by ID
		r.GetU16() // itemId
		r.GetByte() // itemCount
	case 3: // INSPECT_PROFICIENCY
		r.GetU16() // itemId
		r.GetByte() // unknown
	}
}
