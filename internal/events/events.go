package events

import (
	"fmt"
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

type Engine struct {
	OnLogin          []lua.LValue
	OnLook           []lua.LValue
	OnMoveItem       []lua.LValue
	OnGainExperience []lua.LValue
	OnDeath          []lua.LValue
	L                *lua.LState
}

var GlobalEngine *Engine

func NewEngine(L *lua.LState) *Engine {
	e := &Engine{L: L}
	GlobalEngine = e
	return e
}

func (e *Engine) Register(callbackTable *lua.LTable) {
	if val := callbackTable.RawGetString("onLogin"); val != lua.LNil {
		e.OnLogin = append(e.OnLogin, val)
	}
	if val := callbackTable.RawGetString("playerOnLook"); val != lua.LNil {
		e.OnLook = append(e.OnLook, val)
	}
	// Fallback/alias
	if val := callbackTable.RawGetString("onLook"); val != lua.LNil {
		e.OnLook = append(e.OnLook, val)
	}
	if val := callbackTable.RawGetString("onMoveItem"); val != lua.LNil {
		e.OnMoveItem = append(e.OnMoveItem, val)
	}
	if val := callbackTable.RawGetString("onGainExperience"); val != lua.LNil {
		e.OnGainExperience = append(e.OnGainExperience, val)
	}
	if val := callbackTable.RawGetString("onDeath"); val != lua.LNil {
		e.OnDeath = append(e.OnDeath, val)
	}
}

func (e *Engine) ExecuteOnLogin(player *game.Player) bool {
	L := e.L
	for _, fn := range e.OnLogin {
		L.Push(fn)

		pUd := L.NewUserData()
		pUd.Value = player
		L.SetMetatable(pUd, L.GetTypeMetatable("Player"))
		L.Push(pUd)

		if err := L.PCall(1, 1, nil); err != nil {
			fmt.Printf("Lua execution error in onLogin: %v\n", err)
			continue
		}

		ret := L.Get(-1)
		L.Pop(1)

		if luaBool, ok := ret.(lua.LBool); ok {
			if !bool(luaBool) {
				return false
			}
		}
	}
	return true
}

func (e *Engine) ExecuteOnLook(player *game.Player, thing interface{}, position game.Position, distance int) bool {
	L := e.L
	for _, fn := range e.OnLook {
		L.Push(fn)

		pUd := L.NewUserData()
		pUd.Value = player
		L.SetMetatable(pUd, L.GetTypeMetatable("Player"))
		L.Push(pUd)

		tUd := L.NewUserData()
		tUd.Value = thing
		if _, ok := thing.(*game.Item); ok {
			L.SetMetatable(tUd, L.GetTypeMetatable("Item"))
		} else {
			L.SetMetatable(tUd, L.GetTypeMetatable("Thing"))
		}
		L.Push(tUd)

		pPosUd := L.NewUserData()
		pPosUd.Value = position
		L.SetMetatable(pPosUd, L.GetTypeMetatable("Position"))
		L.Push(pPosUd)

		L.Push(lua.LNumber(distance))

		if err := L.PCall(4, 1, nil); err != nil {
			fmt.Printf("Lua execution error in onLook: %v\n", err)
			continue
		}

		ret := L.Get(-1)
		L.Pop(1)

		if luaBool, ok := ret.(lua.LBool); ok {
			if !bool(luaBool) {
				return false
			}
		}
	}
	return true
}

func (e *Engine) ExecuteOnMoveItem(player *game.Player, item *game.Item, count uint16, fromPos game.Position, toPos game.Position) bool {
	L := e.L
	for _, fn := range e.OnMoveItem {
		L.Push(fn)

		pUd := L.NewUserData()
		pUd.Value = player
		L.SetMetatable(pUd, L.GetTypeMetatable("Player"))
		L.Push(pUd)

		iUd := L.NewUserData()
		iUd.Value = item
		L.SetMetatable(iUd, L.GetTypeMetatable("Item"))
		L.Push(iUd)
		
		L.Push(lua.LNumber(count))

		fPosUd := L.NewUserData()
		fPosUd.Value = fromPos
		L.SetMetatable(fPosUd, L.GetTypeMetatable("Position"))
		L.Push(fPosUd)

		tPosUd := L.NewUserData()
		tPosUd.Value = toPos
		L.SetMetatable(tPosUd, L.GetTypeMetatable("Position"))
		L.Push(tPosUd)

		if err := L.PCall(5, 1, nil); err != nil {
			fmt.Printf("Lua execution error in onMoveItem: %v\n", err)
			continue
		}

		ret := L.Get(-1)
		L.Pop(1)

		if luaBool, ok := ret.(lua.LBool); ok {
			if !bool(luaBool) {
				return false
			}
		}
	}
	return true
}

func (e *Engine) ExecuteOnGainExperience(player *game.Player, source game.Creature, exp uint64, rawExp uint64) uint64 {
	L := e.L
	finalExp := exp
	for _, fn := range e.OnGainExperience {
		L.Push(fn)

		pUd := L.NewUserData()
		pUd.Value = player
		L.SetMetatable(pUd, L.GetTypeMetatable("Player"))
		L.Push(pUd)

		if source != nil {
			sUd := L.NewUserData()
			sUd.Value = source
			L.SetMetatable(sUd, L.GetTypeMetatable("Creature"))
			L.Push(sUd)
		} else {
			L.Push(lua.LNil)
		}

		L.Push(lua.LNumber(finalExp))
		L.Push(lua.LNumber(rawExp))

		if err := L.PCall(4, 1, nil); err != nil {
			fmt.Printf("Lua execution error in onGainExperience: %v\n", err)
			continue
		}

		ret := L.Get(-1)
		L.Pop(1)

		if num, ok := ret.(lua.LNumber); ok {
			finalExp = uint64(num)
		}
	}
	return finalExp
}

func (e *Engine) ExecuteOnDeath(player *game.Player, killer game.Creature) bool {
	L := e.L
	for _, fn := range e.OnDeath {
		L.Push(fn)

		pUd := L.NewUserData()
		pUd.Value = player
		L.SetMetatable(pUd, L.GetTypeMetatable("Player"))
		L.Push(pUd)
		
		if killer != nil {
			kUd := L.NewUserData()
			kUd.Value = killer
			L.SetMetatable(kUd, L.GetTypeMetatable("Creature"))
			L.Push(kUd)
		} else {
			L.Push(lua.LNil)
		}

		if err := L.PCall(2, 1, nil); err != nil {
			fmt.Printf("Lua execution error in onDeath: %v\n", err)
			continue
		}

		ret := L.Get(-1)
		L.Pop(1)

		if luaBool, ok := ret.(lua.LBool); ok {
			if !bool(luaBool) {
				return false
			}
		}
	}
	return true
}
