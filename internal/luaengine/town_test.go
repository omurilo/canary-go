package luaengine

import (
	"log/slog"
	"os"
	"testing"

	"github.com/omurilo/canary-go/internal/game"
)

func TestTownClassAndSetTown(t *testing.T) {
	w := game.NewWorld()
	w.TownsByID[8] = game.Position{X: 5556, Y: 5098, Z: 7}
	w.TownNames[8] = "Thais"
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	defer e.Close()

	// Town(8) resolves name + temple; unknown town is nil.
	if err := e.L.DoString(`
		local town = Town(8)
		assert(town ~= nil, "Town(8) nil")
		assert(town:getId() == 8, "id")
		assert(town:getName() == "Thais", "name=" .. tostring(town:getName()))
		local p = town:getTemplePosition()
		assert(p.x == 5556 and p.y == 5098 and p.z == 7, "temple pos")

		local townByName = Town("Thais")
		assert(townByName ~= nil, "Town('Thais') nil")
		assert(townByName:getId() == 8, "townByName id")

		assert(Town(999) == nil, "unknown town should be nil")
		assert(Town("UnknownTown") == nil, "unknown town name should be nil")
	`); err != nil {
		t.Fatalf("Town class: %v", err)
	}

	// player:setTown updates TownID + LoginPosition.
	p := &game.Player{Name: "Tester"}
	ud := e.L.NewUserData()
	ud.Value = p
	e.L.SetMetatable(ud, e.L.GetTypeMetatable("Player"))
	e.L.SetGlobal("_p", ud)
	if err := e.L.DoString(`_p:setTown(Town(8))`); err != nil {
		t.Fatalf("setTown: %v", err)
	}
	if p.TownID != 8 {
		t.Errorf("TownID = %d, want 8", p.TownID)
	}
	if p.LoginPosition != (game.Position{X: 5556, Y: 5098, Z: 7}) {
		t.Errorf("LoginPosition = %+v, want temple", p.LoginPosition)
	}
}
