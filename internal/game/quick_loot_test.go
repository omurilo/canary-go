package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/items"
)

func TestQuickLoot_FilterAndTransfer(t *testing.T) {
	cat := items.NewCatalog(
		&items.ItemType{
			ID:    1987,
			Name:  "Bag",
			Group: items.GroupContainer,
		},
		&items.ItemType{
			ID:   2160,
			Name: "Crystal Coin",
		},
		&items.ItemType{
			ID:   2148,
			Name: "Gold Coin",
		},
	)

	world := NewWorld()
	world.Items = cat
	world.Map = NewMap()

	player := &Player{
		ID:                      1,
		Name:                    "Tester",
		Pos:                     Position{X: 100, Y: 100, Z: 7},
		QuickLootFilter:         QuickLootFilterAccepted,
		QuickLootList:           []uint16{2160}, // Only Crystal Coins accepted
		QuickLootFallbackToMain: true,
	}
	world.players[player.ID] = player

	// Player's main backpack
	mainBag := &Item{ID: 1987, Contents: []*Item{}}
	player.Inventory[ConstSlotBackpack] = mainBag

	// Corpse container on tile
	corpsePos := Position{X: 101, Y: 100, Z: 7}
	corpse := &Item{
		ID: 1987,
		Contents: []*Item{
			{ID: 2148, Count: 100}, // Gold Coin (should be skipped by whitelist)
			{ID: 2160, Count: 10},  // Crystal Coin (should be looted)
		},
	}
	tile := &Tile{
		Items: []*Item{corpse},
	}
	world.Map.SetTile(corpsePos, tile)

	// Execute Quick Loot
	world.PlayerQuickLoot(player.ID, corpsePos, 0, 0, false)

	// Verify Crystal Coin was moved to mainBag
	if len(mainBag.Contents) != 1 {
		t.Fatalf("expected 1 item in mainBag, got %d", len(mainBag.Contents))
	}
	if mainBag.Contents[0].ID != 2160 {
		t.Errorf("expected looted item ID 2160, got %d", mainBag.Contents[0].ID)
	}

	// Verify Gold Coin remained in corpse
	if len(corpse.Contents) != 1 {
		t.Fatalf("expected 1 item remaining in corpse, got %d", len(corpse.Contents))
	}
	if corpse.Contents[0].ID != 2148 {
		t.Errorf("expected remaining item ID 2148, got %d", corpse.Contents[0].ID)
	}
}
