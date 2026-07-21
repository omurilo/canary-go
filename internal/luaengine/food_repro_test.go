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
	// revscriptsys.lua overrides the Item metatable __index with ItemIndex
	// (getmetatable(self).getId(self)) — this is what made item.itemid crash at
	// revscriptsys.lua:66. player.lua defines the real Player.feed (food regen
	// accumulation). Load both so the test exercises the real production path.
	if _, err := os.Stat(core + "/libs/functions/revscriptsys.lua"); err == nil {
		_ = e.DoFile(core + "/libs/functions/revscriptsys.lua")
	}
	_ = e.DoFile(core + "/libs/functions/player.lua")
	if err := e.DoFile(foods); err != nil {
		t.Fatalf("load foods.lua: %v", err)
	}

	act := actions.FindAction(&game.Item{ID: 3577})
	if act == nil {
		t.Fatalf("no action registered for food id 3577 — FindAction returned nil")
	}

	p := &game.Player{Name: "Tester", MaxHealth: 150, Health: 150, Vocation: 1}
	pos := game.Position{X: 100, Y: 100, Z: 7}

	// Eat one meat: it must be consumed and add food (RegenTicks > 0).
	meat := &game.Item{ID: 3577, Count: 1}
	if !e.CallAction(act, p, meat, pos, nil, pos, false) {
		t.Fatalf("food action returned false")
	}
	if meat.Count != 0 {
		t.Errorf("meat not consumed: Count = %d, want 0", meat.Count)
	}
	if p.RegenTicks <= 0 {
		t.Fatalf("RegenTicks = %d, want > 0 (feed should add food)", p.RegenTicks)
	}
	first := p.RegenTicks

	// Eat a second meat: food must ACCUMULATE (not reset).
	meat2 := &game.Item{ID: 3577, Count: 1}
	e.CallAction(act, p, meat2, pos, nil, pos, false)
	if p.RegenTicks <= first {
		t.Errorf("RegenTicks did not accumulate: %d <= %d", p.RegenTicks, first)
	}

	// Eat until full: eventually the script must refuse (item not consumed) and
	// RegenTicks stops growing ("You are full").
	full := false
	for i := 0; i < 20; i++ {
		m := &game.Item{ID: 3577, Count: 1}
		e.CallAction(act, p, m, pos, nil, pos, false)
		if m.Count == 1 { // not consumed => full
			full = true
			break
		}
	}
	if !full {
		t.Errorf("player never became full after many meals; RegenTicks=%d", p.RegenTicks)
	}
}
