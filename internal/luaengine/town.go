package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

const townTypeName = "Town"

// registerTown implements the Lua Town class backed by the world's OTBM towns.
// Town(id) returns a town userdata; getId/getName/getTemplePosition resolve
// against World.TownsByID/TownNames. This powers the citizen/temple "set town"
// movements (data-otservbr-global/scripts/movements/teleport/citizen.lua).
func (e *Engine) registerTown() {
	mt := e.L.NewTypeMetatable(townTypeName)
	methods := map[string]lua.LGFunction{
		"getId": func(L *lua.LState) int {
			L.Push(lua.LNumber(checkTown(L)))
			return 1
		},
		"getName": func(L *lua.LState) int {
			id := checkTown(L)
			name := ""
			if e.world != nil {
				name = e.world.TownNameByID(id)
			}
			L.Push(lua.LString(name))
			return 1
		},
		"getTemplePosition": func(L *lua.LState) int {
			id := checkTown(L)
			if e.world != nil {
				if pos, ok := e.world.TempleByTownID(id); ok {
					pushPosition(L, pos)
					return 1
				}
				pushPosition(L, e.world.DefaultSpawn)
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
	}
	e.L.SetFuncs(mt, methods)
	e.L.SetField(mt, "__index", mt)

	// Town(idOrName) constructor. Returns nil for an unknown town so scripts that do
	// `if not town then return end` behave correctly.
	classTable := e.L.NewTable()
	e.L.SetFuncs(classTable, methods)
	ctorMt := e.L.NewTypeMetatable(townTypeName + "_ClassCtor")
	e.L.SetField(ctorMt, "__call", e.L.NewFunction(func(L *lua.LState) int {
		if e.world == nil {
			L.Push(lua.LNil)
			return 1
		}
		var townID uint16
		found := false
		arg := L.Get(2) // arg 1 is the class table, arg 2 is the parameter passed to Town(...)

		switch arg.Type() {
		case lua.LTNumber:
			id := uint16(lua.LVAsNumber(arg))
			if _, ok := e.world.TempleByTownID(id); ok {
				townID = id
				found = true
			}
		case lua.LTString:
			if id, ok := e.world.TownIDByName(arg.String()); ok {
				townID = id
				found = true
			}
		case lua.LTUserData:
			if id, ok := arg.(*lua.LUserData).Value.(uint16); ok {
				townID = id
				found = true
			}
		}

		if !found {
			L.Push(lua.LNil)
			return 1
		}
		pushTown(L, townID)
		return 1
	}))
	e.L.SetMetatable(classTable, ctorMt)
	e.L.SetGlobal(townTypeName, classTable)
}

// playerSettown sets the player's home town from a Town userdata, updating the
// respawn/login position to that town's temple. Mirrors Player::setTown.
func (e *Engine) playerSettown(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	id := townArgID(L, 2)
	if id != 0 {
		p.TownID = id
		if e.world != nil {
			if pos, ok := e.world.TempleByTownID(id); ok {
				p.LoginPosition = pos
			}
		}
	}
	L.Push(lua.LTrue)
	return 1
}

// playerGettown returns the player's home town as a Town userdata.
func (e *Engine) playerGettown(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	pushTown(L, p.TownID)
	return 1
}

// townArgID extracts a town id from arg n (a Town userdata or a numeric id).
func townArgID(L *lua.LState, n int) uint16 {
	switch v := L.Get(n); v.Type() {
	case lua.LTUserData:
		if id, ok := v.(*lua.LUserData).Value.(uint16); ok {
			return id
		}
	case lua.LTNumber:
		return uint16(lua.LVAsNumber(v))
	}
	return 0
}

func pushTown(L *lua.LState, id uint16) {
	ud := L.NewUserData()
	ud.Value = id
	L.SetMetatable(ud, L.GetTypeMetatable(townTypeName))
	L.Push(ud)
}

// checkTown returns the town id from arg 1.
func checkTown(L *lua.LState) uint16 {
	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if id, ok := ud.Value.(uint16); ok {
			return id
		}
	}
	return 0
}
