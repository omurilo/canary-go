package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerItemClassificationType() {
	mt := e.L.NewTypeMetatable("ItemClassification")
	e.L.SetField(mt, "addTier", e.L.NewFunction(func(L *lua.LState) int { return 0 }))
	e.L.SetField(mt, "__index", mt)
	e.L.SetGlobal("ItemClassification", e.L.NewFunction(func(L *lua.LState) int {
		ud := L.NewUserData()
		ud.Value = struct{}{}
		L.SetMetatable(ud, mt)
		L.Push(ud)
		return 1
	}))
}
