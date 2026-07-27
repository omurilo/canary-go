package protocol

import (
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseStashAction handles opcode 0x28 (stash stow/withdraw).
// Wire format: [u8 action][Position*][u16 itemId][u8 stackpos][u8 count] (*action 0-2 only)
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
	case 0: // STOW_ITEM: Position(5), itemId(2), stackpos(1), count(1)
		_ = r.GetPosition()
		itemID := r.GetU16()
		_ = r.GetByte()
		_ = r.GetByte()
		g.player.StowItem(itemID, 0, false)
		g.SendOpenStash()

	case 1: // STOW_CONTAINER
		_ = r.GetPosition()
		itemID := r.GetU16()
		_ = r.GetByte()
		_ = r.GetByte()
		g.player.StowItem(itemID, 0, true)
		g.SendOpenStash()

	case 2: // STOW_STACK
		_ = r.GetPosition()
		itemID := r.GetU16()
		_ = r.GetByte()
		_ = r.GetByte()
		g.player.StowItem(itemID, 0, true)
		g.SendOpenStash()

	case 3: // WITHDRAW: itemId(2), count(4), stackpos(1)
		itemID := r.GetU16()
		count := r.GetU32()
		_ = r.GetByte()
		if g.player.GetFreeCapacity() < 100 {
			return
		}
		if g.player.RemoveFromStash(itemID, count) {
			g.SendOpenStash()
		}
	}
}

func (g *GameProtocol) SendOpenStash() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x29)
	if g.player.Stash == nil {
		w.AddU16(0)
		g.SendToClient(w)
		return
	}
	w.AddU16(uint16(len(g.player.Stash)))
	for id, cnt := range g.player.Stash {
		if cnt == 0 {
			continue
		}
		w.AddU16(id)
		w.AddU32(cnt)
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
