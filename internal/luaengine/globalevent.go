package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

type globalEvent struct {
	name      string
	eventType string
	onStartup *lua.LFunction
}

var registeredGlobalEvents []*globalEvent

func (e *Engine) registerGlobalEventClass() {
	mt := e.L.NewTypeMetatable("GlobalEvent")
	methods := map[string]lua.LGFunction{
		"type": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				ge.eventType = L.CheckString(2)
			}
			L.Push(L.Get(1))
			return 1
		},
		"onStartup": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				if fn, ok := L.Get(2).(*lua.LFunction); ok {
					ge.onStartup = fn
				}
			}
			L.Push(L.Get(1))
			return 1
		},
		"onThink": func(L *lua.LState) int {
			L.Push(L.Get(1))
			return 1
		},
		"onTime": func(L *lua.LState) int {
			L.Push(L.Get(1))
			return 1
		},
		"onShutdown": func(L *lua.LState) int {
			L.Push(L.Get(1))
			return 1
		},
		"onRecord": func(L *lua.LState) int {
			L.Push(L.Get(1))
			return 1
		},
		"onPeriodChange": func(L *lua.LState) int {
			L.Push(L.Get(1))
			return 1
		},
		"onSave": func(L *lua.LState) int {
			L.Push(L.Get(1))
			return 1
		},
		"register": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				registeredGlobalEvents = append(registeredGlobalEvents, ge)
			}
			L.Push(lua.LTrue)
			return 1
		},
	}

	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), methods))

	// GlobalEvent("name") constructor
	classTable := e.L.NewTable()
	callMt := e.L.NewTable()
	e.L.SetField(callMt, "__call", e.L.NewFunction(func(L *lua.LState) int {
		name := L.OptString(2, "")
		ge := &globalEvent{name: name}
		ud := L.NewUserData()
		ud.Value = ge
		L.SetMetatable(ud, mt)
		L.Push(ud)
		return 1
	}))
	e.L.SetMetatable(classTable, callMt)
	e.L.SetGlobal("GlobalEvent", classTable)
}

// RunStartupGlobalEvents runs all registered GlobalEvent onStartup callbacks.
func (e *Engine) RunStartupGlobalEvents() {
	count := 0
	for _, ge := range registeredGlobalEvents {
		if ge.onStartup != nil {
			count++
			err := e.L.CallByParam(lua.P{
				Fn:      ge.onStartup,
				NRet:    0,
				Protect: true,
			})
			if err != nil {
				e.log.Warn("GlobalEvent onStartup error", "event", ge.name, "err", err)
			}
		}
	}
	e.log.Info("executed startup global events", "count", count)
}
