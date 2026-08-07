package game

import (
	"sync"

	"github.com/omurilo/canary-go/internal/items"
)

// Tile holds the contents of a single map cell.
type Tile struct {
	Ground    *Item
	Items     []*Item // stacked items (top + down), excluding creatures
	Creatures []Creature
	Flags     uint32 // tile flags (e.g. Protection Zone, No-PVP, etc.)
	HouseID   uint32 // 0 = not a house tile; >0 = owned by house with this ID
}

// TileFlags_t (items_definitions.hpp:443). Only the subset the map loader and
// the engine actually set is named here; the Lua enum table carries all of them.
const (
	TileFlagProtectionZone uint32 = 1 << 7
	TileFlagNoPvpZone      uint32 = 1 << 8
	TileFlagNoLogout       uint32 = 1 << 9
	TileFlagPvpZone        uint32 = 1 << 10
)

// IsProtectionZone reports whether the tile has the protection zone flag.
func (t *Tile) IsProtectionZone() bool {
	if t == nil {
		return false
	}
	return (t.Flags & TileFlagProtectionZone) != 0
}

// WalkableFor reports whether mover may stand on or walk through the tile.
func (t *Tile) WalkableFor(mover Creature, catalog *items.Catalog, worldType uint8) bool {
	if t == nil || t.Ground == nil {
		return false
	}
	if catalog != nil {
		if ct := catalog.Get(t.Ground.ID); ct != nil && ct.BlockSolid {
			return false
		}
		for _, it := range t.Items {
			if ct := catalog.Get(it.ID); ct != nil && ct.BlockSolid {
				return false
			}
		}
	}

	if len(t.Creatures) > 0 {
		moverPlayer, isMoverPlayer := mover.(*Player)
		for _, other := range t.Creatures {
			if other == mover {
				continue
			}
			otherPlayer, isOtherPlayer := other.(*Player)
			if isOtherPlayer {
				if otherPlayer.Ghost {
					continue
				}
				if isMoverPlayer && moverPlayer.GroupID >= 3 {
					continue
				}
				// Normal players cannot occupy the same tile as another non-ghost player.
				return false
			} else {
				return false
			}
		}
	}
	return true
}

// Walkable reports whether a creature may stand on the tile.
func (t *Tile) Walkable(catalog *items.Catalog) bool {
	return t.WalkableFor(nil, catalog, 1)
}

// HeightCount returns how many items on the tile (ground + stack) carry the
// height/elevation flag, mirroring Tile::hasHeight's accumulation. Stairs/ramps
// are tiles with 3+ such items.
func (t *Tile) HeightCount(catalog *items.Catalog) int {
	if t == nil || catalog == nil {
		return 0
	}
	n := 0
	if t.Ground != nil {
		if ct := catalog.Get(t.Ground.ID); ct != nil && ct.HasHeight {
			n++
		}
	}
	for _, it := range t.Items {
		if ct := catalog.Get(it.ID); ct != nil && ct.HasHeight {
			n++
		}
	}
	return n
}

// BlocksSolid reports whether the tile's ground or items block movement.
func (t *Tile) BlocksSolid(catalog *items.Catalog) bool {
	if t == nil || catalog == nil {
		return false
	}
	if t.Ground != nil {
		if ct := catalog.Get(t.Ground.ID); ct != nil && ct.BlockSolid {
			return true
		}
	}
	for _, it := range t.Items {
		if ct := catalog.Get(it.ID); ct != nil && ct.BlockSolid {
			return true
		}
	}
	return false
}

// Map is a sparse tile store keyed by sector position.
type Map struct {
	mu       sync.RWMutex
	sectors  map[uint32]*Sector
	navCache *navCache
}

// NewMap returns an empty map.
func NewMap() *Map {
	return &Map{
		sectors:  make(map[uint32]*Sector),
		navCache: newNavCache(),
	}
}

func sectorIndex(x, y int) uint32 {
	return uint32((x>>4)&0xFFFF) | (uint32((y>>4)&0xFFFF) << 16)
}

func localIndex(x, y int) int {
	return ((x & 15) << 4) | (y & 15)
}

// GetSectorSnapshot returns a walkability snapshot for the sector at (x,y,z).
func (m *Map) GetSectorSnapshot(catalog *items.Catalog, x, y, z int) *NavSectorSnapshot {
	return m.navCache.getOrCreateSnapshot(m, catalog, x, y, z)
}

// getSector gets or creates a sector for the given coordinates under a write lock.
func (m *Map) getSector(x, y, z int) *SectorFloor {
	idx := sectorIndex(x, y)
	s := m.sectors[idx]
	if s == nil {
		s = &Sector{}
		m.sectors[idx] = s
	}
	f := s.Floors[z]
	if f == nil {
		f = &SectorFloor{}
		s.Floors[z] = f
	}
	return f
}

// GetTile returns the tile at pos, or nil. It returns the materialized tile if it exists,
// or the base tile if it hasn't been modified.
func (m *Map) GetTile(pos Position) *Tile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx := sectorIndex(int(pos.X), int(pos.Y))
	s := m.sectors[idx]
	if s == nil {
		return nil
	}
	if pos.Z >= 16 {
		return nil
	}
	f := s.Floors[pos.Z]
	if f == nil {
		return nil
	}
	lidx := localIndex(int(pos.X), int(pos.Y))
	if t := f.Tiles[lidx]; t != nil {
		return t
	}
	return f.Base[lidx]
}

// RangeRect invokes fn for every loaded tile whose (x,y) lies inside the
// inclusive rectangle [x0,x1]x[y0,y1] on floor z, under a single read lock.
//
// It exists for the creature-vision queries (SpectatorCreatures and friends),
// which used to walk the whole w.creatures map — O(all creatures) per query.
// With the OTServBR world that is ~86k entries, and the monster AI runs the
// query per monster per think, so the single-threaded dispatcher spent all its
// time in those scans and never reached the NPC think loop. Enumerating the
// viewport's tiles instead makes the query cost proportional to the area the
// caller can actually see.
func (m *Map) RangeRect(x0, y0, x1, y1 int, z int, fn func(t *Tile)) {
	if fn == nil || x1 < x0 || y1 < y0 {
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for y := y0; y <= y1; y++ {
		if y < 0 {
			continue
		}
		for x := x0; x <= x1; x++ {
			if x < 0 {
				continue
			}
			idx := sectorIndex(x, y)
			s := m.sectors[idx]
			if s == nil {
				continue
			}
			if z >= 16 {
				continue
			}
			f := s.Floors[z]
			if f == nil {
				continue
			}
			lidx := localIndex(x, y)
			t := f.Tiles[lidx]
			if t == nil {
				t = f.Base[lidx]
			}
			if t != nil {
				fn(t)
			}
		}
	}
}

// Range invokes fn for every loaded tile. fn returns false to stop early.
func (m *Map) Range(fn func(pos Position, t *Tile) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for idx, s := range m.sectors {
		for z, f := range s.Floors {
			if f == nil {
				continue
			}
			baseX := (idx & 0xFFFF) * SectorSize
			baseY := (idx >> 16) * SectorSize
			for i := 0; i < SectorSize*SectorSize; i++ {
				t := f.Tiles[i]
				if t == nil {
					t = f.Base[i]
				}
				if t != nil {
					pos := Position{
						X: uint16(baseX) + uint16(i/SectorSize),
						Y: uint16(baseY) + uint16(i%SectorSize),
						Z: uint8(z),
					}
					if !fn(pos, t) {
						return
					}
				}
			}
		}
	}
}

// SetBaseTile stores a base tile directly from OTBM parsing.
func (m *Map) SetBaseTile(pos Position, t *Tile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pos.Z >= 16 {
		return
	}
	f := m.getSector(int(pos.X), int(pos.Y), int(pos.Z))
	f.Base[localIndex(int(pos.X), int(pos.Y))] = t
}

// SetTile stores a tile as an active materialized tile.
func (m *Map) SetTile(pos Position, t *Tile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pos.Z >= 16 {
		return
	}
	f := m.getSector(int(pos.X), int(pos.Y), int(pos.Z))
	f.Tiles[localIndex(int(pos.X), int(pos.Y))] = t
	m.navCache.invalidate(int(pos.X), int(pos.Y), int(pos.Z))
}

// GetOrCreateTile returns a materialized *Tile for mutation.
// It clones the Base tile if the active tile hasn't been created yet.
func (m *Map) GetOrCreateTile(pos Position) *Tile {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getTileForUpdate(pos)
}

// getTileForUpdate returns a materialized *Tile for mutation.
// It clones the Base tile if the active tile hasn't been created yet.
func (m *Map) getTileForUpdate(pos Position) *Tile {
	if pos.Z >= 16 {
		return nil
	}
	f := m.getSector(int(pos.X), int(pos.Y), int(pos.Z))
	lidx := localIndex(int(pos.X), int(pos.Y))
	if f.Tiles[lidx] != nil {
		return f.Tiles[lidx]
	}
	base := f.Base[lidx]
	if base == nil {
		return nil
	}
	// clone
	cloned := *base
	if len(base.Items) > 0 {
		cloned.Items = make([]*Item, len(base.Items))
		copy(cloned.Items, base.Items)
	}
	f.Tiles[lidx] = &cloned
	return &cloned
}

// AddItem inserts an item to the front of the tile stack at pos. Returns false if there is no
// tile there.
func (m *Map) AddItem(pos Position, it *Item) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.getTileForUpdate(pos)
	if t == nil {
		return false
	}
	t.Items = append([]*Item{it}, t.Items...)
	return true
}

// RemoveItemPtr removes a specific item pointer from the tile stack at pos.
func (m *Map) RemoveItemPtr(pos Position, item *Item) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.getTileForUpdate(pos)
	if t == nil {
		return false
	}
	for i, it := range t.Items {
		if it == item {
			t.Items = append(t.Items[:i], t.Items[i+1:]...)
			return true
		}
	}
	return false
}

// GenerateFlatField fills a square of ground tiles centered on center (on its
// own floor) with the given ground item id. Playable fallback until OTBM
// loading is wired in.
func (m *Map) GenerateFlatField(center Position, radius int, groundID uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if center.Z >= 16 {
		return
	}
	for dx := -radius; dx <= radius; dx++ {
		for dy := -radius; dy <= radius; dy++ {
			x := int(center.X) + dx
			y := int(center.Y) + dy
			f := m.getSector(x, y, int(center.Z))
			lidx := localIndex(x, y)
			if f.Tiles[lidx] == nil && f.Base[lidx] == nil {
				f.Base[lidx] = &Tile{Ground: &Item{ID: groundID}}
			}
		}
	}
}

// BlocksProjectile reports CONST_PROP_BLOCKPROJECTILE for the tile: whether a
// shot or a spell can pass over it. Used by the sight line, so a monster does
// not fire through a wall.
func (t *Tile) BlocksProjectile(catalog *items.Catalog) bool {
	if t == nil || catalog == nil {
		return false
	}
	if t.Ground != nil {
		if it := catalog.Get(t.Ground.ID); it != nil && it.BlockProjectile {
			return true
		}
	}
	for _, item := range t.Items {
		if item == nil {
			continue
		}
		if it := catalog.Get(item.ID); it != nil && it.BlockProjectile {
			return true
		}
	}
	return false
}
