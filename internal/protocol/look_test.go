package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
)

func TestBuildPlayerDescription(t *testing.T) {
	viewer := &game.Player{
		ID:       1001,
		Name:     "Viewer",
		Level:    100,
		Vocation: 1,
		Sex:      1,
	}

	target := &game.Player{
		ID:       1002,
		Name:     "Target Player",
		Level:    50,
		Vocation: 2,
		Sex:      0, // Female
	}

	// Test looking at self
	descSelf := BuildPlayerDescription(viewer, viewer)
	expectedSelf := "You see yourself."
	if !contains(descSelf, expectedSelf) {
		t.Errorf("BuildPlayerDescription(self) = %q, want it to contain %q", descSelf, expectedSelf)
	}

	// Test looking at another player (female, Druid/Vocation 2)
	descOther := BuildPlayerDescription(viewer, target)
	expectedOther := "You see Target Player (Level 50)."
	if !contains(descOther, expectedOther) {
		t.Errorf("BuildPlayerDescription(other) = %q, want it to contain %q", descOther, expectedOther)
	}

	// Test looking at player with guild
	target.GuildName = "Antigravity Devs"
	target.GuildRankName = "Leader"
	target.GuildNick = "The AI"
	descGuild := BuildPlayerDescription(viewer, target)
	expectedGuild := "She is Leader of the Antigravity Devs (The AI)."
	if !contains(descGuild, expectedGuild) {
		t.Errorf("BuildPlayerDescription(guild) = %q, want it to contain %q", descGuild, expectedGuild)
	}
}

func TestBuildItemDescription(t *testing.T) {
	itemType := &items.ItemType{
		ID:          2160,
		Name:        "crystal coin",
		Article:     "a",
		Description: "It's a valuable currency.",
		Weight:      10, // 0.10 oz
		ShowCharges: false,
	}

	wandType := &items.ItemType{
		ID:          2184,
		Name:        "training wand",
		Article:     "a",
		Description: "A powerful magic wand.",
		ShowCharges: true,
		Charges:     500,
	}

	cat := items.NewCatalog(itemType, wandType)

	item := &game.Item{
		ID:    2160,
		Count: 5,
	}

	desc := BuildItemDescription(nil, item, cat)
	expectedName := "You see a crystal coin."
	expectedWeight := "It weighs 0.10 oz."
	if !contains(desc, expectedName) {
		t.Errorf("BuildItemDescription = %q, want to contain %q", desc, expectedName)
	}
	if !contains(desc, expectedWeight) {
		t.Errorf("BuildItemDescription = %q, want to contain %q", desc, expectedWeight)
	}

	wand := &game.Item{
		ID: 2184,
	}

	descWand := BuildItemDescription(nil, wand, cat)
	expectedWandCharges := "that has 500 charges left"
	if !contains(descWand, expectedWandCharges) {
		t.Errorf("BuildItemDescription(wand) = %q, want to contain %q", descWand, expectedWandCharges)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s[0:len(substr)] == substr || s[len(s)-len(substr):] == substr || find(s, substr))
}

func find(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
