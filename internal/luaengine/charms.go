package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/charms"
	lua "github.com/yuin/gopher-lua"
)

const luaCharmName = "Charm"

// checkCharm returns the *charms.Charm behind the first argument's userdata.
func checkCharm(L *lua.LState) *charms.Charm {
	ud := L.CheckUserData(1)
	if c, ok := ud.Value.(*charms.Charm); ok {
		return c
	}
	L.ArgError(1, "Charm expected")
	return nil
}

// readTierArray reads up to 3 numbers from a Lua array into dst.
func readTierArray(t *lua.LTable) [3]uint16 {
	var out [3]uint16
	for i := range uint8(3) {
		if v := t.RawGetInt(int(i) + 1); v.Type() == lua.LTNumber {
			out[i] = uint16(lua.LVAsNumber(v))
		}
	}
	return out
}

// registerCharmType installs the real Charm type (overriding the api.go mock)
// and rewires Game.createBestiaryCharm so the datapack's bestiary_charms.lua
// populates the world charm registry. Mirrors Game.createBestiaryCharm +
// the charm:* setters (charm_functions.cpp) and IOBestiary::getBestiaryCharm.
func (e *Engine) registerCharmType() {
	mt := e.L.NewTypeMetatable(luaCharmName)

	methods := map[string]lua.LGFunction{
		"id": func(L *lua.LState) int {
			L.Push(lua.LNumber(checkCharm(L).ID))
			return 1
		},
		"name": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.Name = L.CheckString(2)
				return 0
			}
			L.Push(lua.LString(c.Name))
			return 1
		},
		"description": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.Description = L.CheckString(2)
				return 0
			}
			L.Push(lua.LString(c.Description))
			return 1
		},
		"category": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.Category = uint8(L.CheckInt(2))
				return 0
			}
			L.Push(lua.LNumber(c.Category))
			return 1
		},
		"type": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.Type = uint8(L.CheckInt(2))
				return 0
			}
			L.Push(lua.LNumber(c.Type))
			return 1
		},
		"damageType": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.DamageType = L.CheckInt(2)
				return 0
			}
			L.Push(lua.LNumber(c.DamageType))
			return 1
		},
		"percentage": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.Percent = float64(L.CheckNumber(2))
				return 0
			}
			L.Push(lua.LNumber(c.Percent))
			return 1
		},
		"chance": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.Chance = readTierArray(L.CheckTable(2))
				return 0
			}
			return 0
		},
		"points": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.Points = readTierArray(L.CheckTable(2))
				return 0
			}
			return 0
		},
		"effect": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.Effect = uint16(L.CheckInt(2))
				return 0
			}
			L.Push(lua.LNumber(c.Effect))
			return 1
		},
		"castSound": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.CastSound = uint16(L.CheckInt(2))
			}
			return 0
		},
		"impactSound": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.ImpactSound = uint16(L.CheckInt(2))
			}
			return 0
		},
		"messageCancel": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.MessageCancel = L.CheckString(2)
			}
			return 0
		},
		"messageServerLog": func(L *lua.LState) int {
			c := checkCharm(L)
			if L.GetTop() >= 2 {
				c.MessageServerLog = L.ToBool(2)
			}
			return 0
		},
		// register(config) applies the whole datapack config table at once,
		// mirroring Charm:register / registerCharm in register_bestiary_charm.lua.
		"register": func(L *lua.LState) int {
			c := checkCharm(L)
			t := L.CheckTable(2)
			if v := t.RawGetString("name"); v.Type() == lua.LTString {
				c.Name = v.String()
			}
			if v := t.RawGetString("description"); v.Type() == lua.LTString {
				c.Description = v.String()
			}
			if v := t.RawGetString("category"); v.Type() == lua.LTNumber {
				c.Category = uint8(lua.LVAsNumber(v))
			}
			if v := t.RawGetString("type"); v.Type() == lua.LTNumber {
				c.Type = uint8(lua.LVAsNumber(v))
			}
			if v := t.RawGetString("damageType"); v.Type() == lua.LTNumber {
				c.DamageType = int(lua.LVAsNumber(v))
			}
			if v := t.RawGetString("percent"); v.Type() == lua.LTNumber {
				c.Percent = float64(lua.LVAsNumber(v))
			}
			if v := t.RawGetString("chance"); v.Type() == lua.LTTable {
				c.Chance = readTierArray(v.(*lua.LTable))
			}
			if v := t.RawGetString("points"); v.Type() == lua.LTTable {
				c.Points = readTierArray(v.(*lua.LTable))
			}
			if v := t.RawGetString("effect"); v.Type() == lua.LTNumber {
				c.Effect = uint16(lua.LVAsNumber(v))
			}
			if v := t.RawGetString("messageCancel"); v.Type() == lua.LTString {
				c.MessageCancel = v.String()
			}
			if v := t.RawGetString("messageServerLog"); v.Type() == lua.LTBool {
				c.MessageServerLog = lua.LVAsBool(v)
			}
			return 0
		},
	}
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), methods))

	// Rewire Game.createBestiaryCharm(id) to build a real charm, add it to the
	// registry immediately (like getBestiaryCharm force=true), and return the
	// userdata the datapack then configures via :register.
	gameTable := e.L.GetGlobal("Game")
	if gameTable.Type() == lua.LTTable {
		e.L.SetField(gameTable, "createBestiaryCharm", e.L.NewFunction(func(L *lua.LState) int {
			id := uint8(L.CheckInt(1))
			c := &charms.Charm{ID: id}
			if e.world != nil && e.world.Charms != nil {
				e.world.Charms.Add(c)
				// Add stores a pointer; re-fetch so mutations hit the stored entry.
				c = e.world.Charms.Get(id)
			}
			ud := L.NewUserData()
			ud.Value = c
			L.SetMetatable(ud, mt)
			L.Push(ud)
			return 1
		}))
	}
}
