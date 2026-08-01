package luaengine

import (
	"log/slog"
	"os"
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/moveevents"
)

// TestCitizenTempleTeleport drives the full temple/citizen "set town" chain:
// OTBM unique id -> StepIn movement lookup -> creature:getPlayer() ->
// Town(id):getTemplePosition() -> player:setTown(). Regression for the bug
// where stepping on a temple tile did nothing.
func TestCitizenTempleTeleport(t *testing.T) {
	repo := "../../.."
	citizen := repo + "/data-otservbr-global/scripts/movements/teleport/citizen.lua"
	if _, err := os.Stat(citizen); err != nil {
		t.Skip("datapack not present")
	}

	w := game.NewWorld()
	w.TownsByID[8] = game.Position{X: 5556, Y: 5098, Z: 7} // Thais temple
	w.TownNames[8] = "Thais"
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	defer e.Close()

	// Bootstrap the libs citizen.lua depends on, then the movement itself.
	for _, f := range []string{
		"/data/global.lua",
		"/data/libs/functions/revscriptsys.lua",
		"/data-otservbr-global/lib/tables/town.lua", // TOWNS_LIST
		"/data/libs/functions/creature.lua",
		"/data/libs/functions/player.lua",
	} {
		_ = e.DoFile(repo + f)
	}
	if err := e.DoFile(citizen); err != nil {
		t.Fatalf("load citizen.lua: %v", err)
	}

	// The Thais citizen tile has unique id 9057.
	evt := moveevents.FindStepInByUniqueID(9057)
	if evt == nil {
		t.Fatalf("citizen movement not registered by unique id 9057")
	}

	p := &game.Player{Name: "Tester", TownID: 1, MaxHealth: 100, Health: 100, Vocation: 1}
	tile := &game.Item{ID: 1949}
	uid := uint16(9057)
	tile.Attr = &game.ItemAttributes{UniqueID: &uid}
	pos := game.Position{X: 100, Y: 100, Z: 7}

	e.CallStepIn(evt, p, tile, pos, pos)

	if p.TownID != 8 {
		t.Errorf("TownID after stepping on Thais temple tile = %d, want 8", p.TownID)
	}
}
