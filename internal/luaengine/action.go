package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/actions"
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const luaActionTypeName = "Action"

// registerAction registers the Action global constructor and metatable
func (e *Engine) registerAction() {
	mt := e.L.NewTypeMetatable(luaActionTypeName)
	e.setClassConstructor("Action", actionConstructor, actionMethods)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), actionMethods))
	e.L.SetField(mt, "__newindex", e.L.NewFunction(actionNewIndex))
}

func actionConstructor(L *lua.LState) int {
	a := &actions.Action{}
	ud := L.NewUserData()
	ud.Value = a
	L.SetMetatable(ud, L.GetTypeMetatable(luaActionTypeName))
	L.Push(ud)
	return 1
}

var actionMethods = map[string]lua.LGFunction{
	"id":          actionId,
	"aid":         actionAid,
	"uid":         actionUid,
	"allowFarUse": actionAllowFarUse,
	"onUse":       actionOnUse,
	"register":    actionRegister,
}

func checkAction(L *lua.LState) *actions.Action {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*actions.Action); ok {
		return v
	}
	L.ArgError(1, "Action expected")
	return nil
}

func actionNewIndex(L *lua.LState) int {
	a := checkAction(L)
	key := L.CheckString(2)
	val := L.CheckAny(3)

	if key == "onUse" {
		a.OnUse = val.(*lua.LFunction)
	}
	return 0
}

func actionId(L *lua.LState) int {
	a := checkAction(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		a.ItemIDs = append(a.ItemIDs, uint16(L.CheckInt(i)))
	}
	L.Push(L.Get(1))
	return 1
}

func actionAid(L *lua.LState) int {
	a := checkAction(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		a.ActionIDs = append(a.ActionIDs, uint16(L.CheckInt(i)))
	}
	L.Push(L.Get(1))
	return 1
}

func actionUid(L *lua.LState) int {
	a := checkAction(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		a.UniqueIDs = append(a.UniqueIDs, uint16(L.CheckInt(i)))
	}
	L.Push(L.Get(1))
	return 1
}

func actionRegister(L *lua.LState) int {
	a := checkAction(L)
	actions.Register(a)
	L.Push(lua.LTrue)
	return 1
}

func actionAllowFarUse(L *lua.LState) int {
	a := checkAction(L)
	if L.GetTop() >= 2 {
		a.AllowFarUse = L.CheckBool(2)
	} else {
		a.AllowFarUse = true
	}
	L.Push(L.Get(1))
	return 1
}

func actionOnUse(L *lua.LState) int {
	a := checkAction(L)
	if L.GetTop() >= 2 {
		if fn, ok := L.Get(2).(*lua.LFunction); ok {
			a.OnUse = fn
		}
	}
	L.Push(L.Get(1))
	return 1
}

// CallAction executes the action's OnUseFunc.
func (e *Engine) CallAction(a *actions.Action, player *game.Player, item *game.Item, fromPos game.Position, target any, toPos game.Position, isHotkey bool) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	L := e.L
	if a.OnUse == nil || a.OnUse.Type() != lua.LTFunction {
		return false
	}

	// args: (player, item, fromPosition, target, toPosition, isHotkey)
	playerUd := L.NewUserData()
	playerUd.Value = player
	L.SetMetatable(playerUd, L.GetTypeMetatable("Player"))

	itemUd := L.NewUserData()
	itemUd.Value = luaItem{item: item, pos: fromPos}
	L.SetMetatable(itemUd, L.GetTypeMetatable("Item"))

	fromPosUd := L.NewUserData()
	fromPosUd.Value = fromPos
	L.SetMetatable(fromPosUd, L.GetTypeMetatable("Position"))

	var targetUd lua.LValue = lua.LNil
	if target != nil {
		switch t := target.(type) {
		case *game.Item:
			ud := L.NewUserData()
			ud.Value = luaItem{item: t, pos: toPos}
			L.SetMetatable(ud, L.GetTypeMetatable("Item"))
			targetUd = ud
		case game.Creature:
			ud := L.NewUserData()
			ud.Value = t
			L.SetMetatable(ud, L.GetTypeMetatable(metatableForCreature(t)))
			targetUd = ud
		}
	}

	toPosUd := L.NewUserData()
	toPosUd.Value = toPos
	L.SetMetatable(toPosUd, L.GetTypeMetatable("Position"))

	err := L.CallByParam(lua.P{Fn: a.OnUse, NRet: 1, Protect: true},
		playerUd, itemUd, fromPosUd, targetUd, toPosUd, lua.LBool(isHotkey))
	if err != nil {
		e.log.Error("action error", "err", err)
		return false
	}
	
	ret := L.Get(-1)
	L.Pop(1)
	if lua.LVIsFalse(ret) {
		return false
	}
	return true
}
