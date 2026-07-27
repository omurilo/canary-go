package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseStashAction handles opcode 0x28 (stash stow/withdraw).
func (g *GameProtocol) parseStashAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	if g.player.IsUIExhausted(500) {
		return
	}
	g.player.UpdateUIExhausted()

	action := r.GetByte()
	switch action {
	case 0: // STOW_ITEM: Position (5) + itemId (2) + stackpos (1) + count (1)
		pos := r.GetPosition()
		itemID := r.GetU16()
		_ = r.GetByte() // stackpos
		count := r.GetByte()
		// C++: internalGetThing(pos) → item → player->stowItem(item, count, false)
		if pos.X == 0xFFFF && int(pos.Y) < len(g.player.Inventory) {
			g.player.StowInventoryItem(int(pos.Y), uint32(count), false)
		} else {
			g.player.StowItem(itemID, uint32(count), count == 0)
		}
		g.sendStashRefresh()

	case 1: // STOW_CONTAINER: Position + itemId + stackpos
		_ = r.GetPosition()
		itemID := r.GetU16()
		_ = r.GetByte()
		g.player.StowItem(itemID, 0, true)
		g.sendStashRefresh()

	case 2: // STOW_STACK: Position + itemId + stackpos
		_ = r.GetPosition()
		itemID := r.GetU16()
		_ = r.GetByte()
		g.player.StowItem(itemID, 0, true)
		g.sendStashRefresh()

	case 3: // WITHDRAW: itemId + count + stackpos
		itemID := r.GetU16()
		count := r.GetU32()
		_ = r.GetByte()
		if g.player.GetFreeCapacity() < 100 {
			return
		}
		if !g.player.RemoveFromStash(itemID, count) {
			return
		}
		// Cria o item no inicio da backpack (C++: internalAddItem no inicio)
		item := &game.Item{ID: itemID, Count: uint16(count)}
		bp := g.player.Inventory[game.ConstSlotBackpack]
		if bp != nil {
			bp.Contents = append([]*game.Item{item}, bp.Contents...)
		}
		g.sendStashRefresh()
	}
}

// sendStashRefresh atualiza stash + inventário + containers abertos.
func (g *GameProtocol) sendStashRefresh() {
	g.SendOpenStash()
	if g.player.Session == nil {
		return
	}
	g.player.Session.SendInventoryIds()
	// Refresh open containers (backpack, bag, etc.)
	for cid, oc := range g.player.OpenContainersSnapshot() {
		if oc.Container != nil {
			g.sendContainer(uint8(cid), oc.Container, oc.Container.Parent != nil)
		}
	}
}

func (g *GameProtocol) SendOpenStash() {
	if g.player == nil || g.player.Stash == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x29)
	w.AddU16(uint16(len(g.player.Stash)))
	for id, cnt := range g.player.Stash {
		if cnt > 0 {
			w.AddU16(id)
			w.AddU32(cnt)
		}
	}
	g.SendToClient(w)
}

func (g *GameProtocol) sendSpecialContainersAvailable() {
	w := netmsg.NewWriter()
	w.AddByte(0x2A)
	w.AddByte(1)
	w.AddByte(1)
	g.SendToClient(w)
}
