package protocol

import (
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseStashAction handles opcode 0x28 (stash stow/withdraw actions).
// Mirrors C++ ProtocolGame::parseStashWithdraw.
func (g *GameProtocol) parseStashAction(r *netmsg.Reader) {
	if g.deps.Log != nil { g.deps.Log.Info("stash: parseStashAction called") }
	if g.player == nil {
		return
	}
	action := r.GetByte()
	switch action {
	case 0: // STASH_ACTION_STOW_ITEM — stow items of this type from inventory
		pos := r.GetByte()
		clientID := r.GetU16()
		count := r.GetByte()
		g.deps.Log.Info("stash: stow item", "pos", pos, "clientID", clientID, "count", count)
		// Use the item at the position if possible; otherwise scan all inventory
		var itemID uint16
		if int(pos) < len(g.player.Inventory) && g.player.Inventory[pos] != nil {
			itemID = g.player.Inventory[pos].ID
		}
		if itemID == 0 {
			// No item at position — try to look up by position in containers,
			// or just reject. The client might send a container position.
			g.deps.Log.Info("stash: no item at pos, trying full scan")
		}
		if itemID > 0 {
			allItems := (count == 0)
			stowed := g.player.StowItem(itemID, uint32(count), allItems)
			if stowed > 0 {
				g.sendStashAndInventory()
			}
		}

	case 1: // STASH_ACTION_STOW_CONTAINER — stow all items of type from container
		itemID := r.GetU16()
		_ = g.player.StowItem(itemID, 0, true)
		g.sendStashAndInventory()

	case 2: // STASH_ACTION_STOW_STACK — stow all matching items from inventory+depot
		itemID := r.GetU16()
		_ = g.player.StowItem(itemID, 0, true)
		g.sendStashAndInventory()

	case 3: // STASH_ACTION_WITHDRAW — withdraw from stash
		itemID := r.GetU16()
		count := r.GetU32()
		g.withdrawFromStash(itemID, count)

	default:
		if g.deps.Log != nil {
			g.deps.Log.Info("stash: unknown action", "action", action)
		}
	}
}

// sendStashAndInventory refreshes the stash window, inventory and all open containers.
func (g *GameProtocol) sendStashAndInventory() {
	g.sendOpenStash()
	g.player.Session.SendInventoryIds()
	// Refresh open containers so the client sees removed items (C++ end of stashContainer)
	for cid, oc := range g.player.OpenContainersSnapshot() {
		if oc.Container != nil {
			g.sendContainer(uint8(cid), oc.Container, oc.Container.Parent != nil)
		}
	}
}

// withdrawFromStash moves items from stash to the player's inventory.
func (g *GameProtocol) withdrawFromStash(itemID uint16, count uint32) {
	p := g.player
	if !p.RemoveFromStash(itemID, count) {
		return
	}
	g.deps.Log.Info("stash: withdrawn", "player", p.Name, "itemId", itemID, "count", count)
	g.sendOpenStash()
}

// SendOpenStash is the Session entry point used by player:openStash().
func (g *GameProtocol) SendOpenStash() {
	g.sendOpenStash()
}

// sendOpenStash sends the stash contents (opcode 0x29).
// Mirrors C++ ProtocolGame::sendOpenStash.
func (g *GameProtocol) sendOpenStash() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x29)
	if g.player.Stash == nil || len(g.player.Stash) == 0 {
		w.AddU16(0)
		g.SendToClient(w)
		return
	}
	w.AddU16(uint16(len(g.player.Stash)))
	for itemID, count := range g.player.Stash {
		if count == 0 {
			continue
		}
		w.AddU16(itemID)
		w.AddU32(count)
	}
	g.SendToClient(w)
}

// sendSpecialContainersAvailable sends opcode 0x2A (stash/market availability).
// Mirrors C++ ProtocolGame::sendSpecialContainersAvailable.
func (g *GameProtocol) sendSpecialContainersAvailable() {
	w := netmsg.NewWriter()
	w.AddByte(0x2A)
	w.AddByte(1) // stashAvailable
	w.AddByte(1) // marketAvailable
	g.SendToClient(w)
}
