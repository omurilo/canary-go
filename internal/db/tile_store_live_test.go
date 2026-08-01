package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/omurilo/canary-go/internal/config"
	"github.com/omurilo/canary-go/internal/game"
)

// liveDB connects to the MariaDB the compose stack runs on 3307, or skips. The unit
// tests above only exercise the codec in memory; this is the one that proves the
// table itself round-trips, which is the whole claim being made about house items
// no longer being lost.
func liveDB(t *testing.T) (*DB, context.Context) {
	t.Helper()
	if os.Getenv("CANARY_SKIP_DB_TESTS") != "" {
		t.Skip("CANARY_SKIP_DB_TESTS is set")
	}
	cfg := &config.Config{
		DBHost: "127.0.0.1", DBPort: 3307,
		DBUser: "root", DBPassword: "root", DBName: "canary",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	d, err := Connect(ctx, cfg)
	if err != nil {
		t.Skipf("no live database: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := d.SQL.PingContext(ctx); err != nil {
		t.Skipf("database unreachable: %v", err)
	}
	return d, ctx
}

func TestTileStorePersistsAgainstLiveDB(t *testing.T) {
	d, ctx := liveDB(t)

	// tile_store has a foreign key onto houses, so the house has to exist first.
	const houseID = 999001
	if _, err := d.SQL.ExecContext(ctx, "DELETE FROM `tile_store` WHERE house_id = ?", houseID); err != nil {
		t.Fatalf("clean tile_store: %v", err)
	}
	if _, err := d.SQL.ExecContext(ctx, "DELETE FROM `houses` WHERE id = ?", houseID); err != nil {
		t.Fatalf("clean houses: %v", err)
	}
	if _, err := d.SQL.ExecContext(ctx,
		"INSERT INTO `houses` (id, owner, name, town_id, rent, size, beds) VALUES (?, 0, 'tile store test', 1, 0, 1, 0)",
		houseID); err != nil {
		t.Skipf("cannot create a test house (schema mismatch?): %v", err)
	}
	t.Cleanup(func() {
		d.SQL.ExecContext(context.Background(), "DELETE FROM `tile_store` WHERE house_id = ?", houseID)
		d.SQL.ExecContext(context.Background(), "DELETE FROM `houses` WHERE id = ?", houseID)
	})

	pos := game.Position{X: 30000, Y: 30001, Z: 7}
	count := uint16(42)

	// A world holding one house tile with a table and a bag of coins on it.
	saveWorld := game.NewWorld()
	saveWorld.Items = tileStoreCatalog()
	saveWorld.Map.SetTile(pos, &game.Tile{
		Ground: &game.Item{ID: 1},
		Items: []*game.Item{
			{ID: 1650},
			{ID: 1987, Contents: []*game.Item{
				{ID: 3031, Count: count, Attr: &game.ItemAttributes{HasCount: true}},
			}},
		},
	})
	saveWorld.Houses = map[uint32]*game.House{
		houseID: {ID: houseID, HouseTiles: []game.Position{pos}},
	}

	saved, err := d.SaveHouseItems(ctx, saveWorld)
	if err != nil {
		t.Fatalf("SaveHouseItems: %v", err)
	}
	if saved != 1 {
		t.Fatalf("saved %d tiles, want 1", saved)
	}

	// A different world with the same map geometry and nothing on the tile: exactly
	// what a restart looks like.
	loadWorld := game.NewWorld()
	loadWorld.Items = tileStoreCatalog()
	loadWorld.Map.SetTile(pos, &game.Tile{Ground: &game.Item{ID: 1}})

	restored, err := d.LoadHouseItems(ctx, loadWorld)
	if err != nil {
		t.Fatalf("LoadHouseItems: %v", err)
	}
	if restored != 2 {
		t.Fatalf("restored %d items, want 2", restored)
	}

	tile := loadWorld.Map.GetTile(pos)
	if len(tile.Items) != 2 {
		t.Fatalf("tile came back with %d items, want 2", len(tile.Items))
	}
	// Reversed on save, so the bag is first.
	bag := tile.Items[0]
	if bag.ID != 1987 {
		t.Fatalf("first item = %d, want the bag (1987)", bag.ID)
	}
	if len(bag.Contents) != 1 || bag.Contents[0].ID != 3031 {
		t.Fatalf("the bag's contents did not survive the database: %+v", bag.Contents)
	}
	if got := bag.Contents[0].Count; got != count {
		t.Errorf("coin count through the database = %d, want %d", got, count)
	}
	if tile.Items[1].ID != 1650 {
		t.Errorf("second item = %d, want the table (1650)", tile.Items[1].ID)
	}
}
