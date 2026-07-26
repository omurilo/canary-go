package protocol

import (
	"fmt"
	"log/slog"

	"github.com/opentibiabr/canary-go/internal/game"
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
		// Character not found / tournament characters not supported.
		g.sendCyclopediaNoData(characterInfoType, 2)
		return
	}

	switch characterInfoType {
	case cyclopediaCharacterInfoInspection: // 9
		g.sendCyclopediaCharacterInspection(g.player)
	case cyclopediaCharacterInfoAchievements: // 5
		g.sendCyclopediaCharacterAchievements(g.player)
	default:
		// Not yet implemented — send "no data" so the client renders empty.
		g.sendCyclopediaNoData(characterInfoType, 1)
	}
}

// sendCyclopediaCharacterAchievements sends the player's achievements (cyclopedia tab).
// Mirrors C++ ProtocolGame::sendCyclopediaCharacterAchievements:
//   [0xDA][u8 type=5][u8 error]
//   [u16 totalPoints][u16 secretsUnlocked][u16 count]
//   per-achievement: [u16 id][u32 timestamp][u8 hasInfo(?name/desc/grade)]
//   hasInfo is 1 for secret achievements, which includes name/description/grade.
func (g *GameProtocol) sendCyclopediaCharacterAchievements(p *game.Player) {
	if p == nil {
		return
	}
	reg := g.deps.World.Achievements
	if reg == nil {
		g.sendCyclopediaNoData(cyclopediaCharacterInfoAchievements, 1)
		return
	}

	// Build entries from the player's unlocked achievements.
	type entry struct {
		ach        *game.Achievement
		timestamp  uint32
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
	w.AddByte(0x00)                                  // no error
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

// sendCyclopediaNoData sends a "not available" response for a cyclopedia tab, mirroring
// C++ ProtocolGame::sendCyclopediaCharacterNoData: [0xDA][u8 type][u8 errorCode].
// errorCode: 0=success, 1=not available, 2=character not found
func (g *GameProtocol) sendCyclopediaNoData(infoType uint8, errorCode uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(infoType)
	w.AddByte(errorCode)
	g.SendToClient(w)
}

func (g *GameProtocol) parseInspectionObject(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	inspectionType := r.GetByte()
	if inspectionType == 0 { // INSPECT_NORMALOBJECT
		pos := r.GetPosition()
		gamePos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
		tile := g.deps.World.Map.GetTile(gamePos)
		if tile != nil {
			if len(tile.Creatures) > 0 {
				if targetP, ok := tile.Creatures[0].(*game.Player); ok {
					g.sendCyclopediaCharacterInspection(targetP)
					return
				}
			}
			if len(tile.Items) > 0 {
				g.sendItemInspection(tile.Items[0], false)
				return
			}
			if tile.Ground != nil {
				g.sendItemInspection(tile.Ground, false)
				return
			}
		}
	} else if inspectionType == 1 || inspectionType == 3 { // INSPECT_NPCTRADE / INSPECT_CYCLOPEDIA
		itemId := r.GetU16()
		_ = r.GetByte() // itemCount
		dummyItem := &game.Item{ID: itemId, Count: 1}
		g.sendItemInspection(dummyItem, inspectionType == 3)
	}
}

func (g *GameProtocol) sendItemInspection(item *game.Item, cyclopedia bool) {
	if item == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x76)
	w.AddByte(0x00)
	if cyclopedia {
		w.AddByte(0x01)
	} else {
		w.AddByte(0x00)
	}
	w.AddU32(g.player.ID)
	w.AddByte(0x01)

	name := "Item"
	if t := g.deps.Items.Get(item.ID); t != nil && t.Name != "" {
		name = t.Name
	}
	w.AddString(name)
	g.addItem(w, item)
	w.AddByte(0) // imbuement slots
	w.AddByte(0) // description pairs

	g.SendToClient(w)
}

func (g *GameProtocol) sendCyclopediaCharacterInspection(target *game.Player) {
	if target == nil {
		return
	}
	slog.Default().Info("sendCyclopediaCharacterInspection", "target", target.Name, "deps.Items", g.deps.Items)
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoInspection) // 9
	w.AddByte(0x00)                              // error code (0 = success)

	// Count inventory items
	var invItems []*game.Item
	var invSlots []uint8
	for slot := uint8(game.ConstSlotFirst); slot <= uint8(game.ConstSlotLast); slot++ {
		if int(slot) < len(target.Inventory) && target.Inventory[slot] != nil {
			invItems = append(invItems, target.Inventory[slot])
			invSlots = append(invSlots, slot)
		}
	}

	w.AddByte(byte(len(invItems)))
	slog.Default().Info("sendCyclopediaCharacterInspection: items", "count", len(invItems))
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

	w.AddString(target.Name)
	addOutfit(w, target.Outfit)

	summary := []struct{ Key, Val string }{
		{"Level", fmt.Sprintf("%d", target.Level)},
		{"Health", fmt.Sprintf("%d / %d", target.Health, target.MaxHealth)},
		{"Mana", fmt.Sprintf("%d / %d", target.Mana, target.MaxMana)},
		{"Speed", fmt.Sprintf("%d", target.Speed)},
		{"Position", fmt.Sprintf("(%d, %d, %d)", target.Pos.X, target.Pos.Y, target.Pos.Z)},
	}

	w.AddByte(byte(len(summary)))
	for _, s := range summary {
		w.AddString(s.Key)
		w.AddString(s.Val)
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
