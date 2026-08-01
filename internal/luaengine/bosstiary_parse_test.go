package luaengine

import (
	"log/slog"
	"os"
	"testing"

	"github.com/omurilo/canary-go/internal/bosstiary"
	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
)

// TestMonsterBosstiaryParse verifies monster.bosstiary parses bossRace via the
// RARITY_* enum, so an Archfoe boss with 6 kills is unlocked (level >= 1). If
// the enum were missing at parse time, bossRace would fall back to Bane (thr 25)
// and 6 kills would read as level 0 -> slot never unlocks.
func TestMonsterBosstiaryParse(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err := e.DoString(`
		local m = Game.createMonsterType("Test Archfoe")
		m:register({ bosstiary = { bossRaceId = 900, bossRace = RARITY_ARCHFOE } })
	`); err != nil {
		t.Fatalf("register: %v", err)
	}
	mt := w.TypeRegistry.Monsters["test archfoe"]
	if mt == nil {
		t.Fatal("monster not registered")
	}
	if mt.BosstiaryRaceID != 900 {
		t.Fatalf("BosstiaryRaceID = %d, want 900", mt.BosstiaryRaceID)
	}
	if mt.BosstiaryRace != bosstiary.RarityArchfoe {
		t.Fatalf("BosstiaryRace = %d, want Archfoe(%d) — RARITY_ARCHFOE enum missing at parse?", mt.BosstiaryRace, bosstiary.RarityArchfoe)
	}
	if !mt.IsBoss() {
		t.Fatal("IsBoss() false")
	}
	if lvl := bosstiary.Level(mt.BosstiaryRace, 6); lvl < 1 {
		t.Fatalf("Level(Archfoe, 6 kills) = %d, want >= 1 (slot should unlock)", lvl)
	}
}
