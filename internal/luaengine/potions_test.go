package luaengine

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/actions"
	"github.com/opentibiabr/canary-go/internal/game"
)

func TestPotionsAction(t *testing.T) {
	w := game.NewWorld()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer e.Close()

	potionsScript := "../../../data/scripts/actions/items/potions.lua"
	if _, err := os.Stat(potionsScript); err != nil {
		t.Skip("potions.lua not found")
	}

	_ = e.DoFile("../../../data/global.lua")
	_ = filepath.Walk("../../../data/libs", func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && filepath.Ext(path) == ".lua" {
			_ = e.DoFile(path)
		}
		return nil
	})

	if err := e.DoFile(potionsScript); err != nil {
		t.Fatalf("load potions.lua error: %v", err)
	}

	p := &game.Player{ID: 100, Name: "Tester", Level: 100, MaxHealth: 500, Health: 100, MaxMana: 500, Mana: 50}
	p.Vocation = 1 // Sorcerer
	pos := game.Position{X: 100, Y: 100, Z: 7}

	// Test Great Mana Potion (item ID 238)
	manaPotion := &game.Item{ID: 238, Count: 1}
	act := actions.FindAction(manaPotion, game.Position{})
	if act == nil {
		t.Fatalf("no action registered for item 238")
	}

	ok := e.CallAction(act, p, manaPotion, pos, p, pos, false)
	if !ok {
		t.Fatalf("potions CallAction returned false")
	}

	if p.Mana <= 50 {
		t.Errorf("player mana after drinking mana potion = %d, want > 50", p.Mana)
	}
	if manaPotion.Count != 0 {
		t.Errorf("mana potion count after drinking = %d, want 0", manaPotion.Count)
	}

	// Test Health Potion (item ID 266)
	healthPotion := &game.Item{ID: 266, Count: 1}
	actHealth := actions.FindAction(healthPotion, game.Position{})
	if actHealth == nil {
		t.Fatalf("no action registered for item 266")
	}

	ok = e.CallAction(actHealth, p, healthPotion, pos, p, pos, false)
	if !ok {
		t.Fatalf("potions CallAction returned false for health potion")
	}

	if p.Health <= 100 {
		t.Errorf("player health after drinking health potion = %d, want > 100", p.Health)
	}
}
