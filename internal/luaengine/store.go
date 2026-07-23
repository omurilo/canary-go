package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	lua "github.com/yuin/gopher-lua"
)

// DispatchStorePacket routes a client store packet to the gamestore Lua module's
// global onRecvbyte(player, msg, byte) handler. `data` is the packet payload
// after the opcode byte. Returns false when the module isn't loaded or the
// handler errored, so the caller can fall back. Mirrors the C++ hook that calls
// the module's onRecvbyte for store opcodes.
func (e *Engine) DispatchStorePacket(p *game.Player, opcode byte, data []byte) bool {
	if p == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	L := e.L
	fn := L.GetGlobal("onRecvbyte")
	if fn.Type() != lua.LTFunction {
		e.log.Warn("store packet received but gamestore onRecvbyte is not defined (module not loaded?)", "opcode", opcode)
		return false
	}
	e.log.Info("dispatching store packet to gamestore module", "opcode", opcode)

	playerUd := L.NewUserData()
	playerUd.Value = game.Creature(p)
	L.SetMetatable(playerUd, L.GetTypeMetatable(metatableForCreature(p)))

	msgUd := L.NewUserData()
	msgUd.Value = &luaNetworkMessage{w: netmsg.NewWriter(), r: netmsg.NewReader(data)}
	L.SetMetatable(msgUd, L.GetTypeMetatable(networkMessageTypeName))

	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true},
		playerUd, msgUd, lua.LNumber(opcode)); err != nil {
		e.log.Error("store onRecvbyte error", "opcode", opcode, "err", err)
		return false
	}
	L.Pop(L.GetTop())
	return true
}
