package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const luaCreatureEventTypeName = "CreatureEvent"

type LuaCreatureEvent struct {
	Name        string
	OnLogin     *lua.LFunction
	OnLogout    *lua.LFunction
	OnModalWindow *lua.LFunction
}

// registerCreatureEvent registers the CreatureEvent global constructor and metatable
func (e *Engine) registerCreatureEvent() {
	mt := e.L.NewTypeMetatable(luaCreatureEventTypeName)
	methods := map[string]lua.LGFunction{
		"register": e.creatureEventRegister,
		"type":     creatureEventType,
		"onLogin": func(L *lua.LState) int {
			ev := checkCreatureEvent(L)
			if fn, ok := L.Get(2).(*lua.LFunction); ok {
				ev.OnLogin = fn
			}
			return 0
		},
		"onLogout": func(L *lua.LState) int {
			ev := checkCreatureEvent(L)
			if fn, ok := L.Get(2).(*lua.LFunction); ok {
				ev.OnLogout = fn
			}
			return 0
		},
		"onThink":          func(L *lua.LState) int { return 0 },
		"onPrepareDeath":   func(L *lua.LState) int { return 0 },
		"onDeath":          func(L *lua.LState) int { return 0 },
		"onKill":           func(L *lua.LState) int { return 0 },
		"onAdvance":        func(L *lua.LState) int { return 0 },
		"onModalWindow": func(L *lua.LState) int {
		ev := checkCreatureEvent(L)
		if fn, ok := L.Get(2).(*lua.LFunction); ok {
			ev.OnModalWindow = fn
		}
		return 0
	},
		"onTextEdit":       func(L *lua.LState) int { return 0 },
		"onHealthChange":   func(L *lua.LState) int { return 0 },
		"onManaChange":     func(L *lua.LState) int { return 0 },
		"onExtendedOpcode": func(L *lua.LState) int { return 0 },
	}
	e.setClassConstructor("CreatureEvent", creatureEventConstructor, methods)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), methods))
	e.L.SetField(mt, "__newindex", e.L.NewFunction(creatureEventNewIndex))
}

func creatureEventConstructor(L *lua.LState) int {
	name := L.CheckString(2) // Arg 1 is the class table, arg 2 is the string name
	ev := &LuaCreatureEvent{
		Name: name,
	}
	ud := L.NewUserData()
	ud.Value = ev
	L.SetMetatable(ud, L.GetTypeMetatable(luaCreatureEventTypeName))
	L.Push(ud)
	return 1
}

func checkCreatureEvent(L *lua.LState) *LuaCreatureEvent {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*LuaCreatureEvent); ok {
		return v
	}
	L.ArgError(1, "CreatureEvent expected")
	return nil
}

func creatureEventNewIndex(L *lua.LState) int {
	ev := checkCreatureEvent(L)
	key := L.CheckString(2)
	val := L.Get(3)

	if key == "onLogin" {
		if fn, ok := val.(*lua.LFunction); ok {
			ev.OnLogin = fn
		}
	} else if key == "onLogout" {
		if fn, ok := val.(*lua.LFunction); ok {
			ev.OnLogout = fn
		}
	} else if key == "onModalWindow" {
		if fn, ok := val.(*lua.LFunction); ok {
			ev.OnModalWindow = fn
		}
	}
	return 0
}

func creatureEventType(L *lua.LState) int {
	L.Push(L.Get(1))
	return 1
}

func (e *Engine) creatureEventRegister(L *lua.LState) int {
	ev := checkCreatureEvent(L)
	if ev.OnLogin != nil {
		e.creatureEventsOnLogin = append(e.creatureEventsOnLogin, ev.OnLogin)
	}
	if ev.OnLogout != nil {
		e.creatureEventsOnLogout = append(e.creatureEventsOnLogout, ev.OnLogout)
	}
	if ev.OnModalWindow != nil {
		e.creatureEventsOnModalWindow = append(e.creatureEventsOnModalWindow, ev.OnModalWindow)
	}
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) ExecuteCreatureOnLogin(player *game.Player) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, fn := range e.creatureEventsOnLogin {
		pUd := e.L.NewUserData()
		pUd.Value = player
		e.L.SetMetatable(pUd, e.L.GetTypeMetatable("Player"))

		e.L.Push(fn)
		e.L.Push(pUd)

		if err := e.L.PCall(1, 1, nil); err != nil {
			e.log.Warn("Error executing CreatureEvent onLogin", "err", err)
			continue
		}

		ret := e.L.Get(-1)
		e.L.Pop(1)

		if luaBool, ok := ret.(lua.LBool); ok {
			if !bool(luaBool) {
				return false
			}
		}
	}
	return true
}

func (e *Engine) ExecuteCreatureOnLogout(player *game.Player) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, fn := range e.creatureEventsOnLogout {
		pUd := e.L.NewUserData()
		pUd.Value = player
		e.L.SetMetatable(pUd, e.L.GetTypeMetatable("Player"))

		e.L.Push(fn)
		e.L.Push(pUd)

		if err := e.L.PCall(1, 1, nil); err != nil {
			e.log.Warn("Error executing CreatureEvent onLogout", "err", err)
			continue
		}

		ret := e.L.Get(-1)
		e.L.Pop(1)

		if luaBool, ok := ret.(lua.LBool); ok {
			if !bool(luaBool) {
				return false
			}
		}
	}
	return true
}

// ExecuteCreatureOnModalWindow fires all registered creature-event onModalWindow
// callbacks. Each receives (player, modalWindowId, buttonId, choiceId).
func (e *Engine) ExecuteCreatureOnModalWindow(player *game.Player, modalWindowID uint32, buttonID uint8, choiceID uint8) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, fn := range e.creatureEventsOnModalWindow {
		pUd := e.L.NewUserData()
		pUd.Value = player
		e.L.SetMetatable(pUd, e.L.GetTypeMetatable("Player"))

		e.L.Push(fn)
		e.L.Push(pUd)
		e.L.Push(lua.LNumber(modalWindowID))
		e.L.Push(lua.LNumber(buttonID))
		e.L.Push(lua.LNumber(choiceID))

		if err := e.L.PCall(5, 0, nil); err != nil {
			e.log.Warn("Error executing CreatureEvent onModalWindow", "err", err)
		}
	}
}
