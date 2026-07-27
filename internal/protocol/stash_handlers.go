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
			// C++: addItemFromStash — usa InternalAddItem que mergeia em
			// stacks existentes, splitando por stackSize.
			if _, ok := g.player.InternalAddItem(g.deps.Items, itemID, count, -1, game.ConstSlotWhereever); !ok {
				// Rollback: devolve ao stash se não coube na backpack
				g.player.AddToStash(itemID, count)
				return
			}
		g.sendStashRefresh()
	}

	g.player.UpdateUIExhausted()
}

// resolveStowItem resolves the actual Item* from a protocol position for stow
// actions. Ported 1:1 from C++ Game::internalGetThing (game.cpp:1115) with
// STACKPOS_TOPDOWN_ITEM type.
func (g *GameProtocol) resolveStowItem(pos netmsg.Position, stackpos int, itemID uint16) *game.Item {
	if pos.X == 0xFFFF {
		if pos.Y >= 0x40 {
			// Container reference: pos.Y & 0x0F = CID, pos.Z = index within container
			// C++: player->getContainerByID(pos.y & 0x0F) + pos.z
			cid := uint8(pos.Y & 0x0F)
			container := g.player.GetContainerByID(cid)
			if container == nil {
				return nil
			}
			slot := int(pos.Z)
			// C++: player->getContainerIndex(cid) + slot → getItemByIndex
			// Simplified: slot is the 0-based index (containerIndex offset not tracked yet)
			if slot < 0 || slot >= len(container.Contents) {
				return nil
			}
			candidate := container.Contents[slot]
			if candidate == nil || candidate.ID != itemID {
				return nil
			}
			return candidate
		}

		// Inventory slot: pos.Y = slot index
		if int(pos.Y) >= len(g.player.Inventory) {
			return nil
		}
		slotItem := g.player.Inventory[pos.Y]
		if slotItem == nil {
			return nil
		}
		// C++: if index (stackpos) > 0, look inside the container at that slot
		// (getContainer → getItemByIndex(stackpos - 1))
		if stackpos > 0 && len(slotItem.Contents) > 0 {
			idx := stackpos - 1
			if idx < len(slotItem.Contents) {
				candidate := slotItem.Contents[idx]
				if candidate != nil && candidate.ID == itemID {
					return candidate
				}
			}
		}
		// Direct slot item
		if slotItem.ID != itemID {
			return nil
		}
		return slotItem
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
