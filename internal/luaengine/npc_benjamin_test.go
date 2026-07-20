package luaengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

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
	if err := e.DoFile(benjamin); err != nil {
		t.Fatalf("load benjamin: %v", err)
	}

	npcType := e.world.TypeRegistry.Npcs["benjamin"]
	npc := game.NewNpc(2, "Benjamin", npcType)
	player := &game.Player{ID: 1, Name: "Tester", Level: 8, Health: 100, MaxHealth: 100}

	// This logs the error via e.log if the callback raises; capture it directly
	// by calling the stored onSay through PCall so the test sees the message.
	e.npcCallbacksMu.Lock()
	cbs := e.npcCallbacks["benjamin"]
	e.npcCallbacksMu.Unlock()
	if cbs == nil || cbs["onSay"] == nil {
		t.Fatalf("benjamin onSay callback not registered (callbacks=%v)", cbs != nil)
	}

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
		t.Fatalf("onSay error (this is the repro): %v", err)
	}
	t.Log("onSay ran without error")
}
