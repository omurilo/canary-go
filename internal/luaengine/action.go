package luaengine

import (
	"log/slog"
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
	a := actions.New()
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
	"position":    actionPosition,
	"allowFarUse": actionAllowFarUse,
	"blockWalls":  actionBlockWalls,
	"checkFloor":  actionCheckFloor,
	"onUse":       actionOnUse,
	"register":    actionRegister,
}

func actionPosition(L *lua.LState) int {
	a := checkAction(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		if L.Get(i) == lua.LNil {
			continue
		}
		a.Positions = append(a.Positions, checkPosition(L, i))
	}
	L.Push(L.Get(1))
	return 1
}

// actionBlockWalls is action:blockWalls(bool) — Action::setCheckLineOfSight.
// The Lua name and the field name disagree upstream too.
func actionBlockWalls(L *lua.LState) int {
	a := checkAction(L)
	a.CheckLineOfSight = L.GetTop() < 2 || L.CheckBool(2)
	L.Push(L.Get(1))
	return 1
}

// actionCheckFloor is action:checkFloor(bool) — Action::setCheckFloor. Both of
// these used to discard the argument, so a script turning a check off was
// silently ignored and one turning it on got no protection either.
func actionCheckFloor(L *lua.LState) int {
	a := checkAction(L)
	a.CheckFloor = L.GetTop() < 2 || L.CheckBool(2)
	L.Push(L.Get(1))
	return 1
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
		if L.Get(i) == lua.LNil {
			continue
		}
		a.ItemIDs = append(a.ItemIDs, uint16(L.CheckInt(i)))
	}
	L.Push(L.Get(1))
	return 1
}

func actionAid(L *lua.LState) int {
	a := checkAction(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		if L.Get(i) == lua.LNil {
			continue
		}
		a.ActionIDs = append(a.ActionIDs, uint16(L.CheckInt(i)))
	}
	L.Push(L.Get(1))
	return 1
}

func actionUid(L *lua.LState) int {
	a := checkAction(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		if L.Get(i) == lua.LNil {
			continue
		}
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

	if fromPos.X == 0xFFFF {
		fromPos = player.GetPosition()
	}
	if toPos.X == 0xFFFF {
		toPos = player.GetPosition()
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
			// A nil *game.Item still makes `target != nil` true — the interface holds a
			// type — so this case used to wrap a nil item and hand Lua an Item that
			// answers to nothing.
			if t == nil {
				break
			}
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
	// Substituting a placeholder item for "no target" is how a failed lookup turns
	// into a silent no-op: exercise_training_weapons.lua opens with
	//
	//   if not target or type(target) ~= "userdata" or not target:isItem() then return true end
	//
	// which is written to notice a missing target — and then asks isDummy(getId()).
	// Handed item id 1 it sails past the guard, fails the dummy check and returns
	// with no message at all, which is exactly what using a wand on a dummy does.
	//
	// The placeholder stays, because scripts that index target unconditionally would
	// otherwise error, but it is logged: a real target that failed to resolve is a
	// bug upstream of here and should not be silent.
	if targetUd == lua.LNil {
		slog.Default().Warn("action target did not resolve",
			"item", item.ID, "toX", toPos.X, "toY", toPos.Y, "toZ", toPos.Z)
		dummyItem := &game.Item{ID: 1, Count: 0}
		ud := L.NewUserData()
		ud.Value = luaItem{item: dummyItem, pos: toPos}
		L.SetMetatable(ud, L.GetTypeMetatable("Item"))
		targetUd = ud
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
