package luaengine

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// TestNpcKeywordReply greets an NPC then says a simple StdModule.say keyword and
// asserts the NPC actually replies (via the delayed SayEvent → OnCreatureSay).
// Reproduces "NPC ignores keywords after greeting" for non-trade keywords
// (sail/information/balance/deposit all route through this path).
func TestNpcKeywordReply(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	datapack := filepath.Join(repo, "data-otservbr-global")
	core := filepath.Join(repo, "data")
	npcFile := filepath.Join(datapack, "npc", "hyacinth.lua")
	if _, err := os.Stat(npcFile); err != nil {
		t.Skip("datapack not available")
	}

	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()

	var mu sync.Mutex
	var said []string
	w.OnCreatureSay = func(c game.Creature, talkType byte, text string) {
		mu.Lock()
		said = append(said, text)
		mu.Unlock()
	}

	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	e.L.SetGlobal("DATA_DIRECTORY", lua.LString(datapack))
	e.L.SetGlobal("CORE_DIRECTORY", lua.LString(core))
	walkLoad(t, e, filepath.Join(datapack, "lib"))
	_ = e.DoFile(filepath.Join(core, "global.lua"))
	for _, sub := range []string{"lib", "libs", "npclib"} {
		walkLoad(t, e, filepath.Join(core, sub))
	}
	// register_npc_type.lua: the datapack shim that applies npcConfig. Without it
	// npcType:register does not exist, here or in C++.
	walkLoad(t, e, filepath.Join(core, "scripts", "lib"))
	if err := e.DoFile(npcFile); err != nil {
		t.Fatalf("load hyacinth: %v", err)
	}

	npc := game.NewNpc(100001, "Hyacinth", w.TypeRegistry.Npcs["hyacinth"])
	w.AddCreature(npc)
	player := &game.Player{Name: "Asker", Level: 8, Health: 100, MaxHealth: 100}
	w.AddPlayer(player, nil)

	say := func(talkType byte, text string) {
		e.npcCallbacksMu.Lock()
		fn := e.npcCallbacks["hyacinth"]["onSay"]
		e.npcCallbacksMu.Unlock()
		e.mu.Lock()
		L := e.L
		L.Push(fn)
		un := L.NewUserData()
		un.Value = npc
		L.SetMetatable(un, L.GetTypeMetatable("Npc"))
		L.Push(un)
		up := L.NewUserData()
		up.Value = player
		L.SetMetatable(up, L.GetTypeMetatable("Player"))
		L.Push(up)
		L.Push(lua.LNumber(talkType))
		L.Push(lua.LString(text))
		if err := L.PCall(4, 0, nil); err != nil {
			t.Fatalf("onSay(%q): %v", text, err)
		}
		e.mu.Unlock()
	}

	say(1, "hi")     // greet
	say(12, "name")  // TALKTYPE_PRIVATE_PN keyword
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, s := range said {
		if s == "I'm Hyacinth." {
			found = true
		}
	}
	if !found {
		t.Errorf("NPC did not reply to 'name' keyword; said=%v", said)
	}
}
