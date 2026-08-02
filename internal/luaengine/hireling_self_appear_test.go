package luaengine

import "testing"

// TestHirelingSelfAppearBindsByPosition covers the hireling-lamp talk crash:
// data-otservbr-global/npc/hireling.lua:626 failed with "attempt to index a
// non-table object(nil) with key 'canTalkTo'" because the Lua `hireling` local
// is bound in npcType.onAppear from the NPC's OWN position
// (getHirelingByPosition(creature:getPosition())) — and the port's
// notifyNpcsAround skipped the NPC itself, so that callback never fired. This
// exercises the exact binding logic: at the NPC's own position, getHirelingByPosition
// must return the hireling.
func TestHirelingSelfAppearBindsByPosition(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.DoFile("../../data/libs/systems/hireling.lua"); err != nil {
		t.Fatalf("loading hireling.lua: %v", err)
	}

	// A hireling parked at (100, 200, 7), as the lamp spawn sets it.
	if err := e.L.DoString(`
		HIRELINGS = {}
		local h = Hireling:new()
		h.id = 1
		h.player_id = 1000
		h.name = "Bob"
		h.posx = 100; h.posy = 200; h.posz = 7
		HIRELINGS[1] = h

		-- npcType.onAppear body, fired with the NPC itself:
		local hireling = getHirelingByPosition({ x = 100, y = 200, z = 7 })
		assert(hireling ~= nil, "self-appear must bind the hireling at its own position")
		assert(hireling:getId() == 1, "wrong hireling bound")

		-- A player standing elsewhere must NOT bind it (the canTalkTo guard).
		local elsewhere = getHirelingByPosition({ x = 101, y = 200, z = 7 })
		assert(elsewhere == nil, "player on an adjacent tile must not bind the hireling")
	`); err != nil {
		t.Fatalf("hireling self-appear binding failed: %v", err)
	}
}
