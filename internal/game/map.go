package game

import "sync"

// Tile holds the contents of a single map cell.
type Tile struct {
	Ground *Item
	Items  []*Item // stacked items (top + down), excluding creatures
}

// Walkable reports whether a creature may stand on the tile.
func (t *Tile) Walkable() bool {
	return t != nil && t.Ground != nil
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
