package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// resolveTargetPlayer resolves arg n to an online player: a Player userdata, a
// name string, or a numeric creature id. Returns nil when not found/online.
func (e *Engine) resolveTargetPlayer(L *lua.LState, n int) *game.Player {
	v := L.Get(n)
	switch v.Type() {
	case lua.LTUserData:
		if p, ok := v.(*lua.LUserData).Value.(*game.Player); ok {
			return p
		}
	case lua.LTString:
		if e.world != nil {
			return e.world.PlayerByName(v.String())
		}
	case lua.LTNumber:
		if e.world != nil {
			if c, ok := e.world.CreatureByID(uint32(lua.LVAsNumber(v))).(*game.Player); ok {
				return c
			}
		}
	}
	return nil
}

func (e *Engine) registerBank() {
	bank := e.L.NewTable()

	e.L.SetField(bank, "balance", e.L.NewFunction(func(L *lua.LState) int {
		p := checkPlayer(L)
		if p == nil {
			L.Push(lua.LNumber(0))
			return 1
		}
		L.Push(lua.LNumber(p.BankBalance))
		return 1
	}))

	e.L.SetField(bank, "deposit", e.L.NewFunction(func(L *lua.LState) int {
		p := checkPlayer(L)
		amount := uint64(L.CheckNumber(2))
		// Deposit moves inventory cash into the bank; never pull from the bank.
		if p == nil || !p.RemoveMoney(amount, false) {
			L.Push(lua.LFalse)
			return 1
		}
		p.BankBalance += amount
		L.Push(lua.LTrue)
		return 1
	}))

	e.L.SetField(bank, "withdraw", e.L.NewFunction(func(L *lua.LState) int {
		p := checkPlayer(L)
		amount := uint64(L.CheckNumber(2))
		if p == nil || p.BankBalance < amount {
			L.Push(lua.LFalse)
			return 1
		}
		p.BankBalance -= amount
		p.AddMoney(amount)
		L.Push(lua.LTrue)
		return 1
	}))

	e.L.SetField(bank, "debit", e.L.NewFunction(func(L *lua.LState) int {
		p := checkPlayer(L)
		amount := uint64(L.CheckNumber(2))
		if p == nil || p.BankBalance < amount {
			L.Push(lua.LFalse)
			return 1
		}
		p.BankBalance -= amount
		L.Push(lua.LTrue)
		return 1
	}))

	e.L.SetField(bank, "credit", e.L.NewFunction(func(L *lua.LState) int {
		p := e.resolveTargetPlayer(L, 1)
		amount := uint64(L.CheckNumber(2))
		if p != nil {
			p.BankBalance += amount
			L.Push(lua.LTrue)
			return 1
		}
		v := L.Get(1)
		if v.Type() == lua.LTString && e.database != nil && e.database.SQL != nil {
			name := v.String()
			_, err := e.database.SQL.Exec("UPDATE players SET balance = balance + ? WHERE LOWER(name) = LOWER(?)", amount, name)
			if err == nil {
				L.Push(lua.LTrue)
				return 1
			}
		}
		L.Push(lua.LFalse)
		return 1
	}))

	e.L.SetField(bank, "transfer", e.L.NewFunction(func(L *lua.LState) int {
		src := checkPlayer(L)
		amount := uint64(L.CheckNumber(3))
		if src == nil || amount == 0 || src.BankBalance < amount {
			L.Push(lua.LFalse)
			return 1
		}
		target := e.resolveTargetPlayer(L, 2)
		if target == nil {
			// Offline targets require a direct DB write, which the Lua engine
			// has no handle for yet; only online transfers are supported.
			L.Push(lua.LFalse)
			return 1
		}
		if target == src { // cannot transfer to self
			L.Push(lua.LFalse)
			return 1
		}
		src.BankBalance -= amount
		target.BankBalance += amount
		if target.Session != nil {
			target.Session.SendStats()
		}
		L.Push(lua.LTrue)
		return 1
	}))

	e.L.SetGlobal("Bank", bank)
}
