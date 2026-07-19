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

// creatureIndex maps tile positions to the creatures standing on them.
type creatureIndex map[posKey][]*game.Player

func (g *GameProtocol) buildCreatureIndex(center game.Position) creatureIndex {
	idx := make(creatureIndex)
	for _, p := range g.deps.World.Spectators(center, 0) {
		k := posKey{p.Pos.X, p.Pos.Y, p.Pos.Z}
		idx[k] = append(idx[k], p)
	}
	// Include self (Spectators excludes id 0 only; self is included since we
	// pass excludeID 0, but ensure presence).
	if g.player != nil {
		k := posKey{g.player.Pos.X, g.player.Pos.Y, g.player.Pos.Z}
		found := false
		for _, c := range idx[k] {
			if c.ID == g.player.ID {
				found = true
				break
			}
		}
		if !found {
			idx[k] = append(idx[k], g.player)
		}
	}
	return idx
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
func (g *GameProtocol) addCreature(w *netmsg.Writer, c *game.Player) {
	known := g.known[c.ID]
	if known {
		w.AddU16(creatureKnown)
		w.AddU32(c.ID)
	} else {
		g.known[c.ID] = true
		w.AddU16(creatureNew)
		w.AddU32(0) // removedKnownId (cache not full)
		w.AddU32(c.ID)
		w.AddByte(0) // creatureType: PLAYER
		w.AddString(c.Name)
	}

	healthPct := byte(100)
	if c.MaxHealth > 0 {
		healthPct = byte(c.Health * 100 / c.MaxHealth)
	}
	w.AddByte(healthPct)
	w.AddByte(byte(c.Direction))
	addOutfit(w, c.Outfit)
	w.AddByte(c.LightLevel)
	w.AddByte(c.LightColor)
	w.AddU16(c.Speed)
	w.AddByte(0) // creature icons count
	w.AddByte(0) // skull
	w.AddByte(0) // party shield
	if !known {
		w.AddByte(0) // guild emblem
	}
	w.AddByte(0)    // creature type (again)
	w.AddByte(0)    // vocation client id (PLAYER)
	w.AddByte(0)    // speech bubble
	w.AddByte(0xFF) // mark (unmarked)
	w.AddByte(0)    // inspection type
	w.AddByte(0)    // walkthrough (can walk through: 0)
}

// isTopItem reports whether an item stacks below creatures (always-on-top).
func (g *GameProtocol) isTopItem(it *game.Item) bool {
	t := g.deps.Items.Get(it.ID)
	return t != nil && t.AlwaysOnTop()
}

// addTileDescription writes a tile's things in the client's stack order, mirroring
// GetTileDescription: ground, always-on-top items, creatures, then normal items.
// Placing creatures between the two item groups is what keeps creature stackpos in
// sync with the client (a mismatch makes 0x6D moves reference the wrong thing).
func (g *GameProtocol) addTileDescription(w *netmsg.Writer, t *game.Tile, idx creatureIndex, pos game.Position) {
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
	creatures := idx[posKey{pos.X, pos.Y, pos.Z}]
	for i := len(creatures) - 1; i >= 0; i-- {
		if count >= 10 {
			return
		}
		g.addCreature(w, creatures[i])
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
func (g *GameProtocol) addFloorDescription(w *netmsg.Writer, x, y, z, width, height, offset int, idx creatureIndex, skip *int) {
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
				g.addTileDescription(w, tile, idx, pos)
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
func (g *GameProtocol) addMapDescription(w *netmsg.Writer, x, y int, z uint8, width, height int, idx creatureIndex) {
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
		g.addFloorDescription(w, x, y, nz, width, height, int(z)-nz, idx, &skip)
	}
	if skip >= 0 {
		w.AddByte(byte(skip))
		w.AddByte(0xFF)
	}
}

// stackPosOf returns the tile stack position of the given creature, matching the
// order addTileDescription emits: ground, always-on-top items, then creatures.
func (g *GameProtocol) stackPosOf(pos game.Position, creatureID uint32, idx creatureIndex) uint8 {
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
	}
	creatures := idx[posKey{pos.X, pos.Y, pos.Z}]
	for i := len(creatures) - 1; i >= 0; i-- {
		if creatures[i].ID == creatureID {
			return uint8(stack)
		}
		stack++
	}
	return uint8(stack)
}
