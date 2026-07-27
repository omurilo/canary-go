package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseStashAction handles opcode 0x28 (stash stow/withdraw). Ported 1:1 from
// C++ ProtocolGame::parseStashWithdraw (protocolgame.cpp:10759).
func (g *GameProtocol) parseStashAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	// C++: check stash menu available (player->isStashMenuAvailable())
	if !g.player.IsStashMenuAvailable() {
		return
	}

	if g.player.IsUIExhausted(500) {
		return
	}

	action := r.GetByte()
	switch action {
	case 0: // STASH_ACTION_STOW_ITEM (0): pos + itemId + stackpos + count, allItems=false
		pos := r.GetPosition()
		itemID := r.GetU16()
		stackpos := r.GetByte()
		count := uint32(r.GetByte())

		item := g.resolveStowItem(pos, int(stackpos), itemID)
		if item == nil {
			return
		}
		// C++: playerStowItem with allItems=false
		g.player.StowItem(item, count, false)
		g.sendStashRefresh()

	case 1: // STASH_ACTION_STOW_CONTAINER (1): pos + itemId + stackpos, allItems=false
		pos := r.GetPosition()
		itemID := r.GetU16()
		stackpos := r.GetByte()
		// C++: no count byte

		item := g.resolveStowItem(pos, int(stackpos), itemID)
		if item == nil {
			return
		}
		// C++: playerStowItem with count=0, allItems=false
		// When item is a container, scans container's contents
		g.player.StowItem(item, 0, false)
		g.sendStashRefresh()

	case 2: // STASH_ACTION_STOW_STACK (2): pos + itemId + stackpos, allItems=true
		pos := r.GetPosition()
		itemID := r.GetU16()
		stackpos := r.GetByte()

		item := g.resolveStowItem(pos, int(stackpos), itemID)
		if item == nil {
			return
		}
		// C++: playerStowItem with count=0, allItems=true
		g.player.StowItem(item, 0, true)
		g.sendStashRefresh()

	case 3: // STASH_ACTION_WITHDRAW (3): itemId + count + stackpos
		itemID := r.GetU16()
		count := r.GetU32()
		_ = r.GetByte() // stackpos (unused)

		if g.player.GetFreeCapacity() < 100 {
			return
		}
		if !g.player.RemoveFromStash(itemID, count) {
			return
		}
		// C++: creates item in backpack
		item := &game.Item{ID: itemID, Count: uint16(count)}
		bp := g.player.Inventory[game.ConstSlotBackpack]
		if bp != nil {
			bp.Contents = append([]*game.Item{item}, bp.Contents...)
		}
		g.sendStashRefresh()
	}

	g.player.UpdateUIExhausted()
}

// resolveStowItem resolves the actual Item* from a protocol position for stow actions.
// C++ equivalent: internalGetThing(player, pos, stackpos, itemId, STACKPOS_TOPDOWN_ITEM)
func (g *GameProtocol) resolveStowItem(pos netmsg.Position, stackpos int, itemID uint16) *game.Item {
	if pos.X == 0xFFFF {
		// Inventory / open container reference
		if pos.Y >= 0x40 {
			// pos.Y & 0x3F = container window CID
			cid := uint8(pos.Y - 0x40)
			container := g.player.GetContainerByID(cid)
			if container == nil {
				return nil
			}
			if stackpos < 0 || stackpos >= len(container.Contents) {
				return nil
			}
			candidate := container.Contents[stackpos]
			if candidate == nil || candidate.ID != itemID {
				return nil
			}
			return candidate
		}
		// Direct inventory slot
		if int(pos.Y) >= len(g.player.Inventory) {
			return nil
		}
		candidate := g.player.Inventory[pos.Y]
		if candidate == nil || candidate.ID != itemID {
			return nil
		}
		return candidate
	}

	// Map position → find in open containers by matching position
	gamePos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
	return g.player.FindItemInOpenContainers(gamePos, stackpos, itemID)
}

// sendStashRefresh envia stash + inventário + containers abertos atualizados.
func (g *GameProtocol) sendStashRefresh() {
	g.SendOpenStash()
	if g.player.Session == nil {
		return
	}
	g.player.Session.SendInventoryIds()
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
	w.AddByte(1) // stash available
	w.AddByte(1) // market available
	g.SendToClient(w)
}
