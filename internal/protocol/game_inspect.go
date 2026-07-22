package protocol

import (
	"fmt"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

const (
	cyclopediaCharacterInfoInspection = 9
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
		return
	}
	characterID := r.GetU32()
	characterInfoType := r.GetByte()
	if characterID == 0 {
		characterID = g.player.ID
	}
	target := g.findPlayerByID(characterID)
	if target == nil {
		target = g.player
	}

	if characterInfoType == cyclopediaCharacterInfoInspection {
		g.sendCyclopediaCharacterInspection(target)
	}
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
