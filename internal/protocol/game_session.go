package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// This file implements the game.Session methods added for the essential-
// playability slice (inventory / containers / stats / shop). They are the
// exported surface the game model and the Lua engine call after mutating the
// player; most delegate to the existing unexported protocol helpers.

// SendInventoryItem pushes a single equipment slot (0x78).
func (g *GameProtocol) SendInventoryItem(slot uint8, it *game.Item) {
	if it == nil {
		g.sendInventoryEmpty(slot)
		return
	}
	g.sendInventoryItem(slot, it)
}

// SendInventoryEmpty clears an equipment slot (0x79).
func (g *GameProtocol) SendInventoryEmpty(slot uint8) { g.sendInventoryEmpty(slot) }

// SendStats pushes the player stats block (0xA0).
func (g *GameProtocol) SendStats() { g.sendStats() }

// SendSkills pushes the player skills block (0xA1).
func (g *GameProtocol) SendSkills() {
	w := netmsg.NewWriter()
	g.addSkills(w)
	g.conn.Send(w)
}

// OpenContainer allocates/reuses a client container id and sends the window.
func (g *GameProtocol) OpenContainer(c *game.Item) {
	if c != nil {
		g.openContainer(c)
	}
}

// RefreshContainer re-sends the 0x6E window for every open cid showing c.
func (g *GameProtocol) RefreshContainer(c *game.Item) {
	if c == nil {
		return
	}
	for cid, open := range g.rangeContainers() {
		if open == c {
			g.sendContainer(cid, c, c.Parent != nil)
		}
	}
}

// CloseClientContainer unregisters a container window and tells the client to
// close it (0x6F).
func (g *GameProtocol) CloseClientContainer(cid uint8) {
	if g.player != nil {
		g.player.CloseContainer(cid)
	}
	w := netmsg.NewWriter()
	w.AddByte(opContainerClose)
	w.AddByte(cid)
	g.SendToClient(w)
}

// SendCloseShop tells the client to close the shop window (0x7C). Sent when the
// server closes the shop (e.g. NPC walks out of range).
func (g *GameProtocol) SendCloseShop() {
	w := netmsg.NewWriter()
	w.AddByte(inCloseShop) // 0x7C
	g.SendToClient(w)
}

// SendChangeSpeed sends the creature's new speed to the client (0x8F).
func (g *GameProtocol) SendChangeSpeed(c game.Creature) {
	w := netmsg.NewWriter()
	w.AddByte(0x8F)
	w.AddU32(c.GetID())
	w.AddU16(c.GetBaseSpeed())
	w.AddU16(c.GetSpeed())
	g.SendToClient(w)
}

// SendIcons sends the player's active condition icons (0xA2).
func (g *GameProtocol) SendIcons() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xA2)
	w.AddU64(g.player.GetIcons())
	w.AddByte(0) // IconBakragore::None
	g.SendToClient(w)
}

// SendInventoryIds sends the aggregated inventory id/tier/count list (0xF5),
// mirroring ProtocolGame::sendInventoryIds. The client's crafting/loot UIs read
// this. Amounts >= 0x40000000 are skipped (unencodable by writeCount).
func (g *GameProtocol) SendInventoryIds() {
	if g.player == nil {
		return
	}
	type key struct {
		id   uint16
		tier uint8
	}
	counts := map[key]uint32{}
	var order []key
	var walk func(items []*game.Item)
	walk = func(items []*game.Item) {
		for _, it := range items {
			if it == nil {
				continue
			}
			// C++ builds this from getAllItemTypeCount, which is keyed by real item
			// types, so an id of 0 cannot appear. Here an empty or half-initialised
			// slot can carry one, and the client reads the list as appearances: two
			// zero ids in it is exactly the "field has more than one zero id
			// appearance" it throws on, right after entering the world.
			if it.ID == 0 {
				if g.deps != nil && g.deps.Log != nil {
					g.deps.Log.Warn("inventory holds an item with id 0; skipping it in the 0xF5 list",
						"player", g.player.Name, "count", it.Count)
				}
				continue
			}
			var tier uint8
			if it.Attr != nil && it.Attr.Tier != nil {
				tier = *it.Attr.Tier
			}
			k := key{it.ID, tier}
			amt := uint32(it.Count)
			if amt == 0 {
				amt = 1
			}
			if _, seen := counts[k]; !seen {
				order = append(order, k)
			}
			counts[k] += amt
			if len(it.Contents) > 0 {
				walk(it.Contents)
			}
		}
	}
	walk(g.player.Inventory[:])

	body := netmsg.NewWriter()
	var total uint16
	for _, k := range order {
		amt := counts[k]
		if amt >= 0x40000000 {
			continue
		}
		body.AddU16(k.id)
		body.AddByte(k.tier)
		writeCount(body, amt)
		total++
	}

	w := netmsg.NewWriter()
	w.AddByte(0xF5)
	w.AddU16(total)
	w.AddBytes(body.Bytes())
	g.SendToClient(w)
}

// writeCount encodes a variable-width count, mirroring NetworkMessage::writeCount
// (1 byte < 0x40, 2 bytes < 0x4000, 4 bytes < 0x40000000).
func writeCount(w *netmsg.Writer, count uint32) {
	switch {
	case count < 0x40:
		w.AddByte(byte(count))
	case count < 0x4000:
		w.AddByte(byte(count>>8) | 0x40)
		w.AddByte(byte(count & 0xFF))
	default:
		w.AddByte(byte(count>>24) | 0x80)
		w.AddByte(byte(count >> 16))
		w.AddByte(byte(count >> 8))
		w.AddByte(byte(count))
	}
}
