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
	_ = r.GetByte() // stackpos
	_ = r.GetByte() // index

	if pos.X == 0xFFFF {
		return // inventory / inside-container position — not supported yet
	}
	tile := g.deps.World.Map.GetTile(game.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	if tile == nil {
		return
	}
	item := findTileItem(tile, itemID)
	if item == nil {
		return
	}
	if t := g.deps.Items.Get(item.ID); t == nil || !t.IsContainer() {
		return // only container use is implemented
	}
	g.openContainer(item)
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
