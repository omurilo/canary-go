package luaengine

import (
	"io"
	"log/slog"
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
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
	victim.RegisterEvent("PlayerDeath")
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
	victim := &game.Player{Name: "V"}
	victim.RegisterEvent("Veto")
	if e.ExecuteCreatureOnDeath(victim, nil, nil, nil, false, false) {
		t.Errorf("a handler returning false must veto")
	}
}

// The per-creature model is the fix for the Dawnport bosses: a handler bound to
// one monster's `monster.events` must not fire on a player death, and a
// player-only handler (login.lua's player:registerEvent) must not fire on a
// monster death. Before the fix every onDeath ran for every player death at the
// temple position.
func TestCreatureEventOnDeathPerCreature(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		playerCalls = 0
		monsterCalls = 0
		local pev = CreatureEvent("PlayerOnly")
		function pev.onDeath(creature)
			playerCalls = playerCalls + 1
		end
		pev:register()

		local mev = CreatureEvent("MonsterOnly")
		function mev.onDeath(creature)
			monsterCalls = monsterCalls + 1
		end
		mev:register()
	`); err != nil {
		t.Fatalf("register: %v", err)
	}

	// A player holding only the player event: the monster handler stays quiet.
	player := &game.Player{Name: "P"}
	player.RegisterEvent("PlayerOnly")
	e.ExecuteCreatureOnDeath(player, nil, nil, nil, false, false)
	if n := e.L.GetGlobal("playerCalls"); n != lua.LNumber(1) {
		t.Errorf("player-only handler ran %v times on player death, want 1", n)
	}
	if n := e.L.GetGlobal("monsterCalls"); n != lua.LNumber(0) {
		t.Errorf("monster-only handler ran %v times on player death, want 0", n)
	}

	// Reset, then a monster death holding only the monster event.
	if err := e.L.DoString(`playerCalls = 0; monsterCalls = 0`); err != nil {
		t.Fatal(err)
	}
	monster := game.NewMonster(9, "Rat", nil)
	monster.RegisterEvent("MonsterOnly")
	e.ExecuteMonsterOnDeath(monster, nil, nil, nil, false, false)
	if n := e.L.GetGlobal("monsterCalls"); n != lua.LNumber(1) {
		t.Errorf("monster-only handler ran %v times on monster death, want 1", n)
	}
	if n := e.L.GetGlobal("playerCalls"); n != lua.LNumber(0) {
		t.Errorf("player-only handler ran %v times on monster death, want 0", n)
	}
}

// The boss the handler spawns must appear at the monster's death tile, not at a
// temple: the monster is not relocated on death, so creature:getPosition() in
// the onDeath handler resolves to where the monster actually died.
func TestMonsterDeathSpawnsBossAtDeathTile(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := New(w, log)
	defer e.Close()

	pos := game.Position{X: 100, Y: 100, Z: 7}
	w.Map.SetTile(pos, &game.Tile{Ground: &game.Item{ID: 1}})
	w.TypeRegistry.Monsters["baron from below"] = &creatures.MonsterType{Name: "Baron From Below", MaxHealth: 500}

	if err := e.L.DoString(`
		spawnedName = nil
		local ev = CreatureEvent("BaronBoss")
		function ev.onDeath(creature)
			local boss = Game.createMonster("Baron From Below", creature:getPosition(), true, true)
			spawnedName = boss and boss:getName() or nil
		end
		ev:register()
	`); err != nil {
		t.Fatalf("register: %v", err)
	}

	victim := game.NewMonster(w.GenerateCreatureID(), "Astral Glyph", nil)
	victim.SetPosition(pos)
	victim.RegisterEvent("BaronBoss")
	e.ExecuteMonsterOnDeath(victim, nil, nil, nil, false, false)

	if got := e.L.GetGlobal("spawnedName"); got != lua.LString("Baron From Below") {
		t.Fatalf("spawned boss = %v, want Baron From Below", got)
	}
	boss := w.CreatureByName("Baron From Below")
	if boss == nil {
		t.Fatal("boss not registered in the world")
	}
	if boss.GetPosition() != pos {
		t.Errorf("boss spawned at %v, want death tile %v", boss.GetPosition(), pos)
	}
}
