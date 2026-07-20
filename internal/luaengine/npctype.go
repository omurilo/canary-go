package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/creatures"
	lua "github.com/yuin/gopher-lua"
)

const luaNpcTypeName = "NpcType"

func (e *Engine) registerNpcType() {
	mt := e.L.NewTypeMetatable(luaNpcTypeName)

	npcTypeMethods := map[string]lua.LGFunction{
		"name": func(L *lua.LState) int {
			n := checkNpcType(L)
			L.Push(lua.LString(n.Name))
			return 1
		},
		"register": func(L *lua.LState) int {
			n := checkNpcType(L)
			table := L.CheckTable(2)
			
			if val := table.RawGetString("health"); val.Type() == lua.LTNumber {
				n.MaxHealth = uint32(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("maxHealth"); val.Type() == lua.LTNumber && n.MaxHealth == 0 {
				n.MaxHealth = uint32(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("speed"); val.Type() == lua.LTNumber {
				n.Speed = uint32(lua.LVAsNumber(val))
			}
			if outfitTable := table.RawGetString("outfit"); outfitTable.Type() == lua.LTTable {
				tb := outfitTable.(*lua.LTable)
				if val := tb.RawGetString("lookType"); val.Type() == lua.LTNumber {
					n.Outfit.LookType = uint16(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookHead"); val.Type() == lua.LTNumber {
					n.Outfit.Head = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookBody"); val.Type() == lua.LTNumber {
					n.Outfit.Body = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookLegs"); val.Type() == lua.LTNumber {
					n.Outfit.Legs = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookFeet"); val.Type() == lua.LTNumber {
					n.Outfit.Feet = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookAddons"); val.Type() == lua.LTNumber {
					n.Outfit.Addons = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookMount"); val.Type() == lua.LTNumber {
					n.Outfit.LookMount = uint16(lua.LVAsNumber(val))
				}
			}

			if e != nil && e.world != nil && e.world.TypeRegistry != nil {
				e.world.TypeRegistry.Npcs[strings.ToLower(n.Name)] = n
			}
			
			L.Push(lua.LTrue)
			return 1
		},
	}
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), npcTypeMethods))

	// Game.createNpcType
	gameTable := e.L.GetGlobal("Game")
	if gameTable.Type() == lua.LTTable {
		e.L.SetField(gameTable, "createNpcType", e.L.NewFunction(func(L *lua.LState) int {
			name := L.CheckString(1)
			nType := &creatures.NpcType{
				Name: name,
				Speed: 200,
				MaxHealth: 100,
			}
			ud := L.NewUserData()
			ud.Value = nType
			L.SetMetatable(ud, mt)
			L.Push(ud)
			return 1
		}))
	}
}

func checkNpcType(L *lua.LState) *creatures.NpcType {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*creatures.NpcType); ok {
		return v
	}
	L.ArgError(1, "NpcType expected")
	return nil
}
