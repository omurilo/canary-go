package actions

import (
	lua "github.com/yuin/gopher-lua"
)

const luaActionTypeName = "Action"

// RegisterActionClass registers the Action constructor and metatable to Lua.
func RegisterActionClass(L *lua.LState) {
	mt := L.NewTypeMetatable(luaActionTypeName)
	L.SetGlobal(luaActionTypeName, mt)
	
	// static attributes
	L.SetField(mt, "new", L.NewFunction(actionNew))
	
	// Set Action() to call Action.new()
	mtMetatable := L.NewTable()
	L.SetField(mtMetatable, "__call", L.NewFunction(func(L *lua.LState) int {
		// Action() called
		return actionNew(L)
	}))
	L.SetMetatable(mt, mtMetatable)

	// methods
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), actionMethods))
	L.SetField(mt, "__newindex", L.NewFunction(actionNewIndex))
}

func actionNew(L *lua.LState) int {
	action := &Action{
		ItemIDs:   make([]uint16, 0),
		ActionIDs: make([]uint16, 0),
		UniqueIDs: make([]uint16, 0),
	}
	ud := L.NewUserData()
	ud.Value = action
	L.SetMetatable(ud, L.GetTypeMetatable(luaActionTypeName))
	L.Push(ud)
	return 1
}

func checkAction(L *lua.LState) *Action {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*Action); ok {
		return v
	}
	L.ArgError(1, "action expected")
	return nil
}

var actionMethods = map[string]lua.LGFunction{
	"id":       actionId,
	"aid":      actionAid,
	"uid":      actionUid,
	"register": actionRegister,
}

func actionId(L *lua.LState) int {
	action := checkAction(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		id := uint16(L.CheckNumber(i))
		action.ItemIDs = append(action.ItemIDs, id)
	}
	// return self for chaining
	L.Push(L.Get(1))
	return 1
}

func actionAid(L *lua.LState) int {
	action := checkAction(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		aid := uint16(L.CheckNumber(i))
		action.ActionIDs = append(action.ActionIDs, aid)
	}
	L.Push(L.Get(1))
	return 1
}

func actionUid(L *lua.LState) int {
	action := checkAction(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		uid := uint16(L.CheckNumber(i))
		action.UniqueIDs = append(action.UniqueIDs, uid)
	}
	L.Push(L.Get(1))
	return 1
}

func actionRegister(L *lua.LState) int {
	action := checkAction(L)
	if Engine != nil {
		Engine.Register(action)
	}
	return 0
}

func actionNewIndex(L *lua.LState) int {
	action := checkAction(L)
	key := L.CheckString(2)

	if key == "onUse" {
		fn := L.CheckFunction(3)
		action.OnUse = fn
	} else {
		L.RaiseError("invalid property '%s' on Action", key)
	}
	return 0
}
