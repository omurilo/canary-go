package protocol

import (
	"fmt"
	"strings"
	"time"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/game/imbuements"
	"github.com/omurilo/canary-go/internal/game/vocations"
	"github.com/omurilo/canary-go/internal/items"
	"github.com/omurilo/canary-go/internal/moveevents"
	"github.com/omurilo/canary-go/internal/netmsg"
	"github.com/omurilo/canary-go/internal/talkactions"
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

	if oldPos.Z == newPos.Z {
		g.SendCreatureMove(oldPos, oldStack, newPos)
		g.sendMapShift(dir, newPos)
	}

	// Trigger StepIn events
	if tile := g.deps.World.Map.GetTile(newPos); tile != nil {
		var stepInEvents []*moveevents.MoveEvent
		var stepInItems []*game.Item
		// Dedupe by event: the same MoveEvent may be reachable via the ground's
		// and an item's action id (and by position), but it must fire at most once
		// per tile entry. teleportTo queues its move asynchronously, so p.Pos is
		// unchanged when the loop's post-teleport break is evaluated; without this
		// dedup the event would run twice (the second time after the script cleared
		// its own storage), e.g. sending the adventurers'-guild return to the home
		// town instead of the stored one.
		seen := make(map[*moveevents.MoveEvent]bool)
		add := func(evt *moveevents.MoveEvent, it *game.Item) {
			if evt == nil || seen[evt] {
				return
			}
			seen[evt] = true
			stepInEvents = append(stepInEvents, evt)
			stepInItems = append(stepInItems, it)
		}

		if tile.Ground != nil {
			add(findStepIn(tile.Ground), tile.Ground)
		}
		for _, it := range tile.Items {
			add(findStepIn(it), it)
		}
		if evt := moveevents.FindStepInByPosition(newPos); evt != nil {
			ground := tile.Ground
			if ground == nil {
				ground = &game.Item{}
			}
			add(evt, ground)
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
	// EventCallback playerOnTurn(player, direction) — (bool), so a false return
	// cancels the turn.
	if g.deps.Events != nil && g.player != nil {
		if !g.deps.Events.ExecutePlayerOnTurn(g.player, uint8(dir)) {
			return
		}
	}
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
	g.player.Direction = dir
	BroadcastCreatureTurn(g.deps.World, g.player)
}

// BroadcastCreatureTurn tells everyone who can see a creature that it changed
// facing. It is Game::internalCreatureTurn's send half, and monsters need it as
// much as players do — Monster::updateLookDirection turns a monster towards
// whatever it is attacking on every think.
//
// Per Player::sendCreatureTurn (src/creatures/players/player.cpp:8671) the stack
// position is resolved in EACH receiver's view — one value taken from the
// turning creature's own view is wrong for anyone whose tile stack differs (an
// invisible creature between them, say). Out of range, C++ falls back to a full
// tile update rather than naming a stackpos the client cannot address.
func BroadcastCreatureTurn(world *game.World, c game.Creature) {
	if world == nil || c == nil {
		return
	}
	pos, id, dir := c.GetPosition(), c.GetID(), c.GetDirection()

	notify := func(gp *GameProtocol) {
		if !gp.canSeeCreature(c) {
			return
		}
		stack := gp.ClientIndexOfCreature(pos, id)
		if stack < 0 || stack >= 10 {
			gp.sendUpdateTile(pos, world.Map.GetTile(pos))
			return
		}
		w := netmsg.NewWriter()
		w.AddByte(opTileTransform)
		w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
		w.AddByte(uint8(stack))
		w.AddU16(creatureTurnMark)
		w.AddU32(id)
		w.AddByte(byte(dir))
		w.AddByte(0x01) // can-walk-through flag (0x01 = blocking, as ProtocolGame::sendCreatureTurn for version >= 953)
		gp.SendToClient(w)
	}

	if p, ok := c.(*game.Player); ok {
		if gp, ok := p.Session.(*GameProtocol); ok {
			notify(gp)
		}
	}
	for _, s := range world.Spectators(pos, id) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			notify(gp)
		}
	}
}

// SpeakClasses enum values matching Tibia/Canary definitions
const (
	talkTypePrivateTo    = 5
	talkTypeChannelY     = 7
	talkTypeChannelO     = 8
	talkTypeChannelR1    = 14
	talkTypePrivateRedTo = 16
)

// handleSay parses a chat message and broadcasts it.
func (g *GameProtocol) handleSay(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	talkType := r.GetByte()
	receiver := ""
	channelID := uint16(0)
	switch talkType {
	case talkTypePrivateTo, talkTypePrivateRedTo:
		receiver = r.GetString() // receiver name
	case talkTypeChannelY, talkTypeChannelR1:
		channelID = r.GetU16() // channel ID
	}
	text := r.GetString()
	if len(text) > 255 {
		text = text[:255]
	}
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
	// Route channel messages through the chat manager.
	if talkType == talkTypeChannelY || talkType == talkTypeChannelR1 {
		if g.deps.World.TalkToChannel(g.player, channelID, talkType, text) {
			return
		}
	}
	// Route private messages through the world.
	if talkType == talkTypePrivateTo || talkType == talkTypePrivateRedTo {
		if receiver != "" {
			target := g.deps.World.PlayerByName(receiver)
			if target != nil {
				g.deps.World.SendPrivateMessage(g.player, target, talkType, text)
				return
			}
		}
	}
	g.broadcastSay(g.player, talkType, text, receiver)
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

func hasTalkActionPermission(p *game.Player, ta *talkactions.TalkAction) bool {
	// If Access is strictly true, require god/admin access (AccountType >= 5 or GroupID >= 4)
	if ta.Access {
		if p.AccountType < 5 && p.GroupID < 4 {
			return false
		}
	}

	// Check explicitly set AccountType requirement
	if ta.AccountType > 0 {
		if p.AccountType < ta.AccountType {
			return false
		}
	}

	// Check GroupType requirement
	if ta.GroupType != "" {
		if !hasGroupPermission(p, ta.GroupType) {
			return false
		}
	}

	return true
}

func hasGroupPermission(p *game.Player, group string) bool {
	req := requiredGroupLevel(group)
	if req <= 1 {
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

	if !hasTalkActionPermission(g.player, ta) {
		g.sendStatusText("You cannot execute this command.")
		return true
	}

	// Call lua callback
	_ = g.deps.Lua.CallTalkAction(ta, g.player, talkType, ta.Words, param)
	return true
}

func (g *GameProtocol) broadcastSay(speaker *game.Player, talkType byte, text string, receiver string) {
	// Player→NPC speech (PRIVATE_PN) is routed ONLY to NPC spectators, never
	// echoed to player clients (C++ Game::playerSay uses npcsSpectators). Echoing
	// it — with or without a position — crashes the client, so deliver it to NPCs
	// and stop.
	if talkType == talkTypePrivatePN {
		for _, n := range g.deps.World.SpectatingNpcs(speaker.Pos) {
			if receiver != "" && !strings.EqualFold(n.Name, receiver) {
				continue
			}
			if g.deps.Lua.CallNpcOnCreatureSay(n, speaker, talkType, text) {
				break
			}
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

// broadcastRemove tells nearby players a creature left. It must run while the
// creature is still on its tile — the stack position is resolved per receiver
// (Map::moveCreature builds the same per-spectator vector before the removal), so
// a single value from the leaver's own view is wrong for anyone whose stack differs.
func (g *GameProtocol) broadcastRemove(p game.Creature) {
	pos := p.GetPosition()
	for _, s := range g.deps.World.Spectators(pos, p.GetID()) {
		gp, ok := s.Session.(*GameProtocol)
		if !ok {
			continue
		}
		stack := gp.ClientIndexOfCreature(pos, p.GetID())
		if stack < 0 {
			continue // this client never had it on the tile
		}
		gp.SendRemoveCreatureAt(pos, uint8(stack))
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

	// sendAddCreature opens with `if (!canSee(pos)) return;` and then drops anything
	// at stackpos >= 10 (protocolgame.cpp). Spectator range is wider than the client
	// window, so without the first check every widened spectator is told to add a
	// creature to a tile it does not hold.
	if !g.canSee(pos) {
		return
	}
	// C++ only reaches sendAddCreature with an index the tile actually yielded;
	// an add at a made-up stackpos desyncs the client's tile stack.
	idx := g.ClientIndexOfCreature(pos, p.GetID())
	if idx < 0 || idx >= 10 {
		return
	}
	stack := uint8(idx)
	w := netmsg.NewWriter()
	w.AddByte(0x6A) // TileAddThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.addCreature(w, p)
	g.SendToClient(w)
}

// SendRemoveCreatureAt removes a thing at a tile stack position.
func (g *GameProtocol) SendRemoveCreatureAt(pos game.Position, stack uint8) {
	// sendRemoveTileThing is canSee-gated in C++ (protocolgame.cpp): a remove naming
	// a tile the client does not hold is exactly what it reports as "no thing at pos".
	if !g.canSee(pos) {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x6C) // TileRemoveThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.SendToClient(w)
}

func (g *GameProtocol) parseLookAt(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
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
			if cont, offset, ok := g.openContainerByCID(cid); ok {
				fromSlot := int(pos.Z) + offset
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
	if tier := item.GetTier(); tier > 0 {
		s.WriteString(fmt.Sprintf(" (Tier %d)", tier))
	}

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

	// Imbuements
	var imbReg *imbuements.Registry
	if viewer != nil {
		if w := viewer.GetWorld(); w != nil {
			imbReg = w.Imbuements
		}
	}
	if imbSlots, ok := itemType.Stats["imbuementslot"]; ok && imbSlots > 0 {
		s.WriteString("\nImbuements: (")
		for slot := uint8(0); slot < uint8(imbSlots); slot++ {
			if slot > 0 {
				s.WriteString(", ")
			}
			info, found := item.GetImbuementInfo(slot)
			if !found {
				s.WriteString("Empty Slot")
				continue
			}
			if imbReg == nil {
				continue
			}
			imb := imbReg.GetImbuement(info.ID)
			if imb == nil {
				continue
			}
			baseImb := imbReg.GetBaseByID(imb.BaseID)
			if baseImb == nil {
				continue
			}
			minutes := info.Duration / 60
			hours := minutes / 60
			s.WriteString(fmt.Sprintf("%s %s %02d:%02dh", baseImb.Name, imb.Name, hours, minutes%60))
		}
		s.WriteString(").")
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

	if target.GetID() == g.player.ID {
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

// uiExhaustionMS is the default of Player::isUIExhausted (player.hpp:1248): a
// shop action within 250ms of the last one is refused, which is what stops a
// held-down buy button from racing the inventory refresh.
const uiExhaustionMS = 250

func (g *GameProtocol) parseBuyItem(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	// Modern (13.x) layout: itemId U16, count(subType) U8, amount U16,
	// ignoreCap U8, inBackpacks U8. Reading amount as a byte (the old bug)
	// desynced every subsequent packet.
	itemID := r.GetU16()
	if itemID == 0 {
		return
	}
	subType := r.GetByte()
	amount := r.GetU16()
	ignoreCap := r.GetByte() != 0
	inBackpacks := r.GetByte() != 0

	if amount == 0 {
		return
	}

	npc := g.shopOwner()
	if npc == nil {
		return
	}

	it := g.deps.Items.Get(itemID)
	if it == nil {
		return
	}

	// Upstream REFUSES an over-large amount, it does not clamp it
	// (game.cpp:6234). Clamping — which this did — turns a malformed packet into
	// a purchase the player never asked for.
	stackable := it.Stackable
	if (stackable && amount > 10000) || (!stackable && amount > 100) {
		return
	}

	if !g.player.HasShopItemForSale(npc, itemID, subType) {
		return
	}

	if g.player.IsUIExhausted(uiExhaustionMS) {
		g.player.SendCancelMessage("You are exhausted.")
		return
	}

	// The container guards belong to Game::playerBuyItem, not to the NPC: a full
	// backpack tree or a tile already holding 20 items stops the purchase before
	// the merchant is ever asked.
	if inBackpacks || it.IsContainer() {
		if g.player.ContainerHoldingCountExceeded(g.deps.World.MaxContainer) {
			g.player.SendCancelMessage("This container is full.")
			return
		}
		if g.deps.World.TileItemCount(g.player.Pos) >= 20 {
			g.player.SendCancelMessage("This container is full.")
			return
		}
	}

	// Npc::onPlayerBuyItem owns the room / tile-limit / price / funds checks and
	// the script hand-off. Reimplementing them here is what left the ported
	// method unreachable and let the two copies drift: this one priced off the
	// type's shop list, so a per-player list installed by a quest NPC was ignored.
	npc.OnPlayerBuyItem(g.deps.World, g.player, itemID, subType, amount, ignoreCap, inBackpacks)
	g.player.UpdateUIExhausted()

	g.refreshAfterTrade()
}

// shopOwner is Game::getInteractableShopOwner: the NPC the player currently has
// a shop open with, or nil once it is gone or out of range.
//
// It replaced a parallel shopOwnerType that returned the NpcType instead. Two
// lookups meant two shop lists, and the type-based one could not see a
// per-player list at all.
func (g *GameProtocol) shopOwner() *game.Npc {
	if g.player == nil || g.player.ShopOwnerID == 0 {
		return nil
	}
	npc, ok := g.deps.World.CreatureByID(g.player.ShopOwnerID).(*game.Npc)
	if !ok {
		g.player.CloseShop()
		g.SendCloseShop()
		return nil
	}
	if !sameFloorWithin(g.player.Pos, npc.GetPosition(), 4) {
		g.player.CloseShop()
		g.SendCloseShop()
		return nil
	}
	return npc
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
	if g.player == nil {
		return
	}
	// Modern layout: itemId U16, count(subType) U8, amount U16, ignoreEquipped U8.
	itemID := r.GetU16()
	if itemID == 0 {
		return
	}
	subType := r.GetByte()
	amount := r.GetU16()
	ignoreEquipped := r.GetByte() != 0

	if amount == 0 {
		return
	}

	npc := g.shopOwner()
	if npc == nil {
		return
	}

	it := g.deps.Items.Get(itemID)
	if it == nil {
		return
	}
	// Refused, not clamped — same as the buy path (game.cpp:6291).
	if (it.Stackable && amount > 10000) || (!it.Stackable && amount > 100) {
		return
	}

	if g.player.IsUIExhausted(uiExhaustionMS) {
		g.player.SendCancelMessage("You are exhausted.")
		return
	}

	// Npc::onPlayerSellItem owns the whole sale: it prices against the per-player
	// shop list, removes the items, credits the proceeds through the AUTOBANK
	// path, and only then fires onSellItem as a notification. The copy that used
	// to live here priced off the type's list and always paid into the purse.
	npc.OnPlayerSellItem(g.deps.World, g.player, itemID, subType, uint32(amount), ignoreEquipped)
	g.player.UpdateUIExhausted()

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

func (g *GameProtocol) parseCloseChannel(r *netmsg.Reader) {
	if g.player == nil || g.deps == nil {
		return
	}
	channelID := r.GetU16()

	// Remove player from the channel via ChatManager.
	if g.deps.World != nil {
		g.deps.World.RemoveUserFromChannel(g.player, channelID)
	}

	// Also handle NPC interaction close (existing behaviour).
	if g.deps.World != nil {
		for _, cr := range g.deps.World.Creatures() {
			if npc, ok := cr.(*game.Npc); ok && npc.IsInteractingWithPlayer(g.player.ID) {
				if g.deps.Lua != nil {
					g.deps.Lua.CallNpcCloseChannel(npc, g.player)
				}
				npc.RemovePlayerInteraction(g.player.ID)
			}
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

func (g *GameProtocol) parseNpcGreet(r *netmsg.Reader) {
	npcId := r.GetU32()
	if npcId == 0 {
		return
	}

	p := g.player
	if p == nil {
		return
	}

	if c := g.deps.World.CreatureByID(npcId); c != nil {
		if npc, ok := c.(*game.Npc); ok {
			g.deps.Lua.CallNpcOnCreatureSay(npc, p, 1, "hi") // TALKTYPE_SAY = 1
		}
	}
}

// walkToThenRetry is queuePlayerAutoWalk plus setNextWalkActionTask, the pattern
// C++ uses whenever an action targets something out of arm's reach: walk to within
// one tile, then run the action again (src/game/game.cpp:5401-5419 for the wrap
// case; the same shape appears in playerUseItem, playerMoveThing and others).
//
// Without it every out-of-reach action was simply refused, so unwrapping a kit
// across the room answered "You are too far away" where upstream walks over and
// does it.
//
// Returns false when there is no path, which is the caller's cue to send
// RETURNVALUE_THEREISNOWAY. A true return means the retry has been scheduled, not
// that the action succeeded.
func (g *GameProtocol) walkToThenRetry(target game.Position, retry func()) bool {
	if g.player == nil {
		return false
	}
	from := g.player.Pos
	path := game.FindPath(g.deps.World.Map, g.deps.Items, from, target, 1000)
	if len(path) == 0 {
		return false
	}
	// getPathTo(..., minTargetDist 0, maxTargetDist 1): stop ADJACENT to the target,
	// never on top of it — the tile itself may well be blocked by what we are
	// reaching for.
	if path[len(path)-1] == target {
		path = path[:len(path)-1]
	}
	if len(path) == 0 {
		return false
	}

	dirs := make([]game.Direction, 0, len(path))
	prev := from
	for _, step := range path {
		dirs = append(dirs, game.StepDirection(prev, step))
		prev = step
	}

	gen := g.walkGen.Add(1)
	g.logWalkRetry("start", from, target, len(dirs))
	go func() {
		g.walkPath(dirs, gen)
		// A cancelled or blocked path must not fire the action: walkPath returns
		// early in both cases, and the generation check catches the first.
		//
		// Both of these used to drop the action with no message and no log, which
		// made an interrupted walk indistinguishable from a server that ignored the
		// request — the exact hole that cost a round of diagnosis.
		if g.walkGen.Load() != gen {
			g.logWalkRetry("cancelled", g.player.Pos, target, len(dirs))
			return
		}
		if chebyshev(g.player.Pos, target) > 1 {
			g.logWalkRetry("blocked", g.player.Pos, target, len(dirs))
			g.sendCancelMessage("There is no way.")
			return
		}
		g.logWalkRetry("arrived", g.player.Pos, target, len(dirs))
		g.actionMu.Lock()
		defer g.actionMu.Unlock()
		retry()
	}()
	return true
}

// logWalkRetry records each outcome of the walk-then-retry, at WARN so it shows
// without the opcode dump.
func (g *GameProtocol) logWalkRetry(stage string, from, target game.Position, steps int) {
	if g.deps == nil || g.deps.Log == nil {
		return
	}
	player := ""
	if g.player != nil {
		player = g.player.Name
	}
	g.deps.Log.Warn("walk-to-action", "stage", stage, "player", player,
		"from", fmt.Sprintf("%d,%d,%d", from.X, from.Y, from.Z),
		"target", fmt.Sprintf("%d,%d,%d", target.X, target.Y, target.Z),
		"steps", steps)
}
