package luaengine

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
	lua "github.com/yuin/gopher-lua"
)

// Module scripts (data/modules/scripts/*, registered in modules.xml) each define
// a GLOBAL `onRecvbyte(player, msg, byte)`. Loading them into one shared Lua
// state means each new module overwrites that global, so only the last-loaded
// (the gamestore) ever dispatched. The fix mirrors Modules::executeOnRecvbyte
// (modules.cpp:83): capture each module's onRecvbyte right after it loads,
// keyed by the bytes it owns, and dispatch by opcode at packet time.

// CaptureOnRecvbyteFor stores the current global onRecvbyte as the handler for
// each byte in bytes. Call immediately after loading a module script, before
// the next module clobbers the global.
func (e *Engine) CaptureOnRecvbyteFor(bytes []uint8) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn := e.L.GetGlobal("onRecvbyte")
	if fn.Type() != lua.LTFunction {
		return
	}
	if e.moduleCallbacks == nil {
		e.moduleCallbacks = make(map[uint8]*lua.LFunction)
	}
	for _, b := range bytes {
		e.moduleCallbacks[b] = fn.(*lua.LFunction)
	}
}

// DispatchModulePacket routes an inbound packet to the module registered for
// opcode. Returns true when the module consumed bytes (advanced the reader), so
// the caller can skip its own handling — the port of C++'s startBufferPosition
// check in parseSetOutfit (protocolgame.cpp:2431-2434).
func (e *Engine) DispatchModulePacket(p *game.Player, opcode byte, data []byte) bool {
	if p == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.moduleCallbacks == nil {
		return false
	}
	fn := e.moduleCallbacks[opcode]
	if fn == nil {
		return false
	}

	playerUd := e.L.NewUserData()
	playerUd.Value = game.Creature(p)
	e.L.SetMetatable(playerUd, e.L.GetTypeMetatable(metatableForCreature(p)))

	reader := netmsg.NewReader(data)
	msgUd := e.L.NewUserData()
	msgUd.Value = &luaNetworkMessage{r: reader}
	e.L.SetMetatable(msgUd, e.L.GetTypeMetatable(networkMessageTypeName))

	start := reader.Pos()
	if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true},
		playerUd, msgUd, lua.LNumber(opcode)); err != nil {
		e.log.Error("module onRecvbyte error", "opcode", opcode, "err", err)
		return false
	}
	e.L.Pop(e.L.GetTop())
	return reader.Pos() > start
}
