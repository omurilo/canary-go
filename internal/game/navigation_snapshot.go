package game

import "sync"

// NavCell stores walkability flags for a single tile position.
type NavCell struct {
	HasGround    bool
	BlockSolid   bool
	HarmfulField bool
}

// NavSectorSnapshot caches walkability for a sector (8x8 tiles) at one z-level.
type NavSectorSnapshot struct {
	X, Y, Z int
	cells   [64]NavCell // 8x8 grid
}

// Cell returns the NavCell at the local sector offset.
func (s *NavSectorSnapshot) Cell(localX, localY int) NavCell {
	if localX < 0 || localX > 7 || localY < 0 || localY > 7 {
		return NavCell{}
	}
	return s.cells[localY*8+localX]
}

// sectorKey uniquely identifies a sector.
type sectorKey struct {
	X, Y, Z int
}

// navCache manages cached NavSectorSnapshots with revision tracking.
type navCache struct {
	mu       sync.RWMutex
	sectors  map[sectorKey]*NavSectorSnapshot
	topology uint64 // incremented when map topology changes
}

func newNavCache() *navCache {
	return &navCache{
		sectors: make(map[sectorKey]*NavSectorSnapshot),
	}
}

// getOrCreateSnapshot returns a cached snapshot for the sector containing (x,y,z),
// or builds one from the map.
func (nc *navCache) getOrCreateSnapshot(m *Map, x, y, z int) *NavSectorSnapshot {
	sx, sy := sectorOrigin(x, y)
	key := sectorKey{X: sx, Y: sy, Z: z}

	nc.mu.RLock()
	snap, ok := nc.sectors[key]
	nc.mu.RUnlock()
	if ok {
		return snap
	}

	// Build snapshot from scratch.
	snap = buildSectorSnapshot(m, sx, sy, z)
	nc.mu.Lock()
	nc.sectors[key] = snap
	nc.mu.Unlock()
	return snap
}

// invalidate removes the sector containing (x,y,z) from the cache.
func (nc *navCache) invalidate(x, y, z int) {
	sx, sy := sectorOrigin(x, y)
	key := sectorKey{X: sx, Y: sy, Z: z}
	nc.mu.Lock()
	delete(nc.sectors, key)
	nc.topology++
	nc.mu.Unlock()
}

// sectorOrigin returns the sector origin (top-left corner) for a position.
func sectorOrigin(x, y int) (int, int) {
	return (x / 8) * 8, (y / 8) * 8
}

// buildSectorSnapshot creates a NavSectorSnapshot from the map for an 8x8 area.
func buildSectorSnapshot(m *Map, sx, sy, z int) *NavSectorSnapshot {
	snap := &NavSectorSnapshot{X: sx, Y: sy, Z: z}
	for dy := 0; dy < 8; dy++ {
		for dx := 0; dx < 8; dx++ {
			pos := Position{X: uint16(sx + dx), Y: uint16(sy + dy), Z: uint8(z)}
			tile := m.GetTile(pos)
			idx := dy*8 + dx
			if tile == nil {
				snap.cells[idx] = NavCell{}
				continue
			}
			cell := NavCell{
				HasGround:  tile.Ground != nil,
				BlockSolid: tile.Ground == nil,
			}
			snap.cells[idx] = cell
		}
	}
	return snap
}
