package luaengine

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
)

func TestPlayerConstructorByName(t *testing.T) {
	w := game.NewWorld()
	e := New(w, nil)

	p := &game.Player{ID: 100, Name: "Target Player"}
	w.AddPlayer(p, nil)

	L := e.L
	err := L.DoString(`
		local p = Player("Target Player")
		if not p then
			error("expected Player('Target Player') to return player userdata, got nil")
		end
		if p:getName() ~= "Target Player" then
			error("expected player name 'Target Player', got " .. tostring(p:getName()))
		end

		local missing = Player("NonExistentPlayer")
		if missing ~= nil then
			error("expected Player('NonExistentPlayer') to return nil, got " .. tostring(missing))
		end
	`)
	if err != nil {
		t.Fatalf("Lua execution error: %v", err)
	}
}
