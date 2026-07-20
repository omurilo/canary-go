package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/creatures"
	lua "github.com/yuin/gopher-lua"
)

const luaMonsterTypeName = "MonsterType"

func (e *Engine) registerMonsterType() {
	mt := e.L.NewTypeMetatable(luaMonsterTypeName)

	monsterTypeMethods := map[string]lua.LGFunction{
		"name": func(L *lua.LState) int {
			m := checkMonsterType(L)
			L.Push(lua.LString(m.Name))
			return 1
		},
		"register": func(L *lua.LState) int {
			m := checkMonsterType(L)
			table := L.CheckTable(2)
			
			if val := table.RawGetString("health"); val.Type() == lua.LTNumber {
				m.MaxHealth = uint32(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("maxHealth"); val.Type() == lua.LTNumber && m.MaxHealth == 0 {
				m.MaxHealth = uint32(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("speed"); val.Type() == lua.LTNumber {
				m.Speed = uint32(lua.LVAsNumber(val))
			}
			if outfitTable := table.RawGetString("outfit"); outfitTable.Type() == lua.LTTable {
				tb := outfitTable.(*lua.LTable)
				if val := tb.RawGetString("lookType"); val.Type() == lua.LTNumber {
					m.Outfit.LookType = uint16(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookHead"); val.Type() == lua.LTNumber {
					m.Outfit.Head = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookBody"); val.Type() == lua.LTNumber {
					m.Outfit.Body = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookLegs"); val.Type() == lua.LTNumber {
					m.Outfit.Legs = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookFeet"); val.Type() == lua.LTNumber {
					m.Outfit.Feet = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookAddons"); val.Type() == lua.LTNumber {
					m.Outfit.Addons = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookMount"); val.Type() == lua.LTNumber {
					m.Outfit.LookMount = uint16(lua.LVAsNumber(val))
				}
			}

			if e != nil && e.world != nil && e.world.TypeRegistry != nil {
				e.world.TypeRegistry.Monsters[strings.ToLower(m.Name)] = m
			}
			
			L.Push(lua.LTrue)
			return 1
		},
	}
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), monsterTypeMethods))

	// Game.createMonsterType
	gameTable := e.L.GetGlobal("Game")
	if gameTable.Type() == lua.LTTable {
		e.L.SetField(gameTable, "createMonsterType", e.L.NewFunction(func(L *lua.LState) int {
			name := L.CheckString(1)
			mType := &creatures.MonsterType{
				Name: name,
				Speed: 200,
				MaxHealth: 100,
			}
			ud := L.NewUserData()
			ud.Value = mType
			L.SetMetatable(ud, mt)
			L.Push(ud)
			return 1
		}))
	}
}

func checkMonsterType(L *lua.LState) *creatures.MonsterType {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*creatures.MonsterType); ok {
		return v
	}
	L.ArgError(1, "MonsterType expected")
	return nil
}
