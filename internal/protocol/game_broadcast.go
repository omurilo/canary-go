package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// talkTypePrivatePN is the player→NPC speech class (TALKTYPE_PRIVATE_PN). The
// server delivers it only to NPC spectators, never to player clients — echoing
// it to a client crashes it (unexpected 0xAA form).
const talkTypePrivatePN = 12

// BroadcastCreatureSay sends a creature speech to spectators. Creature/NPC
// speech goes through the C++ sendCreatureSay path, which ALWAYS carries a
// position (including NPC PRIVATE_NP replies), so we always append it.
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
			msg.AddPosition(netmsg.Position{X: c.GetPosition().X, Y: c.GetPosition().Y, Z: c.GetPosition().Z})
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
