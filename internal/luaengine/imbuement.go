package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerImbuementType() {
	mt := e.L.NewTypeMetatable("Imbuement")
	methods := map[string]lua.LGFunction{
		"getName":          imbuementGetName,
		"getId":            imbuementGetId,
		"getItems":         imbuementGetItems,
		"getBase":          imbuementGetBase,
		"getCategory":      imbuementGetCategory,
		"isPremium":        imbuementIsPremium,
		"getElementDamage": imbuementGetElementDamage,
		"getCombatType":    imbuementGetCombatType,
	}
	e.L.SetFuncs(mt, methods)
	e.L.SetField(mt, "__index", mt)
	e.L.SetGlobal("Imbuement", e.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNil)
		return 1
	}))
}

func imbuementGetName(L *lua.LState) int { L.Push(lua.LString("")); return 1 }
func imbuementGetId(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }
func imbuementGetItems(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
func imbuementGetBase(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }
func imbuementGetCategory(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }
func imbuementIsPremium(L *lua.LState) int { L.Push(lua.LFalse); return 1 }
func imbuementGetElementDamage(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }
func imbuementGetCombatType(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }
