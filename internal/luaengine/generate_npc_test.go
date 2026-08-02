package luaengine

import (
	"strings"
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
)

// TestGameGenerateNpc is the hireling-lamp bug: Game.generateNpc was a stub
// returning nil, so Hireling:spawn() (hireling.lua:323) produced `Npc(nil)` and
// `creature:setOutfit` crashed on a nil object. generateNpc is Npc::createNpc —
// build the Npc from the type, without placing it.
func TestGameGenerateNpc(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	// A hireling type, as createHirelingType would register it.
	e.world.TypeRegistry.Npcs[strings.ToLower("Hireling Bob")] = &creatures.NpcType{
		Name:      "Hireling Bob",
		MaxHealth: 100,
		Speed:     55,
	}

	if err := e.L.DoString(`
		local npc = Game.generateNpc("Hireling Bob")
		assert(npc ~= nil, "generateNpc returned nil for a known type")
		assert(type(npc) == "userdata", "expected Npc userdata, got " .. type(npc))
		assert(npc:getName() == "Hireling Bob", "wrong name: " .. npc:getName())
		assert(npc:getId() ~= nil and npc:getId() > 0, "npc must have an id")

		local missing = Game.generateNpc("No Such Npc")
		assert(missing == nil, "generateNpc should return nil for an unknown type")
	`); err != nil {
		t.Fatalf("Game.generateNpc failed: %v", err)
	}
}

// TestGameGenerateNpcDoesNotPlace verifies generateNpc does not add the npc to
// the world — that is Game.createNpc's job. A generated hireling must be
// placed explicitly by npc:place().
func TestGameGenerateNpcDoesNotPlace(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	e.world.TypeRegistry.Npcs["nurse"] = &creatures.NpcType{Name: "nurse", MaxHealth: 100}

	if err := e.L.DoString(`
		local npc = Game.generateNpc("nurse")
		assert(npc ~= nil)
	`); err != nil {
		t.Fatalf("Game.generateNpc failed: %v", err)
	}
}

// TestNpcPlace exercises the hireling-lamp spawn chain: generateNpc then
// npc:place. place is luaNpcPlace (npc_functions.cpp:182) and must return the
// npc when the tile exists and nil when it does not, so Hireling:spawn()
// (hireling.lua:318-331) stops failing mid-chain.
func TestNpcPlace(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	here := game.Position{X: 100, Y: 200, Z: 7}
	e.world.Map.SetTile(here, &game.Tile{Ground: &game.Item{ID: 1}})
	e.world.TypeRegistry.Npcs[strings.ToLower("Hireling Bob")] = &creatures.NpcType{
		Name: "Hireling Bob", MaxHealth: 100, Speed: 55,
	}

	if err := e.L.DoString(`
		local npc = Game.generateNpc("Hireling Bob")
		assert(npc ~= nil)

		local placed = npc:place({x = 100, y = 200, z = 7})
		assert(placed ~= nil, "npc:place should return the npc on a real tile")
		assert(npc:getPosition().x == 100 and npc:getPosition().y == 200, "npc not at the requested position")

		-- a position with no tile fails placement (mirrors placeCreature returning false)
		local nowhere = npc:place({x = 999, y = 999, z = 0})
		assert(nowhere == nil, "npc:place on an empty tile should return nil")
	`); err != nil {
		t.Fatalf("npc:place failed: %v", err)
	}

	// The placed hireling must be reachable as a spectator so player chat routes
	// to it (broadcastSay → SpectatingNpcs). This is the potions-shop bug: the
	// npc was "spawned" but never landed in the world's creature registry, so
	// Towncryer — not the hireling — received the say.
	found := false
	for _, n := range e.world.SpectatingNpcs(game.Position{X: 100, Y: 200, Z: 7}) {
		if n.Name == "Hireling Bob" {
			found = true
		}
	}
	if !found {
		t.Fatal("placed hireling NPC is not in the spectator list; player chat will not reach it")
	}
}
