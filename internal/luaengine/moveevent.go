package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/moveevents"
	lua "github.com/yuin/gopher-lua"
)

const luaMoveEventTypeName = "MoveEvent"

func (e *Engine) registerMoveEvent() {
	mt := e.L.NewTypeMetatable(luaMoveEventTypeName)
	e.setClassConstructor("MoveEvent", moveEventConstructor, moveEventMethods)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), moveEventMethods))
	e.L.SetField(mt, "__newindex", e.L.NewFunction(moveEventNewIndex))
}

func moveEventConstructor(L *lua.LState) int {
	m := &moveevents.MoveEvent{}
	ud := L.NewUserData()
	ud.Value = m
	L.SetMetatable(ud, L.GetTypeMetatable(luaMoveEventTypeName))
	L.Push(ud)
	return 1
}

var moveEventMethods = map[string]lua.LGFunction{
	"type":         moveEventType,
	"id":           moveEventId,
	"aid":          moveEventAid,
	"uid":          moveEventUid,
	"onStepIn":     moveEventOnStepIn,
	"onStepOut":    moveEventOnStepOut,
	"onEquip":      moveEventNoOp,
	"onDeEquip":    moveEventNoOp,
	"onAddItem":    moveEventNoOp,
	"onRemoveItem": moveEventNoOp,
	"register":     moveEventRegister,
}

func checkMoveEvent(L *lua.LState) *moveevents.MoveEvent {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*moveevents.MoveEvent); ok {
		return v
	}
	L.ArgError(1, "MoveEvent expected")
	return nil
}

func moveEventNewIndex(L *lua.LState) int {
	m := checkMoveEvent(L)
	key := L.CheckString(2)
	val := L.CheckAny(3)

	if key == "onStepIn" {
		m.OnStepIn = val.(*lua.LFunction)
	} else if key == "onStepOut" {
		m.OnStepOut = val.(*lua.LFunction)
	}
	return 0
}

func moveEventType(L *lua.LState) int {
	m := checkMoveEvent(L)
	m.Type = L.CheckString(2)
	L.Push(L.Get(1))
	return 1
}

func moveEventId(L *lua.LState) int {
	m := checkMoveEvent(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		m.ItemIDs = append(m.ItemIDs, uint16(L.CheckInt(i)))
	}
	L.Push(L.Get(1))
	return 1
}

func moveEventAid(L *lua.LState) int {
	m := checkMoveEvent(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		m.ActionIDs = append(m.ActionIDs, uint16(L.CheckInt(i)))
	}
	L.Push(L.Get(1))
	return 1
}

func moveEventUid(L *lua.LState) int {
	m := checkMoveEvent(L)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		m.UniqueIDs = append(m.UniqueIDs, uint16(L.CheckInt(i)))
	}
	L.Push(L.Get(1))
	return 1
}

func moveEventOnStepIn(L *lua.LState) int {
	m := checkMoveEvent(L)
	if L.GetTop() >= 2 {
		if fn, ok := L.Get(2).(*lua.LFunction); ok {
			m.OnStepIn = fn
		}
	}
	L.Push(L.Get(1))
	return 1
}

func moveEventOnStepOut(L *lua.LState) int {
	m := checkMoveEvent(L)
	if L.GetTop() >= 2 {
		if fn, ok := L.Get(2).(*lua.LFunction); ok {
			m.OnStepOut = fn
		}
	}
	L.Push(L.Get(1))
	return 1
}

func moveEventNoOp(L *lua.LState) int {
	L.Push(L.Get(1))
	return 1
}

func moveEventRegister(L *lua.LState) int {
	m := checkMoveEvent(L)
	moveevents.Register(m)
	L.Push(lua.LTrue)
	return 1
}

// CallStepIn executes the move event's OnStepIn func.
func (e *Engine) CallStepIn(m *moveevents.MoveEvent, creature game.Creature, item *game.Item, pos game.Position, fromPos game.Position) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	L := e.L
	if m.OnStepIn == nil || m.OnStepIn.Type() != lua.LTFunction {
		return false
	}

	// Wrap the creature with its CONCRETE type's metatable so type predicates
	// resolve — StepIn scripts commonly do `creature:getPlayer()` which is
	// `self:isPlayer() and self or nil`; with the generic "Creature" metatable
	// isPlayer() is absent and getPlayer() returns nil, silently aborting the
	// script (this is why the citizen/temple set-town movement did nothing).
	creatureUd := L.NewUserData()
	creatureUd.Value = creature
	L.SetMetatable(creatureUd, L.GetTypeMetatable(metatableForCreature(creature)))

	itemUd := L.NewUserData()
	itemUd.Value = luaItem{item: item, pos: pos}
	L.SetMetatable(itemUd, L.GetTypeMetatable("Item"))

	posUd := L.NewUserData()
	posUd.Value = pos
	L.SetMetatable(posUd, L.GetTypeMetatable("Position"))

	fromPosUd := L.NewUserData()
	fromPosUd.Value = fromPos
	L.SetMetatable(fromPosUd, L.GetTypeMetatable("Position"))

	err := L.CallByParam(lua.P{Fn: m.OnStepIn, NRet: 1, Protect: true},
		creatureUd, itemUd, posUd, fromPosUd)
	if err != nil {
		e.log.Error("moveevent stepin error", "err", err)
		return false
	}
	
	ret := L.Get(-1)
	L.Pop(1)
	if lua.LVIsFalse(ret) {
		return false
	}
	return true
}
