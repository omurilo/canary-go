package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
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
		w.AddU16(0)
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
	w.AddByte(0)    // speech bubble
	w.AddByte(0xFF) // mark (unmarked)
	w.AddByte(0)    // inspection type
	walkthrough := byte(0x01)
	if g.canWalkthroughEx(g.player, c) {
		walkthrough = byte(0x00)
	}
	w.AddByte(walkthrough) // walkthrough (can walk through: 0, solid: 1)

	// OTCR extension: shader name + attached effects list.
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
	for i := len(t.Creatures) - 1; i >= 0; i-- {
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
// target that this player can see — walked in reverse, because the client stacks
// the most recently added creature lowest. Returns -1 when the creature is not on
// the tile, exactly like the C++; -1 means "send no packet", not "stackpos 0".
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
	for i := len(tile.Creatures) - 1; i >= 0; i-- {
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
	if c == nil || !g.canSeeCreature(c) {
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
