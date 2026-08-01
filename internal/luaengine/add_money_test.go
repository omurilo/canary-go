package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
)

func TestAddMoneyTalkactionFlow(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	p := &game.Player{
		ID:          1,
		Name:        "Admin",
		BankBalance: 0,
	}
	e.world.AddPlayer(p, nil)

	// Test Game.getNormalizedPlayerName
	err := e.L.DoString(`
		local norm = Game.getNormalizedPlayerName("admin")
		assert(norm == "Admin", "expected Admin, got " .. tostring(norm))
	`)
	if err != nil {
		t.Fatalf("Game.getNormalizedPlayerName failed: %v", err)
	}

	// Test Bank.credit with string name
	err = e.L.DoString(`
		local ok = Bank.credit("admin", 100000)
		assert(ok == true, "expected Bank.credit to return true")
	`)
	if err != nil {
		t.Fatalf("Bank.credit failed: %v", err)
	}

	if p.BankBalance != 100000 {
		t.Errorf("expected BankBalance 100000, got %d", p.BankBalance)
	}
}
