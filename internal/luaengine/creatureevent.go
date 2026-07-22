package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const luaCreatureEventTypeName = "CreatureEvent"

type LuaCreatureEvent struct {
	Name     string
	OnLogin  *lua.LFunction
	OnLogout *lua.LFunction
}

// registerCreatureEvent registers the CreatureEvent global constructor and metatable
func (e *Engine) registerCreatureEvent() {
	mt := e.L.NewTypeMetatable(luaCreatureEventTypeName)
	e.setClassConstructor("CreatureEvent", creatureEventConstructor, map[string]lua.LGFunction{
		"register": e.creatureEventRegister,
		"type":     creatureEventType,
	})
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), map[string]lua.LGFunction{
		"register": e.creatureEventRegister,
		"type":     creatureEventType,
	}))
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
