package luaengine

import (
	"log/slog"
	"os"
	"testing"

	"github.com/opentibiabr/canary-go/internal/bosstiary"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

func TestBosstiaryKillBindings(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	// Archfoe: stages {5->10},{20->30},{60->60}.
	w.TypeRegistry.Monsters["bibby bloodbath"] = &creatures.MonsterType{
		Name: "Bibby Bloodbath", BosstiaryRaceID: 900, BosstiaryRace: bosstiary.RarityArchfoe,
	}
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	p := &game.Player{Name: "Hunter"}
	w.AddPlayer(p, &recordSession{p: p})

	e.mu.Lock()
	up := e.L.NewUserData()
	up.Value = p
	e.L.SetMetatable(up, e.L.GetTypeMetatable("Player"))
	e.L.SetGlobal("_p", up)
	e.mu.Unlock()

	run := func(src string) {
		if err := e.DoString(src); err != nil {
			t.Fatalf("%s: %v", src, err)
		}
	}
	// 4 kills -> level 0 (needs 5); 0 points.
	run(`_p:addBosstiaryKill("Bibby Bloodbath", 4)`)
	if p.GetBossPoints() != 0 {
		t.Fatalf("after 4 kills bossPoints=%d want 0", p.GetBossPoints())
	}
	// 5th kill -> level 1 (Prowess), award 10 points.
	run(`_p:addBosstiaryKill("Bibby Bloodbath")`)
	if p.GetBossPoints() != 10 {
		t.Fatalf("after 5 kills bossPoints=%d want 10", p.GetBossPoints())
	}
	// verify getters via Lua
	run(`
		if _p:getBosstiaryKills("Bibby Bloodbath") ~= 5 then error("kills != 5") end
		if _p:getBosstiaryLevel("Bibby Bloodbath") ~= 1 then error("level != 1") end
		if _p:getBossPoints() ~= 10 then error("points != 10") end
	`)
	// jump to 20 kills -> level 2, award 30 more (total 40).
	run(`_p:addBosstiaryKill("Bibby Bloodbath", 15)`)
	if p.GetBossPoints() != 40 {
		t.Fatalf("after 20 kills bossPoints=%d want 40", p.GetBossPoints())
	}
	_ = lua.LNil
}
