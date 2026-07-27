package protocol

import (
	"log/slog"

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
	case 0: // STASH_ACTION_STOW_ITEM — stow a single item
		_ = r.GetByte() // pos (inventory slot)
		_ = r.GetU16()  // clientID (item client id)
		_ = r.GetByte() // count
	case 1: // STASH_ACTION_STOW_CONTAINER — stow all items of type from container
		_ = r.GetU16() // itemId
	case 2: // STASH_ACTION_STOW_STACK — stow all matching items from inventory+depot
		_ = r.GetU16() // itemId
	case 3: // STASH_ACTION_WITHDRAW — withdraw from stash
		itemID := r.GetU16()
		count := r.GetU32()
		slog.Default().Info("stash: withdraw", "player", g.player.Name, "itemId", itemID, "count", count)
	default:
		slog.Default().Info("stash: unknown action", "action", action)
	}
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
	if g.player.Stash == nil {
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
	slog.Default().Info("stash: sent 0x29", "player", g.player.Name, "count", len(g.player.Stash))
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
