package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/imbuements"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

func (g *GameProtocol) SendImbuementWindow(action game.ImbuementAction, item *game.Item) {
	if g.player == nil {
		return
	}

	if item == nil && action == game.ImbuementActionPickItem {
		return
	}

	neededItems := make(map[uint16]uint16)

	w := netmsg.NewWriter()
	w.AddByte(0xEB)
	w.AddByte(byte(action))

	hasScrolls := byte(0)
	if g.player.GetItemTypeCount(g.deps.Items, 25013, -1) > 0 {
		hasScrolls = 1
	}
	w.AddByte(hasScrolls)

	switch action {
	case game.ImbuementActionOpen:
		w.AddU16(0)

	case game.ImbuementActionPickItem:
		g.player.ImbuingItem = item
		w.AddU16(item.ID)

		it := g.deps.Items.Get(item.ID)
		classification := uint8(0)
		if it != nil {
			classification = it.UpgradeClassification
		}
		if classification > 0 {
			w.AddByte(item.GetTier())
		}

		imbuementSlots := uint8(0)
		if it != nil {
			if v, ok := it.Stats["imbuementslot"]; ok {
				imbuementSlots = uint8(v)
			}
		}
		w.AddByte(imbuementSlots)

		for slotID := uint8(0); slotID < imbuementSlots; slotID++ {
			w.AddByte(0x00)
		}

		g.addAvailableImbuementsInfo(w, item, neededItems)

	case game.ImbuementActionScroll:
		freeSlots := g.player.GetFreeBackpackSlots(g.deps.Items)
		if freeSlots > 0 {
			w.AddByte(0x01)
		} else {
			w.AddByte(0x00)
		}
		w.AddByte(0)
		g.addAvailableImbuementsInfo(w, nil, neededItems)
	}

	w.AddU32(uint32(len(neededItems)))
	for id, amount := range neededItems {
		w.AddU16(id)
		w.AddU16(amount)
	}

	g.SendToClient(w)
}

func (g *GameProtocol) addImbuementInfo(w *netmsg.Writer, imb *imbuements.Imbuement) {
	baseImb := g.deps.World.Imbuements.GetBaseByID(imb.BaseID)
	if baseImb == nil {
		return
	}

	w.AddU32(uint32(imb.ID))
	w.AddString(baseImb.Name + " " + imb.Name)
	w.AddString(imb.Description)
	w.AddByte(uint8(imb.BaseID - 1))
	w.AddU16(uint16(imb.IconID))
	w.AddU32(baseImb.Duration)

	items := imb.Items
	w.AddByte(uint8(len(items)))
	for _, imbItem := range items {
		it := g.deps.Items.Get(imbItem.ID)
		name := "unknown"
		if it != nil {
			name = it.Name
		}
		w.AddU16(imbItem.ID)
		w.AddString(name)
		w.AddU16(imbItem.Count)
	}

	w.AddU32(baseImb.Price)
	w.AddU32(baseImb.ProtectionPrice)
	w.AddU32(uint32(baseImb.Percent))
}

func (g *GameProtocol) addAvailableImbuementsInfo(w *netmsg.Writer, item *game.Item, neededItems map[uint16]uint16) {
	imbReg := g.deps.World.Imbuements

	var itemTiers map[string]int32
	if item != nil {
		if it := g.deps.Items.Get(item.ID); it != nil {
			itemTiers = it.Stats
		}
	}

	allImb := imbReg.GetAllImbuements()

	var filtered []*imbuements.Imbuement
	for _, imb := range allImb {
		cat := imbReg.GetCategoryByID(imb.CategoryID)
		if cat == nil {
			continue
		}

		if itemTiers != nil {
			maxTier, ok := itemTiers[cat.Name]
			if !ok {
				continue
			}
			if int32(imb.BaseID) > maxTier {
				continue
			}
		}

		filtered = append(filtered, imb)
	}

	w.AddU16(uint16(len(filtered)))
	for _, imb := range filtered {
		g.addImbuementInfo(w, imb)
		for _, imbItem := range imb.Items {
			neededItems[imbItem.ID] = imbItem.Count
		}
	}
}

func (g *GameProtocol) parseImbuementAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	action := game.ImbuementAction(r.GetByte())

	var item *game.Item
	if action == game.ImbuementActionPickItem {
		pos := r.GetPosition()
		itemID := r.GetU16()
		stackpos := r.GetByte()
		item = g.getItemAt(pos, itemID, stackpos)
		if item == nil || item.ID != itemID {
			return
		}
	}

	g.SendImbuementWindow(action, item)
}

func (g *GameProtocol) parseImbuementApply(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	slot := r.GetByte()
	imbuementID := r.GetU32()
	if imbuementID > 0xFFFF {
		return
	}
	g.applyImbuement(slot, uint16(imbuementID))
}

func (g *GameProtocol) parseImbuementClear(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	slot := r.GetByte()
	g.clearImbuementSlot(slot)
}

func (g *GameProtocol) parseCloseImbuementWindow(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	g.player.ImbuingItem = nil
}

func (g *GameProtocol) applyImbuement(slot uint8, imbuementID uint16) {
	imbReg := g.deps.World.Imbuements
	imb := imbReg.GetImbuement(imbuementID)
	if imb == nil {
		g.player.SendTextMessage(0x14, "Invalid imbuement.")
		return
	}

	baseImb := imbReg.GetBaseByID(imb.BaseID)
	if baseImb == nil {
		g.player.SendTextMessage(0x14, "Invalid imbuement base.")
		return
	}

	cost := uint64(baseImb.Price)
	if !g.player.RemoveMoney(cost, true) {
		g.player.SendTextMessage(0x14, "You don't have enough money for this imbuement.")
		return
	}

	g.player.SendTextMessage(0x13, "Imbuement applied successfully!")
}

func (g *GameProtocol) clearImbuementSlot(slot uint8) {
	g.player.ImbuingItem = nil
	g.player.SendTextMessage(0x13, "Imbuement cleared.")
}
