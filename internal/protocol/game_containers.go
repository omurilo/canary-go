package protocol

import (
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

	var item *game.Item
	if pos.X == 0xFFFF {
		if pos.Y >= 0x40 {
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
			}
		}
	} else {
		tile := g.deps.World.Map.GetTile(game.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
		if tile != nil {
			item = g.findTileItemByStackPos(tile, itemID, stackpos)
		}
	}

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
		gamePos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
		originalPos := g.player.Pos
		if g.deps.Lua.CallAction(action, g.player, item, gamePos, nil, gamePos, false) {
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
		if pos.X == 0xFFFF {
			g.player.OpenContainerAt(index, item)
			g.sendContainer(index, item, item.Parent != nil)
		} else {
			g.openContainer(item)
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
	if g.player == nil {
		return
	}
	cid := g.player.AddContainer(item) // reuses an existing cid or allocates one
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
