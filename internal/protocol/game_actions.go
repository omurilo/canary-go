package protocol

import (
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// walk handles a directional movement request.
// walk moves the player one tile in dir. It returns false (and cancels the walk
// on the client) when the destination is blocked.
func (g *GameProtocol) walk(dir game.Direction) bool {
	p := g.player
	oldPos := p.Pos
	idxOld := g.buildCreatureIndex(oldPos)
	oldStack := g.stackPosOf(oldPos, p.ID, idxOld)

	newPos, ok := g.deps.World.TryMove(p, dir)
	if !ok {
		// Cancel walk: restore client to current direction.
		w := netmsg.NewWriter()
		w.AddByte(0xB5) // walk cancel
		w.AddByte(byte(p.Direction))
		g.SendToClient(w)
		return false
	}

	// Tell everyone (including self) the creature moved.
	g.broadcastMove(p, oldPos, oldStack, newPos)

	// Self: shift the visible map in the walk direction.
	g.sendMapShift(dir, newPos)
	return true
}

// autoWalkDir maps the client's auto-walk direction codes to game directions,
// mirroring ProtocolGame::translateAutoWalkDirection.
func autoWalkDir(raw byte) (game.Direction, bool) {
	switch raw {
	case 1:
		return game.DirEast, true
	case 2:
		return game.DirNE, true
	case 3:
		return game.DirNorth, true
	case 4:
		return game.DirNW, true
	case 5:
		return game.DirWest, true
	case 6:
		return game.DirSW, true
	case 7:
		return game.DirSouth, true
	case 8:
		return game.DirSE, true
	default:
		return 0, false
	}
}

// manualWalk runs a single key-initiated step, cancelling any auto-walk path in
// flight (like the real client, a manual step interrupts click-to-move).
func (g *GameProtocol) manualWalk(dir game.Direction) {
	g.walkGen.Add(1)
	g.actionMu.Lock()
	g.walk(dir)
	g.actionMu.Unlock()
}

// manualTurn runs a turn serialized against movement.
func (g *GameProtocol) manualTurn(dir game.Direction) {
	g.actionMu.Lock()
	g.turn(dir)
	g.actionMu.Unlock()
}

// stopAutoWalk cancels the in-flight auto-walk path (0x69).
func (g *GameProtocol) stopAutoWalk() { g.walkGen.Add(1) }

// autoWalk parses a click-to-move path (0x64) and walks it step by step, paced by
// the per-tile step duration so the character walks smoothly instead of teleporting.
// The first direction byte is the first step (ProtocolGame::parseAutoWalk fills the
// path from the back and consumes from the back).
func (g *GameProtocol) autoWalk(r *netmsg.Reader) {
	n := int(r.GetByte())
	if n == 0 || r.Remaining() < n {
		return
	}
	dirs := make([]game.Direction, 0, n)
	for i := 0; i < n; i++ {
		d, ok := autoWalkDir(r.GetByte())
		if !ok {
			return
		}
		dirs = append(dirs, d)
	}
	gen := g.walkGen.Add(1) // cancel any prior path; claim this generation
	go g.walkPath(dirs, gen)
}

// walkPath walks a path one tile at a time, pausing the step duration between
// tiles. It stops early if the path is cancelled (walkGen changed), a step is
// blocked, or the connection closes.
func (g *GameProtocol) walkPath(dirs []game.Direction, gen uint64) {
	for _, d := range dirs {
		if g.walkGen.Load() != gen {
			return
		}
		g.actionMu.Lock()
		ok := g.walk(d)
		var dur time.Duration
		if ok {
			dur = g.stepDuration(d)
		}
		g.actionMu.Unlock()
		if !ok {
			return
		}
		select {
		case <-g.pingStop:
			return
		case <-time.After(dur):
		}
	}
}

// stepDuration mirrors Creature::getStepDuration: ceil(1000*groundSpeed/speed to
// the server beat), tripled on the diagonal.
func (g *GameProtocol) stepDuration(dir game.Direction) time.Duration {
	speed := g.player.Speed
	if speed == 0 {
		speed = 220
	}
	groundSpeed := uint16(150)
	if tile := g.deps.World.Map.GetTile(g.player.Pos); tile != nil && tile.Ground != nil {
		if t := g.deps.Items.Get(tile.Ground.ID); t != nil && t.GroundSpeed > 0 {
			groundSpeed = t.GroundSpeed
		}
	}
	const beat = 50
	d := 1000 * int(groundSpeed) / int(speed)
	d = ((d + beat - 1) / beat) * beat
	if dir == game.DirNE || dir == game.DirNW || dir == game.DirSE || dir == game.DirSW {
		d *= 3 // WALK_DIAGONAL_EXTRA_COST
	}
	return time.Duration(d) * time.Millisecond
}

// sendMapShift sends the newly revealed strip after the player moved.
func (g *GameProtocol) sendMapShift(dir game.Direction, pos game.Position) {
	idx := g.buildCreatureIndex(pos)
	w := netmsg.NewWriter()
	switch dir {
	case game.DirNorth:
		w.AddByte(opMapNorth)
		g.addMapDescription(w, int(pos.X)-viewportX, int(pos.Y)-viewportY, pos.Z, mapWidth, 1, idx)
	case game.DirSouth:
		w.AddByte(opMapSouth)
		g.addMapDescription(w, int(pos.X)-viewportX, int(pos.Y)+viewportY+1, pos.Z, mapWidth, 1, idx)
	case game.DirEast:
		w.AddByte(opMapEast)
		g.addMapDescription(w, int(pos.X)+viewportX+1, int(pos.Y)-viewportY, pos.Z, 1, mapHeight, idx)
	case game.DirWest:
		w.AddByte(opMapWest)
		g.addMapDescription(w, int(pos.X)-viewportX, int(pos.Y)-viewportY, pos.Z, 1, mapHeight, idx)
	default:
		// Diagonal: send a full map redraw.
		w.AddByte(opFullMap)
		w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
		g.addMapDescription(w, int(pos.X)-viewportX, int(pos.Y)-viewportY, pos.Z, mapWidth, mapHeight, idx)
	}
	g.SendToClient(w)
}

// turn updates the player's facing and notifies spectators.
func (g *GameProtocol) turn(dir game.Direction) {
	p := g.player
	p.Direction = dir
	idx := g.buildCreatureIndex(p.Pos)
	stack := g.stackPosOf(p.Pos, p.ID, idx)

	notify := func(gp *GameProtocol) {
		w := netmsg.NewWriter()
		w.AddByte(opTileTransform)
		w.AddPosition(netmsg.Position{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z})
		w.AddByte(stack)
		w.AddU16(creatureTurnMark)
		w.AddU32(p.ID)
		w.AddByte(byte(dir))
		w.AddByte(0x01) // can-walk-through flag (0x01 = blocking, as ProtocolGame::sendCreatureTurn for version >= 953)
		gp.SendToClient(w)
	}
	notify(g)
	for _, s := range g.deps.World.Spectators(p.Pos, p.ID) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			notify(gp)
		}
	}
}

// handleSay parses a chat message and broadcasts it.
func (g *GameProtocol) handleSay(r *netmsg.Reader) {
	talkType := r.GetByte()
	switch talkType {
	case 0x05, 0x06, 0x07: // channel variants carry a channel id
		_ = r.GetU16()
	case 0x04, 0x0A: // private message carries a receiver
		_ = r.GetString()
	}
	text := r.GetString()
	if text == "" {
		return
	}
	if g.handleCommand(text) {
		return // GM command — handled, not broadcast as chat
	}
	g.broadcastSay(g.player, talkType, text)
	g.deps.Lua.Call("onPlayerSay", g.player.Name, text)
}

func (g *GameProtocol) broadcastSay(speaker *game.Player, talkType byte, text string) {
	g.statementID++
	send := func(gp *GameProtocol) {
		w := netmsg.NewWriter()
		w.AddByte(opCreatureSay)
		w.AddU32(g.statementID)
		w.AddString(speaker.Name)
		w.AddU16(speaker.Level)
		w.AddByte(talkType)
		w.AddPosition(netmsg.Position{X: speaker.Pos.X, Y: speaker.Pos.Y, Z: speaker.Pos.Z})
		w.AddString(text)
		gp.SendToClient(w)
	}
	send(g)
	for _, s := range g.deps.World.Spectators(speaker.Pos, speaker.ID) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			send(gp)
		}
	}
}

// broadcastMove informs the mover and all relevant spectators.
func (g *GameProtocol) broadcastMove(p *game.Player, oldPos game.Position, oldStack uint8, newPos game.Position) {
	// Self already handled via map shift; also send explicit move so the client
	// relocates the creature marker.
	moveTo := func(gp *GameProtocol) {
		w := netmsg.NewWriter()
		w.AddByte(opCreatureMove)
		w.AddPosition(netmsg.Position{X: oldPos.X, Y: oldPos.Y, Z: oldPos.Z})
		w.AddByte(oldStack)
		w.AddPosition(netmsg.Position{X: newPos.X, Y: newPos.Y, Z: newPos.Z})
		gp.SendToClient(w)
	}
	moveTo(g)

	visited := map[uint32]bool{p.ID: true}
	for _, s := range g.deps.World.Spectators(oldPos, p.ID) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok || visited[s.ID] {
			continue
		}
		visited[s.ID] = true
		if s.Pos.InRangeOf(newPos) && gp.known[p.ID] {
			moveTo(gp)
		} else {
			gp.sendRemoveCreatureAt(oldPos, oldStack)
		}
	}
	for _, s := range g.deps.World.Spectators(newPos, p.ID) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok || visited[s.ID] {
			continue
		}
		visited[s.ID] = true
		gp.sendAppendCreature(p, newPos)
	}
}

// broadcastAppear tells nearby players a creature entered their view.
func (g *GameProtocol) broadcastAppear(p *game.Player) {
	for _, s := range g.deps.World.Spectators(p.Pos, p.ID) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.sendAppendCreature(p, p.Pos)
		}
	}
}

// broadcastRemove tells nearby players a creature left.
func (g *GameProtocol) broadcastRemove(p *game.Player) {
	idx := g.buildCreatureIndex(p.Pos)
	stack := g.stackPosOf(p.Pos, p.ID, idx)
	for _, s := range g.deps.World.Spectators(p.Pos, p.ID) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.sendRemoveCreatureAt(p.Pos, stack)
		}
	}
}

// sendAppendCreature adds a creature onto a tile in this client's view.
func (g *GameProtocol) sendAppendCreature(p *game.Player, pos game.Position) {
	idx := g.buildCreatureIndex(pos)
	stack := g.stackPosOf(pos, p.ID, idx)
	w := netmsg.NewWriter()
	w.AddByte(0x6A) // TileAddThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.addCreature(w, p)
	g.SendToClient(w)
}

// sendRemoveCreatureAt removes a thing at a tile stack position.
func (g *GameProtocol) sendRemoveCreatureAt(pos game.Position, stack uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0x6C) // TileRemoveThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.SendToClient(w)
}
