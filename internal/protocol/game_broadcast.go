package protocol

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
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

// CaptureStackPositions snapshots, for every player spectating pos, the client
// stack index c currently occupies in that player's view. It is wired into
// World.CaptureStackPositions and runs INSIDE the world lock, before the creature
// leaves the tile — the port of the oldStackPosVector loop in Map::moveCreature
// (src/map/map.cpp:739-747). A spectator who cannot see c is recorded as -1 and
// receives no packet, matching the `if (stackpos != -1)` guard at map.cpp:783.
func CaptureStackPositions(w *game.World, pos game.Position, c game.Creature) map[uint32]int {
	if c == nil {
		return nil
	}
	out := make(map[uint32]int)
	for _, s := range w.SpectatorsLocked(pos, c.GetID()) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok {
			continue
		}
		if !gp.canSeeCreature(c) {
			out[s.ID] = -1
			continue
		}
		out[s.ID] = gp.clientIndexOfCreatureLocked(pos, c.GetID())
	}
	return out
}

// moveAction is what a single spectator must receive for one creature move.
type moveAction int

const (
	// moveActionNone sends nothing at all. C++ reaches it via the
	// `if (stackpos != -1)` guard at src/map/map.cpp:783.
	moveActionNone      moveAction = iota
	moveActionShift                // 0x6D, the cheap same-floor step
	moveActionRemoveAdd            // 0x6C + 0x6A, when a shift cannot express the move
	moveActionRemove               // 0x6C only, the creature left this client's view
	moveActionAdd                  // 0x6A only, it walked into this client's view
)

// creatureMoveAction is the branch table of ProtocolGame::sendMoveCreature
// (protocolgame.cpp:8700), restricted to the non-self case — a moving player's own
// client is re-centred separately, above. captured reports whether the spectator
// had a stack position snapshotted at all; oldStack < 0 means they could not see
// the creature. Either way they get nothing: a remove at a guessed stackpos
// deletes whatever else the client has on that tile.
func creatureMoveAction(oldStack int, captured, seesOld, seesNew, teleport, known bool) moveAction {
	if !captured || oldStack < 0 {
		return moveActionNone
	}
	// The four canSee branches of sendMoveCreature, in its order.
	if !seesOld {
		if seesNew {
			return moveActionAdd
		}
		return moveActionNone
	}
	if !seesNew {
		return moveActionRemove
	}
	// A teleport, a floor change, or a stackpos past the client's 10-thing window
	// cannot be followed by a shift, so C++ degrades to remove + add. `known` is
	// the Go stand-in for sendAddCreature's known-creature handshake: a 0x6D naming
	// a creature the client has never seen has nothing to move.
	if teleport || oldStack >= 10 || !known {
		return moveActionRemoveAdd
	}
	return moveActionShift
}

// BroadcastCreatureMove tells spectators about a creature's movement. oldStackPos
// carries the per-spectator stack index captured before the creature left the old
// tile; it cannot be recomputed here, because the creature is already gone.
func BroadcastCreatureMove(w *game.World, c game.Creature, oldPos game.Position, newPos game.Position, oldStackPos map[uint32]int) {
	// A far move (teleport: different floor or beyond an adjacent step) has no
	// client-side map shift, so the moved player's OWN client must be re-centred
	// with a full map description. Normal 1-tile walks are handled by walk()
	// and skipped here. Without this, Lua teleportTo (temple/citizen, quest
	// teleports) moved the player server-side but their screen never updated.
	if p, ok := c.(*game.Player); ok {
		if gp, ok := p.Session.(*GameProtocol); ok {
			gp.SendIcons()
		}
		if oldPos.Z != newPos.Z || chebyshev(oldPos, newPos) > 1 {
			if gp, ok := p.Session.(*GameProtocol); ok {
				gp.sendFullMapAt(newPos)
			}
		}
	}

	// Same predicate the self-recentre above uses, and the same one C++ derives in
	// Map::moveCreature (map.cpp:706): anything that is not an adjacent same-floor
	// step cannot be expressed as a 0x6D shift.
	teleport := oldPos.Z != newPos.Z || chebyshev(oldPos, newPos) > 1

	visited := map[uint32]bool{c.GetID(): true}

	for _, s := range w.Spectators(oldPos, c.GetID()) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok || visited[s.ID] {
			continue
		}
		visited[s.ID] = true

		oldStack, captured := oldStackPos[s.ID]
		// gp.canSee, not s.Pos.InRangeOf: the client's window is asymmetric, and one
		// column of over-reporting is enough to emit a 0x6D for a tile it does not hold.
		switch creatureMoveAction(oldStack, captured, gp.canSee(oldPos), gp.canSee(newPos), teleport, gp.isKnown(c.GetID())) {
		case moveActionShift:
			gp.SendCreatureMove(oldPos, uint8(oldStack), newPos)
		case moveActionRemoveAdd:
			gp.SendRemoveCreatureAt(oldPos, uint8(oldStack))
			gp.SendAppendCreature(c, newPos)
		case moveActionRemove:
			gp.SendRemoveCreatureAt(oldPos, uint8(oldStack))
		case moveActionAdd:
			gp.SendAppendCreature(c, newPos)
		}
	}
	for _, s := range w.Spectators(newPos, c.GetID()) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok || visited[s.ID] {
			continue
		}
		visited[s.ID] = true
		// Spectators() is the wider MAP_MAX_VIEW_PORT-ish net; only clients whose
		// description actually covers newPos may be told to add a creature there.
		if !gp.canSee(newPos) {
			continue
		}
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

// BroadcastCreatureOutfit tells spectators a creature's appearance changed
// (creature:setOutfit → internalCreatureChangeOutfit). Without it the hireling
// outfit change updated the server-side looktype but the client never saw it.
func BroadcastCreatureOutfit(w *game.World, c game.Creature) {
	for _, s := range w.Spectators(c.GetPosition(), c.GetID()) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.SendCreatureOutfit(c, c.GetOutfit())
		}
	}
}

// BroadcastCreatureRemove tells spectators a creature was removed. oldStackPos
// holds the per-spectator stack index captured while it was still on the tile.
func BroadcastCreatureRemove(w *game.World, c game.Creature, oldStackPos map[uint32]int) {
	for _, s := range w.Spectators(c.GetPosition(), c.GetID()) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok {
			continue
		}
		stack, captured := oldStackPos[s.ID]
		if !captured || stack < 0 {
			continue
		}
		gp.SendRemoveCreatureAt(c.GetPosition(), uint8(stack))
	}
}

// BroadcastGhostModeChange notifies spectators when a player toggles ghost mode.
func BroadcastGhostModeChange(w *game.World, p *game.Player) {
	for _, s := range w.Spectators(p.Pos, p.ID) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok || s.ID == p.ID {
			continue
		}
		if p.Ghost {
			if !gp.canSeeCreature(p) {
				// Resolved in the spectator's own view, and only if it is actually
				// there: a remove at a guessed stackpos deletes the wrong thing.
				if stack := gp.ClientIndexOfCreature(p.Pos, p.ID); stack >= 0 {
					gp.SendRemoveCreatureAt(p.Pos, uint8(stack))
					gp.setKnown(p.ID, false)
				}
			}
		} else {
			if gp.canSeeCreature(p) {
				gp.SendAppendCreature(p, p.Pos)
			}
		}
	}
}

// BroadcastChangeSpeed tells spectators a creature changed speed.
func BroadcastChangeSpeed(w *game.World, c game.Creature) {
	for _, s := range w.Spectators(c.GetPosition(), 0) {
		if s.Session != nil {
			s.Session.SendChangeSpeed(c)
		}
	}
}
