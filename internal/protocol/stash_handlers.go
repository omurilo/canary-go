package protocol

import (
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseStashAction handles opcode 0x28 (stash stow/withdraw actions).
// Mirrors C++ ProtocolGame::parseStashWithdraw.
func (g *GameProtocol) parseStashAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	action := r.GetByte()
	switch action {
	case 0: // STASH_ACTION_STOW_ITEM — stow a single item from a position
		_ = r.GetByte() // pos (inventory slot)
		_ = r.GetU16()  // clientID
		_ = r.GetByte() // count
		// StowItem not yet implemented
	case 1: // STASH_ACTION_STOW_CONTAINER — stow all items of type from container
		_ = r.GetU16() // itemId
		// StowContainer not yet implemented
	case 2: // STASH_ACTION_STOW_STACK — stow all matching items from inventory+depot
		_ = r.GetU16() // itemId
		// StowStack not yet implemented
	case 3: // STASH_ACTION_WITHDRAW — withdraw from stash
		itemID := r.GetU16()
		count := r.GetU32()
		g.withdrawFromStash(itemID, count)
	default:
		g.deps.Log.Info("stash: unknown action", "action", action)
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
