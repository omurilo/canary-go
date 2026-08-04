package protocol

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
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
			if container.Container == nil {
				return nil
			}
			// C++: player->getContainerIndex(cid) + slot → getItemByIndex
			slot := int(pos.Z) + int(g.player.GetContainerIndex(cid))
			if slot < 0 || slot >= len(container.Container.Contents) {
				return nil
			}
			candidate := container.Container.Contents[slot]
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
		if stackpos > 0 && slotItem.Container != nil && len(slotItem.Container.Contents) > 0 {
			idx := stackpos - 1
			if idx < len(slotItem.Container.Contents) {
				candidate := slotItem.Container.Contents[idx]
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

	// A map position means the TILE. This only searched the open containers, so an
	// item lying on the floor could never be resolved — dropping a decoration kit
	// in your own house and choosing unwrap answered "Sorry, not possible", because
	// the kit was never found in the first place.
	gamePos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
	if tile := g.deps.World.Map.GetTile(gamePos); tile != nil {
		if it := g.tileFindThing(tile, stackpos); it != nil {
			return it
		}
	}
	// Browse-field and other open-container views of the same position remain a
	// fallback, which is how an item inside a container on the tile is reached.
	return g.player.FindItemInOpenContainers(gamePos, stackpos, itemID)
}

// tileFindThing is internalGetThing's STACKPOS_FIND_THING branch
// (src/game/game.cpp:1156-1167): the thing at the client's index, else the tile's
// door, else its top down item.
func (g *GameProtocol) tileFindThing(tile *game.Tile, index int) *game.Item {
	if it := g.tileThingAt(tile, index); it != nil {
		return it
	}
	// getDoorItem, then getTopDownItem (tile.cpp:1956, and the top of the down
	// items). Both fall back to the ground on an empty tile, as C++ does.
	if len(tile.Items) == 0 {
		return tile.Ground
	}
	for _, it := range tile.Items {
		if it == nil {
			continue
		}
		if t := g.deps.Items.Get(it.ID); t != nil && t.IsDoor {
			return it
		}
	}
	for i := len(tile.Items) - 1; i >= 0; i-- {
		if tile.Items[i] != nil {
			return tile.Items[i]
		}
	}
	return nil
}

// tileThingAt is Tile::getThing (src/items/tile.cpp:1570-1599). The order is
// ground, top items, creatures, down items — and note C++ counts EVERY creature
// here, not only the ones the asking player can see.
//
// The Go tile keeps one Items slice rather than C++'s split vector, so the two
// groups are walked separately instead of indexed by offset.
func (g *GameProtocol) tileThingAt(tile *game.Tile, index int) *game.Item {
	if index < 0 {
		return nil
	}
	if tile.Ground != nil {
		if index == 0 {
			return tile.Ground
		}
		index--
	}

	tops := make([]*game.Item, 0, len(tile.Items))
	downs := make([]*game.Item, 0, len(tile.Items))
	for _, it := range tile.Items {
		if it == nil {
			continue
		}
		if g.isTopItem(it) {
			tops = append(tops, it)
		} else {
			downs = append(downs, it)
		}
	}

	if index < len(tops) {
		return tops[index]
	}
	index -= len(tops)

	if index < len(tile.Creatures) {
		return nil // a creature is not an item; the caller falls through
	}
	index -= len(tile.Creatures)

	if index < len(downs) {
		return downs[index]
	}
	return nil
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
			g.sendContainer(uint8(cid), oc.Container, oc.Container.Container != nil && oc.Container.Container.Parent != nil)
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
