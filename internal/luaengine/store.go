package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	lua "github.com/yuin/gopher-lua"
)

// LogStoreCatalogStatus inspects GameStore.Categories and logs how many
// categories / total offers loaded, so a silently-empty catalog is visible.
func (e *Engine) LogStoreCatalogStatus() {
	e.mu.Lock()
	defer e.mu.Unlock()
	gs, ok := e.L.GetGlobal("GameStore").(*lua.LTable)
	if !ok {
		e.log.Warn("store: GameStore global not defined after module load")
		return
	}
	cats, ok := e.L.GetField(gs, "Categories").(*lua.LTable)
	if !ok {
		e.log.Warn("store: GameStore.Categories not a table (catalog failed to load)")
		return
	}
	catCount := cats.Len()
	offerCount := 0
	for i := 1; i <= catCount; i++ {
		if cat, ok := e.L.RawGetInt(cats, i).(*lua.LTable); ok {
			if offers, ok := e.L.GetField(cat, "offers").(*lua.LTable); ok {
				offerCount += offers.Len()
			}
		}
	}
	e.log.Info("store catalog loaded", "categories", catCount, "offers", offerCount)
}

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
