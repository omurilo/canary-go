package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerTeleportType() {
	mt := e.L.NewTypeMetatable("Teleport")
	methods := map[string]lua.LGFunction{
		"getDestination": teleportGetDestination,
		"setDestination": teleportSetDestination,
	}
	e.L.SetFuncs(mt, methods)
	e.L.SetField(mt, "__index", mt)
	e.L.SetGlobal("Teleport", e.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNil)
		return 1
	}))
}

func teleportGetDestination(L *lua.LState) int { L.Push(lua.LNil); return 1 }
func teleportSetDestination(L *lua.LState) int { return 0 }
