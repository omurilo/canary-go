package luaengine

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// CreatureEvent:onDeath was a no-op, so death.lua registered its handler into
// nothing and player_deaths was never written. The signature matters: death.lua
// reads all six arguments to build the row, and a short call would record the wrong
// killer.
func TestCreatureEventOnDeathDispatch(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		calls = 0
		gotPlayer, gotKiller, gotMost, gotUnjust, gotMostUnjust = nil, nil, nil, nil, nil
		local ev = CreatureEvent("PlayerDeath")
		ev:type("death")
		function ev.onDeath(player, corpse, killer, mostDamageKiller, unjustified, mostDamageUnjustified)
			calls = calls + 1
			gotPlayer = player:getName()
			gotKiller = killer and killer:getName() or nil
			gotMost = mostDamageKiller and mostDamageKiller:getName() or nil
			gotUnjust = unjustified
			gotMostUnjust = mostDamageUnjustified
			return true
		end
		ev:register()
	`); err != nil {
		t.Fatalf("register: %v", err)
	}

	victim := &game.Player{Name: "Victim"}
	killer := game.NewMonster(9, "Rat", nil)

	if !e.ExecuteCreatureOnDeath(victim, nil, killer, killer, true, false) {
		t.Errorf("a handler returning true must not veto")
	}
	if n := e.L.GetGlobal("calls"); n != lua.LNumber(1) {
		t.Fatalf("handler ran %v times, want 1", n)
	}
	if got := e.L.GetGlobal("gotPlayer"); got != lua.LString("Victim") {
		t.Errorf("player argument = %v, want Victim", got)
	}
	if got := e.L.GetGlobal("gotKiller"); got != lua.LString("Rat") {
		t.Errorf("killer argument = %v, want Rat", got)
	}
	if got := e.L.GetGlobal("gotMost"); got != lua.LString("Rat") {
		t.Errorf("mostDamageKiller argument = %v, want Rat", got)
	}
	if got := e.L.GetGlobal("gotUnjust"); got != lua.LTrue {
		t.Errorf("unjustified argument = %v, want true", got)
	}
	if got := e.L.GetGlobal("gotMostUnjust"); got != lua.LFalse {
		t.Errorf("mostDamageUnjustified argument = %v, want false", got)
	}

	// An environmental death has no killer at all: the script guards on nil, so nil
	// must arrive as nil rather than as a userdata wrapping a nil creature.
	if err := e.L.DoString(`calls = 0`); err != nil {
		t.Fatal(err)
	}
	e.ExecuteCreatureOnDeath(victim, nil, nil, nil, false, false)
	if n := e.L.GetGlobal("calls"); n != lua.LNumber(1) {
		t.Errorf("handler ran %v times for a killerless death, want 1", n)
	}
	if got := e.L.GetGlobal("gotKiller"); got != lua.LNil {
		t.Errorf("killer for a drowning = %v, want nil", got)
	}
}

// A handler returning false vetoes, like the other creature events.
func TestCreatureEventOnDeathVeto(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	if err := e.L.DoString(`
		local ev = CreatureEvent("Veto")
		function ev.onDeath() return false end
		ev:register()
	`); err != nil {
		t.Fatal(err)
	}
	if e.ExecuteCreatureOnDeath(&game.Player{Name: "V"}, nil, nil, nil, false, false) {
		t.Errorf("a handler returning false must veto")
	}
}
