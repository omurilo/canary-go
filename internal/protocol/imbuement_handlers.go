package protocol

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/imbuements"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

const emptyImbuementScrollID = 51442

func (g *GameProtocol) SendImbuementWindow(action game.ImbuementAction, item *game.Item) {
	if g.player == nil {
		return
	}

	if item == nil && action == game.ImbuementActionPickItem {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0xEB)
	w.AddByte(byte(action))

	neededItems := make(map[uint16]uint16)

	hasScrolls := g.player.GetItemTypeCount(g.deps.Items, emptyImbuementScrollID, -1) > 0
	if hasScrolls {
		w.AddByte(0x01)
	} else {
		w.AddByte(0x00)
	}

	switch action {
	case game.ImbuementActionOpen:
		w.AddU16(0)

	case game.ImbuementActionPickItem:
		g.player.ImbuingItem = item
		w.AddU16(item.ID)

		it := g.deps.Items.Get(item.ID)
		imbuementSlots := uint8(0)
		if it != nil {
			if v, ok := it.Stats["imbuementslot"]; ok {
				imbuementSlots = uint8(v)
			}
		}

		if it != nil && it.UpgradeClassification > 0 {
			w.AddByte(item.GetTier())
		}

		w.AddByte(imbuementSlots)

		imbReg := g.deps.World.Imbuements
		for i := uint8(0); i < imbuementSlots; i++ {
			info, ok := item.GetImbuementInfo(i)
			if !ok {
				w.AddByte(0x00)
				continue
			}
			w.AddByte(0x01)
			if imb := imbReg.GetImbuement(info.ID); imb != nil {
				g.addImbuementInfo(w, imb, false)
			}
			w.AddU32(info.Duration)
			if imb := imbReg.GetImbuement(info.ID); imb != nil {
				if base := imbReg.GetBaseByID(imb.BaseID); base != nil {
					w.AddU32(base.RemoveCost)
				} else {
					w.AddU32(0)
				}
			} else {
				w.AddU32(0)
			}
		}

		g.addAvailableImbuementsInfo(w, item, neededItems, false)

	case game.ImbuementActionScroll:
		freeSlots := g.player.GetFreeBackpackSlots(g.deps.Items)
		if freeSlots > 0 {
			w.AddByte(0x01)
		} else {
			w.AddByte(0x00)
		}
		w.AddByte(0)

		g.addAvailableImbuementsInfo(w, nil, neededItems, true)
	}

	w.AddU32(uint32(len(neededItems)))
	for id, amount := range neededItems {
		w.AddU16(id)
		w.AddU16(amount)
	}

	g.sendResourceBalance(0x00, g.player.BankBalance)
	g.sendResourceBalance(0x01, g.player.GetMoney())

	g.SendToClient(w)
}

func (g *GameProtocol) addAvailableImbuementsInfo(w *netmsg.Writer, item *game.Item, neededItems map[uint16]uint16, isScrollAction bool) {
	imbReg := g.deps.World.Imbuements
	if imbReg == nil {
		w.AddU16(0)
		return
	}

	var filtered []*imbuements.Imbuement
	allImb := imbReg.GetAllImbuements()

	for _, imb := range allImb {
		if isScrollAction && imb.ScrollID == 0 {
			continue
		}
		if item != nil {
			cat := imbReg.GetCategoryByID(imb.CategoryID)
			if cat == nil {
				continue
			}
			it := g.deps.Items.Get(item.ID)
			if it == nil {
				continue
			}
			lookupKey := normalizeImbuementCatName(cat.Name)
			maxTier, ok := it.Stats[lookupKey]
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
		g.addImbuementInfo(w, imb, isScrollAction)
		for _, imbItem := range imb.Items {
			if _, ok := neededItems[imbItem.ID]; ok {
				continue
			}
			invCount := g.player.GetItemTypeCount(g.deps.Items, imbItem.ID, -1)
			if invCount > 0xFFFF {
				invCount = 0xFFFF
			}
			neededItems[imbItem.ID] = uint16(invCount)
		}
	}

	if isScrollAction {
		if _, ok := neededItems[emptyImbuementScrollID]; !ok {
			scrollCount := g.player.GetItemTypeCount(g.deps.Items, emptyImbuementScrollID, -1)
			if scrollCount > 0xFFFF {
				scrollCount = 0xFFFF
			}
			neededItems[emptyImbuementScrollID] = uint16(scrollCount)
		}
	}
}

func (g *GameProtocol) addImbuementInfo(w *netmsg.Writer, imb *imbuements.Imbuement, isScrollAction bool) {
	baseImb := g.deps.World.Imbuements.GetBaseByID(imb.BaseID)
	if baseImb == nil {
		if g.deps != nil && g.deps.Log != nil {
			g.deps.Log.Warn("imbuement missing base", "imbID", imb.ID, "baseID", imb.BaseID)
		}
		w.AddU32(uint32(imb.ID))
		w.AddString(imb.Name)
		w.AddString("")
		w.AddByte(0)
		w.AddU16(0)
		w.AddU32(0)
		w.AddByte(0)
		w.AddU32(0)
		return
	}

	w.AddU32(uint32(imb.ID))
	w.AddString(baseImb.Name + " " + imb.Name)
	w.AddString(imb.Description)
	w.AddByte(byte(imb.BaseID - 1))
	w.AddU16(uint16(imb.IconID))
	w.AddU32(baseImb.Duration)

	items := make([]imbuements.ImbuementItem, len(imb.Items))
	copy(items, imb.Items)
	if isScrollAction {
		items = append(items, imbuements.ImbuementItem{ID: emptyImbuementScrollID, Count: 1})
	}

	w.AddByte(uint8(len(items)))
	for _, imbItem := range items {
		itemType := g.deps.Items.Get(imbItem.ID)
		name := "unknown"
		if itemType != nil {
			name = itemType.Name
		}
		w.AddU16(imbItem.ID)
		w.AddString(name)
		w.AddU16(imbItem.Count)
	}

	w.AddU32(baseImb.Price)
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

		it := g.deps.Items.Get(item.ID)
		hasSlots := false
		if it != nil {
			if v, ok := it.Stats["imbuementslot"]; ok && v > 0 {
				hasSlots = true
			}
		}
		if !hasSlots {
			g.sendCancelMessage("This item is not imbuable.")
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

	item := g.player.ImbuingItem
	if item == nil {
		g.player.SendTextMessage(0x14, "No item selected for imbuement.")
		return
	}

	for _, imbItem := range imb.Items {
		count := g.player.GetItemTypeCount(g.deps.Items, imbItem.ID, -1)
		if count < uint32(imbItem.Count) {
			g.player.SendTextMessage(0x14, "You don't have the required items.")
			return
		}
	}

	cost := uint64(baseImb.Price)
	if !g.player.RemoveMoney(cost, true) {
		g.player.SendTextMessage(0x14, "You don't have enough money for this imbuement.")
		return
	}

	for _, imbItem := range imb.Items {
		g.player.RemoveItemOfType(g.deps.Items, imbItem.ID, uint32(imbItem.Count), -1, false)
	}

	item.SetImbuement(slot, imbuementID, baseImb.Duration)
	g.refreshContainers()

	g.player.SendTextMessage(0x13, "Imbuement applied successfully!")
	g.SendImbuementWindow(game.ImbuementActionPickItem, item)
}

func (g *GameProtocol) clearImbuementSlot(slot uint8) {
	item := g.player.ImbuingItem
	if item == nil {
		return
	}

	info, ok := item.GetImbuementInfo(slot)
	if !ok {
		g.player.SendTextMessage(0x14, "This slot has no imbuement to clear.")
		return
	}

	imbReg := g.deps.World.Imbuements
	imb := imbReg.GetImbuement(info.ID)
	if imb == nil {
		return
	}
	baseImb := imbReg.GetBaseByID(imb.BaseID)
	if baseImb != nil && baseImb.RemoveCost > 0 {
		cost := uint64(baseImb.RemoveCost)
		if !g.player.RemoveMoney(cost, true) {
			g.player.SendTextMessage(0x14, "You don't have enough money to remove this imbuement.")
			return
		}
	}

	item.ClearImbuement(slot)
	g.refreshContainers()

	g.player.SendTextMessage(0x13, "Imbuement cleared.")
	g.SendImbuementWindow(game.ImbuementActionPickItem, item)
}

func (g *GameProtocol) refreshContainers() {
	for cid, container := range g.rangeContainers() {
		if container != nil {
			g.sendContainer(cid, container, container.Parent != nil)
		}
	}
}

func normalizeImbuementCatName(name string) string {
	key := strings.ToLower(name)
	key = strings.ReplaceAll(key, "(", "")
	key = strings.ReplaceAll(key, ")", "")
	if strings.HasPrefix(key, "skillboost ") && strings.HasSuffix(key, " fighting") {
		key = strings.TrimSuffix(key, " fighting")
	}
	return strings.TrimSpace(key)
}
