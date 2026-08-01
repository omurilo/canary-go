package luaengine

import (
	"github.com/omurilo/canary-go/internal/events"

	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerEventCallback() {
	L := e.L
	// EventCallback global constructor
	eventCallbackConstructor := L.NewTable()
	// Must be a TYPE metatable: revscriptsys.lua:121 does
	// rawgetmetatable("EventCallback").__newindex = ..., which looks the name up in
	// the type registry. A plain table is invisible there, so that assignment hit
	// nil and aborted revscriptsys — taking the rest of the file with it.
	eventCallbackMt := L.NewTypeMetatable("EventCallback")
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
