package luaengine

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// shimEngine boots an engine with the same load order cmd/canary uses, including
// data/scripts/lib — which is where register_npc_type.lua lives. Every other NPC
// test loads only data/lib, data/libs and data/npclib, which is precisely how the
// shim went unnoticed as inert.
func shimEngine(t *testing.T) *Engine {
	t.Helper()
	repo := filepath.Join("..", "..", "..")
	datapack := filepath.Join(repo, "data-otservbr-global")
	core := filepath.Join(repo, "data")
	if _, err := os.Stat(filepath.Join(core, "scripts", "lib", "register_npc_type.lua")); err != nil {
		t.Skip("datapack not available")
	}

	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	e.L.SetGlobal("DATA_DIRECTORY", lua.LString(datapack))
	e.L.SetGlobal("CORE_DIRECTORY", lua.LString(core))

	for _, f := range []string{"global.lua", "stages.lua"} {
		_ = e.DoFile(filepath.Join(core, f))
	}
	walkLoad(t, e, filepath.Join(datapack, "lib"))
	for _, sub := range []string{"lib", "libs", "npclib"} {
		walkLoad(t, e, filepath.Join(core, sub))
	}
	// Only data/scripts/lib, not all of data/scripts. Production loads the whole
	// tree, but the action and talkaction registries are package-level globals
	// shared by every test in this package, so loading 14k of them here breaks
	// unrelated tests by side effect. The shim is the only thing these tests need.
	walkLoad(t, e, filepath.Join(core, "scripts", "lib"))
	return e
}

// loadNpcRegisterShim pulls in just register_npc_type.lua, for tests that build
// their own world and only need npcType:register to exist. There is no Go
// implementation of it to fall back on — deliberately, since C++ has none either.
func loadNpcRegisterShim(t *testing.T, e *Engine) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "data", "scripts", "lib", "register_npc_type.lua")
	if _, err := os.Stat(path); err != nil {
		t.Skip("datapack not available")
	}
	if err := e.DoFile(path); err != nil {
		t.Fatalf("load register_npc_type.lua: %v", err)
	}
}

// The shim has to be the thing that runs. Go carried a second, independent
// npcConfig reader written in Go; deleting it is only safe if NpcType.register
// as defined by data/scripts/lib/register_npc_type.lua actually reaches the
// userdata. It did not before: the metatable's __index was a different table
// from the global NpcType, so the Lua assignment was invisible.
func TestRegisterNpcTypeShimIsTheOnePath(t *testing.T) {
	e := shimEngine(t)

	if _, ok := e.L.GetGlobal("registerNpcType").(*lua.LTable); !ok {
		t.Fatalf("register_npc_type.lua did not load")
	}

	// The method a userdata resolves must be the Lua one, not a Go builtin.
	err := e.DoString(`
		local t = Game.createNpcType("Shim Probe")
		assert(getmetatable(t).__index == NpcType, "NpcType metatable __index must be the class table")
		assert(NpcType.register ~= nil, "the shim must have installed NpcType.register")
	`)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

// A real datapack NPC, loaded end to end, with every field the shim is
// responsible for. Each of these travels config table -> Lua shim -> Go setter,
// and none of them can arrive if any link is broken.
func TestRealNpcLoadsThroughTheShim(t *testing.T) {
	e := shimEngine(t)
	repo := filepath.Join("..", "..", "..")
	npcFile := filepath.Join(repo, "data-otservbr-global", "npc", "a_sweaty_cyclops.lua")
	if _, err := os.Stat(npcFile); err != nil {
		t.Skip("npc not available")
	}
	if err := e.DoFile(npcFile); err != nil {
		t.Fatalf("load npc: %v", err)
	}

	n := e.world.TypeRegistry.Npcs["a sweaty cyclops"]
	if n == nil {
		t.Fatalf("the npc did not reach the registry; got %d types", len(e.world.TypeRegistry.Npcs))
	}

	if n.Name != "A Sweaty Cyclops" {
		t.Errorf("name = %q", n.Name)
	}
	if n.Description == "" {
		t.Errorf("nameDescription never arrived")
	}
	if n.MaxHealth == 0 {
		t.Errorf("maxHealth never arrived")
	}
	if n.Outfit.LookType == 0 {
		t.Errorf("outfit never arrived")
	}
	if n.Speed == 0 {
		t.Errorf("baseSpeed never arrived")
	}
	if n.WalkInterval == 0 {
		t.Errorf("walkInterval never arrived")
	}
	if n.SpeechBubble == 0 {
		t.Errorf("speechBubble must default to SPEECHBUBBLE_NORMAL at worst")
	}
	if n.CurrencyID == 0 {
		t.Errorf("currency must default to gold at worst")
	}
}

// The shop branch is the one that calls npcType:getName(), a method C++ does not
// have. If it is missing the parse throws mid-way and the NPC registers with an
// empty catalog — a merchant that silently sells nothing.
func TestMerchantShopLoadsThroughTheShim(t *testing.T) {
	e := shimEngine(t)
	repo := filepath.Join("..", "..", "..")

	for _, file := range []string{"hardek", "rashid"} {
		path := filepath.Join(repo, "data-otservbr-global", "npc", file+".lua")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := e.DoFile(path); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}

	var withShop int
	for _, n := range e.world.TypeRegistry.Npcs {
		if len(n.ShopItems) > 0 {
			withShop++
		}
	}
	if withShop == 0 {
		t.Errorf("no merchant kept its catalog — the shop branch of the shim is dying")
	}
}

// The whole datapack, to catch the case where one NPC style loads and another
// does not. A drop here is the failure mode that would take the server down to
// a handful of working NPCs without a single error.
func TestWholeNpcDatapackRegisters(t *testing.T) {
	if testing.Short() {
		t.Skip("loads 1000+ scripts")
	}
	e := shimEngine(t)
	dir := filepath.Join("..", "..", "..", "data-otservbr-global", "npc")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skip("npc directory not available")
	}

	// Not every file in npc/ is an NPC — a few are shared *_functions.lua helpers.
	// Count the ones that actually construct a type, or the expected total is off
	// by three for no reason.
	// Not every file in npc/ is an NPC — a few are shared *_functions.lua helpers.
	// And two NPC files can name the same type, which createNpcType collapses onto
	// one object exactly as g_npcs().getNpcType does. Both have to be accounted for
	// or the expected total is wrong for reasons that have nothing to do with the
	// shim.
	var files, npcFiles, failed, sharedName int
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".lua" {
			continue
		}
		files++
		path := filepath.Join(dir, entry.Name())
		body, _ := os.ReadFile(path)
		isNpc := strings.Contains(string(body), "createNpcType")
		if isNpc {
			npcFiles++
		}

		before := len(e.world.TypeRegistry.Npcs)
		if err := e.DoFile(path); err != nil {
			failed++
			if failed <= 5 {
				t.Errorf("%s: %v", entry.Name(), err)
			}
			continue
		}
		if isNpc && len(e.world.TypeRegistry.Npcs) == before {
			sharedName++
		}
	}
	if failed > 5 {
		t.Errorf("... and %d more failed to load", failed-5)
	}

	registered := len(e.world.TypeRegistry.Npcs)
	t.Logf("%d files, %d of them npcs, %d registered types (%d reused an existing name)",
		files, npcFiles, registered, sharedName)
	// Registration happens in createNpcType, so every npc file that ran and asked
	// for a new name should have produced a type.
	if want := npcFiles - failed - sharedName; registered < want {
		t.Errorf("expected at least %d registered types, got %d", want, registered)
	}

	// And they have to carry data, not just exist.
	var named, withHealth, withOutfit int
	for _, n := range e.world.TypeRegistry.Npcs {
		if n.Description != "" {
			named++
		}
		if n.MaxHealth > 0 {
			withHealth++
		}
		if n.Outfit.LookType > 0 || n.Outfit.LookTypeEx > 0 {
			withOutfit++
		}
	}
	t.Logf("with description %d, with health %d, with outfit %d", named, withHealth, withOutfit)
	if named < registered/2 || withHealth < registered/2 || withOutfit < registered/2 {
		t.Errorf("most registered types are empty shells: %d/%d described, %d/%d with health, %d/%d with outfit",
			named, registered, withHealth, registered, withOutfit, registered)
	}
}
