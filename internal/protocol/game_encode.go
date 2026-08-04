package protocol

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// Client view dimensions.
const (
	viewportX     = 8
	viewportY     = 6
	mapWidth      = 18 // (viewportX+1)*2
	mapHeight     = 14 // (viewportY+1)*2
	maxLayers     = 16
	surfaceZ      = 7
	creatureNew   = 0x0061
	creatureKnown = 0x0062
)

// posKey identifies a tile for the spectator index.
type posKey struct {
	x uint16
	y uint16
	z uint8
}

// addOutfit writes an Outfit with mount.
func addOutfit(w *netmsg.Writer, o game.Outfit) {
	addOutfitNoMount(w, o)
	w.AddU16(o.LookMount)
	if o.LookMount != 0 {
		w.AddByte(o.MountHead)
		w.AddByte(o.MountBody)
		w.AddByte(o.MountLegs)
		w.AddByte(o.MountFeet)
	}
}

// addOutfitNoMount writes an Outfit without mount bytes. Used in contexts
// where the caller passes false for addMount to AddOutfit (e.g. cyclopedia
// character inspection), matching C++ ProtocolGame::AddOutfit(addMount=false).
func addOutfitNoMount(w *netmsg.Writer, o game.Outfit) {
	w.AddU16(o.LookType)
	if o.LookType != 0 {
		w.AddByte(o.Head)
		w.AddByte(o.Body)
		w.AddByte(o.Legs)
		w.AddByte(o.Feet)
		w.AddByte(o.Addons)
	} else {
		w.AddU16(o.LookTypeEx)
	}
}

// addItem writes an item using the appearance catalog to decide which extra
// bytes the client expects (stackable count, fluid subtype, container marker,
// podium, tier, decay, charges, wrap kit) — matching the C++ AddItem branches.
func (g *GameProtocol) addItem(w *netmsg.Writer, it *game.Item) {
	w.AddU16(it.ID)
	t := g.deps.Items.Get(it.ID)
	if t == nil {
		// C++ indexes Item::items[id] and keeps going with the dummy type, which is
		// only safe because an item with an unknown id cannot exist there:
		// Item::CreateItem returns null for one. Here it can, and the cost is
		// twofold — the client reads a zero appearance id, and the count/subtype
		// byte this type would have contributed is missing, so every field after it
		// in the frame shifts. That is what "field has more than one zero id
		// appearance" looks like from the client side.
		if g.deps.Log != nil {
			g.deps.Log.Warn("encoding item with unknown type: the client will read a zero appearance id and the rest of the frame will shift",
				"itemId", it.ID, "count", it.Count)
		}
		return
	}
	switch {
	case t.Stackable:
		count := it.Count
		if count == 0 {
			count = 1
		}
		if count > 255 {
			count = 255
		}
		w.AddByte(byte(count))
	case t.IsSplash() || t.IsFluidContainer():
		w.AddByte(byte(it.Count)) // fluid subtype
	}
	if t.IsContainer() {
		w.AddByte(0x00) // container special (none)
	}
	if t.Podium {
		w.AddU16(0) // lookType
		w.AddU16(0) // lookTypeEx
		w.AddU16(0) // mount
		w.AddByte(2) // direction
		w.AddByte(0x01)
	}
	if t.UpgradeClassification > 0 {
		w.AddByte(it.GetTier())
	}
	if t.Expire || t.ExpireStop || t.ClockExpire {
		w.AddU32(0) // decay time
		w.AddByte(0)
	}
	if t.WearOut {
		w.AddU32(0) // charges
		w.AddByte(0)
	}
	if t.WrapKit {
		// The unwrap id the item is carrying, not a constant zero. C++ reads it from
		// the "unWrapId" custom attribute (AddItem, protocolgame.cpp), and the client
		// uses it to decide whether the item can be unwrapped at all — with 0 it never
		// offers the option, so the request never reaches the server.
		unwrapID := uint16(0)
		if v, ok := it.GetCustomAttribute("unWrapId"); ok {
			unwrapID = customAttrUint16(v)
		}
		w.AddU16(unwrapID)
	}
}

// addCreature writes a creature description. When known, only the id marker is
// sent; otherwise the full appearance follows.
func (g *GameProtocol) addCreature(w *netmsg.Writer, c game.Creature) {
	known := g.isKnown(c.GetID())
	if known {
		w.AddU16(creatureKnown)
		w.AddU32(c.GetID())
	} else {
		g.setKnown(c.GetID(), true)
		w.AddU16(creatureNew)
		w.AddU32(0) // removedKnownId (cache not full)
		w.AddU32(c.GetID())
		w.AddByte(c.GetCreatureType()) // creatureType: 0=Player, 1=Monster, 2=NPC
		w.AddString(c.GetName())
	}

	healthPct := byte(100)
	if c.GetMaxHealth() > 0 {
		healthPct = byte(c.GetHealth() * 100 / c.GetMaxHealth())
	}
	w.AddByte(healthPct)
	w.AddByte(byte(c.GetDirection()))
	addOutfit(w, c.GetOutfit())
	w.AddByte(c.GetLightLevel())
	w.AddByte(c.GetLightColor())
	w.AddU16(c.GetSpeed())

	w.AddByte(0) // creature icons count
	w.AddByte(0) // skull
	// Party shield: computed from the viewer's relationship to the target when
	// both are players, else none.
	shield := byte(game.ShieldNone)
	if g.player != nil {
		if target, ok := c.(*game.Player); ok {
			shield = g.player.PartyShield(target)
		}
	}
	w.AddByte(shield) // party shield
	if !known {
		w.AddByte(0) // guild emblem
	}
	w.AddByte(c.GetCreatureType()) // creature type (again)
	// Vocation client id is sent ONLY for players (CREATURETYPE_PLAYER == 0).
	// Sending it for NPCs/monsters adds a spurious byte that desyncs the whole
	// tile/map stream — see ProtocolGame::AddCreature (protocolgame.cpp:9641).
	if c.GetCreatureType() == 0 {
		w.AddByte(0) // vocation client id
	}
	// Speech bubble. Creature::getSpeechBubble is SPEECHBUBBLE_NONE and only Npc
	// overrides it; hardcoding the zero meant no NPC in the game ever rendered
	// the "talk to me" marker over its head (protocolgame.cpp:9646).
	bubble := byte(0)
	if b, ok := c.(interface{ GetSpeechBubble() uint8 }); ok {
		bubble = b.GetSpeechBubble()
	}
	w.AddByte(bubble)
	w.AddByte(0xFF) // mark (unmarked)
	w.AddByte(0)    // inspection type
	walkthrough := byte(0x01)
	if g.canWalkthroughEx(g.player, c) {
		walkthrough = byte(0x00)
	}
	w.AddByte(walkthrough) // walkthrough (can walk through: 0, solid: 1)

	// OTCR extension: shader name + attached effects list. Gated exactly like the
	// tail of the C++ AddCreature (protocolgame.cpp:9659). Every game.Creature
	// embeds BaseCreature and so satisfies this interface, which meant these bytes
	// went to EVERY client — including stock Tibia ones, which do not read them and
	// then parse the following fields three bytes out of alignment.
	if !g.isOTCR() {
		return
	}
	if bc, ok := c.(interface{ GetShader() string; GetAttachedEffects() []uint16 }); ok {
		w.AddString(bc.GetShader())
		effects := bc.GetAttachedEffects()
		w.AddByte(byte(len(effects)))
		for _, id := range effects {
			w.AddU16(id)
		}
	}
}

// isTopItem reports whether an item stacks below creatures (always-on-top).
func (g *GameProtocol) isTopItem(it *game.Item) bool {
	t := g.deps.Items.Get(it.ID)
	return t != nil && t.AlwaysOnTop()
}

func (g *GameProtocol) canWalkthroughEx(observer *game.Player, target game.Creature) bool {
	if observer == nil || target == nil {
		return false
	}
	if targetP, ok := target.(*game.Player); ok {
		if targetP.Ghost {
			return true
		}
		if observer.GroupID >= 3 {
			return true
		}
	}
	return false
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Client viewport limits (src/map/map_const.hpp:12-19).
const (
	mapMaxClientViewPortX = 8
	mapMaxClientViewPortY = 6
	mapInitSurfaceLayer   = 7
	mapLayerViewLimit     = 2
)

// canSee ports ProtocolGame::canSee (protocolgame.cpp) — whether pos is inside the
// window this client actually holds tiles for. The window is NOT symmetric: the
// player sits 8 tiles from the left edge and 9 from the right, 6 from the top and
// 7 from the bottom, because the description is 18x14 with the player off-centre.
//
// game.Position.InRangeOf was standing in for this and is symmetric (±9, ±7), so
// it claimed the column at myX-9 and the row at myY-7 were visible when the client
// has neither. Moving a creature onto one of those emitted a 0x6D naming a tile the
// client does not hold, which it reports as "target field not existing" and the
// stock client dies on. InRangeOf is left alone: it approximates the wider
// MAP_MAX_VIEW_PORT spectator range, which is a different question.
func (g *GameProtocol) canSee(pos game.Position) bool {
	if g.player == nil {
		return false
	}
	my := g.player.Pos
	if my.Z <= mapInitSurfaceLayer {
		// On or above ground level the view spans 7 -> 0.
		if pos.Z > mapInitSurfaceLayer {
			return false
		}
	} else if absInt(int(my.Z)-int(pos.Z)) > mapLayerViewLimit {
		// Underground the view is +/- 2 floors from the one we stand on.
		return false
	}
	// A negative offset means the tile is on a floor below us; the client shifts the
	// window diagonally per floor, so the bounds move with it.
	offsetz := int(my.Z) - int(pos.Z)
	x, y := int(pos.X), int(pos.Y)
	mx, myY := int(my.X), int(my.Y)
	return x >= mx-mapMaxClientViewPortX+offsetz && x <= mx+(mapMaxClientViewPortX+1)+offsetz &&
		y >= myY-mapMaxClientViewPortY+offsetz && y <= myY+(mapMaxClientViewPortY+1)+offsetz
}

func (g *GameProtocol) canSeeCreature(c game.Creature) bool {
	if c == nil {
		return false
	}
	if p, ok := c.(*game.Player); ok && p.Ghost {
		if g.player == nil {
			return false
		}
		if g.player == p {
			return true
		}
		if g.player.GroupID >= 3 && g.player.GroupID >= p.GroupID {
			return true
		}
		return false
	}
	// Player::canSeeCreature (player.cpp:1395): invisibility hides monsters and
	// NPCs but never other players — a player under invisibility is still drawn,
	// which is why the getPlayer() guard is there and not an oversight.
	if _, isPlayer := c.(*game.Player); !isPlayer {
		return game.CanSeeCreature(g.player, c)
	}
	return true
}

// addTileDescription writes a tile's things in the client's stack order, mirroring
// GetTileDescription: ground, always-on-top items, creatures, then normal items.
// Placing creatures between the two item groups is what keeps creature stackpos in
// sync with the client (a mismatch makes 0x6D moves reference the wrong thing).
func (g *GameProtocol) addTileDescription(w *netmsg.Writer, t *game.Tile, pos game.Position) {
	count := 0
	if t.Ground != nil {
		g.addItem(w, t.Ground)
		count++
	}
	// Top items (clip/bottom/top): render below creatures.
	for _, it := range t.Items {
		if count >= 10 {
			return
		}
		if g.isTopItem(it) {
			g.addItem(w, it)
			count++
		}
	}

	// Creatures.
	for i := 0; i < len(t.Creatures); i++ {
		if count >= 10 {
			return
		}
		c := t.Creatures[i]
		if !g.canSeeCreature(c) {
			continue
		}
		g.addCreature(w, c)
		count++
	}
	// Down items (no always-on-top order): render above creatures.
	for _, it := range t.Items {
		if count >= 10 {
			return
		}
		if !g.isTopItem(it) {
			g.addItem(w, it)
			count++
		}
	}
}

// addFloorDescription writes a single floor with run-length skips for empties.
func (g *GameProtocol) addFloorDescription(w *netmsg.Writer, x, y, z, width, height, offset int, skip *int) {
	for nx := 0; nx < width; nx++ {
		for ny := 0; ny < height; ny++ {
			pos := game.Position{
				X: uint16(x + nx + offset),
				Y: uint16(y + ny + offset),
				Z: uint8(z),
			}
			tile := g.deps.World.Map.GetTile(pos)
			if tile != nil {
				if *skip >= 0 {
					w.AddByte(byte(*skip))
					w.AddByte(0xFF)
				}
				*skip = 0
				g.addTileDescription(w, tile, pos)
			} else if *skip == 0xFE {
				w.AddByte(0xFF)
				w.AddByte(0xFF)
				*skip = -1
			} else {
				*skip++
			}
		}
	}
}

// addMapDescription writes a rectangular map area (all visible floors).
func (g *GameProtocol) addMapDescription(w *netmsg.Writer, x, y int, z uint8, width, height int) {
	g.deps.World.RLock()
	defer g.deps.World.RUnlock()

	skip := -1
	var startz, endz, zstep int
	if z > surfaceZ {
		startz = int(z) - 2
		endz = min(maxLayers-1, int(z)+2)
		zstep = 1
	} else {
		startz = surfaceZ
		endz = 0
		zstep = -1
	}
	for nz := startz; nz != endz+zstep; nz += zstep {
		g.addFloorDescription(w, x, y, nz, width, height, int(z)-nz, &skip)
	}
	if skip >= 0 {
		w.AddByte(byte(skip))
		w.AddByte(0xFF)
	}
}

// ClientIndexOfCreature ports Tile::getClientIndexOfCreature
// (src/items/tile.cpp:1433). It is the index this client's tile stack assigns to
// creatureID: ground, then always-on-top items, then the creatures ABOVE the
// target that this player can see. Walked in the same order the client draws the
// pile — ground, always-on-top items, then the visible creatures below the target
// — so the index it returns is the target's slot in the client's stack.
// Returns -1 when the creature is not on the tile, exactly like the C++; -1 means
// "send no packet", not "stackpos 0".
//
// Note the upstream method has no cap: the 10-thing limit is applied by the
// sender (sendMoveCreature falls back to remove+add when oldStackPos >= 10), not
// here. Tile::getStackposOfCreature is the capped variant and has no callers.
func (g *GameProtocol) ClientIndexOfCreature(pos game.Position, creatureID uint32) int {
	g.deps.World.RLock()
	defer g.deps.World.RUnlock()
	return g.clientIndexOfCreatureLocked(pos, creatureID)
}

// clientIndexOfCreatureLocked is ClientIndexOfCreature for callers that already
// hold the world lock (the pre-removal capture runs inside it).
func (g *GameProtocol) clientIndexOfCreatureLocked(pos game.Position, creatureID uint32) int {
	// Tile::getClientIndexOfCreature (tile.cpp:1433) returns -1 with no viewer.
	if g.player == nil {
		return -1
	}
	tile := g.deps.World.Map.GetTile(pos)
	if tile == nil {
		return -1
	}
	n := 0
	if tile.Ground != nil {
		n = 1
	}
	for _, it := range tile.Items {
		if g.isTopItem(it) {
			n++
		}
	}
	// Walk the creatures in FORWARD order, matching Go's tile slice (newest
	// appended last). The client draws the first thing it receives at the bottom,
	// so the creature at slice index 0 sits lowest. Counting the visible creatures
	// that come before the target (below it in the pile) pushes its index up.
	// This mirrors Tile::getClientIndexOfCreature (tile.cpp:1433), which walks the
	// vector's reverse_view over an array that is itself newest-first.
	for i := 0; i < len(tile.Creatures); i++ {
		c := tile.Creatures[i]
		if c.GetID() == creatureID {
			return n
		}
		if g.canSeeCreature(c) {
			n++
		}
	}
	return -1
}

// StackPosOf returns the tile stack position of a creature known to be on the
// tile: it is only used for the moving player's own view, taken before the step,
// mirroring the newStackPos argument C++ passes to sendAddCreature. Everything
// that resolves another player's view must use ClientIndexOfCreature and honour
// its -1.
func (g *GameProtocol) StackPosOf(pos game.Position, creatureID uint32) uint8 {
	idx := g.ClientIndexOfCreature(pos, creatureID)
	if idx < 0 {
		return 0
	}
	return uint8(idx)
}

// SendCreatureWalkthrough sends opcode 0x92 to update a creature's walkthrough state on client.
func (g *GameProtocol) SendCreatureWalkthrough(c game.Creature, walkthrough bool) {
	// Two different questions: canSeeCreature is about the creature (ghost,
	// invisible), canSee is about whether its tile is in this client's window. C++
	// checks the position one (protocolgame.cpp sendCreatureWalkthrough).
	if c == nil || !g.canSeeCreature(c) || !g.canSee(c.GetPosition()) {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x92)
	w.AddU32(c.GetID())
	if walkthrough {
		w.AddByte(0x00) // 0 = can walk through
	} else {
		w.AddByte(0x01) // 1 = solid
	}
	g.SendToClient(w)
}
