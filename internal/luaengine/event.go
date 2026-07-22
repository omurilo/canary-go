package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/events"

	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerEventCallback() {
	L := e.L
	// EventCallback global constructor
	eventCallbackConstructor := L.NewTable()
	eventCallbackMt := L.NewTable()
	L.SetField(eventCallbackMt, "__call", L.NewFunction(eventCallbackCreate))
	L.SetMetatable(eventCallbackConstructor, eventCallbackMt)

	L.SetGlobal("EventCallback", eventCallbackConstructor)
}

func eventCallbackCreate(L *lua.LState) int {
	// EventCallback([name])
	name := L.OptString(2, "")

	t := L.NewTable()
	t.RawSetString("name", lua.LString(name))

	// Provide a register method
	t.RawSetString("register", L.NewFunction(eventCallbackRegister))

	L.Push(t)
	return 1
}

func eventCallbackRegister(L *lua.LState) int {
	callbackTable := L.CheckTable(1) // self
	if events.GlobalEngine != nil {
		events.GlobalEngine.Register(callbackTable)
	}
	L.Push(lua.LBool(true))
	return 1
}
