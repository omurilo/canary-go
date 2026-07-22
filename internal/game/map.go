package game

import (
	"sync"

	"github.com/opentibiabr/canary-go/internal/items"
)

// Tile holds the contents of a single map cell.
type Tile struct {
	Ground    *Item
	Items     []*Item // stacked items (top + down), excluding creatures
	Creatures []Creature
	Flags     uint32  // tile flags (e.g. Protection Zone, No-PVP, etc.)
}

// IsProtectionZone reports whether the tile has the protection zone flag.
func (t *Tile) IsProtectionZone() bool {
	if t == nil {
		return false
	}
	return (t.Flags & 1) != 0
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

// Map is a sparse tile store keyed by position.
type Map struct {
	mu    sync.RWMutex
	tiles map[Position]*Tile
}

// NewMap returns an empty map.
func NewMap() *Map {
	return &Map{tiles: make(map[Position]*Tile)}
}

// GetTile returns the tile at pos, or nil.
func (m *Map) GetTile(pos Position) *Tile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tiles[pos]
}

// Range invokes fn for every loaded tile. fn returns false to stop early.
func (m *Map) Range(fn func(pos Position, t *Tile) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for pos, t := range m.tiles {
		if !fn(pos, t) {
			return
		}
	}
}

// SetTile stores a tile.
func (m *Map) SetTile(pos Position, t *Tile) {
	m.mu.Lock()
	m.tiles[pos] = t
	m.mu.Unlock()
}

// AddItem appends an item to the tile stack at pos. Returns false if there is no
// tile there. The item lands on top of the existing stack.
func (m *Map) AddItem(pos Position, it *Item) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tiles[pos]
	if t == nil {
		return false
	}
	t.Items = append(t.Items, it)
	return true
}

// RemoveItemPtr removes a specific item pointer from the tile stack at pos.
func (m *Map) RemoveItemPtr(pos Position, item *Item) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tiles[pos]
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
	for dx := -radius; dx <= radius; dx++ {
		for dy := -radius; dy <= radius; dy++ {
			pos := Position{
				X: uint16(int(center.X) + dx),
				Y: uint16(int(center.Y) + dy),
				Z: center.Z,
			}
			if _, exists := m.tiles[pos]; !exists {
				m.tiles[pos] = &Tile{Ground: &Item{ID: groundID}}
			}
		}
	}
}
