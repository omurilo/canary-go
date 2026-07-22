package luaengine

import (
	"encoding/xml"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/spawns"
)

// monsterDataDir locates the otservbr monster data relative to the repo, or ""
// if it is not present (so the test can be skipped in minimal checkouts).
func monsterDataDir() string {
	// internal/luaengine -> repo root is ../.. ; data pack is a sibling of the
	// canary-go module.
	candidates := []string{
		filepath.Join("..", "..", "..", "data-otservbr-global", "monster"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

func newTestEngine() *Engine {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(w, log)
}

// TestLoadRatFromLua loads the real rat.lua through the engine and asserts the
// full monster-data surface (health, exp, corpse, speed, race, attacks, loot,
// flags) parses faithfully.
func TestLoadRatFromLua(t *testing.T) {
	dir := monsterDataDir()
	if dir == "" {
		t.Skip("otservbr monster data not available")
	}
	rat := filepath.Join(dir, "mammals", "rat.lua")
	if _, err := os.Stat(rat); err != nil {
		t.Skipf("rat.lua not found: %v", err)
	}

	e := newTestEngine()
	defer e.Close()

	if err := e.DoFile(rat); err != nil {
		t.Fatalf("loading rat.lua: %v", err)
	}

	mt := e.world.TypeRegistry.Monsters["rat"]
	if mt == nil {
		t.Fatal("rat not registered")
	}
	if mt.MaxHealth != 20 {
		t.Errorf("MaxHealth = %d, want 20", mt.MaxHealth)
	}
	if mt.Experience != 5 {
		t.Errorf("Experience = %d, want 5", mt.Experience)
	}
	if mt.Corpse != 5964 {
		t.Errorf("Corpse = %d, want 5964", mt.Corpse)
	}
	if mt.Speed != 67 {
		t.Errorf("Speed = %d, want 67", mt.Speed)
	}
	if mt.RaceID != 21 {
		t.Errorf("RaceID = %d, want 21", mt.RaceID)
	}

	// Attacks: one melee, interval 2000, chance 100, 0..-8.
	if len(mt.Attacks) != 1 {
		t.Fatalf("Attacks len = %d, want 1", len(mt.Attacks))
	}
	atk := mt.Attacks[0]
	if !atk.IsMelee() || atk.Interval != 2000 || atk.Chance != 100 || atk.MinDamage != 0 || atk.MaxDamage != -8 {
		t.Errorf("melee attack = %+v, want melee/2000/100/0/-8", atk)
	}

	// Loot: gold coin (by name, maxCount 4) + cheese id 3607.
	if len(mt.Loot) != 2 {
		t.Fatalf("Loot len = %d, want 2", len(mt.Loot))
	}
	if mt.Loot[0].Name != "gold coin" || mt.Loot[0].Chance != 100000 || mt.Loot[0].CountMax != 4 {
		t.Errorf("loot[0] = %+v, want gold coin/100000/4", mt.Loot[0])
	}
	if mt.Loot[1].ID != 3607 || mt.Loot[1].Chance != 39410 {
		t.Errorf("loot[1] = %+v, want id 3607/39410", mt.Loot[1])
	}

	// Flags.
	if !mt.Flags.Hostile || !mt.Flags.Attackable || !mt.Flags.LootDrop {
		t.Errorf("flags = %+v, want hostile/attackable/lootDrop", mt.Flags)
	}
	if mt.Flags.StaticAttackChance != 90 || mt.Flags.RunHealth != 5 {
		t.Errorf("staticAttackChance=%d runHealth=%d, want 90/5", mt.Flags.StaticAttackChance, mt.Flags.RunHealth)
	}
}

// TestLoadAllMonsters loads the entire monster tree and reports how many types
// register, guarding against a regression that drops the count to zero.
func TestLoadAllMonsters(t *testing.T) {
	dir := monsterDataDir()
	if dir == "" {
		t.Skip("otservbr monster data not available")
	}

	e := newTestEngine()
	defer e.Close()

	// Load lib first, mimicking main.go loadScripts
	libDir := filepath.Join(dir, "lib")
	_ = filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".lua" {
			return nil
		}
		if err := e.DoFile(path); err != nil {
			t.Logf("lib script error: %s: %v", path, err)
		}
		return nil
	})

	var files, errs int
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".lua" {
			return nil
		}
		// Skip lib since we loaded it first
		if strings.Contains(path, string(filepath.Separator)+"lib"+string(filepath.Separator)) {
			return nil
		}
		files++
		if err := e.DoFile(path); err != nil {
			errs++
			t.Logf("script error: %s: %v", path, err)
		}
		return nil
	})

	count := len(e.world.TypeRegistry.Monsters)
	t.Logf("monster lua files=%d load-errors=%d registered types=%d", files, errs, count)
	if count == 0 {
		t.Fatal("no monster types registered from Lua")
	}
}

// TestDiagnosticRealSpawnsMatching loads the real monsters and the real monster spawns,
// checking if all spawned monster names exist in the registry.
func TestDiagnosticRealSpawnsMatching(t *testing.T) {
	monsterDir := "../../../data-otservbr-global/monster"
	spawnsFile := "../../../data-otservbr-global/world/otservbr-monster.xml"

	if _, err := os.Stat(monsterDir); err != nil {
		t.Skip("otservbr monster data not available")
	}
	if _, err := os.Stat(spawnsFile); err != nil {
		t.Skip("otservbr spawns file not available")
	}

	// Create real Lua engine and register types
	w := game.NewWorld()
	reg := creatures.NewTypeRegistry()
	w.TypeRegistry = reg
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := New(w, logger)
	defer e.Close()

	// Load lib first, mimicking main.go loadScripts
	libDir := filepath.Join(monsterDir, "lib")
	_ = filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".lua" {
			return nil
		}
		_ = e.DoFile(path)
		return nil
	})

	// Load monster files
	_ = filepath.WalkDir(monsterDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".lua" {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"lib"+string(filepath.Separator)) {
			return nil
		}
		_ = e.DoFile(path)
		return nil
	})

	t.Logf("Total monster types loaded: %d", len(reg.Monsters))

	// Load spawns
	data, err := os.ReadFile(spawnsFile)
	if err != nil {
		t.Fatalf("failed to read spawns file: %v", err)
	}

	var spawnsData spawns.SpawnsData
	if err := xml.Unmarshal(data, &spawnsData); err != nil {
		t.Fatalf("failed to parse spawns XML: %v", err)
	}

	matched := 0
	mismatched := 0
	mismatchedNames := make(map[string]int)

	allNodes := append(spawnsData.Spawns, spawnsData.Monsters...)
	allNodes = append(allNodes, spawnsData.NPCs...)
	for _, sn := range allNodes {
		for _, mn := range sn.Monsters {
			key := strings.ToLower(mn.Name)
			if _, ok := reg.Monsters[key]; ok {
				matched++
			} else {
				mismatched++
				mismatchedNames[mn.Name]++
			}
		}
	}

	t.Logf("Matched spawns: %d, Mismatched spawns: %d", matched, mismatched)
	t.Logf("Distinct mismatched names: %d", len(mismatchedNames))

	if mismatched > 0 {
		t.Logf("Top mismatched monster names:")
		count := 0
		for name, occurrences := range mismatchedNames {
			t.Logf("  - %q (%d occurrences)", name, occurrences)
			count++
			if count >= 30 {
				break
			}
		}
	}
}


