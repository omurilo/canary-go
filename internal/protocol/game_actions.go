package protocol

import (
	"fmt"
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/vocations"
	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/moveevents"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/talkactions"
)

// walk handles a directional movement request.
// walk moves the player one tile in dir. It returns false (and cancels the walk
// on the client) when the destination is blocked.
func (g *GameProtocol) walk(dir game.Direction) bool {
	p := g.player
	oldPos := p.Pos

	oldStack := g.StackPosOf(oldPos, p.ID)

	// Floor changes are resolved BEFORE the flat walkability check — the tile
	// you step onto often has no ground on the current floor (a hole) or is an
	// up-ramp that redirects you elsewhere. Two mechanisms, mirroring C++:
	//   1. height-based stairs/ramps (Game::internalMoveCreature), and
	//   2. explicit floor-change tiles with directional flags (Tile::queryDestination).
	if dest, ok := g.resolveStairMove(dir); ok {
		g.floorChangeMove(dest, dir)
		return true
	}
	if dest, ok := g.resolveFloorChange(p.Pos.Offset(dir)); ok {
		g.floorChangeMove(dest, dir)
		return true
	}

	newPos, ok := g.deps.World.TryMove(p, dir)
	if !ok {
		// Cancel walk: restore client to current direction.
		w := netmsg.NewWriter()
		w.AddByte(0xB5) // walk cancel
		w.AddByte(byte(p.Direction))
		g.SendToClient(w)

		g.player.SendTextMessage(0x14, "Sorry, not possible.") // MESSAGE_STATUS_SMALL
		return false
	}

	// Teleport item check: if any item on the destination tile has a teleport
	// destination attribute (attrTeleDest), teleport the player there. Many
	// map teleports store a (0,0,0) dest because their real behavior is driven
	// by a Lua movement/action script — teleporting to that zero position would
	// drop the player into the void ("limbo"). Only honor a static dest that
	// points at a real, loaded tile.
	if tile := g.deps.World.Map.GetTile(newPos); tile != nil {
		for _, it := range tile.Items {
			if it.Attr != nil && it.Attr.TeleDest != nil {
				dest := *it.Attr.TeleDest
				if (dest.X == 0 && dest.Y == 0) || g.deps.World.Map.GetTile(dest) == nil {
					continue // scripted / invalid teleport — not a static jump
				}
				g.broadcastRemove(p)
				g.deps.World.SetPosition(p, dest)

				w := netmsg.NewWriter()
				w.AddByte(opFullMap)
				w.AddPosition(netmsg.Position{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z})
				g.addMapDescription(w, int(p.Pos.X)-viewportX, int(p.Pos.Y)-viewportY, p.Pos.Z, mapWidth, mapHeight)
				g.SendToClient(w)

				g.sendMagicEffect(dest, 11) // CONST_ME_TELEPORT
				g.broadcastAppear(p)
				return true
			}
		}
	}

	// Self: shift the visible map in the walk direction.
	g.SendCreatureMove(oldPos, oldStack, newPos)
	g.sendMapShift(dir, newPos)

	// Trigger StepIn events
	if tile := g.deps.World.Map.GetTile(newPos); tile != nil {
		var stepInEvents []*moveevents.MoveEvent
		var stepInItems []*game.Item

		if tile.Ground != nil {
			if evt := findStepIn(tile.Ground); evt != nil {
				stepInEvents = append(stepInEvents, evt)
				stepInItems = append(stepInItems, tile.Ground)
			}
		}
		for _, it := range tile.Items {
			if evt := findStepIn(it); evt != nil {
				stepInEvents = append(stepInEvents, evt)
				stepInItems = append(stepInItems, it)
			}
		}

		for i, evt := range stepInEvents {
			it := stepInItems[i]
			g.deps.Lua.CallStepIn(evt, p, it, newPos, oldPos)
			// If lua script changed player position (e.g. teleportTo), update client
			if p.Pos != newPos {
				teleportedTo := p.Pos
				p.Pos = newPos
				g.broadcastRemove(p)
				p.Pos = teleportedTo

				w := netmsg.NewWriter()
				w.AddByte(opFullMap)
				w.AddPosition(netmsg.Position{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z})
				g.addMapDescription(w, int(p.Pos.X)-viewportX, int(p.Pos.Y)-viewportY, p.Pos.Z, mapWidth, mapHeight)
				g.SendToClient(w)
				g.broadcastAppear(p)
				break
			}
		}
	}

	g.CheckMapContainersDistance()
	return true
}

// findStepIn resolves a StepIn move-event for a tile item, checking its unique
// id and action id (from the OTBM attributes) before the item id. Map-placed
// movements like the citizen/temple "set town" tiles register only by unique
// id, so an item-id-only lookup would never fire them.
func findStepIn(it *game.Item) *moveevents.MoveEvent {
	if it == nil {
		return nil
	}
	if it.Attr != nil {
		if it.Attr.UniqueID != nil {
			if evt := moveevents.FindStepInByUniqueID(*it.Attr.UniqueID); evt != nil {
				return evt
			}
		}
		if it.Attr.ActionID != nil {
			if evt := moveevents.FindStepInByActionID(*it.Attr.ActionID); evt != nil {
				return evt
			}
		}
	}
	return moveevents.FindStepInByItemID(it.ID)
}

// floorChangeMove relocates the player to a different floor (stairs/ramps/holes)
// and re-centres their client, broadcasting the vanish/appear to spectators.
func (g *GameProtocol) floorChangeMove(dest game.Position, dir game.Direction) {
	p := g.player
	p.Direction = dir
	g.broadcastRemove(p)
	g.deps.World.SetPosition(p, dest)
	g.sendFullMapAt(dest)
	g.broadcastAppear(p)
	g.CheckMapContainersDistance()
}

// fcFlags is the set of directional floor-change flags aggregated over a tile's
// items (from ItemType.FloorChange).
type fcFlags struct {
	down, north, south, east, west, southAlt, eastAlt bool
	any                                               bool
}

// tileFC aggregates the floor-change flags of every item on a tile.
func tileFC(cat *items.Catalog, t *game.Tile) fcFlags {
	var f fcFlags
	if t == nil {
		return f
	}
	add := func(id uint16) {
		it := cat.Get(id)
		if it == nil || it.FloorChange == "" {
			return
		}
		f.any = true
		switch it.FloorChange {
		case "down":
			f.down = true
		case "north":
			f.north = true
		case "south":
			f.south = true
		case "east":
			f.east = true
		case "west":
			f.west = true
		case "southalt":
			f.southAlt = true
		case "eastalt":
			f.eastAlt = true
		}
	}
	if t.Ground != nil {
		add(t.Ground.ID)
	}
	for _, it := range t.Items {
		add(it.ID)
	}
	return f
}

// resolveFloorChange mirrors Tile::queryDestination for a tile stepped onto:
// a "down" floor-change descends (z+1) with a directional offset read from the
// tile below and its neighbours; any up floor-change ascends (z-1) with its own
// offset. Returns the redirected destination and true when a floor change applies.
func (g *GameProtocol) resolveFloorChange(newPos game.Position) (game.Position, bool) {
	return floorChangeDestination(g.deps.World.Map, g.deps.Items, newPos)
}

// floorChangeDestination is the pure floor-change resolver (extracted for tests).
func floorChangeDestination(m *game.Map, cat *items.Catalog, newPos game.Position) (game.Position, bool) {
	fc := tileFC(cat, m.GetTile(newPos))
	if !fc.any {
		return game.Position{}, false
	}

	if fc.down {
		dx, dy := newPos.X, newPos.Y
		dz := newPos.Z + 1
		if tileFC(cat, m.GetTile(game.Position{X: dx, Y: dy - 1, Z: dz})).southAlt {
			dy -= 2
		} else if tileFC(cat, m.GetTile(game.Position{X: dx - 1, Y: dy, Z: dz})).eastAlt {
			dx -= 2
		} else {
			d := tileFC(cat, m.GetTile(game.Position{X: dx, Y: dy, Z: dz}))
			if d.north {
				dy++
			}
			if d.south {
				dy--
			}
			if d.southAlt {
				dy -= 2
			}
			if d.east {
				dx--
			}
			if d.eastAlt {
				dx -= 2
			}
			if d.west {
				dx++
			}
		}
		return game.Position{X: dx, Y: dy, Z: dz}, true
	}

	// Up floor-change (north/south/east/west/alt on the stepped-onto tile).
	dx, dy := newPos.X, newPos.Y
	dz := newPos.Z - 1
	if fc.north {
		dy--
	}
	if fc.south {
		dy++
	}
	if fc.east {
		dx++
	}
	if fc.west {
		dx--
	}
	if fc.southAlt {
		dy += 2
	}
	if fc.eastAlt {
		dx += 2
	}
	return game.Position{X: dx, Y: dy, Z: dz}, true
}

// resolveStairMove implements Tibia's height-based stair up/down for a walk
// step (Game::internalMoveCreature): climbing off a 3-height tile onto a lower
// floor with ground, or descending onto a tile whose floor-below is a 3-height
// step. Returns the z-adjusted destination and true when a floor change applies.
// Only cardinal moves trigger stairs (diagonal moves never change floor).
func (g *GameProtocol) resolveStairMove(dir game.Direction) (game.Position, bool) {
	return stairDestination(g.deps.World.Map, g.deps.Items, g.player.Pos, dir)
}

// stairDestination is the pure floor-change resolver used by resolveStairMove
// (extracted for testing).
func stairDestination(m *game.Map, cat *items.Catalog, cur game.Position, dir game.Direction) (game.Position, bool) {
	if dir >= game.DirSW { // diagonal
		return game.Position{}, false
	}
	dest := cur.Offset(dir) // horizontal step, same floor

	// Try to go UP: standing on a step (height>=3), the tile above us is
	// open, and the destination one floor up has walkable ground.
	if cur.Z != 8 {
		if curTile := m.GetTile(cur); curTile != nil && curTile.HeightCount(cat) >= 3 {
			above := m.GetTile(game.Position{X: cur.X, Y: cur.Y, Z: cur.Z - 1})
			if above == nil || (above.Ground == nil && !above.BlocksSolid(cat)) {
				up := game.Position{X: dest.X, Y: dest.Y, Z: dest.Z - 1}
				if t := m.GetTile(up); t != nil && t.Ground != nil && !t.BlocksSolid(cat) {
					return up, true
				}
			}
		}
	}

	// Try to go DOWN: the destination on this floor is open (no ground), and the
	// tile one floor below it is a step (height>=3).
	if cur.Z != 7 {
		destTile := m.GetTile(dest)
		if destTile == nil || (destTile.Ground == nil && !destTile.BlocksSolid(cat)) {
			down := game.Position{X: dest.X, Y: dest.Y, Z: dest.Z + 1}
			if t := m.GetTile(down); t != nil && t.HeightCount(cat) >= 3 {
				return down, true
			}
		}
	}
	return game.Position{}, false
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
	speed := g.player.GetSpeed()
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
	
	w := netmsg.NewWriter()
	switch dir {
	case game.DirNorth:
		w.AddByte(opMapNorth)
		g.addMapDescription(w, int(pos.X)-viewportX, int(pos.Y)-viewportY, pos.Z, mapWidth, 1)
	case game.DirSouth:
		w.AddByte(opMapSouth)
		g.addMapDescription(w, int(pos.X)-viewportX, int(pos.Y)+viewportY+1, pos.Z, mapWidth, 1)
	case game.DirEast:
		w.AddByte(opMapEast)
		g.addMapDescription(w, int(pos.X)+viewportX+1, int(pos.Y)-viewportY, pos.Z, 1, mapHeight)
	case game.DirWest:
		w.AddByte(opMapWest)
		g.addMapDescription(w, int(pos.X)-viewportX, int(pos.Y)-viewportY, pos.Z, 1, mapHeight)
	default:
		// Diagonal: send a full map redraw.
		w.AddByte(opFullMap)
		w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
		g.addMapDescription(w, int(pos.X)-viewportX, int(pos.Y)-viewportY, pos.Z, mapWidth, mapHeight)
	}
	g.SendToClient(w)
}

// turn updates the player's facing and notifies spectators.
func (g *GameProtocol) turn(dir game.Direction) {
	p := g.player
	p.Direction = dir
	
	stack := g.StackPosOf(p.Pos, p.ID)

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

// SpeakClasses enum values matching Tibia/Canary definitions
const (
	talkTypePrivateTo    = 5
	talkTypeChannelY     = 7
	talkTypeChannelR1    = 14
	talkTypePrivateRedTo = 16
)

// handleSay parses a chat message and broadcasts it.
func (g *GameProtocol) handleSay(r *netmsg.Reader) {
	talkType := r.GetByte()
	switch talkType {
	case talkTypePrivateTo, talkTypePrivateRedTo:
		_ = r.GetString() // receiver name
	case talkTypeChannelY, talkTypeChannelR1:
		_ = r.GetU16()    // channel ID
	}
	text := r.GetString()
	if text == "" {
		return
	}
	g.deps.Log.Debug("handleSay parsed chat message", "player", g.player.Name, "talkType", talkType, "text", text)
	if g.tryTalkAction(talkType, text) {
		return
	}
	if g.handleCommand(text) {
		return // GM command — handled, not broadcast as chat
	}
	// Instant spells are tried before normal chat (Game::playerSay ->
	// playerSaySpell, src/game/game.cpp:7402). A match casts the spell (and
	// broadcasts the words itself on success); the raw words are not chatted.
	if g.tryCastSpell(talkType, text) {
		return
	}
	g.broadcastSay(g.player, talkType, text)
	g.deps.Lua.Call("onPlayerSay", g.player.Name, text)
}

func requiredGroupLevel(groupType string) uint8 {
	switch strings.ToLower(groupType) {
	case "tutor":
		return 2
	case "seniortutor", "senior tutor":
		return 3
	case "gamemaster", "gm":
		return 4
	case "communitymanager", "community manager", "cm":
		return 5
	case "god":
		return 5
	default:
		return 1
	}
}

func hasTalkActionPermission(p *game.Player, groupType string) bool {
	req := requiredGroupLevel(groupType)
	if req <= 1 {
		return true
	}
	if p.AccountType >= 5 || p.GroupID >= 5 {
		return true
	}
	playerLevel := p.AccountType
	if p.GroupID > uint16(playerLevel) {
		playerLevel = uint8(p.GroupID)
	}
	return playerLevel >= req
}

func (g *GameProtocol) tryTalkAction(talkType byte, text string) bool {
	ta, param := talkactions.FindByWords(text)
	if ta == nil {
		return false
	}
	
	if !hasTalkActionPermission(g.player, ta.GroupType) {
		g.sendStatusText("You cannot execute this command.")
		return true
	}

	// Call lua callback
	_ = g.deps.Lua.CallTalkAction(ta, g.player, talkType, ta.Words, param)
	return true
}

func (g *GameProtocol) broadcastSay(speaker *game.Player, talkType byte, text string) {
	// Player→NPC speech (PRIVATE_PN) is routed ONLY to NPC spectators, never
	// echoed to player clients (C++ Game::playerSay uses npcsSpectators). Echoing
	// it — with or without a position — crashes the client, so deliver it to NPCs
	// and stop.
	if talkType == talkTypePrivatePN {
		for _, n := range g.deps.World.SpectatingNpcs(speaker.Pos) {
			g.deps.Lua.CallNpcOnCreatureSay(n, speaker, talkType, text)
		}
		return
	}

	g.statementID++
	send := func(gp *GameProtocol) {
		w := netmsg.NewWriter()
		w.AddByte(opCreatureSay)
		w.AddU32(g.statementID)
		w.AddString(speaker.Name)
		w.AddByte(0) // Show (Traded)
		w.AddU16(speaker.Level)
		w.AddByte(talkType)
		// Player speech (say/whisper/yell) travels the sendCreatureSay path,
		// which always carries a position.
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
	for _, n := range g.deps.World.SpectatingNpcs(speaker.Pos) {
		g.deps.Lua.CallNpcOnCreatureSay(n, speaker, talkType, text)
	}
}

// broadcastAppear tells nearby players a creature entered their view.
func (g *GameProtocol) broadcastAppear(p game.Creature) {
	for _, s := range g.deps.World.Spectators(p.GetPosition(), p.GetID()) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.SendAppendCreature(p, p.GetPosition())
		}
	}
}

// broadcastRemove tells nearby players a creature left.
func (g *GameProtocol) broadcastRemove(p game.Creature) {
	
	stack := g.StackPosOf(p.GetPosition(), p.GetID())
	for _, s := range g.deps.World.Spectators(p.GetPosition(), p.GetID()) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.SendRemoveCreatureAt(p.GetPosition(), stack)
		}
	}
}

// SendCreatureMove sends a move packet.
func (g *GameProtocol) SendCreatureMove(oldPos game.Position, oldStack uint8, newPos game.Position) {
	w := netmsg.NewWriter()
	w.AddByte(opCreatureMove)
	w.AddPosition(netmsg.Position{X: oldPos.X, Y: oldPos.Y, Z: oldPos.Z})
	w.AddByte(oldStack)
	w.AddPosition(netmsg.Position{X: newPos.X, Y: newPos.Y, Z: newPos.Z})
	g.SendToClient(w)
}

// SendAppendCreature adds a creature onto a tile in this client's view.
func (g *GameProtocol) SendAppendCreature(p game.Creature, pos game.Position) {
	
	stack := g.StackPosOf(pos, p.GetID())
	w := netmsg.NewWriter()
	w.AddByte(0x6A) // TileAddThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.addCreature(w, p)
	g.SendToClient(w)
}

// SendRemoveCreatureAt removes a thing at a tile stack position.
func (g *GameProtocol) SendRemoveCreatureAt(pos game.Position, stack uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0x6C) // TileRemoveThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.SendToClient(w)
}

func (g *GameProtocol) parseLookAt(r *netmsg.Reader) {
	pos := r.GetPosition()
	spriteID := r.GetU16()
	stackPos := r.GetByte() // stackPos

	var item *game.Item
	var targetCreature game.Creature
	if pos.X != 0xFFFF {
		// Map position
		tile := g.deps.World.Map.GetTile(game.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
		if tile != nil {
			if spriteID == creatureTurnMark {
				// Stack position for creatures
				// The client stackpos counts from top to bottom.
				// For now, just grab the first creature if any.
				if len(tile.Creatures) > 0 {
					targetCreature = tile.Creatures[0]
				}
			} else {
				item = g.findTileItemByStackPos(tile, spriteID, stackPos)
			}
		}
	} else {
		if pos.Y >= 0x40 {
			// Container
			cid := uint8(pos.Y - 0x40)
			if cont, ok := g.openContainerByCID(cid); ok {
				fromSlot := int(pos.Z)
				if fromSlot < len(cont.Contents) {
					item = cont.Contents[fromSlot]
				}
			}
		} else {
			// Inventory
			slot := uint8(pos.Y)
			if slot > 0 && slot <= 10 {
				item = g.player.Inventory[slot]
			}
		}
	}

	if item == nil && targetCreature == nil {
		return
	}

	if item != nil && item.ID != spriteID {
		return
	}

	if targetCreature != nil {
		if g.deps.Events != nil {
			if !g.deps.Events.ExecuteOnLook(g.player, targetCreature, game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}, 0) {
				return
			}
		}
		
		desc := fmt.Sprintf("You see %s.", targetCreature.GetName())
		if targetPlayer, ok := targetCreature.(*game.Player); ok {
			desc = BuildPlayerDescription(g.player, targetPlayer)
		}

		w := netmsg.NewWriter()
		w.AddByte(opTextMessage)
		w.AddByte(22) // MESSAGE_LOOK (green description center screen + console)
		w.AddString(desc)
		g.SendToClient(w)
		return
	}

	if item != nil {
		if g.deps.Events != nil {
			if !g.deps.Events.ExecuteOnLook(g.player, item, game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}, 0) {
				return
			}
		}
		
		desc := BuildItemDescription(g.player, item, g.deps.Items)

		w := netmsg.NewWriter()
		w.AddByte(opTextMessage)
		w.AddByte(22) // MESSAGE_LOOK (green description center screen + console)
		w.AddString(desc)
		g.SendToClient(w)
		return
	}
}

// BuildPlayerDescription formats a player description with level, vocation, guild, and party details, mirroring Player::getDescription.
func BuildPlayerDescription(viewer *game.Player, target *game.Player) string {
	isSelf := viewer.ID == target.ID
	var s strings.Builder

	// Line 1: Name and Level
	if isSelf {
		s.WriteString("You see yourself.")
	} else {
		s.WriteString(fmt.Sprintf("You see %s (Level %d).", target.Name, target.Level))
	}

	// Determine pronoun and verb matching character sex
	var subject string
	if isSelf {
		subject = "You"
	} else if target.Sex == 0 {
		subject = "She"
	} else {
		subject = "He"
	}

	var verb string
	if isSelf {
		verb = "are"
	} else {
		verb = "is"
	}

	// Vocation description
	var vocStr string
	if voc := vocations.GetVocation(uint32(target.Vocation)); voc != nil && voc.ID != 0 {
		name := voc.Name
		article := "a"
		if len(name) > 0 && (name[0] == 'A' || name[0] == 'E' || name[0] == 'I' || name[0] == 'O' || name[0] == 'U' || name[0] == 'a' || name[0] == 'e' || name[0] == 'i' || name[0] == 'o' || name[0] == 'u') {
			article = "an"
		}
		vocStr = fmt.Sprintf(" %s %s %s %s.", subject, verb, article, name)
	} else {
		if isSelf {
			vocStr = " You have no vocation."
		} else {
			vocStr = fmt.Sprintf(" %s has no vocation.", subject)
		}
	}
	s.WriteString(vocStr)

	// Party status description
	if target.Party != nil {
		memberCount := target.Party.MemberCount() + 1
		invitationCount := len(target.Party.Invitees())
		
		var part1, part2 string
		if memberCount == 1 {
			part1 = "1 member"
		} else {
			part1 = fmt.Sprintf("%d members", memberCount)
		}
		if invitationCount == 1 {
			part2 = "1 pending invitation"
		} else {
			part2 = fmt.Sprintf("%d pending invitations", invitationCount)
		}

		if isSelf {
			s.WriteString(fmt.Sprintf(" Your party has %s and %s.", part1, part2))
		} else {
			s.WriteString(fmt.Sprintf(" %s %s in a party with %s and %s.", subject, verb, part1, part2))
		}
	}

	// Guild status description
	if target.GuildName != "" && target.GuildRankName != "" {
		var gPart string
		nickStr := ""
		if target.GuildNick != "" {
			nickStr = fmt.Sprintf(" (%s)", target.GuildNick)
		}
		if isSelf {
			gPart = fmt.Sprintf(" You are %s of the %s%s.", target.GuildRankName, target.GuildName, nickStr)
		} else {
			gPart = fmt.Sprintf(" %s %s %s of the %s%s.", subject, verb, target.GuildRankName, target.GuildName, nickStr)
		}
		s.WriteString(gPart)
	}

	return s.String()
}

// BuildItemDescription constructs a detailed item description including weight and remaining charges, mirroring Item::getDescription.
func BuildItemDescription(viewer *game.Player, item *game.Item, catalog *items.Catalog) string {
	spriteID := item.ID
	itemType := catalog.Get(spriteID)
	if itemType == nil {
		return fmt.Sprintf("You see an item of type %d.", spriteID)
	}

	var s strings.Builder

	// Build the item's name prefix and article
	article := itemType.Article
	if article == "" {
		article = "a"
		if len(itemType.Name) > 0 {
			c := itemType.Name[0]
			if c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u' ||
			   c == 'A' || c == 'E' || c == 'I' || c == 'O' || c == 'U' {
				article = "an"
			}
		}
	}

	name := itemType.Name
	if name == "" {
		name = fmt.Sprintf("item of type %d", spriteID)
	}

	s.WriteString(fmt.Sprintf("You see %s %s", article, name))

	// Display charges when requested
	if itemType.ShowCharges {
		charges := uint32(0)
		if item.Attr != nil && item.Attr.Charges != nil {
			charges = uint32(*item.Attr.Charges)
		} else if item.Count > 0 {
			charges = uint32(item.Count)
		} else {
			charges = itemType.Charges
		}

		if charges > 0 {
			pluralSuffix := "s"
			if charges == 1 {
				pluralSuffix = ""
			}
			s.WriteString(fmt.Sprintf(" that has %d charge%s left", charges, pluralSuffix))
		}
	}

	s.WriteString(".")

	// Include custom description attributes if populated
	if itemType.Description != "" {
		s.WriteString(" " + itemType.Description)
		if !strings.HasSuffix(itemType.Description, ".") {
			s.WriteString(".")
		}
	}

	// Weight statistics
	weight := item.GetWeight(catalog)
	if weight > 0 {
		weightFloat := float64(weight) / 100.0
		s.WriteString(fmt.Sprintf(" It weighs %.2f oz.", weightFloat))
	}

	return s.String()
}

func (g *GameProtocol) parseAttack(r *netmsg.Reader) {
	creatureID := r.GetU32()
	_ = r.GetU32() // seq

	p := g.player
	if p == nil {
		return
	}

	target := g.deps.World.CreatureByID(creatureID)
	if target == nil {
		g.sendCancelTarget()
		p.SetAttackTarget(0)
		return
	}

	if _, isNpc := target.(*game.Npc); isNpc {
		w := netmsg.NewWriter()
		w.AddByte(opTextMessage)
		w.AddByte(0x15) // MESSAGE_FAILURE (21)
		w.AddString("You may not attack this creature.")
		g.SendToClient(w)

		g.sendCancelTarget()
		p.SetAttackTarget(0)
		return
	}

	p.SetAttackTarget(creatureID)
}

func (g *GameProtocol) sendCancelTarget() {
	w := netmsg.NewWriter()
	w.AddByte(opCancelTarget)
	w.AddU32(0)
	g.SendToClient(w)
}

func (g *GameProtocol) parseBuyItem(r *netmsg.Reader) {
	// Modern (13.x) layout: itemId U16, count(subType) U8, amount U16,
	// ignoreCap U8, inBackpacks U8. Reading amount as a byte (the old bug)
	// desynced every subsequent packet.
	itemID := r.GetU16()
	subType := r.GetByte()
	amount := r.GetU16()
	_ = r.GetByte() // ignoreCapacity (capacity is not gated yet)
	_ = r.GetByte() // buyWithBackpacks (shopping bags not modelled yet)

	nType := g.shopOwnerType()
	if nType == nil {
		return
	}

	var price uint32
	var found bool
	for _, si := range nType.ShopItems {
		if si.ID == itemID && (si.SubType == 0 || si.SubType == subType) {
			price = si.BuyPrice
			found = true
			break
		}
	}
	if !found || price == 0 {
		g.player.SendTextMessage(0x13, "This item is not available.")
		return
	}

	// Cap amounts like the client (stackable ≤ 10000, non-stackable ≤ 100).
	it := g.deps.Items.Get(itemID)
	stackable := it != nil && it.Stackable
	maxAmount := uint16(100)
	if stackable {
		maxAmount = 10000
	}
	if amount == 0 {
		return
	}
	if amount > maxAmount {
		amount = maxAmount
	}

	totalCost := uint64(price) * uint64(amount)
	g.deps.Log.Debug("parseBuyItem: starting transaction", "player", g.player.Name, "itemID", itemID, "amount", amount, "price", price, "totalCost", totalCost, "playerMoney", g.player.GetMoney(), "bankBalance", g.player.BankBalance)
	
	invMoney := g.player.GetMoney()
	if invMoney+g.player.BankBalance < totalCost {
		g.player.SendTextMessage(0x13, "You do not have enough money.")
		return
	}

	// Safely deduct the funds first. Keep track of how much bank balance is utilized.
	bankDebited := uint64(0)
	if invMoney < totalCost {
		bankDebited = totalCost - invMoney
	}
	if !g.player.RemoveMoney(totalCost, true) {
		g.player.SendTextMessage(0x13, "You do not have enough money.")
		return
	}

	// Deliver the items
	placed, _ := g.player.InternalAddItem(g.deps.Items, itemID, uint32(amount), int(subType), game.ConstSlotWhereever)
	deliveredCount := deliveredUnits(placed)
	if deliveredCount == 0 {
		// Full refund
		if bankDebited > 0 {
			g.player.BankBalance += bankDebited
			g.player.AddMoney(invMoney)
		} else {
			g.player.AddMoney(totalCost)
		}
		g.player.SendTextMessage(0x13, "You do not have enough room to carry this item.")
		g.refreshAfterTrade()
		return
	}

	charge := uint64(price) * uint64(deliveredCount)
	if deliveredCount < uint32(amount) {
		// Partial refund
		refund := totalCost - charge
		if bankDebited > 0 {
			toBank := refund
			if toBank > bankDebited {
				toBank = bankDebited
			}
			g.player.BankBalance += toBank
			toInv := refund - toBank
			if toInv > 0 {
				g.player.AddMoney(toInv)
			}
		} else {
			g.player.AddMoney(refund)
		}
	}

	g.player.SendTextMessage(0x14, fmt.Sprintf("Bought %dx %s for %d gold.", deliveredCount, itemName(it, itemID), charge))
	g.refreshAfterTrade()
}

// deliveredUnits sums the stack counts of the items actually placed by a buy.
func deliveredUnits(placed []*game.Item) uint32 {
	var n uint32
	for _, it := range placed {
		if it == nil {
			continue
		}
		if it.Count == 0 {
			n++
		} else {
			n += uint32(it.Count)
		}
	}
	return n
}

func itemName(it *items.ItemType, id uint16) string {
	if it != nil && it.Name != "" {
		return it.Name
	}
	return fmt.Sprintf("item %d", id)
}

// shopOwnerType returns the NPC type the player is currently trading with, or
// nil when there is no valid shop owner in range.
func (g *GameProtocol) shopOwnerType() *creatures.NpcType {
	if g.player == nil || g.player.ShopOwnerID == 0 {
		return nil
	}
	npc, ok := g.deps.World.CreatureByID(g.player.ShopOwnerID).(*game.Npc)
	if !ok {
		// Owner vanished — close the shop client-side.
		g.player.CloseShop()
		g.SendCloseShop()
		return nil
	}
	// Auto-close when the NPC walks out of interaction range (same floor,
	// chebyshev distance > 4), mirroring getInteractableShopOwner.
	if !sameFloorWithin(g.player.Pos, npc.GetPosition(), 4) {
		g.player.CloseShop()
		g.SendCloseShop()
		return nil
	}
	return g.deps.World.TypeRegistry.Npcs[strings.ToLower(npc.Name)]
}

func sameFloorWithin(a, b game.Position, dist int) bool {
	if a.Z != b.Z {
		return false
	}
	dx := int(a.X) - int(b.X)
	dy := int(a.Y) - int(b.Y)
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx <= dist && dy <= dist
}

// refreshAfterTrade re-sends the inventory, open containers, stats, and shop
// resource/goods lists after a buy or sell, in the order the client expects
// (container/slot refresh, then stats, then 0xEE balances + 0x7B goods).
func (g *GameProtocol) refreshAfterTrade() {
	p := g.player
	p.UpdateInventoryWeight(g.deps.Items)
	for slot := game.ConstSlotFirst; slot <= game.ConstSlotLast; slot++ {
		if item := p.Inventory[slot]; item != nil {
			g.sendInventoryItem(uint8(slot), item)
		} else {
			g.sendInventoryEmpty(uint8(slot))
		}
	}
	for cid, c := range g.rangeContainers() {
		g.deps.Log.Debug("refreshAfterTrade: refreshing container", "cid", cid, "itemId", c.ID, "contentsCount", len(c.Contents))
		g.refreshContainerIfOpen(c)
	}
	g.sendStats()
	g.sendShopGoods()
}

// refreshContainerIfOpen re-sends a container's window to the client when the
// player currently has it open, so item add/remove is reflected live.
func (g *GameProtocol) refreshContainerIfOpen(container *game.Item) {
	for cid, open := range g.rangeContainers() {
		if open == container {
			g.sendContainer(cid, container, container.Parent != nil)
			return
		}
	}
}

func (g *GameProtocol) parseSellItem(r *netmsg.Reader) {
	// Modern layout: itemId U16, count(subType) U8, amount U16, ignoreEquipped U8.
	itemID := r.GetU16()
	subType := r.GetByte()
	amount := r.GetU16()
	_ = r.GetByte() // ignoreEquipped

	nType := g.shopOwnerType()
	if nType == nil {
		return
	}

	var price uint32
	var found bool
	for _, si := range nType.ShopItems {
		if si.ID == itemID && (si.SubType == 0 || si.SubType == subType) {
			price = si.SellPrice
			found = true
			break
		}
	}
	if !found || price == 0 {
		g.player.SendTextMessage(0x13, "This NPC does not buy this item.")
		return
	}
	if amount == 0 {
		return
	}

	// Scan the whole inventory tree (skipping tiered items) and remove up to
	// `amount`, mirroring Npc::removeItemsFromInventory.
	sub := -1
	if subType != 0 {
		sub = int(subType)
	}
	sold := g.player.RemoveForSale(g.deps.Items, itemID, uint32(amount), sub)
	if sold == 0 {
		g.player.SendTextMessage(0x13, "You do not have so many of this item.")
		return
	}

	totalGain := uint64(price) * uint64(sold)
	// Proceeds go to inventory coins (visible to the player). AUTOBANK routing
	// to the bank is a config-driven follow-up.
	g.player.AddMoney(totalGain)

	it := g.deps.Items.Get(itemID)
	g.player.SendTextMessage(0x14, fmt.Sprintf("Sold %dx %s for %d gold.", sold, itemName(it, itemID), totalGain))
	g.refreshAfterTrade()
}

func (g *GameProtocol) parseCloseShop(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	npcID := g.player.ShopOwnerID
	g.player.CloseShop()
	// Fire the NPC's onCloseChannel so dialogue state resets, mirroring
	// Npc::onPlayerCloseChannel.
	if npcID != 0 {
		if npc, ok := g.deps.World.CreatureByID(npcID).(*game.Npc); ok {
			g.deps.Lua.CallNpcCloseChannel(npc, g.player)
		}
	}
}

func (g *GameProtocol) parseFightModes(r *netmsg.Reader) {
	fightMode := r.GetByte()  // 1 = offensive, 2 = balanced, 3 = defensive
	chaseMode := r.GetByte()  // 0 = stand, 1 = chase
	secureMode := r.GetByte() // 0 = secure, 1 = insecure

	p := g.player
	if p == nil {
		return
	}

	p.FightMode = fightMode
	p.ChaseMode = chaseMode != 0
	p.SecureMode = secureMode != 0

	g.sendStats()
}

