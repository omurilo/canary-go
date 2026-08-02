package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
)

// TestGameGetOfflinePlayerOnline covers the online half of luaGameGetOfflinePlayer
// (game_functions.cpp:816): an id that matches a creature in the world returns
// that player. The DB half (LoadPlayerByGUID) shares loadPlayer's code path and
// is exercised against the MariaDB harness in a live run.
func TestGameGetOfflinePlayerOnline(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	owner := &game.Player{}
	owner.ID = 1000
	owner.Name = "HirelingOwner"
	e.world.AddCreature(owner)

	if err := e.L.DoString(`
		local p = Game.getOfflinePlayer(1000)
		assert(p ~= nil, "online player by id must be found")
		assert(p:getName() == "HirelingOwner", "wrong player: " .. tostring(p:getName()))

		local byName = Game.getOfflinePlayer("HirelingOwner")
		assert(byName ~= nil, "online player by name must be found")
		assert(byName:getName() == "HirelingOwner")

		local missing = Game.getOfflinePlayer(999999)
		assert(missing == nil, "unknown player must be nil")
	`); err != nil {
		t.Fatalf("Game.getOfflinePlayer failed: %v", err)
	}
}
