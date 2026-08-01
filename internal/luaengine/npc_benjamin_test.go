package luaengine

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// errCaptureHandler records ERROR-level log messages so tests can assert that
// timer-fired callbacks (the delayed NPC say scheduled via addEvent) don't
// raise Lua errors — those run after the synchronous onSay returns and would
// otherwise only be logged and silently missed.
type errCaptureHandler struct {
	mu   sync.Mutex
	errs *[]string
}

func (h *errCaptureHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= slog.LevelError
}
func (h *errCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	msg := r.Message
	r.Attrs(func(a slog.Attr) bool {
		msg += " " + a.Key + "=" + a.Value.String()
		return true
	})
	h.mu.Lock()
	*h.errs = append(*h.errs, msg)
	h.mu.Unlock()
	return nil
}
func (h *errCaptureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *errCaptureHandler) WithGroup(string) slog.Handler      { return h }

// walkLoad mimics cmd/canary loadScripts: lib/ first, then every other .lua.
func walkLoad(t *testing.T, e *Engine, dir string) {
	t.Helper()
	libDir := filepath.Join(dir, "lib")
	filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && filepath.Ext(path) == ".lua" {
			_ = e.DoFile(path)
		}
		return nil
	})
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "lib" && path == libDir {
			return filepath.SkipDir
		}
		if !d.IsDir() && filepath.Ext(path) == ".lua" {
			_ = e.DoFile(path)
		}
		return nil
	})
}

// TestBenjaminOnSay reproduces the runtime "attempt to call a non-function
// object" the user hit when talking to an NPC, to pin down the exact file:line
// (chunks are now named after their path).
func TestBenjaminOnSay(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	datapack := filepath.Join(repo, "data-otservbr-global")
	core := filepath.Join(repo, "data")
	benjamin := filepath.Join(datapack, "npc", "benjamin.lua")
	if _, err := os.Stat(benjamin); err != nil {
		t.Skip("datapack not available")
	}

	e := newTestEngine()
	e.L.SetGlobal("DATA_DIRECTORY", lua.LString(datapack))
	e.L.SetGlobal("CORE_DIRECTORY", lua.LString(core))

	walkLoad(t, e, filepath.Join(datapack, "lib"))
	for _, sub := range []string{"lib", "libs", "npclib"} {
		walkLoad(t, e, filepath.Join(core, sub))
	}
	// data/scripts/lib/register_npc_type.lua is what turns an npcConfig table into
	// NpcType setter calls — there is no Go implementation of it, exactly as there
	// is no C++ one. Leaving it out here is what let it sit inert unnoticed.
	walkLoad(t, e, filepath.Join(core, "scripts", "lib"))
	if err := e.DoFile(benjamin); err != nil {
		t.Fatalf("load benjamin: %v", err)
	}

	greetNpc(t, e, "Benjamin", "benjamin")
}

// TestNpcGreetSmoke greets a spread of real NPCs with "hi" and asserts none of
// their onSay handlers raise — a regression guard for the Creature/Player method
// surface, the addEvent/stopEvent scheduler and NetworkMessage that NPC dialogue
// depends on.
func TestNpcGreetSmoke(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	datapack := filepath.Join(repo, "data-otservbr-global")
	core := filepath.Join(repo, "data")
	if _, err := os.Stat(filepath.Join(datapack, "npc")); err != nil {
		t.Skip("datapack not available")
	}

	var capturedErrs []string
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	e := New(w, slog.New(&errCaptureHandler{errs: &capturedErrs}))
	e.L.SetGlobal("DATA_DIRECTORY", lua.LString(datapack))
	e.L.SetGlobal("CORE_DIRECTORY", lua.LString(core))
	walkLoad(t, e, filepath.Join(datapack, "lib"))
	for _, sub := range []string{"lib", "libs", "npclib"} {
		walkLoad(t, e, filepath.Join(core, sub))
	}
	// data/scripts/lib/register_npc_type.lua is what turns an npcConfig table into
	// NpcType setter calls — there is no Go implementation of it, exactly as there
	// is no C++ one. Leaving it out here is what let it sit inert unnoticed.
	walkLoad(t, e, filepath.Join(core, "scripts", "lib"))

	names := []string{
		"walter_jaeger", "dallheim", "barbara", "seymour", "gamemaster",
		"a_sweaty_cyclops", "captain_bluebear", "eclesius", "hardek", "rashid",
	}
	for _, file := range names {
		path := filepath.Join(datapack, "npc", file+".lua")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := e.DoFile(path); err != nil {
			t.Errorf("%s: load failed: %v", file, err)
			continue
		}
	}
	// Greet each registered NPC.
	e.npcCallbacksMu.Lock()
	regNames := make([]string, 0, len(e.npcCallbacks))
	for n := range e.npcCallbacks {
		regNames = append(regNames, n)
	}
	e.npcCallbacksMu.Unlock()
	for _, n := range regNames {
		greetNpc(t, e, n, n)
	}

	// The greeting's reply is sent via addEvent (SayEvent) on a ~1s timer, so
	// its Lua errors surface after onSay returns. Wait for those timers, then
	// assert none of the delayed callbacks raised (e.g. getWorldTime/Blessings
	// missing, the actual crash the user hit).
	time.Sleep(1500 * time.Millisecond)
	if len(capturedErrs) > 0 {
		for _, msg := range capturedErrs {
			t.Errorf("delayed NPC callback error: %s", msg)
		}
	}
}

var greetSeq int

// greetNpc invokes the NPC's stored onSay callback with "hi" and fails on error.
// The npc and a player are registered in the world so the delayed SayEvent
// reply (which does Npc(npcId)/Player(playerId) lookups) resolves them.
func greetNpc(t *testing.T, e *Engine, display, key string) {
	t.Helper()
	e.npcCallbacksMu.Lock()
	cbs := e.npcCallbacks[key]
	e.npcCallbacksMu.Unlock()
	if cbs == nil || cbs["onSay"] == nil {
		t.Errorf("%s: onSay callback not registered", display)
		return
	}

	greetSeq++
	npc := game.NewNpc(uint32(100000+greetSeq), display, e.world.TypeRegistry.Npcs[key])
	e.world.AddCreature(npc)
	player := &game.Player{Name: display + "_tester", Level: 8, Health: 100, MaxHealth: 100}
	e.world.AddPlayer(player, nil)

	e.mu.Lock()
	L := e.L
	L.Push(cbs["onSay"])
	ud := L.NewUserData()
	ud.Value = npc
	L.SetMetatable(ud, L.GetTypeMetatable("Npc"))
	L.Push(ud)
	up := L.NewUserData()
	up.Value = player
	L.SetMetatable(up, L.GetTypeMetatable("Player"))
	L.Push(up)
	L.Push(lua.LNumber(1))
	L.Push(lua.LString("hi"))
	err := L.PCall(4, 0, nil)
	e.mu.Unlock()
	if err != nil {
		t.Errorf("%s: onSay error: %v", display, err)
	}
}
