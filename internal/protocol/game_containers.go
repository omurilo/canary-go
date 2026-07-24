package protocol

import (
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/actions"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// Container-related opcodes.
const (
	opContainerOpen  = 0x6E
	opContainerClose = 0x6F
)

// parseUseItem handles a use-item request (0x82). For now only map positions and
// the "open a container" outcome are supported, mirroring the container branch of
// Game::playerUseItem. Layout: position, itemId u16, stackpos u8, index u8.
func (g *GameProtocol) parseUseItem(r *netmsg.Reader) {
	pos := r.GetPosition()
	itemID := r.GetU16()
	stackpos := r.GetByte() // stackpos
	index := r.GetByte()    // index

	item := g.getItemAt(pos, itemID, stackpos)
	if item == nil {
		return
	}

	t := g.deps.Items.Get(item.ID)
	if t == nil {
		return
	}
	
	// Execute Lua action first
	action := actions.FindAction(item)
	if action != nil {
		isEx := g.isExAction(item)
		if isEx {
			if !g.player.CanDoPotionAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		} else {
			if !g.player.CanDoAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		}

		gamePos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
		originalPos := g.player.Pos
		beforeCount := item.Count
		if g.deps.Lua.CallAction(action, g.player, item, gamePos, nil, gamePos, false) {
			if isEx {
				g.player.SetNextPotionAction(1000 * time.Millisecond)
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(1000)
			} else {
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(200)
			}

			// If the script consumed/changed the item (e.g. food, runes calling
			// item:remove), reflect it on the client.
			if item.Count != beforeCount {
				g.reconcileUsedItem(item, pos, stackpos)
			}
			if g.player.Pos != originalPos {
				teleportedTo := g.player.Pos
				g.player.Pos = originalPos
				g.broadcastRemove(g.player)
				g.player.Pos = teleportedTo

				w := netmsg.NewWriter()
				w.AddByte(opFullMap)
				w.AddPosition(netmsg.Position{X: g.player.Pos.X, Y: g.player.Pos.Y, Z: g.player.Pos.Z})
				g.addMapDescription(w, int(g.player.Pos.X)-viewportX, int(g.player.Pos.Y)-viewportY, g.player.Pos.Z, mapWidth, mapHeight)
				g.SendToClient(w)
				g.broadcastAppear(g.player)
			}
			return // Handled by Lua script
		}
	}

	// Fallback to FloorChange if the item has it
	if t.FloorChange != "" {
		teleportPos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
		// Typically, using a ladder/sewer drops you at the same X/Y but different Z, 
		// but let's apply the floor change shift if any.
		switch t.FloorChange {
		case "down":
			teleportPos.Z++
		case "north":
			teleportPos.Z--
			teleportPos.Y--
		case "south":
			teleportPos.Z--
			teleportPos.Y++
		case "east":
			teleportPos.Z--
			teleportPos.X++
		case "west":
			teleportPos.Z--
			teleportPos.X--
		}
		
		g.broadcastRemove(g.player)
		g.deps.World.SetPosition(g.player, teleportPos)
		
		w := netmsg.NewWriter()
		w.AddByte(opFullMap)
		w.AddPosition(netmsg.Position{X: g.player.Pos.X, Y: g.player.Pos.Y, Z: g.player.Pos.Z})
		g.addMapDescription(w, int(g.player.Pos.X)-viewportX, int(g.player.Pos.Y)-viewportY, g.player.Pos.Z, mapWidth, mapHeight)
		g.SendToClient(w)
		
		g.broadcastAppear(g.player)
		return
	}

	if t.IsContainer() {
		if cid := g.player.GetContainerID(item); cid != -1 {
			g.player.CloseContainer(uint8(cid))
			w := netmsg.NewWriter()
			w.AddByte(opContainerClose)
			w.AddByte(uint8(cid))
			g.SendToClient(w)
			return
		}

		if pos.X == 0xFFFF {
			var isOnMap bool
			var containerPos game.Position
			if pos.Y >= 0x40 {
				parentCid := uint8(pos.Y - 0x40)
				if parentOc, ok := g.player.OpenContainersSnapshot()[parentCid]; ok {
					if parentOc.IsOnMap {
						isOnMap = true
						containerPos = parentOc.Position
					}
				}
			}
			g.player.OpenContainerAtWithPos(index, item, containerPos, isOnMap)
			g.sendContainer(index, item, item.Parent != nil)
		} else {
			g.openContainerWithPos(item, game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}, true)
		}
	} else if t.FloorChange != "" {
		teleportPos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
		switch t.FloorChange {
		case "down":
			teleportPos.Z++
		case "north":
			teleportPos.Z--
			teleportPos.Y--
		case "south":
			teleportPos.Z--
			teleportPos.Y++
		case "east":
			teleportPos.Z--
			teleportPos.X++
		case "west":
			teleportPos.Z--
			teleportPos.X--
		case "southalt":
			teleportPos.Z--
			teleportPos.Y+=2
		case "eastalt":
			teleportPos.Z--
			teleportPos.X+=2
		}

		g.broadcastRemove(g.player)
		g.deps.World.SetPosition(g.player, teleportPos)

		w := netmsg.NewWriter()
		w.AddByte(opFullMap)
		w.AddPosition(netmsg.Position{X: teleportPos.X, Y: teleportPos.Y, Z: teleportPos.Z})
		g.addMapDescription(w, int(teleportPos.X)-viewportX, int(teleportPos.Y)-viewportY, teleportPos.Z, mapWidth, mapHeight)
		g.SendToClient(w)

		g.broadcastAppear(g.player)
	} else if (t.ForceUse || t.IsLadder) && pos.X != 0xFFFF {
		teleportPos := game.Position{X: pos.X, Y: pos.Y + 1, Z: pos.Z - 1}
		p := g.player
		
		g.broadcastRemove(p)
		
		g.deps.World.SetPosition(p, teleportPos)
		
		w := netmsg.NewWriter()
		w.AddByte(opFullMap)
		w.AddPosition(netmsg.Position{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z})
		g.addMapDescription(w, int(p.Pos.X)-viewportX, int(p.Pos.Y)-viewportY, p.Pos.Z, mapWidth, mapHeight)
		g.SendToClient(w)
		
		g.broadcastAppear(p)
	}
}

// reconcileUsedItem updates the client after a use-action mutated an item's
// stack count (e.g. eating food, using a rune). When the stack is emptied the
// item is removed from its container/inventory slot; otherwise the reduced
// stack is re-sent. `pos` is the item's source location as sent by the client
// and `stackpos` is the map stack index (only used for map items).
func (g *GameProtocol) reconcileUsedItem(item *game.Item, pos netmsg.Position, stackpos uint8) {
	consumed := item.Count == 0
	if pos.X == 0xFFFF {
		if pos.Y == 0 && g.player != nil {
			foundSlot := uint8(0)
			for slot := uint8(1); slot <= 10; slot++ {
				if g.player.Inventory[slot] == item {
					foundSlot = slot
					break
				}
			}
			if foundSlot > 0 {
				pos.Y = uint16(foundSlot)
			} else {
				foundCID := uint8(255)
				foundContSlot := uint8(0)
				for cid := uint8(0); cid < 16; cid++ {
					if cont, ok := g.openContainerByCID(cid); ok {
						for i, contItem := range cont.Contents {
							if contItem == item {
								foundCID = cid
								foundContSlot = uint8(i)
								break
							}
						}
						if foundCID != 255 {
							break
						}
					}
				}
				if foundCID != 255 {
					pos.Y = uint16(0x40 + foundCID)
					pos.Z = uint8(foundContSlot)
				}
			}
		}
		if pos.Y >= 0x40 { // inside a container
			cid := uint8(pos.Y - 0x40)
			slot := uint8(pos.Z)
			cont, ok := g.openContainerByCID(cid)
			if !ok {
				return
			}
			if consumed {
				if int(slot) < len(cont.Contents) {
					cont.Contents = append(cont.Contents[:slot], cont.Contents[slot+1:]...)
				}
				g.sendRemoveContainerItem(cid, slot, nil)
				g.refreshContainerIfOpen(cont)
			} else {
				g.sendUpdateContainerItem(cid, slot, item)
			}
			return
		}
		// equipment slot
		slot := uint8(pos.Y)
		if slot == 0 || slot > 10 {
			return
		}
		if consumed {
			g.player.Inventory[slot] = nil
			g.sendInventoryEmpty(slot)
		} else {
			g.sendInventoryItem(slot, item)
		}
		return
	}
	// On the map.
	gp := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
	if consumed {
		g.deps.World.Map.RemoveItemPtr(gp, item)
		g.broadcastRemoveTileThing(gp, stackpos)
	} else {
		g.broadcastUpdateTileThing(gp, stackpos, item)
	}
}

// findTileItem returns the first ground/stacked item on the tile with the id.
func findTileItem(tile *game.Tile, id uint16) *game.Item {
	if tile.Ground != nil && tile.Ground.ID == id {
		return tile.Ground
	}
	for _, it := range tile.Items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

// openContainer assigns a client container id and sends the container window.
func (g *GameProtocol) openContainer(item *game.Item) {
	g.openContainerWithPos(item, game.Position{}, false)
}

// openContainerWithPos assigns a client container id, setting explicit Position / IsOnMap metadata.
func (g *GameProtocol) openContainerWithPos(item *game.Item, pos game.Position, isOnMap bool) {
	if g.player == nil {
		return
	}
	cid := g.player.AddContainerWithPos(item, pos, isOnMap)
	if cid < 0 {
		return // all 16 container slots in use
	}
	g.sendContainer(uint8(cid), item, item.Parent != nil)
}

// sendContainer sends the container window (0x6E), mirroring the modern layout of
// ProtocolGame::sendContainer for a normal (non store-inbox) container.
func (g *GameProtocol) sendContainer(cid uint8, item *game.Item, hasParent bool) {
	t := g.deps.Items.Get(item.ID)
	name := "Container"
	movable := byte(0)
	if t != nil {
		if t.Name != "" {
			name = t.Name
		}
		if t.Pickupable {
			movable = 1
		}
	}
	contents := item.Contents
	// Real container capacity (Container::capacity), clamped to at least the
	// number of items currently shown and to the byte range the packet allows.
	capacity := int(item.ContainerCapacity(g.deps.Items))
	if capacity < len(contents) {
		capacity = len(contents)
	}
	if capacity < 1 {
		capacity = 1
	}
	if capacity > 0xFF {
		capacity = 0xFF
	}
	unlocked := byte(1) // drag & drop allowed unless explicitly locked
	pagination := boolByte(item.Pagination)
	firstIndex := uint16(0)
	if g.player != nil {
		firstIndex = g.player.GetContainerIndex(cid)
	}
	page := len(contents)
	if page > 0xFF {
		page = 0xFF
	}

	w := netmsg.NewWriter()
	w.AddByte(opContainerOpen)
	w.AddByte(cid)
	g.addItem(w, item) // the container item itself
	w.AddString(name)
	w.AddByte(byte(capacity))
	w.AddByte(boolByte(hasParent))
	w.AddByte(0) // depot search available
	w.AddByte(unlocked)
	w.AddByte(pagination)
	w.AddU16(uint16(len(contents)))
	w.AddU16(firstIndex)
	w.AddByte(byte(page))
	for i := 0; i < page; i++ {
		g.addItem(w, contents[i])
	}
	// 13.21+ trailer for a normal container.
	w.AddByte(0x00)
	w.AddByte(0x00)
	w.AddByte(movable) // is movable
	w.AddByte(0)       // held by a player
	g.SendToClient(w)
}

// parseCloseContainer handles a close-container request (0x87) and confirms it.
func (g *GameProtocol) parseCloseContainer(r *netmsg.Reader) {
	cid := r.GetByte()
	if g.player != nil {
		g.player.CloseContainer(cid)
	}
	w := netmsg.NewWriter()
	w.AddByte(opContainerClose)
	w.AddByte(cid)
	g.SendToClient(w)
}

// parseContainerUp handles a container up navigation request (0x88).
func (g *GameProtocol) parseContainerUp(r *netmsg.Reader) {
	cid := r.GetByte()
	if g.player == nil {
		return
	}
	c := g.player.GetContainerByID(cid)
	if c != nil && c.Parent != nil {
		g.player.OpenContainerAt(cid, c.Parent)
		g.sendContainer(cid, c.Parent, c.Parent.Parent != nil)
	}
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func (g *GameProtocol) sendAddContainerItem(cid uint8, slot uint16, item *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x70) // opContainerAddItem
	w.AddByte(cid)
	w.AddU16(slot)
	g.addItem(w, item)
	g.SendToClient(w)
}

func (g *GameProtocol) sendUpdateContainerItem(cid uint8, slot uint8, item *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x71) // opContainerUpdateItem
	w.AddByte(cid)
	w.AddU16(uint16(slot))
	g.addItem(w, item)
	g.SendToClient(w)
}

func (g *GameProtocol) sendRemoveContainerItem(cid uint8, slot uint8, lastItem *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x72) // opContainerRemoveItem
	w.AddByte(cid)
	w.AddU16(uint16(slot))
	if lastItem != nil {
		g.addItem(w, lastItem)
	} else {
		w.AddU16(0x00) // Empty item indicating no more items paginated
	}
	g.SendToClient(w)
}

func (g *GameProtocol) sendInventoryItem(slot uint8, item *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x78) // opInventoryItem
	w.AddByte(slot)
	g.addItem(w, item)
	g.SendToClient(w)
}

func (g *GameProtocol) sendInventoryEmpty(slot uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0x79) // opInventoryEmpty
	w.AddByte(slot)
	g.SendToClient(w)
}

// CheckMapContainersDistance automatically closes any open map/ground containers
// that have exceeded a distance of 2 steps (or any floor change) from the player.
func (g *GameProtocol) CheckMapContainersDistance() {
	if g.player == nil {
		return
	}
	for cid, oc := range g.player.OpenContainersSnapshot() {
		if oc.IsOnMap {
			dx := absDiff(g.player.Pos.X, oc.Position.X)
			dy := absDiff(g.player.Pos.Y, oc.Position.Y)
			dz := absDiffByte(g.player.Pos.Z, oc.Position.Z)
			if dx > 2 || dy > 2 || dz != 0 {
				g.player.CloseContainer(cid)
				w := netmsg.NewWriter()
				w.AddByte(opContainerClose)
				w.AddByte(cid)
				g.SendToClient(w)
			}
		}
	}
}

func absDiff(a, b uint16) uint16 {
	if a > b {
		return a - b
	}
	return b - a
}

func absDiffByte(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// parseUseItemWith handles a use-item-with request (0x83).
func (g *GameProtocol) parseUseItemWith(r *netmsg.Reader) {
	fromPos := r.GetPosition()
	fromItemID := r.GetU16()
	fromStackPos := r.GetByte()
	toPos := r.GetPosition()
	toItemID := r.GetU16()
	toStackPos := r.GetByte()

	g.deps.Log.Debug("parseUseItemWith", "player", g.player.Name, "fromPos", fromPos, "fromItemID", fromItemID, "toPos", toPos, "toItemID", toItemID)

	fromItem := g.getItemAt(fromPos, fromItemID, fromStackPos)
	if fromItem == nil {
		g.deps.Log.Debug("parseUseItemWith: fromItem is nil")
		return
	}

	toItem := g.getItemAt(toPos, toItemID, toStackPos)

	// Execute Lua action
	action := actions.FindAction(fromItem)
	if action != nil {
		isEx := g.isExAction(fromItem)
		if isEx {
			if !g.player.CanDoPotionAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		} else {
			if !g.player.CanDoAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		}

		fromGamePos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
		toGamePos := game.Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z}
		beforeCount := fromItem.Count
		if g.deps.Lua.CallAction(action, g.player, fromItem, fromGamePos, toItem, toGamePos, false) {
			if isEx {
				g.player.SetNextPotionAction(1000 * time.Millisecond)
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(1000)
			} else {
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(200)
			}
			if fromItem.Count != beforeCount {
				g.reconcileUsedItem(fromItem, fromPos, fromStackPos)
			}
			return
		}
	}
}

// parseUseWithCreature handles a use-item-with-creature request (0x84).
func (g *GameProtocol) parseUseWithCreature(r *netmsg.Reader) {
	fromPos := r.GetPosition()
	fromItemID := r.GetU16()
	fromStackPos := r.GetByte()
	creatureID := r.GetU32()

	g.deps.Log.Debug("parseUseWithCreature", "player", g.player.Name, "fromPos", fromPos, "fromItemID", fromItemID, "creatureID", creatureID)

	fromItem := g.getItemAt(fromPos, fromItemID, fromStackPos)
	if fromItem == nil {
		g.deps.Log.Debug("parseUseWithCreature: fromItem is nil")
		return
	}

	targetCreature := g.deps.World.CreatureByID(creatureID)
	if targetCreature == nil {
		g.deps.Log.Debug("parseUseWithCreature: targetCreature is nil", "creatureID", creatureID)
		return
	}

	// Execute Lua action
	action := actions.FindAction(fromItem)
	if action != nil {
		isEx := g.isExAction(fromItem)
		if isEx {
			if !g.player.CanDoPotionAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		} else {
			if !g.player.CanDoAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		}

		fromGamePos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
		toGamePos := targetCreature.GetPosition()
		beforeCount := fromItem.Count
		if g.deps.Lua.CallAction(action, g.player, fromItem, fromGamePos, targetCreature, toGamePos, false) {
			if isEx {
				g.player.SetNextPotionAction(1000 * time.Millisecond)
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(1000)
			} else {
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(200)
			}
			if fromItem.Count != beforeCount {
				g.reconcileUsedItem(fromItem, fromPos, fromStackPos)
			}
			return
		}
	}
}

// getItemAt returns an item from the given client netmsg.Position and stackpos.
func (g *GameProtocol) getItemAt(pos netmsg.Position, itemID uint16, stackpos uint8) *game.Item {
	var item *game.Item
	if pos.X == 0xFFFF {
		if pos.Y == 0 {
			if g.player != nil {
				item = g.player.FindItemOfType(g.deps.Items, itemID, true, -1)
			}
		} else if pos.Y >= 0x40 {
			cid := uint8(pos.Y - 0x40)
			if cont, ok := g.openContainerByCID(cid); ok {
				fromSlot := int(pos.Z)
				if fromSlot < len(cont.Contents) {
					item = cont.Contents[fromSlot]
				}
			}
		} else {
			slot := uint8(pos.Y)
			if slot > 0 && slot <= 10 {
				item = g.player.Inventory[slot]
			} else if slot == 11 { // CONST_SLOT_STORE_INBOX
				if g.player.StoreInbox == nil {
					g.player.StoreInbox = &game.Item{ID: 23396}
				}
				item = g.player.StoreInbox
			}
		}
	} else {
		tile := g.deps.World.Map.GetTile(game.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
		if tile != nil {
			item = g.findTileItemByStackPos(tile, itemID, stackpos)
		}
	}
	if item == nil && itemID != 0 && g.player != nil {
		item = g.player.FindItemOfType(g.deps.Items, itemID, true, -1)
	}
	return item
}

func (g *GameProtocol) sendCancelMessage(text string) {
	w := netmsg.NewWriter()
	w.AddByte(opTextMessage)
	w.AddByte(22) // MESSAGE_FAILURE / STATUS_SMALL
	w.AddString(text)
	g.SendToClient(w)
}

// SendUseItemCooldown sends an item use cooldown packet (0xA6) to the client.
func (g *GameProtocol) SendUseItemCooldown(ms uint32) {
	w := netmsg.NewWriter()
	w.AddByte(0xA6)
	w.AddU32(ms)
	g.SendToClient(w)
}

func (g *GameProtocol) isExAction(item *game.Item) bool {
	if item == nil {
		return false
	}
	if g.deps != nil && g.deps.Items != nil {
		if t := g.deps.Items.Get(item.ID); t != nil {
			if strings.EqualFold(t.TypeName, "potion") || strings.EqualFold(t.TypeName, "rune") || strings.Contains(strings.ToLower(t.Name), "potion") {
				return true
			}
		}
	}
	id := item.ID
	if (id >= 236 && id <= 239) || id == 266 || id == 7618 || id == 7620 || (id >= 8472 && id <= 8473) || (id >= 23373 && id <= 23375) || id == 35563 {
		return true
	}
	return false
}

