package otbm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
)

// Helper to write string with u16 length
func writeStr(s string) []byte {
	return append([]byte{byte(len(s)), byte(len(s) >> 8)}, []byte(s)...)
}

// Helper to write uint16
func u16(v uint16) []byte {
	return []byte{byte(v), byte(v >> 8)}
}

// Helper to write uint32
func u32(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

func TestLoad(t *testing.T) {
	// Build dummy OTBM data
	var data []byte
	// 4 byte identifier
	data = append(data, []byte{'O', 'T', 'B', 'M'}...)

	// Root node start
	data = append(data, 0xFE)
	data = append(data, 1) // root node type (OTBM_ROOTV1)

	// map header
	data = append(data, u32(2)...) // version
	data = append(data, u16(100)...) // width
	data = append(data, u16(100)...) // height
	data = append(data, u32(3)...) // major
	data = append(data, u32(2)...) // minor

	// Root child: MAP_DATA (0xFE)
	data = append(data, 0xFE)
	data = append(data, nodeMapData) // 2

	// Map data attr: description
	data = append(data, attrDescription)
	data = append(data, writeStr("Test map")...)

	// Map data child: TILE_AREA (0xFE)
	data = append(data, 0xFE)
	data = append(data, nodeTileArea) // 4
	data = append(data, u16(100)...) // base_x
	data = append(data, u16(100)...) // base_y
	data = append(data, 7) // base_z

	// Tile child (0xFE)
	data = append(data, 0xFE)
	data = append(data, nodeTile) // 5
	data = append(data, 5) // xOff
	data = append(data, 5) // yOff

	// Tile item inline attr
	data = append(data, attrItem)
	data = append(data, u16(1000)...) // item id

	// Node Item child (0xFE) inside Tile
	data = append(data, 0xFE)
	data = append(data, nodeItem) // 6
	data = append(data, u16(2000)...) // item id
	data = append(data, attrCount)
	data = append(data, 5) // count
	// end nodeItem
	data = append(data, 0xFF)

	// end Tile
	data = append(data, 0xFF)

	// end TileArea
	data = append(data, 0xFF)

	// Map data child: TownsGrp (0xFE)
	data = append(data, 0xFE)
	data = append(data, nodeTownsGrp)
	
	// Town (0xFE)
	data = append(data, 0xFE)
	data = append(data, nodeTown)
	data = append(data, u32(1)...) // id
	data = append(data, writeStr("Thais")...) // name
	data = append(data, u16(100)...) // x
	data = append(data, u16(100)...) // y
	data = append(data, 7) // z
	// end Town
	data = append(data, 0xFF)
	
	// end TownsGrp
	data = append(data, 0xFF)

	// end MapData
	data = append(data, 0xFF)

	// end Root
	data = append(data, 0xFF)

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "test.otbm")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	m := game.NewMap()
	res, err := Load(path, nil, m)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if res.Width != 100 || res.Height != 100 {
		t.Errorf("expected width/height 100/100, got %d/%d", res.Width, res.Height)
	}
	if res.Description != "Test map" {
		t.Errorf("expected description 'Test map', got %q", res.Description)
	}
	if len(res.Towns) != 1 || res.Towns[0].Name != "Thais" {
		t.Errorf("unexpected towns: %+v", res.Towns)
	}
	if res.TileCount != 1 {
		t.Errorf("expected 1 tile, got %d", res.TileCount)
	}
	if res.ItemCount != 2 {
		t.Errorf("expected 2 items, got %d", res.ItemCount)
	}

	// Verify tile
	pos := game.Position{X: 105, Y: 105, Z: 7}
	tile := m.GetTile(pos)
	if tile == nil {
		t.Fatalf("expected tile at %v", pos)
	}
	if len(tile.Items) != 2 {
		t.Fatalf("expected 2 items on tile, got %d", len(tile.Items))
	}
	if tile.Items[0].ID != 1000 {
		t.Errorf("expected first item ID 1000, got %d", tile.Items[0].ID)
	}
	if tile.Items[1].ID != 2000 {
		t.Errorf("expected second item ID 2000, got %d", tile.Items[1].ID)
	}
	if tile.Items[1].Count != 5 {
		t.Errorf("expected second item Count 5, got %d", tile.Items[1].Count)
	}
}
