package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

// registerZone implements the Zone Lua API. Zone objects are backed by a uint16
// identifier (zone id). All methods return stub/zero values since there is no
// Zone type in the Go game package yet.
func (e *Engine) registerZone() {
	mt := e.L.NewTypeMetatable("Zone")
	methods := map[string]lua.LGFunction{
		"getId":            zoneGetId,
		"getName":          zoneGetName,
		"getPosition":      zoneGetPosition,
		"getMonsters":      zoneGetMonsters,
		"getPlayers":       zoneGetPlayers,
		"getNpcs":          zoneGetNpcs,
		"getCreatures":     zoneGetCreatures,
		"getMonsterCount":  zoneGetMonsterCount,
		"getPlayerCount":   zoneGetPlayerCount,
		"getNpcCount":      zoneGetNpcCount,
		"getCreatureCount": zoneGetCreatureCount,
		"getTiles":         zoneGetTiles,
		"getZones":         zoneGetZones,
		"getParent":        zoneGetParent,
		"isForPlayer":      zoneIsForPlayer,
		"isForMonster":     zoneIsForMonster,
		"isForNpc":         zoneIsForNpc,
		"hasFlag":          zoneHasFlag,
		"getFlags":         zoneGetFlags,
		"getArea":          zoneGetArea,
	}
	e.L.SetFuncs(mt, methods)
	e.L.SetField(mt, "__index", mt)
	e.L.SetField(mt, "__eq", e.L.NewFunction(zoneEq))

	// Zone(id) - constructor stub returning nil for now
	ctor := e.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNil)
		return 1
	})
	e.L.SetGlobal("Zone", ctor)
}

// checkZone extracts the zone id from a Zone userdata at the top of the stack.
func checkZone(L *lua.LState) *uint16 {
	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if id, ok := ud.Value.(uint16); ok {
			return &id
		}
	}
	return nil
}

// pushZone pushes a Zone userdata with the given id onto the Lua stack.
func pushZone(L *lua.LState, id uint16) {
	ud := L.NewUserData()
	ud.Value = id
	L.SetMetatable(ud, L.GetTypeMetatable("Zone"))
	L.Push(ud)
}

// zoneEq compares two Zone userdata by their id.
func zoneEq(L *lua.LState) int {
	id1 := checkZone(L)
	var id2 *uint16
	if ud, ok := L.Get(2).(*lua.LUserData); ok {
		if id, ok := ud.Value.(uint16); ok {
			id2 = &id
		}
	}
	if id1 == nil || id2 == nil {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LBool(*id1 == *id2))
	return 1
}

func zoneGetId(L *lua.LState) int {
	id := checkZone(L)
	if id == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(*id))
	return 1
}

func zoneGetName(L *lua.LState) int {
	L.Push(lua.LString(""))
	return 1
}

func zoneGetPosition(L *lua.LState) int {
	L.Push(lua.LNil)
	return 1
}

func zoneGetMonsters(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func zoneGetPlayers(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func zoneGetNpcs(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func zoneGetCreatures(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func zoneGetMonsterCount(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func zoneGetPlayerCount(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func zoneGetNpcCount(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func zoneGetCreatureCount(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func zoneGetTiles(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func zoneGetZones(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func zoneGetParent(L *lua.LState) int {
	L.Push(lua.LNil)
	return 1
}

func zoneIsForPlayer(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func zoneIsForMonster(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func zoneIsForNpc(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func zoneHasFlag(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func zoneGetFlags(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func zoneGetArea(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}
