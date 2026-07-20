package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// sayHasPosition reports whether a 0xAA creature-say of this talk type carries a
// trailing position (local speech: say/whisper/yell/spell/monster). Mirrors the
// C++ ProtocolGame::sendCreatureSay path.
func sayHasPosition(talkType byte) bool {
	switch talkType {
	case 1, 2, 3, 9, 36, 37: // SAY, WHISPER, YELL, SPELL_USE, MONSTER_SAY, MONSTER_YELL
		return true
	}
	return false
}

// sayHasChannel reports whether a 0xAA creature-say carries a trailing channel
// id instead of a position (ProtocolGame::sendToChannel).
func sayHasChannel(talkType byte) bool {
	switch talkType {
	case 6, 7, 8, 14, 0xFF: // CHANNEL_MANAGER, CHANNEL_Y, CHANNEL_O, CHANNEL_R1, CHANNEL_R2
		return true
	}
	return false
}

// appendSayLocus writes the position/channel-id/nothing tail that follows the
// talk-type byte in a 0xAA packet, matching the per-type C++ wire format. Every
// other (private/NPC) type carries neither — sending a spurious position there
// desyncs the client's parser and crashes it.
func appendSayLocus(w *netmsg.Writer, talkType byte, pos game.Position, channelID uint16) {
	switch {
	case sayHasPosition(talkType):
		w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	case sayHasChannel(talkType):
		w.AddU16(channelID)
	}
}

// BroadcastCreatureSay sends a creature speech to spectators.
func BroadcastCreatureSay(w *game.World, c game.Creature, talkType byte, text string) {
	for _, s := range w.Spectators(c.GetPosition(), c.GetID()) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.statementID++
			msg := netmsg.NewWriter()
			msg.AddByte(opCreatureSay)
			msg.AddU32(gp.statementID)
			msg.AddString(c.GetName())
			msg.AddByte(0) // Show (Traded)
			// For players we would send their level, but for NPC we just send 0.
			msg.AddU16(0)
			msg.AddByte(talkType)
			appendSayLocus(msg, talkType, c.GetPosition(), 0)
			msg.AddString(text)
			gp.SendToClient(msg)
		}
	}
}

// BroadcastCreatureMove tells spectators about a creature's movement.
func BroadcastCreatureMove(w *game.World, c game.Creature, oldPos game.Position, newPos game.Position, oldTileIndex int) {
	visited := map[uint32]bool{c.GetID(): true}

	for _, s := range w.Spectators(oldPos, c.GetID()) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok || visited[s.ID] {
			continue
		}
		visited[s.ID] = true
		if s.Pos.InRangeOf(newPos) && gp.known[c.GetID()] {
			// Stack position in the old tile
			oldStack := gp.StackPosWithIndex(oldPos, oldTileIndex)
			gp.SendCreatureMove(oldPos, oldStack, newPos)
		} else {
			oldStack := gp.StackPosWithIndex(oldPos, oldTileIndex)
			gp.SendRemoveCreatureAt(oldPos, oldStack)
		}
	}
	for _, s := range w.Spectators(newPos, c.GetID()) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok || visited[s.ID] {
			continue
		}
		visited[s.ID] = true
		gp.SendAppendCreature(c, newPos)
	}
}

// BroadcastCreatureAppear tells spectators a creature appeared.
func BroadcastCreatureAppear(w *game.World, c game.Creature) {
	for _, s := range w.Spectators(c.GetPosition(), c.GetID()) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.SendAppendCreature(c, c.GetPosition())
		}
	}
}

// BroadcastCreatureRemove tells spectators a creature was removed.
func BroadcastCreatureRemove(w *game.World, c game.Creature) {
	for _, s := range w.Spectators(c.GetPosition(), c.GetID()) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			stack := gp.StackPosOf(c.GetPosition(), c.GetID())
			gp.SendRemoveCreatureAt(c.GetPosition(), stack)
		}
	}
}
