package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const zoneTypeName = "Zone"

// registerZone implements the Zone class, a port of
// src/lua/functions/core/game/zone_functions.cpp. The method set is exactly the
// nineteen names C++ registers, no more: the previous version of this file invented
// getTiles/hasFlag/isForPlayer/getFlags and was missing addArea (45 datapack uses),
// setRemoveDestination, removePlayers and removeMonsters, and its constructor
// returned nil — which is why it sat unwired behind mockClass("Zone").
//
// Zone objects are backed by *game.Zone from the world registry, so they carry real
// positions: the OTBM per-tile zone ids fill them and `<map>-zones.xml` names them.
func (e *Engine) registerZone() {
	mt := e.L.NewTypeMetatable(zoneTypeName)

	pushList := func(L *lua.LState, n int, push func(i int)) int {
		tbl := L.CreateTable(n, 0)
		for i := 0; i < n; i++ {
			push(i)
			tbl.RawSetInt(i+1, L.Get(-1))
			L.Pop(1)
		}
		L.Push(tbl)
		return 1
	}

	methods := map[string]lua.LGFunction{
		"getName": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LString(z.Name()))
			return 1
		},
		// Zone:addArea(fromPos, toPos), Zone:subtractArea(fromPos, toPos)
		"addArea": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LFalse)
				return 1
			}
			z.AddArea(game.Area{From: checkPosition(L, 2), To: checkPosition(L, 3)})
			L.Push(lua.LTrue)
			return 1
		},
		"subtractArea": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LFalse)
				return 1
			}
			z.SubtractArea(game.Area{From: checkPosition(L, 2), To: checkPosition(L, 3)})
			L.Push(lua.LTrue)
			return 1
		},
		"getRemoveDestination": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LNil)
				return 1
			}
			// Where a creature lands depends on who it is — an unset destination falls
			// back to that player's own temple — so the creature is optional.
			var c game.Creature
			if L.GetTop() >= 2 {
				c = getCreature(L, 2)
			}
			pushPosition(L, z.RemoveDestination(c))
			return 1
		},
		"setRemoveDestination": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LFalse)
				return 1
			}
			z.SetRemoveDestination(checkPosition(L, 2))
			L.Push(lua.LTrue)
			return 1
		},
		"getPositions": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LNil)
				return 1
			}
			positions := z.Positions()
			return pushList(L, len(positions), func(i int) { pushPosition(L, positions[i]) })
		},
		"getCreatures": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LNil)
				return 1
			}
			cs := z.Creatures()
			return pushList(L, len(cs), func(i int) { e.pushCreature(L, cs[i]) })
		},
		"getPlayers": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LNil)
				return 1
			}
			ps := z.Players()
			return pushList(L, len(ps), func(i int) { e.pushCreature(L, ps[i]) })
		},
		"getMonsters": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LNil)
				return 1
			}
			ms := z.Monsters()
			return pushList(L, len(ms), func(i int) { e.pushCreature(L, ms[i]) })
		},
		"getNpcs": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LNil)
				return 1
			}
			ns := z.Npcs()
			return pushList(L, len(ns), func(i int) { e.pushCreature(L, ns[i]) })
		},
		"getItems": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LNil)
				return 1
			}
			its := z.Items()
			return pushList(L, len(its), func(i int) { e.pushItem(L, its[i]) })
		},
		"removePlayers": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LFalse)
				return 1
			}
			z.RemovePlayers()
			L.Push(lua.LTrue)
			return 1
		},
		"removeMonsters": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LFalse)
				return 1
			}
			z.RemoveMonsters()
			L.Push(lua.LTrue)
			return 1
		},
		"removeNpcs": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LFalse)
				return 1
			}
			z.RemoveNpcs()
			L.Push(lua.LTrue)
			return 1
		},
		"refresh": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LFalse)
				return 1
			}
			z.Refresh()
			L.Push(lua.LTrue)
			return 1
		},
		"setMonsterVariant": func(L *lua.LState) int {
			z := checkZone(L)
			if z == nil {
				L.Push(lua.LFalse)
				return 1
			}
			z.SetMonsterVariant(L.CheckString(2))
			L.Push(lua.LTrue)
			return 1
		},
	}
	e.L.SetFuncs(mt, methods)
	e.L.SetField(mt, "__index", mt)
	e.L.SetField(mt, "__eq", e.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(checkZone(L) != nil && checkZone(L) == checkZoneAt(L, 2)))
		return 1
	}))

	// Zone(name): the existing zone, or a new one (luaZoneCreate).
	classTable := e.setClassConstructor(zoneTypeName, func(L *lua.LState) int {
		if e.world == nil || e.world.Zones == nil {
			L.Push(lua.LNil)
			return 1
		}
		// __call puts the class table at index 1, so the name arrives at index 2.
		name := L.CheckString(2)
		z := e.world.Zones.ByName(name)
		if z == nil {
			created, err := e.world.Zones.Add(name, 0)
			if err != nil {
				e.log.Warn("Zone(name) could not create the zone", "name", name, "err", err)
				L.Push(lua.LNil)
				return 1
			}
			z = created
		}
		pushZone(L, z)
		return 1
	}, methods)

	// getByName / getByPosition / getAll are called on the class table, not an
	// instance, so their first argument is the real one.
	e.L.SetField(classTable, "getByName", e.L.NewFunction(func(L *lua.LState) int {
		if e.world == nil || e.world.Zones == nil {
			L.Push(lua.LNil)
			return 1
		}
		z := e.world.Zones.ByName(L.CheckString(1))
		if z == nil {
			L.Push(lua.LNil)
			return 1
		}
		pushZone(L, z)
		return 1
	}))
	e.L.SetField(classTable, "getByPosition", e.L.NewFunction(func(L *lua.LState) int {
		if e.world == nil || e.world.Zones == nil {
			L.Push(lua.LNil)
			return 1
		}
		zones := e.world.Zones.At(checkPosition(L, 1))
		return pushList(L, len(zones), func(i int) { pushZone(L, zones[i]) })
	}))
	e.L.SetField(classTable, "getAll", e.L.NewFunction(func(L *lua.LState) int {
		if e.world == nil || e.world.Zones == nil {
			L.Push(lua.LNil)
			return 1
		}
		zones := e.world.Zones.All()
		return pushList(L, len(zones), func(i int) { pushZone(L, zones[i]) })
	}))
}

// checkZone extracts the *game.Zone behind arg 1.
func checkZone(L *lua.LState) *game.Zone { return checkZoneAt(L, 1) }

func checkZoneAt(L *lua.LState, n int) *game.Zone {
	if ud, ok := L.Get(n).(*lua.LUserData); ok {
		if z, ok := ud.Value.(*game.Zone); ok {
			return z
		}
	}
	return nil
}

func pushZone(L *lua.LState, z *game.Zone) {
	ud := L.NewUserData()
	ud.Value = z
	L.SetMetatable(ud, L.GetTypeMetatable(zoneTypeName))
	L.Push(ud)
}
