package luaengine

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/events"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
	lua "github.com/yuin/gopher-lua"
)

// Drives the real loot chain: the datapack's monsterOnDropLoot base script calls
// MonsterType:generateLootRoll and Container:addLoot, both of which are Lua in
// data/libs. The Go side only has to expose mType:getLoot and isRewardBoss — which
// were missing, so the event could not have produced anything.
func TestMonsterOnDropLootFillsCorpse(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	core := filepath.Join(repo, "data")
	datapack := filepath.Join(repo, "data-otservbr-global")
	base := filepath.Join(core, "scripts", "eventcallbacks", "monster", "ondroploot__base.lua")
	if _, err := os.Stat(base); err != nil {
		t.Skip("datapack not available")
	}

	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	w.Items = items.NewCatalog(
		&items.ItemType{ID: 3031, Name: "gold coin", Stackable: true, StackSize: 100},
		&items.ItemType{ID: 3492, Name: "worm"},
		&items.ItemType{ID: 5964, Name: "rat corpse", Group: items.GroupContainer, Capacity: 20},
	)

	// generateLootRoll bails out immediately when RATE_LOOT <= 0, so a config has to
	// be active for the chain to produce anything.
	//
	// A minimal config is built here rather than loading the real config.lua on
	// purpose: that file enables luaScriptBytecodeCache, whose path is relative, so
	// DoFile would write a cache tree under the test's working directory
	// (internal/luaengine/data/...). That polluted checkout then broke
	// TestLoginAndSystemScriptsSmoke, which reads the datapack from disk.
	prev := config.Active
	t.Cleanup(func() { config.Active = prev })
	config.Active = &config.Config{Custom: map[string]lua.LValue{
		"rateloot": lua.LNumber(1),
	}}

	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	defer e.Close()
	e.L.SetGlobal("DATA_DIRECTORY", lua.LString(datapack))
	e.L.SetGlobal("CORE_DIRECTORY", lua.LString(core))

	eventEngine := events.NewEngine(e.L, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	eventEngine.WrapContainer = e.ContainerValue

	// The base script needs data/libs (generateLootRoll, Container:addLoot).
	// global.lua defines SCHEDULE_LOOT_RATE, which getLootRandom multiplies by.
	if err := e.DoFile(filepath.Join(core, "global.lua")); err != nil {
		t.Fatalf("load global.lua: %v", err)
	}
	walkLoad(t, e, filepath.Join(datapack, "lib"))
	for _, sub := range []string{"lib", "libs"} {
		walkLoad(t, e, filepath.Join(core, sub))
	}
	if err := e.DoFile(base); err != nil {
		t.Fatalf("load base loot script: %v", err)
	}

	// A monster whose entire loot table is guaranteed: chance is out of
	// MAX_LOOTCHANCE (100000), so 100000 always drops.
	mType := &creatures.MonsterType{
		Name:   "Test Rat",
		Corpse: 5964,
		Loot: []creatures.LootBlock{
			{ID: 3031, Chance: 100000, CountMin: 5, CountMax: 5},
			{ID: 3492, Chance: 100000, CountMin: 1, CountMax: 1},
		},
	}
	mType.Flags.LootDrop = true
	w.TypeRegistry.Monsters["test rat"] = mType

	monster := game.NewMonster(1, "Test Rat", mType)

	// The roll is probabilistic even at chance 100000: getLootRandom applies a
	// 95-105% jitter, so a single dispatch can legitimately drop nothing. Repeat
	// until every expected item has appeared, which fails loudly when the chain is
	// broken instead of flaking on the boundary.
	seen := map[uint16]uint32{}
	var dispatches int
	for attempt := 0; attempt < 40 && (seen[3031] == 0 || seen[3492] == 0); attempt++ {
		corpse := &game.Item{ID: 5964, Count: 1}
		if !eventEngine.ExecuteMonsterOnDropLoot(monster, corpse) {
			t.Fatal("the drop loot callback reported failure")
		}
		dispatches++
		for _, it := range corpse.Contents {
			if it == nil {
				continue
			}
			c := uint32(it.Count)
			if c == 0 {
				c = 1
			}
			seen[it.ID] += c
			if it.ID != 3031 && it.ID != 3492 {
				t.Fatalf("corpse got an item that is not in the loot table: %d", it.ID)
			}
		}
	}

	if seen[3031] == 0 {
		t.Errorf("gold never dropped in %d dispatches: the loot chain is not producing", dispatches)
	}
	if seen[3492] == 0 {
		t.Errorf("worms never dropped in %d dispatches", dispatches)
	}
	// Gold is declared with minCount/maxCount 5, and generateLootRoll only applies
	// that range when iType:isStackable() is true — which needs ItemType(id) to
	// actually resolve the id. It used to resolve to 0, so gold arrived as a single
	// item; each drop must now be a stack of 5.
	if seen[3031]%5 != 0 {
		t.Errorf("gold arrived in a count that is not a multiple of 5: %d", seen[3031])
	}
	t.Logf("dispatches=%d gold=%d worms=%d", dispatches, seen[3031], seen[3492])
}

// getLoot must hand Lua the key names generateLootRoll reads. A rename here is
// silent: the roll would simply see nil and drop nothing.
func TestGetLootTableShape(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	defer e.Close()

	mType := &creatures.MonsterType{Name: "Shape Test", Loot: []creatures.LootBlock{
		{ID: 3031, Chance: 50000, CountMin: 2, CountMax: 9, SubType: 3, Unique: true,
			ChildLoot: []creatures.LootBlock{{ID: 3492, Chance: 1000}}},
	}}
	mType.Flags.RewardBoss = true
	w.TypeRegistry.Monsters["shape test"] = mType

	setGlobalMonsterType(e, "mt", mType)
	script := `
		local loot = mt:getLoot()
		assert(#loot == 1, "expected one entry")
		local entry = loot[1]
		assert(entry.itemId == 3031, "itemId")
		assert(entry.chance == 50000, "chance")
		assert(entry.minCount == 2, "minCount")
		assert(entry.maxCount == 9, "maxCount")
		assert(entry.unique == true, "unique must round-trip from the loot block")
		assert(entry.subType == 3, "subType")
		assert(entry.actionId == -1, "actionId must default to -1, not nil")
		assert(entry.childLoot ~= nil and #entry.childLoot == 1, "childLoot")
		assert(entry.childLoot[1].itemId == 3492, "child itemId")
		assert(mt:isRewardBoss() == true, "isRewardBoss")
	`
	if err := e.L.DoString(script); err != nil {
		t.Fatalf("loot table shape: %v", err)
	}
}

// A count of 0 must be reported as 1, since a loot entry with no explicit count
// still drops one item.
func TestGetLootDefaultsCountToOne(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	defer e.Close()

	mType := &creatures.MonsterType{Name: "Zero", Loot: []creatures.LootBlock{{ID: 3031, Chance: 100}}}
	setGlobalMonsterType(e, "mt", mType)
	if err := e.L.DoString(`
		local entry = mt:getLoot()[1]
		assert(entry.minCount == 1, "minCount should default to 1, got " .. tostring(entry.minCount))
		assert(entry.maxCount == 1, "maxCount should default to 1, got " .. tostring(entry.maxCount))
		assert(entry.subType == -1, "subType should default to -1, got " .. tostring(entry.subType))
		assert(entry.actionId == -1, "actionId should default to -1, got " .. tostring(entry.actionId))
	`); err != nil {
		t.Fatalf("count defaults: %v", err)
	}
}

// setGlobalMonsterType exposes a MonsterType to Lua under the given global name.
func setGlobalMonsterType(e *Engine, name string, m *creatures.MonsterType) {
	pushMonsterType(e.L, m)
	e.L.SetGlobal(name, e.L.Get(-1))
	e.L.Pop(1)
}
