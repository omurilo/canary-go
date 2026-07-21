package luaengine

import (
	"log/slog"
	"os"
	"testing"

	"github.com/opentibiabr/canary-go/internal/actions"
	"github.com/opentibiabr/canary-go/internal/game"
)

func TestFoodActionRepro(t *testing.T) {
	w := game.NewWorld()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer e.Close()

	// Load core bootstrap (enums/globals) then the food action script.
	core := "../../../data"
	foods := core + "/scripts/actions/items/foods.lua"
	if _, err := os.Stat(foods); err != nil {
		t.Skip("core datapack not present")
	}
	_ = e.DoFile(core + "/global.lua")
	if err := e.DoFile(foods); err != nil {
		t.Fatalf("load foods.lua: %v", err)
	}

	meat := &game.Item{ID: 3577, Count: 1} // meat, in the foods table
	act := actions.FindAction(meat)
	if act == nil {
		t.Fatalf("no action registered for food id 3577 — FindAction returned nil")
	}

	p := &game.Player{Name: "Tester", MaxHealth: 150, Health: 150}
	pos := game.Position{X: 100, Y: 100, Z: 7}
	ok := e.CallAction(act, p, meat, pos, nil, pos, false)
	if !ok {
		t.Fatalf("food action returned false")
	}
	if meat.Count != 0 {
		t.Errorf("meat not consumed: Count = %d, want 0 (item:remove should zero it)", meat.Count)
	}
}
