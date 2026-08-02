package luaengine

import (
	"path/filepath"
	"testing"

	"github.com/omurilo/canary-go/internal/game"
)

// data/libs/functions/teleport.lua opens with `function Teleport.isTeleport(self)`
// and defines SimpleTeleport on line 5. With Teleport bound as a bare function
// the file aborted on line 1, so SimpleTeleport never existed and every script
// calling it failed with "attempt to call a non-function object".
func TestTeleportLibLoads(t *testing.T) {
	e := New(game.NewWorld(), nil)
	defer e.Close()

	if err := e.DoFile(filepath.Join("..", "..", "data", "libs", "functions", "teleport.lua")); err != nil {
		t.Fatalf("teleport.lua: %v", err)
	}
	if err := e.DoString(`
		assert(type(Teleport) == "table", "Teleport must be a class table, got " .. type(Teleport))
		assert(type(Teleport.isTeleport) == "function", "the datapack's method must stick")
		assert(type(SimpleTeleport) == "function", "SimpleTeleport must exist")
	`); err != nil {
		t.Fatalf("%v", err)
	}
}
