package protocol

import (
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
			if cont, ok := g.containers[cid]; ok {
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

	if t.IsContainer() {
		if pos.X == 0xFFFF {
			g.containers[index] = item
			g.sendContainer(index, item, false)
		} else {
			g.openContainer(item)
		}
		return
	}

	if (t.ForceUse || t.IsLadder) && pos.X != 0xFFFF {
		teleportPos := game.Position{X: pos.X, Y: pos.Y + 1, Z: pos.Z - 1}
		p := g.player
		
		g.broadcastRemove(p)
		
		g.deps.World.SetPosition(p, teleportPos)
		
		idx := g.buildCreatureIndex(p.Pos)
		w := netmsg.NewWriter()
		w.AddByte(opFullMap)
		w.AddPosition(netmsg.Position{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z})
		g.addMapDescription(w, int(p.Pos.X)-viewportX, int(p.Pos.Y)-viewportY, p.Pos.Z, mapWidth, mapHeight, idx)
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
	for cid, open := range g.containers {
		if open == item { // already open — just refresh it
			g.sendContainer(cid, item, false)
			return
		}
	}
	cid := g.nextContainerID()
	if cid == 0xFF {
		return // all 16 container slots in use
	}
	g.containers[cid] = item
	g.sendContainer(cid, item, false)
}

// nextContainerID returns the lowest free container id (0-15), or 0xFF if none.
func (g *GameProtocol) nextContainerID() uint8 {
	for i := uint8(0); i < 16; i++ {
		if _, ok := g.containers[i]; !ok {
			return i
		}
	}
	return 0xFF
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
	capacity := len(contents)
	if capacity < 8 {
		capacity = 8
	}
	if capacity > 0xFF {
		capacity = 0xFF
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
	w.AddByte(1) // unlocked (drag & drop)
	w.AddByte(0) // has pagination
	w.AddU16(uint16(len(contents)))
	w.AddU16(0) // first index
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
	delete(g.containers, cid)
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
