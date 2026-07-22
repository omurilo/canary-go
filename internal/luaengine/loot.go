package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/creatures"
	lua "github.com/yuin/gopher-lua"
)

const luaLootTypeName = "Loot"

type luaLoot struct {
	Block creatures.LootBlock
}

func (e *Engine) registerLootClass() {
	mt := e.L.NewTypeMetatable(luaLootTypeName)

	lootMethods := map[string]lua.LGFunction{
		"setId": func(L *lua.LState) int {
			l := checkLoot(L)
			l.Block.ID = uint16(L.CheckInt(2))
			return 0
		},
		"setIdFromName": func(L *lua.LState) int {
			l := checkLoot(L)
			name := L.CheckString(2)
			l.Block.Name = name
			L.Push(lua.LTrue)
			return 1
		},
		"setChance": func(L *lua.LState) int {
			l := checkLoot(L)
			l.Block.Chance = uint32(L.CheckInt(2))
			return 0
		},
		"setMinCount": func(L *lua.LState) int {
			l := checkLoot(L)
			l.Block.CountMin = uint32(L.CheckInt(2))
			return 0
		},
		"setMaxCount": func(L *lua.LState) int {
			l := checkLoot(L)
			l.Block.CountMax = uint32(L.CheckInt(2))
			return 0
		},
		"addChildLoot": func(L *lua.LState) int {
			l := checkLoot(L)
			child := checkLootAt(L, 2)
			l.Block.ChildLoot = append(l.Block.ChildLoot, child.Block)
			return 0
		},
		"setActionId": func(L *lua.LState) int { return 0 },
		"setText": func(L *lua.LState) int { return 0 },
		"setNameItem": func(L *lua.LState) int { return 0 },
		"setArticle": func(L *lua.LState) int { return 0 },
		"setAttack": func(L *lua.LState) int { return 0 },
		"setDefense": func(L *lua.LState) int { return 0 },
		"setExtraDefense": func(L *lua.LState) int { return 0 },
		"setArmor": func(L *lua.LState) int { return 0 },
		"setShootRange": func(L *lua.LState) int { return 0 },
		"setUnique": func(L *lua.LState) int { return 0 },
		"setSubType": func(L *lua.LState) int { return 0 },
	}

	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), lootMethods))

	// Global Loot constructor
	e.L.SetGlobal("Loot", e.L.NewFunction(func(L *lua.LState) int {
		l := &luaLoot{
			Block: creatures.LootBlock{
				CountMin: 1,
				CountMax: 1,
			},
		}
		ud := L.NewUserData()
		ud.Value = l
		L.SetMetatable(ud, mt)
		L.Push(ud)
		return 1
	}))
}

func checkLoot(L *lua.LState) *luaLoot {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*luaLoot); ok {
		return v
	}
	L.ArgError(1, "Loot expected")
	return nil
}

func checkLootAt(L *lua.LState, n int) *luaLoot {
	ud := L.CheckUserData(n)
	if v, ok := ud.Value.(*luaLoot); ok {
		return v
	}
	L.ArgError(n, "Loot expected")
	return nil
}
