package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
)

// TestDispatchModulePacket is the module-system bug: module scripts each define
// a global onRecvbyte and overwrite each other, so only the last-loaded module
// (the gamestore) ever dispatched. CaptureOnRecvbyteFor stores a module's
// handler per opcode and DispatchModulePacket routes by opcode, so the hireling
// outfit helper (byte 0xD3) is no longer shadowed.
func TestDispatchModulePacket(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	// Simulate loading a module script that defines the global onRecvbyte.
	if err := e.L.DoString(`
		function onRecvbyte(player, msg, byte)
			local outfitType = msg:getByte()
			if outfitType == 0 then
				local look = msg:getU16()
				captured = { byte = byte, outfitType = outfitType, look = look }
			end
		end
	`); err != nil {
		t.Fatalf("define onRecvbyte: %v", err)
	}
	e.CaptureOnRecvbyteFor([]uint8{0xD3})

	p := &game.Player{Name: "HirelingOwner"}
	e.pushPlayerUserdata(p)
	e.L.SetGlobal("p", e.L.Get(-1))
	e.L.Pop(1)

	// Payload: outfitType 0, lookType 0x1234.
	if err := e.L.DoString(`captured = nil`); err != nil {
		t.Fatal(err)
	}
	consumed := e.DispatchModulePacket(p, 0xD3, []byte{0x00, 0x34, 0x12})
	if !consumed {
		t.Fatal("module read the packet, dispatch must report consumed")
	}

	if err := e.L.DoString(`
		assert(captured ~= nil, "module onRecvbyte was not called")
		assert(captured.byte == 0xD3, "wrong opcode: " .. captured.byte)
		assert(captured.outfitType == 0)
		assert(captured.look == 0x1234, "look mismatch: " .. tostring(captured.look))
	`); err != nil {
		t.Fatalf("module callback result wrong: %v", err)
	}

	// An opcode with no module registered must not dispatch.
	if consumed := e.DispatchModulePacket(p, 0x99, []byte{1, 2, 3}); consumed {
		t.Fatal("unregistered opcode must not be consumed")
	}
}

// TestDispatchModulePacketNoConsume: a module that returns without reading any
// bytes (e.g. the hireling helper when the player is not changing an outfit)
// must report not-consumed, so the caller proceeds with normal handling.
func TestDispatchModulePacketNoConsume(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		function onRecvbyte(player, msg, byte)
			-- never reads from msg
		end
	`); err != nil {
		t.Fatalf("define onRecvbyte: %v", err)
	}
	e.CaptureOnRecvbyteFor([]uint8{0xD3})

	p := &game.Player{Name: "HirelingOwner"}
	e.pushPlayerUserdata(p)
	e.L.SetGlobal("p", e.L.Get(-1))
	e.L.Pop(1)

	if consumed := e.DispatchModulePacket(p, 0xD3, []byte{1, 2, 3}); consumed {
		t.Fatal("module that read nothing must not report consumed")
	}
}

// TestHirelingModuleLoads exercises the actual loader gap: the old module loop
// dofiled <dir>/<dir>.lua, so data/modules/scripts/hirelings/hireling_module.lua
// was never loaded. Loading it (after the hireling system lib, which defines
// Player:isChangingHirelingOutfit) must define the global onRecvbyte and the
// HirelingModule table, and the outfit packet must NOT be consumed while no
// hireling outfit change is pending — so a normal player outfit change still
// goes through.
func TestHirelingModuleLoads(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.DoFile("../../data/libs/systems/hireling.lua"); err != nil {
		t.Fatalf("loading hireling.lua: %v", err)
	}
	if err := e.DoFile("../../data/modules/scripts/hirelings/hireling_module.lua"); err != nil {
		t.Fatalf("loading hireling_module.lua: %v", err)
	}
	if err := e.L.DoString(`
		assert(type(onRecvbyte) == "function", "global onRecvbyte must be defined")
		assert(type(HirelingModule) == "table", "HirelingModule table missing")
		assert(HirelingModule.C_Packets.ConfirmOutfitChange == 0xD3, "wrong opcode constant")
		assert(type(Player.isChangingHirelingOutfit) == "function", "isChangingHirelingOutfit missing")
	`); err != nil {
		t.Fatalf("hireling module globals wrong: %v", err)
	}

	// The module's onRecvbyte must be capturable and dispatchable for 0xD3.
	e.CaptureOnRecvbyteFor([]uint8{0xD3})
	p := &game.Player{Name: "HirelingOwner"}
	e.pushPlayerUserdata(p)
	e.L.SetGlobal("p", e.L.Get(-1))
	e.L.Pop(1)

	// Player not changing an outfit → module returns without reading → not consumed.
	if consumed := e.DispatchModulePacket(p, 0xD3, []byte{0x00, 0x34, 0x12}); consumed {
		t.Fatal("module must not consume the outfit packet while no hireling outfit change is pending")
	}
}
