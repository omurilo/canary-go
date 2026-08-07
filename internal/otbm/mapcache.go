package otbm

import (
	"hash/fnv"
	"sync"

	"github.com/omurilo/canary-go/internal/game"
)

// tileHash computes a 64-bit hash from the tile's ground ID and item IDs.
func tileHash(t *game.Tile) uint64 {
	h := fnv.New64a()
	if t.Ground != nil {
		h.Write([]byte{byte(t.Ground.ID), byte(t.Ground.ID >> 8)})
	}
	h.Write([]byte{byte(t.Flags), byte(t.Flags >> 8), byte(t.Flags >> 16), byte(t.Flags >> 24)})
	for _, it := range t.Items {
		h.Write([]byte{byte(it.ID), byte(it.ID >> 8)})
	}
	return h.Sum64()
}

// tileHasState reports whether the ground or any item on the tile carries
// per-instance state that makes the tile unsafe to dedup: a unique/action id, a
// teleport destination, or container contents. tileHash ignores these (it only
// reads ground/flag/item ids), so two tiles with identical composition but
// different such state would otherwise share one *Tile.
//
// The ground must be checked too, not just t.Items: the Dreamers carrot field
// puts its unique ids 2241/2242 on the cave-floor GROUND tiles (e.g. a floor id
// 351 shared by thousands of other caves). Deduping them conflated that uid onto
// every other cave floor with the same ground id — stepping on any of them fired
// the carrot crossing and teleported the player to one fixed quest spot.
// itemHasState reports whether one item carries per-instance state.
func itemHasState(it *game.Item) bool {
	if it == nil {
		return false
	}
	if it.Attr != nil && (it.Attr.UniqueID != nil || it.Attr.ActionID != nil || it.Attr.TeleDest != nil) {
		return true
	}
	return it.Container != nil
}

func tileHasState(t *game.Tile) bool {
	if t == nil {
		return false
	}
	if itemHasState(t.Ground) {
		return true
	}
	for _, it := range t.Items {
		if itemHasState(it) {
			return true
		}
	}
	return false
}

// TileCache provides multi-tier deduplication for tiles during OTBM loading.
// Tiles with identical content share the same *Tile pointer, reducing memory.
type TileCache struct {
	mu sync.Mutex
	// Tiers for tiles that can be shared (no per-tile unique state).
	tiles map[uint64]*game.Tile
	// simpleItems is a set of item IDs that are simple enough to dedup.
	simpleItems map[uint16]struct{}
}

// NewTileCache creates a tile cache.
func NewTileCache() *TileCache {
	return &TileCache{
		tiles:       make(map[uint64]*game.Tile),
		simpleItems: make(map[uint16]struct{}),
	}
}

// CreateOrGetTile returns an existing tile with the same content, or stores
// and returns the new one. Tiles with unique state (e.g. house tiles, open
// containers) should NOT use this cache — they bypass it.
func (tc *TileCache) CreateOrGetTile(t *game.Tile) *game.Tile {
	if t == nil {
		return nil
	}
	// House tiles and tiles whose ground/items carry per-item state (unique or
	// action id, teleport destination, container contents) cannot be shared:
	// deduping them would make one tile's attribute leak onto every other tile
	// with the same ground+item composition.
	if t.HouseID > 0 || tileHasState(t) {
		return t
	}
	h := tileHash(t)
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if existing, ok := tc.tiles[h]; ok {
		return existing
	}
	tc.tiles[h] = t
	return t
}

// Reset clears the cache for a fresh map load.
func (tc *TileCache) Reset() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.tiles = make(map[uint64]*game.Tile)
}