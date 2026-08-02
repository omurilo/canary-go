package luaengine

import (
	"github.com/omurilo/canary-go/internal/globalevents"
	lua "github.com/yuin/gopher-lua"
)

// globalEvent is the Go-side struct backing the Lua GlobalEvent userdata.
type globalEvent struct {
	name      string
	eventType string // "startup", "think", "time", "record", "shutdown", "periodchange", "save"

	// Lua function references – the owner Engine's L state owns them.
	onStartup      *lua.LFunction
	onThink        *lua.LFunction
	onTime         *lua.LFunction
	onRecord       *lua.LFunction
	onShutdown     *lua.LFunction
	onPeriodChange *lua.LFunction
	onSave         *lua.LFunction

	// Configuration set by Lua methods.
	interval int64  // milliseconds for think events
	timeStr  string // "HH:MM" for time events
}

// registeredGlobalEvents keeps a Lua-side reference so the GC doesn't collect
// the LFunctions. The globalevents.Engine holds its own copy of the metadata
// and callbacks.
var registeredGlobalEvents []*globalEvent

// registerGlobalEventClass registers the GlobalEvent("name") Lua constructor
// and all its method chain.
func (e *Engine) registerGlobalEventClass() {
	L := e.L
	mt := L.NewTypeMetatable("GlobalEvent")

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
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				if fn, ok := L.Get(2).(*lua.LFunction); ok {
					ge.onThink = fn
				}
			}
			L.Push(L.Get(1))
			return 1
		},

		"onTime": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				if fn, ok := L.Get(2).(*lua.LFunction); ok {
					ge.onTime = fn
				}
			}
			L.Push(L.Get(1))
			return 1
		},

		"onRecord": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				if fn, ok := L.Get(2).(*lua.LFunction); ok {
					ge.onRecord = fn
				}
			}
			L.Push(L.Get(1))
			return 1
		},

		"onShutdown": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				if fn, ok := L.Get(2).(*lua.LFunction); ok {
					ge.onShutdown = fn
				}
			}
			L.Push(L.Get(1))
			return 1
		},

		"onPeriodChange": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				if fn, ok := L.Get(2).(*lua.LFunction); ok {
					ge.onPeriodChange = fn
				}
			}
			L.Push(L.Get(1))
			return 1
		},

		"onSave": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				if fn, ok := L.Get(2).(*lua.LFunction); ok {
					ge.onSave = fn
				}
			}
			L.Push(L.Get(1))
			return 1
		},

		"interval": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				ge.interval = int64(L.CheckInt(2))
			}
			L.Push(L.Get(1))
			return 1
		},

		"time": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				ge.timeStr = L.CheckString(2)
			}
			L.Push(L.Get(1))
			return 1
		},

		"register": func(L *lua.LState) int {
			ud := L.CheckUserData(1)
			if ge, ok := ud.Value.(*globalEvent); ok {
				// Keep a Lua-side reference so callbacks aren't GC'd.
				registeredGlobalEvents = append(registeredGlobalEvents, ge)
				e.registerGlobalEventInEngine(ge)
			}
			L.Push(lua.LTrue)
			return 1
		},
	}

	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), methods))
	L.SetField(mt, "__newindex", L.NewFunction(func(L *lua.LState) int {
		ud := L.CheckUserData(1)
		ge, ok := ud.Value.(*globalEvent)
		if !ok {
			return 0
		}
		key := L.CheckString(2)
		if fn, ok := L.Get(3).(*lua.LFunction); ok {
			switch key {
			case "onStartup":
				ge.onStartup = fn
			case "onThink":
				ge.onThink = fn
			case "onTime":
				ge.onTime = fn
			case "onRecord":
				ge.onRecord = fn
			case "onShutdown":
				ge.onShutdown = fn
			case "onPeriodChange":
				ge.onPeriodChange = fn
			case "onSave":
				ge.onSave = fn
			}
		}
		return 0
	}))

	// GlobalEvent("name") constructor.
	classTable := L.NewTable()
	callMt := L.NewTable()
	L.SetField(callMt, "__call", L.NewFunction(func(L *lua.LState) int {
		name := L.OptString(2, "")
		ge := &globalEvent{name: name}
		ud := L.NewUserData()
		ud.Value = ge
		L.SetMetatable(ud, mt)
		L.Push(ud)
		return 1
	}))
	L.SetMetatable(classTable, callMt)
	L.SetGlobal("GlobalEvent", classTable)
}

// registerGlobalEventInEngine converts a Lua-side globalEvent into a
// globalevents.Event and registers it with the core engine. The callback
// closures lock e.mu so Lua API calls are goroutine-safe.
func (e *Engine) registerGlobalEventInEngine(ge *globalEvent) {
	if e.GlobalEvents == nil {
		return
	}

	var evType globalevents.EventType
	switch ge.eventType {
	case "startup":
		evType = globalevents.TypeStartup
	case "think":
		evType = globalevents.TypeThink
	case "time":
		evType = globalevents.TypeTime
	case "record":
		evType = globalevents.TypeRecord
	case "shutdown":
		evType = globalevents.TypeShutdown
	case "periodchange":
		evType = globalevents.TypePeriodChange
	case "save":
		evType = globalevents.TypeSave
	default:
		// If no explicit type was set by :type(), infer from which callback
		// was provided. This mirrors TFS behaviour where a GlobalEvent script
		// may only define the callback and rely on type inference.
		switch {
		case ge.onStartup != nil:
			evType = globalevents.TypeStartup
		case ge.onThink != nil:
			evType = globalevents.TypeThink
		case ge.onTime != nil:
			evType = globalevents.TypeTime
		case ge.onRecord != nil:
			evType = globalevents.TypeRecord
		case ge.onShutdown != nil:
			evType = globalevents.TypeShutdown
		case ge.onPeriodChange != nil:
			evType = globalevents.TypePeriodChange
		case ge.onSave != nil:
			evType = globalevents.TypeSave
		default:
			e.log.Warn("global event has no type and no recognised callback, skipping",
				"name", ge.name)
			return
		}
	}

	ev := &globalevents.Event{
		Name:     ge.name,
		Type:     evType,
		Interval: ge.interval,
		TimeStr:  ge.timeStr,
	}

	// Wire the callback closure based on type.
	switch evType {
	case globalevents.TypeStartup:
		if ge.onStartup == nil {
			e.log.Warn("startup event has no onStartup callback", "name", ge.name)
			return
		}
		cb := ge.onStartup
		ev.Callback = func() bool {
			e.mu.Lock()
			defer e.mu.Unlock()
			err := e.L.CallByParam(lua.P{Fn: cb, NRet: 0, Protect: true})
			if err != nil {
				e.log.Warn("global event startup error", "name", ge.name, "err", err)
				return false
			}
			return true
		}

	case globalevents.TypeThink:
		if ge.onThink == nil {
			e.log.Warn("think event has no onThink callback", "name", ge.name)
			return
		}
		cb := ge.onThink
		intervalMs := ge.interval
		ev.Callback = func() bool {
			e.mu.Lock()
			defer e.mu.Unlock()
			err := e.L.CallByParam(lua.P{
				Fn:      cb,
				NRet:    0,
				Protect: true,
			}, lua.LNumber(intervalMs), lua.LNumber(0))
			if err != nil {
				e.log.Warn("global event think error", "name", ge.name, "err", err)
				return false
			}
			return true
		}

	case globalevents.TypeTime:
		if ge.onTime == nil {
			e.log.Warn("time event has no onTime callback", "name", ge.name)
			return
		}
		cb := ge.onTime
		intervalMs := ge.interval
		ev.Callback = func() bool {
			e.mu.Lock()
			defer e.mu.Unlock()
			err := e.L.CallByParam(lua.P{
				Fn:      cb,
				NRet:    0,
				Protect: true,
			}, lua.LNumber(intervalMs))
			if err != nil {
				e.log.Warn("global event time error", "name", ge.name, "err", err)
				return false
			}
			return true
		}

	case globalevents.TypeRecord:
		if ge.onRecord == nil {
			e.log.Warn("record event has no onRecord callback", "name", ge.name)
			return
		}
		cb := ge.onRecord
		ev.Callback = func() bool {
			e.mu.Lock()
			defer e.mu.Unlock()
			// onRecord(current, old) – the engine passes dynamic args
			// separately via a different path (CheckRecord). For the
			// closure we just call with 0,0 as placeholders; the real
			// invocation goes through RunRecordGlobalEvent.
			err := e.L.CallByParam(lua.P{
				Fn:      cb,
				NRet:    0,
				Protect: true,
			}, lua.LNumber(0), lua.LNumber(0))
			if err != nil {
				e.log.Warn("global event record error", "name", ge.name, "err", err)
				return false
			}
			return true
		}

	case globalevents.TypeShutdown:
		if ge.onShutdown == nil {
			e.log.Warn("shutdown event has no onShutdown callback", "name", ge.name)
			return
		}
		cb := ge.onShutdown
		ev.Callback = func() bool {
			e.mu.Lock()
			defer e.mu.Unlock()
			err := e.L.CallByParam(lua.P{Fn: cb, NRet: 0, Protect: true})
			if err != nil {
				e.log.Warn("global event shutdown error", "name", ge.name, "err", err)
				return false
			}
			return true
		}

	case globalevents.TypePeriodChange:
		if ge.onPeriodChange == nil {
			e.log.Warn("periodchange event has no onPeriodChange callback", "name", ge.name)
			return
		}
		cb := ge.onPeriodChange
		ev.Callback = func() bool {
			e.mu.Lock()
			defer e.mu.Unlock()
			err := e.L.CallByParam(lua.P{Fn: cb, NRet: 0, Protect: true})
			if err != nil {
				e.log.Warn("global event periodchange error", "name", ge.name, "err", err)
				return false
			}
			return true
		}

	case globalevents.TypeSave:
		if ge.onSave == nil {
			e.log.Warn("save event has no onSave callback", "name", ge.name)
			return
		}
		cb := ge.onSave
		ev.Callback = func() bool {
			e.mu.Lock()
			defer e.mu.Unlock()
			err := e.L.CallByParam(lua.P{Fn: cb, NRet: 0, Protect: true})
			if err != nil {
				e.log.Warn("global event save error", "name", ge.name, "err", err)
				return false
			}
			return true
		}
	}

	e.GlobalEvents.Register(ev)
}

// RunStartupGlobalEvents runs all registered GlobalEvent onStartup callbacks.
// Called from main.go after all scripts have been loaded.
func (e *Engine) RunStartupGlobalEvents() {
	if e.GlobalEvents != nil {
		e.GlobalEvents.ExecuteStartup()
	}
}

// RunShutdownGlobalEvents runs all registered GlobalEvent onShutdown callbacks.
// Called from main.go when the server shuts down. The hireling save lives on
// onShutdown (hireling_save.lua) — without this the active flag and position of
// spawned hirelings were never persisted, so a restart left them lamped.
func (e *Engine) RunShutdownGlobalEvents() {
	if e.GlobalEvents != nil {
		e.GlobalEvents.ExecuteShutdown()
	}
}

// RunRecordGlobalEvent fires record-type events with the actual current and
// old player-count values. Called from the game loop when the player count
// changes.
func (e *Engine) RunRecordGlobalEvent(current, old int) {
	if e.GlobalEvents != nil {
		e.GlobalEvents.CheckRecord(current)
	}
	e.runRecordGlobalEvents(current, old)
}

// runRecordGlobalEvents fires record callbacks with the real current/old count.
func (e *Engine) runRecordGlobalEvents(current, old int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, luaEv := range registeredGlobalEvents {
		if luaEv.eventType != "record" && luaEv.onRecord == nil {
			continue
		}
		fn := luaEv.onRecord
		if fn == nil {
			continue
		}
		err := e.L.CallByParam(lua.P{
			Fn:      fn,
			NRet:    0,
			Protect: true,
		}, lua.LNumber(current), lua.LNumber(old))
		if err != nil {
			e.log.Warn("global event record error", "name", luaEv.name, "err", err)
		}
	}
}
