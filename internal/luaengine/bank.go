package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

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
		if p == nil || !p.RemoveMoney(amount) {
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
		p := checkPlayer(L)
		amount := uint64(L.CheckNumber(2))
		if p != nil {
			p.BankBalance += amount
		}
		L.Push(lua.LTrue)
		return 1
	}))

	e.L.SetField(bank, "transfer", e.L.NewFunction(func(L *lua.LState) int {
		// Just a stub for now
		L.Push(lua.LFalse)
		return 1
	}))

	e.L.SetGlobal("Bank", bank)
}
