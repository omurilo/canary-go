package luaengine

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
)

func TestGameGetSpectatorsLuaMethod(t *testing.T) {
	e := newTestEngine()
	w := e.world

	pos := game.Position{X: 1000, Y: 1000, Z: 7}
	player := &game.Player{Name: "TestPlayer"}
	player.SetPosition(pos)

	tile := &game.Tile{
		Creatures: []game.Creature{player},
	}
	w.Map.SetTile(pos, tile)

	e.registerGame()

	// Test 1: Game.getSpectators returns table with player
	script := `
		local pos = Position(1000, 1000, 7)
		local specs = Game.getSpectators(pos, false, false, 3, 3, 3, 3)
		assert(type(specs) == "table", "specs must be table")
		assert(#specs == 1, "expected 1 spectator, got " .. tostring(#specs))
	`
	if err := e.DoString(script); err != nil {
		t.Fatalf("Game.getSpectators test failed: %v", err)
	}

	// Test 2: Game.getSpectators on empty pos returns empty table ({}), not nil
	scriptEmpty := `
		local pos = Position(2000, 2000, 7)
		local specs = Game.getSpectators(pos, false, false, 3, 3, 3, 3)
		assert(type(specs) == "table", "specs must be table")
		assert(#specs == 0, "expected 0 spectators, got " .. tostring(#specs))
	`
	if err := e.DoString(scriptEmpty); err != nil {
		t.Fatalf("Game.getSpectators empty test failed: %v", err)
	}
}
