package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) DispatchDailyRewardPacket(p *game.Player, opcode byte, data []byte) bool {
	if p == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	L := e.L

	switch opcode {
	case 0xD8:
		dr := L.GetGlobal("DailyReward")
		if dr.Type() != lua.LTTable {
			return false
		}
		fn := L.GetField(dr.(*lua.LTable), "loadDailyReward")
		if fn.Type() != lua.LTFunction {
			return false
		}
		if err := L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, lua.LNumber(p.ID), lua.LNumber(1)); err != nil {
			e.log.Warn("daily_reward: loadDailyReward error", "err", err)
			return false
		}

	case 0xD9:
		playerUd := L.NewUserData()
		playerUd.Value = game.Creature(p)
		L.SetMetatable(playerUd, L.GetTypeMetatable(metatableForCreature(p)))

		pt := L.GetGlobal("Player")
		if pt.Type() != lua.LTTable {
			return false
		}
		fn := L.GetField(pt, "sendRewardHistory")
		if fn.Type() != lua.LTFunction {
			return false
		}
		if err := L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, playerUd); err != nil {
			e.log.Warn("daily_reward: sendRewardHistory error", "err", err)
			return false
		}

	case 0xDA:
		playerUd := L.NewUserData()
		playerUd.Value = game.Creature(p)
		L.SetMetatable(playerUd, L.GetTypeMetatable(metatableForCreature(p)))

		msgUd := L.NewUserData()
		msgUd.Value = &luaNetworkMessage{w: netmsg.NewWriter(), r: netmsg.NewReader(data)}
		L.SetMetatable(msgUd, L.GetTypeMetatable(networkMessageTypeName))

		pt := L.GetGlobal("Player")
		if pt.Type() != lua.LTTable {
			return false
		}
		fn := L.GetField(pt, "selectDailyReward")
		if fn.Type() != lua.LTFunction {
			return false
		}
		if err := L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, playerUd, msgUd); err != nil {
			e.log.Warn("daily_reward: selectDailyReward error", "err", err)
			return false
		}
	}

	L.Pop(L.GetTop())
	return true
}
