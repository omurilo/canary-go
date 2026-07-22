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

// addOutfit writes an Outfit.
func addOutfit(w *netmsg.Writer, o game.Outfit) {
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
	w.AddU16(o.LookMount)
	if o.LookMount != 0 {
		w.AddByte(o.MountHead)
		w.AddByte(o.MountBody)
		w.AddByte(o.MountLegs)
		w.AddByte(o.MountFeet)
	}
}

// addItem writes an item using the appearance catalog to decide which extra
// bytes the client expects (stackable count, fluid subtype, container marker,
// podium, tier, decay, charges, wrap kit) — matching the C++ AddItem branches.
func (g *GameProtocol) addItem(w *netmsg.Writer, it *game.Item) {
	w.AddU16(it.ID)
	t := g.deps.Items.Get(it.ID)
	if t == nil {
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
		w.AddByte(0) // tier
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
	known := g.known[c.GetID()]
	if known {
		w.AddU16(creatureKnown)
		w.AddU32(c.GetID())
	} else {
		g.known[c.GetID()] = true
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

// StackPosOf returns the tile stack position of the given creature, matching the
// order addTileDescription emits: ground, always-on-top items, then creatures.
func (g *GameProtocol) StackPosOf(pos game.Position, creatureID uint32) uint8 {
	g.deps.World.RLock()
	defer g.deps.World.RUnlock()
	stack := 0
	tile := g.deps.World.Map.GetTile(pos)
	if tile != nil {
		if tile.Ground != nil {
			stack++
		}
		for _, it := range tile.Items {
			if g.isTopItem(it) {
				stack++
			}
		}
		for i := len(tile.Creatures) - 1; i >= 0; i-- {
			c := tile.Creatures[i]
			if c.GetID() == creatureID {
				return uint8(stack)
			}
			if g.canSeeCreature(c) {
				stack++
			}
		}
	}
	return uint8(stack)
}

// StackPosWithIndex returns the tile stack position for a creature that was at the
// specified index in the Tile.Creatures slice before it was removed.
func (g *GameProtocol) StackPosWithIndex(pos game.Position, tileIndex int) uint8 {
	g.deps.World.RLock()
	defer g.deps.World.RUnlock()
	stack := 0
	tile := g.deps.World.Map.GetTile(pos)
	if tile != nil {
		if tile.Ground != nil {
			stack++
		}
		for _, it := range tile.Items {
			if g.isTopItem(it) {
				stack++
			}
		}
		if tileIndex >= 0 {
			stack += len(tile.Creatures) - tileIndex
		}
	}
	return uint8(stack)
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
