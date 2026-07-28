package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerWebhookType() {
	e.L.SetGlobal("Webhook", e.L.NewFunction(func(L *lua.LState) int {
		ud := L.NewUserData()
		ud.Value = struct{}{}
		mt := e.L.NewTypeMetatable("Webhook")
		e.L.SetField(mt, "sendMessage", e.L.NewFunction(func(L *lua.LState) int { return 0 }))
		e.L.SetField(mt, "__index", mt)
		L.SetMetatable(ud, mt)
		L.Push(ud)
		return 1
	}))
}
