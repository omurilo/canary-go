package luaengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// TestNpcTurnToCreatureBinding checks the npc:turnToCreature(player, true) Lua
// binding directly: the datapack calls it with an extra boolean second argument,
// and the binding must still resolve the player userdata and change the NPC's
// facing.
func TestNpcTurnToCreatureBinding(t *testing.T) {
	e := newTestEngine()
	nt := &creatures.NpcType{Name: "Facer"}
	npc := game.NewNpc(1, "Facer", nt)
	npc.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	e.world.AddCreature(npc)

	player := &game.Player{Name: "P", ID: 99}
	player.SetPosition(game.Position{X: 101, Y: 100, Z: 7}) // east of the npc

	// npc:turnToCreature(player, true)
	L := e.L
	L.Push(L.NewFunction(e.npcTurntocreature))
	ud := L.NewUserData()
	ud.Value = npc
	L.SetMetatable(ud, L.GetTypeMetatable("Npc"))
	L.Push(ud)
	up := L.NewUserData()
	up.Value = player
	L.SetMetatable(up, L.GetTypeMetatable("Player"))
	L.Push(up)
	L.Push(lua.LBool(true)) // second datapack argument

	e.mu.Lock()
	err := L.PCall(3, 0, nil)
	e.mu.Unlock()
	if err != nil {
		t.Fatalf("npc:turnToCreature failed: %v", err)
	}
	if got := npc.GetDirection(); got != game.DirEast {
		t.Errorf("npc facing = %v, want %v (east toward the player)", got, game.DirEast)
	}
}

// TestNpcGreetTurns loads a real datapack NPC and greets it with "hi", asserting
// the greeting reaches npc:turnToCreature and the NPC actually faces the player.
func TestNpcGreetTurns(t *testing.T) {
	repo := filepath.Join("..", "..")
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
	walkLoad(t, e, filepath.Join(core, "scripts", "lib"))
	if err := e.DoFile(benjamin); err != nil {
		t.Fatalf("load benjamin: %v", err)
	}

	e.npcCallbacksMu.Lock()
	cbs := e.npcCallbacks["benjamin"]
	e.npcCallbacksMu.Unlock()
	if cbs == nil || cbs["onSay"] == nil {
		t.Fatal("benjamin: onSay callback not registered")
	}

	npc := game.NewNpc(100001, "Benjamin", e.world.TypeRegistry.Npcs["benjamin"])
	npc.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	e.world.AddCreature(npc)

	player := &game.Player{Name: "turn_tester", ID: 999, Level: 8, Health: 100, MaxHealth: 100}
	player.SetPosition(game.Position{X: 101, Y: 100, Z: 7}) // east of npc
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
	L.Push(lua.LNumber(1)) // TALKTYPE_SAY
	L.Push(lua.LString("hi"))
	err := L.PCall(4, 0, nil)
	e.mu.Unlock()
	if err != nil {
		t.Fatalf("greet onSay error: %v", err)
	}

	if !npc.IsInteractingWithPlayer(player.ID) {
		t.Error("greeting did not set an interaction — the greet flow never completed")
	}
	if got := npc.GetDirection(); got != game.DirEast {
		t.Errorf("after greeting, npc facing = %v, want %v (east toward player)", got, game.DirEast)
	}
}
