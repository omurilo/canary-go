package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerMetrics() {
	tbl := e.L.NewTable()
	e.L.SetField(tbl, "addCounter", e.L.NewFunction(func(L *lua.LState) int { return 0 }))
	e.L.SetGlobal("metrics", tbl)
}
