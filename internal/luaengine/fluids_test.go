package luaengine

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/actions"
	"github.com/opentibiabr/canary-go/internal/game"
)

func TestFluidsAction(t *testing.T) {
	w := game.NewWorld()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer e.Close()

	core := "../../../data-otservbr-global"
	fluids := core + "/scripts/actions/other/fluids.lua"
	if _, err := os.Stat(fluids); err != nil {
		t.Skip("fluids.lua not found")
	}

	_ = e.DoFile("../../../data/global.lua")
	_ = filepath.Walk("../../../data/libs", func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && filepath.Ext(path) == ".lua" {
			_ = e.DoFile(path)
		}
		return nil
	})

	if err := e.DoFile(fluids); err != nil {
		t.Fatalf("load fluids.lua error: %v", err)
	}

	p := &game.Player{ID: 100, Name: "Tester", MaxHealth: 150, Health: 50, MaxMana: 100, Mana: 10}
	pos := game.Position{X: 100, Y: 100, Z: 7}

	// 1. Test mana fluid (type 10 = FLUID_MANA in fluids.lua)
	vialMana := &game.Item{ID: 2874, Count: 10}
	act := actions.FindAction(vialMana, game.Position{})
	if act == nil {
		t.Fatalf("no action registered for vial 2874")
	}

	ok := e.CallAction(act, p, vialMana, pos, p, pos, false)
	if !ok {
		t.Fatalf("fluids CallAction returned false")
	}

	if vialMana.Count != 0 {
		t.Errorf("vial fluid type after drinking = %d, want 0 (empty)", vialMana.Count)
	}
	if p.Mana <= 10 {
		t.Errorf("player mana after drinking mana fluid = %d, want > 10", p.Mana)
	}

	// 2. Test health fluid (type 11 = FLUID_LIFE in fluids.lua)
	vialHealth := &game.Item{ID: 2874, Count: 11}
	ok = e.CallAction(act, p, vialHealth, pos, p, pos, false)
	if !ok {
		t.Fatalf("fluids CallAction returned false")
	}

	if vialHealth.Count != 0 {
		t.Errorf("vial fluid type after drinking = %d, want 0 (empty)", vialHealth.Count)
	}
	if p.Health <= 50 {
		t.Errorf("player health after drinking health fluid = %d, want > 50", p.Health)
	}

	// 3. Test pouring fluid on ground tile (target item on ground, e.g. tile 101)
	groundTile := &game.Item{ID: 101, Count: 1}
	vialWine := &game.Item{ID: 2874, Count: 2} // wine
	ok = e.CallAction(act, p, vialWine, pos, groundTile, pos, false)
	if !ok {
		t.Fatalf("pouring fluid CallAction returned false")
	}

	if vialWine.Count != 0 {
		t.Errorf("vial fluid type after pouring = %d, want 0 (empty)", vialWine.Count)
	}

	tile := w.Map.GetTile(pos)
	if tile == nil || len(tile.Items) == 0 {
		t.Fatalf("expected pool item to be created on tile, got none")
	}
	pool := tile.Items[len(tile.Items)-1]
	if pool.ID != 2886 { // pool item ID in fluids.lua
		t.Errorf("pool item ID = %d, want 2886", pool.ID)
	}
	if pool.Count != 2 { // wine subtype in pool
		t.Errorf("pool fluid type = %d, want 2", pool.Count)
	}
}
