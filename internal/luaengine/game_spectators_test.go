package luaengine

import (
	"testing"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
)

func TestGameGetSpectatorsLargeRange(t *testing.T) {
	e := newTestEngine()

	p1 := &game.Player{Name: "TargetPlayer"}
	p1.SetPosition(game.Position{X: 3000, Y: 3000, Z: 7})
	e.world.AddPlayer(p1, nil)

	p2 := &game.Player{Name: "SearcherPlayer"}
	p2.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	e.world.AddPlayer(p2, nil)

	start := time.Now()
	// Simulate Lua call Game.getSpectators(pos, true, true, 5000, 5000, 5000, 5000)
	script := `
		local pos = Position(100, 100, 7)
		local specs = Game.getSpectators(pos, true, true, 5000, 5000, 5000, 5000)
		return #specs
	`
	err := e.DoString(script)
	if err != nil {
		t.Fatalf("Lua execution error: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Game.getSpectators with range 5000 took too long: %v (expected < 100ms)", elapsed)
	}

	res := e.L.Get(-1)
	e.L.Pop(1)
	if count, ok := res.(interface{ String() string }); !ok || count.String() != "2" {
		t.Errorf("Expected 2 spectators, got %v", res)
	}
}
