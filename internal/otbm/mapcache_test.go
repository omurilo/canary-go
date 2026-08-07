package otbm

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
)

func pos(x, y uint16, z uint8) *game.Position {
	return &game.Position{X: x, Y: y, Z: z}
}

// The actual regression (phase 6 "map cache"): the Dreamers carrot field puts
// its unique ids 2241/2242 on the cave-floor GROUND tiles, and floor id 351/353
// is shared by thousands of other caves. Deduping conflated that uid onto every
// other cave floor with the same ground id, so stepping on any of them fired the
// carrot crossing and teleported the player to one fixed quest spot.
func TestTileCacheKeepsGroundUniqueIDsDistinct(t *testing.T) {
	tc := NewTileCache()

	carrot := &game.Tile{Ground: &game.Item{ID: 351, Attr: &game.ItemAttributes{UniqueID: ptr16(2241)}}}
	plain := &game.Tile{Ground: &game.Item{ID: 351}}

	c1 := tc.CreateOrGetTile(carrot) // parsed first
	c2 := tc.CreateOrGetTile(plain)

	if c1 == c2 {
		t.Fatal("a cave floor carrying a unique id on its GROUND must not dedup with a plain floor")
	}

	// Regardless of parse order, the plain tile must never gain the uid nor lose it back.
	tc2 := NewTileCache()
	p1 := tc2.CreateOrGetTile(plain)   // parsed first
	cc := tc2.CreateOrGetTile(carrot)  // parsed second
	if p1 == cc {
		t.Fatal("plain floor parsed first must not absorb the next tile's uid")
	}
	if cc.Ground.Attr == nil || cc.Ground.Attr.UniqueID == nil || *cc.Ground.Attr.UniqueID != 2241 {
		t.Fatal("the carrot ground tile must keep its unique id regardless of parse order")
	}
	if p1.Ground.Attr != nil && p1.Ground.Attr.UniqueID != nil {
		t.Fatal("the plain ground tile must not gain a unique id")
	}
}

func TestTileCacheStillDedupsIdenticalTiles(t *testing.T) {
	tc := NewTileCache()
	a := &game.Tile{Ground: &game.Item{ID: 411}, Items: []*game.Item{{ID: 385}}}
	b := &game.Tile{Ground: &game.Item{ID: 411}, Items: []*game.Item{{ID: 385}}}
	if tc.CreateOrGetTile(a) != tc.CreateOrGetTile(b) {
		t.Error("identical state-free tiles should still dedup to one pointer")
	}
}

func ptr16(v uint16) *uint16 { return &v }