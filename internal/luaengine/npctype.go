package luaengine

import (
	"strings"

	"github.com/omurilo/canary-go/internal/creatures"
	lua "github.com/yuin/gopher-lua"
)

const luaNpcTypeName = "NpcType"

func (e *Engine) registerNpcType() {
	mt := e.L.NewTypeMetatable(luaNpcTypeName)

	// There is no C++ NpcType::register. The 1033 npcConfig tables in the datapack
	// are applied by data/scripts/lib/register_npc_type.lua, which assigns
	// NpcType.register in Lua and calls one setter per field.
	//
	// Go used to carry a second, independent implementation of that shim written in
	// Go — a table reader on this map. Two readers of the same config is exactly the
	// divergence this port keeps paying for: the Go one knew nothing of sounds,
	// light, events or nested child shops, and any field added upstream would land
	// in the Lua shim and be silently dropped here. It is gone; the Lua shim is the
	// only path, as in C++.
	npcTypeMethods := map[string]lua.LGFunction{
		"addShopItem": func(L *lua.LState) int {
			n := checkNpcType(L)
			ud := L.CheckUserData(2)
			if shopItem, ok := ud.Value.(*creatures.ShopItem); ok {
				n.ShopItems = append(n.ShopItems, *shopItem)
			}
			L.Push(lua.LTrue)
			return 1
		},
	}

	// Data setters, ported from src/lua/functions/creatures/npc/npc_type_functions.cpp.
	//
	// C++ exposes every npcConfig field as a METHOD, and data/scripts/lib/
	// register_npc_type.lua is what turns the config table into those calls. Go had
	// only the table reader, so a script written in the upstream style —
	// `npcType:walkRadius(2)` — indexed a nil and died. These make both forms work;
	// removing the table reader, so there is exactly one path as in C++, is the
	// step that has to come after the shim is loading and covered by a test.
	//
	// All follow the upstream shape: no argument reads, one argument writes.
	numSetter := func(get func(*creatures.NpcType) lua.LNumber, set func(*creatures.NpcType, lua.LNumber)) lua.LGFunction {
		return func(L *lua.LState) int {
			n := checkNpcType(L)
			if n == nil {
				L.Push(lua.LNil)
				return 1
			}
			if L.GetTop() >= 2 {
				set(n, lua.LVAsNumber(L.Get(2)))
				L.Push(lua.LTrue)
				return 1
			}
			L.Push(get(n))
			return 1
		}
	}
	boolSetter := func(get func(*creatures.NpcType) bool, set func(*creatures.NpcType, bool)) lua.LGFunction {
		return func(L *lua.LState) int {
			n := checkNpcType(L)
			if n == nil {
				L.Push(lua.LNil)
				return 1
			}
			if L.GetTop() >= 2 {
				set(n, lua.LVAsBool(L.Get(2)))
				L.Push(lua.LTrue)
				return 1
			}
			L.Push(lua.LBool(get(n)))
			return 1
		}
	}

	strSetter := func(get func(*creatures.NpcType) string, set func(*creatures.NpcType, string)) lua.LGFunction {
		return func(L *lua.LState) int {
			n := checkNpcType(L)
			if n == nil {
				L.Push(lua.LNil)
				return 1
			}
			if L.GetTop() >= 2 {
				set(n, L.CheckString(2))
				L.Push(lua.LTrue)
				return 1
			}
			L.Push(lua.LString(get(n)))
			return 1
		}
	}

	npcTypeMethods["name"] = strSetter(
		func(n *creatures.NpcType) string { return n.Name },
		func(n *creatures.NpcType, v string) { n.Name = v })
	npcTypeMethods["nameDescription"] = strSetter(
		func(n *creatures.NpcType) string { return n.Description },
		func(n *creatures.NpcType, v string) { n.Description = v })

	// race and getName are NOT C++ NpcType methods. register_npc_type.lua calls
	// both anyway — upstream added getName with the comment "Assuming npcType has
	// a getName method" (canary 0f8929d61) and it does not. Without them here the
	// shim's shop parser dies on the first merchant and the type registers with no
	// catalog, so they exist to make the datapack's own code run. The divergence is
	// a superset, deliberate, and noted so nobody "fixes" it back.
	npcTypeMethods["race"] = strSetter(
		func(n *creatures.NpcType) string { return n.Race },
		func(n *creatures.NpcType, v string) { n.Race = v })
	npcTypeMethods["getName"] = func(L *lua.LState) int {
		n := checkNpcType(L)
		if n == nil {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(n.Name))
		return 1
	}

	npcTypeMethods["health"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.Health) },
		func(n *creatures.NpcType, v lua.LNumber) { n.Health = uint32(v) })
	npcTypeMethods["maxHealth"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.MaxHealth) },
		func(n *creatures.NpcType, v lua.LNumber) { n.MaxHealth = uint32(v) })
	npcTypeMethods["canSpawn"] = boolSetter(
		func(n *creatures.NpcType) bool { return n.CanSpawn },
		func(n *creatures.NpcType, v bool) { n.CanSpawn = v })

	// outfit(table) / outfit() -> table, as luaNpcTypeOutfit does.
	npcTypeMethods["outfit"] = func(L *lua.LState) int {
		n := checkNpcType(L)
		if n == nil {
			L.Push(lua.LNil)
			return 1
		}
		if L.GetTop() >= 2 {
			tb := L.CheckTable(2)
			num := func(key string, dst func(lua.LNumber)) {
				if v := tb.RawGetString(key); v.Type() == lua.LTNumber {
					dst(lua.LVAsNumber(v))
				}
			}
			num("lookType", func(v lua.LNumber) { n.Outfit.LookType = uint16(v) })
			num("lookTypeEx", func(v lua.LNumber) { n.Outfit.LookTypeEx = uint16(v) })
			num("lookHead", func(v lua.LNumber) { n.Outfit.Head = uint8(v) })
			num("lookBody", func(v lua.LNumber) { n.Outfit.Body = uint8(v) })
			num("lookLegs", func(v lua.LNumber) { n.Outfit.Legs = uint8(v) })
			num("lookFeet", func(v lua.LNumber) { n.Outfit.Feet = uint8(v) })
			num("lookAddons", func(v lua.LNumber) { n.Outfit.Addons = uint8(v) })
			num("lookMount", func(v lua.LNumber) { n.Outfit.LookMount = uint16(v) })
			L.Push(lua.LTrue)
			return 1
		}
		out := L.NewTable()
		L.SetField(out, "lookType", lua.LNumber(n.Outfit.LookType))
		L.SetField(out, "lookTypeEx", lua.LNumber(n.Outfit.LookTypeEx))
		L.SetField(out, "lookHead", lua.LNumber(n.Outfit.Head))
		L.SetField(out, "lookBody", lua.LNumber(n.Outfit.Body))
		L.SetField(out, "lookLegs", lua.LNumber(n.Outfit.Legs))
		L.SetField(out, "lookFeet", lua.LNumber(n.Outfit.Feet))
		L.SetField(out, "lookAddons", lua.LNumber(n.Outfit.Addons))
		L.SetField(out, "lookMount", lua.LNumber(n.Outfit.LookMount))
		L.Push(out)
		return 1
	}

	npcTypeMethods["baseSpeed"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.Speed) },
		func(n *creatures.NpcType, v lua.LNumber) { n.Speed = uint32(v) })
	npcTypeMethods["walkInterval"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.WalkInterval) },
		func(n *creatures.NpcType, v lua.LNumber) { n.WalkInterval = uint32(v) })
	npcTypeMethods["walkRadius"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.WalkRadius) },
		func(n *creatures.NpcType, v lua.LNumber) { n.WalkRadius = int32(v) })
	npcTypeMethods["speechBubble"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.SpeechBubble) },
		func(n *creatures.NpcType, v lua.LNumber) { n.SpeechBubble = uint8(v) })
	npcTypeMethods["currency"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.CurrencyID) },
		func(n *creatures.NpcType, v lua.LNumber) { n.CurrencyID = uint16(v) })
	npcTypeMethods["yellChance"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.YellChance) },
		func(n *creatures.NpcType, v lua.LNumber) { n.YellChance = uint32(v) })
	npcTypeMethods["yellSpeedTicks"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.YellInterval) },
		func(n *creatures.NpcType, v lua.LNumber) { n.YellInterval = uint32(v) })
	npcTypeMethods["soundChance"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.SoundChance) },
		func(n *creatures.NpcType, v lua.LNumber) { n.SoundChance = uint32(v) })
	npcTypeMethods["soundSpeedTicks"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.SoundSpeedTicks) },
		func(n *creatures.NpcType, v lua.LNumber) { n.SoundSpeedTicks = uint32(v) })
	npcTypeMethods["respawnTypePeriod"] = numSetter(
		func(n *creatures.NpcType) lua.LNumber { return lua.LNumber(n.RespawnType.Period) },
		func(n *creatures.NpcType, v lua.LNumber) { n.RespawnType.Period = int32(v) })

	npcTypeMethods["floorChange"] = boolSetter(
		func(n *creatures.NpcType) bool { return n.FloorChange },
		func(n *creatures.NpcType, v bool) { n.FloorChange = v })
	npcTypeMethods["canPushItems"] = boolSetter(
		func(n *creatures.NpcType) bool { return n.CanPushItems },
		func(n *creatures.NpcType, v bool) { n.CanPushItems = v })
	npcTypeMethods["canPushCreatures"] = boolSetter(
		func(n *creatures.NpcType) bool { return n.CanPushCreatures },
		func(n *creatures.NpcType, v bool) { n.CanPushCreatures = v })
	npcTypeMethods["isPushable"] = boolSetter(
		func(n *creatures.NpcType) bool { return n.IsPushable },
		func(n *creatures.NpcType, v bool) { n.IsPushable = v })
	npcTypeMethods["respawnTypeIsUnderground"] = boolSetter(
		func(n *creatures.NpcType) bool { return n.RespawnType.Underground },
		func(n *creatures.NpcType, v bool) { n.RespawnType.Underground = v })

	// addVoice(text, interval, chance, yell). C++ keeps interval and chance on the
	// TYPE, not per voice, so the last call wins for those two — mirrored here
	// rather than storing them per entry.
	npcTypeMethods["addVoice"] = func(L *lua.LState) int {
		n := checkNpcType(L)
		if n == nil {
			L.Push(lua.LNil)
			return 1
		}
		v := creatures.NpcVoice{Text: L.CheckString(2)}
		if L.GetTop() >= 3 {
			n.YellInterval = uint32(lua.LVAsNumber(L.Get(3)))
		}
		if L.GetTop() >= 4 {
			n.YellChance = uint32(lua.LVAsNumber(L.Get(4)))
		}
		if L.GetTop() >= 5 {
			v.Yell = lua.LVAsBool(L.Get(5))
		}
		if v.Text != "" {
			n.Voices = append(n.Voices, v)
		}
		L.Push(lua.LTrue)
		return 1
	}
	npcTypeMethods["getVoices"] = func(L *lua.LState) int {
		n := checkNpcType(L)
		tbl := L.NewTable()
		if n != nil {
			for i, v := range n.Voices {
				entry := L.NewTable()
				L.SetField(entry, "text", lua.LString(v.Text))
				L.SetField(entry, "yellText", lua.LBool(v.Yell))
				tbl.RawSetInt(i+1, entry)
			}
		}
		L.Push(tbl)
		return 1
	}
	npcTypeMethods["addSound"] = func(L *lua.LState) int {
		n := checkNpcType(L)
		if n == nil {
			L.Push(lua.LNil)
			return 1
		}
		n.Sounds = append(n.Sounds, uint16(L.CheckInt(2)))
		L.Push(lua.LTrue)
		return 1
	}
	npcTypeMethods["getSounds"] = func(L *lua.LState) int {
		n := checkNpcType(L)
		tbl := L.NewTable()
		if n != nil {
			for i, s := range n.Sounds {
				tbl.RawSetInt(i+1, lua.LNumber(s))
			}
		}
		L.Push(tbl)
		return 1
	}
	// light(level, color) — two values in, a table out.
	npcTypeMethods["light"] = func(L *lua.LState) int {
		n := checkNpcType(L)
		if n == nil {
			L.Push(lua.LNil)
			return 1
		}
		if L.GetTop() >= 3 {
			n.LightLevel = uint8(L.CheckInt(2))
			n.LightColor = uint8(L.CheckInt(3))
			L.Push(lua.LTrue)
			return 1
		}
		L.Push(lua.LNumber(n.LightLevel))
		L.Push(lua.LNumber(n.LightColor))
		return 2
	}
	npcTypeMethods["registerEvent"] = func(L *lua.LState) int {
		n := checkNpcType(L)
		if n != nil {
			n.CreatureEvents = append(n.CreatureEvents, L.CheckString(2))
		}
		L.Push(lua.LTrue)
		return 1
	}
	npcTypeMethods["getCreatureEvents"] = func(L *lua.LState) int {
		n := checkNpcType(L)
		tbl := L.NewTable()
		if n != nil {
			for i, ev := range n.CreatureEvents {
				tbl.RawSetInt(i+1, lua.LString(ev))
			}
		}
		L.Push(tbl)
		return 1
	}

	// Register event callbacks on NpcType
	npcTypeMethods["eventType"] = func(L *lua.LState) int {
		L.Push(L.Get(1))
		return 1
	}
	npcEvents := []string{"onThink", "onAppear", "onDisappear", "onMove", "onSay", "onPlayerAttack", "onSpawn", "onBuyItem", "onSellItem", "onCheckItem", "onCloseChannel"}
	for _, key := range npcEvents {
		k := key
		npcTypeMethods[k] = func(L *lua.LState) int {
			n := checkNpcType(L)
			if L.GetTop() >= 2 {
				if fn, ok := L.Get(2).(*lua.LFunction); ok {
					e.npcCallbacksMu.Lock()
					if e.npcCallbacks == nil {
						e.npcCallbacks = make(map[string]map[string]*lua.LFunction)
					}
					name := strings.ToLower(n.Name)
					if e.npcCallbacks[name] == nil {
						e.npcCallbacks[name] = make(map[string]*lua.LFunction)
					}
					e.npcCallbacks[name][k] = fn
					e.npcCallbacksMu.Unlock()
				}
			}
			L.Push(L.Get(1))
			return 1
		}
	}

	// The metatable's __index IS the global class table, exactly as
	// Lua::registerClass builds it (src/lua/functions/lua_functions_loader.cpp:
	// 784-786, "className.metatable.__index = className").
	//
	// These were two separate tables before, and that is what kept the datapack's
	// own shim inert: register_npc_type.lua assigns NpcType.register on the GLOBAL
	// table, userdata resolved methods through the OTHER one, and the assignment
	// had no effect on any NPC. One table means a Lua-side definition extends or
	// overrides the class for real, which is the whole mechanism upstream relies on.
	var tbl *lua.LTable
	classTable := e.L.GetGlobal(luaNpcTypeName)
	if classTable.Type() == lua.LTTable {
		tbl = classTable.(*lua.LTable)
	} else {
		tbl = e.L.NewTable()
		e.L.SetGlobal(luaNpcTypeName, tbl)
	}
	for k, v := range npcTypeMethods {
		e.L.SetField(tbl, k, e.L.NewFunction(v))
	}
	e.L.SetField(mt, "__index", tbl)
	// Datapack NPC scripts assign event callbacks directly on the userdata, e.g.
	// `npcType.onThink = function(npc, interval) ... end`. Without a __newindex,
	// gopher-lua raises "attempt to index a non-table object(userdata)". Accept
	// (and currently ignore) such assignments so the scripts load; wiring these
	// callbacks (onSay dialogue, onThink, shop handlers) into the NPC runtime is
	// the remaining NPC step.
	e.L.SetField(mt, "__newindex", e.L.NewFunction(func(L *lua.LState) int {
		n := checkNpcType(L)
		key := L.CheckString(2)
		val := L.CheckAny(3)

		if fn, ok := val.(*lua.LFunction); ok {
			e.npcCallbacksMu.Lock()
			if e.npcCallbacks == nil {
				e.npcCallbacks = make(map[string]map[string]*lua.LFunction)
			}
			name := strings.ToLower(n.Name)
			if e.npcCallbacks[name] == nil {
				e.npcCallbacks[name] = make(map[string]*lua.LFunction)
			}
			e.npcCallbacks[name][key] = fn
			e.npcCallbacksMu.Unlock()
		}
		return 0
	}))

	// Game.createNpcType
	gameTable := e.L.GetGlobal("Game")
	if gameTable.Type() == lua.LTTable {
		e.L.SetField(gameTable, "createNpcType", e.L.NewFunction(func(L *lua.LState) int {
			name := L.CheckString(1)
			key := strings.ToLower(name)

			// luaNpcTypeCreate is g_npcs().getNpcType(name, true) — the `true` is
			// "create if missing", and registration happens HERE, not in register().
			// Go used to register at register() time, which only worked because Go
			// also had its own register(); with the Lua shim in charge nothing would
			// ever reach the registry. Returning the existing type on a second call
			// also matches upstream, where reloading a script mutates one object
			// rather than orphaning the first.
			var reg map[string]*creatures.NpcType
			if e != nil && e.world != nil && e.world.TypeRegistry != nil {
				reg = e.world.TypeRegistry.Npcs
			}
			nType := reg[key]
			if nType == nil {
				nType = &creatures.NpcType{
					Name:      name,
					Speed:     200,
					Health:    100,
					MaxHealth: 100,
					// NpcType's own field initializers, which used to be applied as
					// fixups at the end of register().
					SpeechBubble: creatures.SpeechBubbleNormal,
					CurrencyID:   creatures.DefaultNpcCurrency,
					CanSpawn:     true,
				}
				if reg != nil {
					reg[key] = nType
				}
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

func (e *Engine) registerShop() {
	mt := e.L.NewTypeMetatable("Shop")
	methods := map[string]lua.LGFunction{
		"setId": func(L *lua.LState) int {
			s := L.CheckUserData(1).Value.(*creatures.ShopItem)
			s.ID = uint16(L.CheckNumber(2))
			return 0
		},
		"setCount": func(L *lua.LState) int {
			s := L.CheckUserData(1).Value.(*creatures.ShopItem)
			s.SubType = uint8(L.CheckNumber(2))
			return 0
		},
		"setNameItem": func(L *lua.LState) int {
			s := L.CheckUserData(1).Value.(*creatures.ShopItem)
			s.Name = L.CheckString(2)
			return 0
		},
		"setBuyPrice": func(L *lua.LState) int {
			s := L.CheckUserData(1).Value.(*creatures.ShopItem)
			s.BuyPrice = uint32(L.CheckNumber(2))
			return 0
		},
		"setSellPrice": func(L *lua.LState) int {
			s := L.CheckUserData(1).Value.(*creatures.ShopItem)
			s.SellPrice = uint32(L.CheckNumber(2))
			return 0
		},
		"setStorageKey": func(L *lua.LState) int {
			s := L.CheckUserData(1).Value.(*creatures.ShopItem)
			s.StorageKey = int32(L.CheckNumber(2))
			return 0
		},
		"setStorageValue": func(L *lua.LState) int {
			s := L.CheckUserData(1).Value.(*creatures.ShopItem)
			s.StorageValue = int32(L.CheckNumber(2))
			return 0
		},
		// addChildShop nests one entry under another (luaShopAddChildShop). This
		// used to be a no-op with the comment "addShopItem does it" — it does not;
		// only the parent is ever passed to addShopItem, so every child category
		// was discarded.
		"addChildShop": func(L *lua.LState) int {
			parent, ok := L.CheckUserData(1).Value.(*creatures.ShopItem)
			if !ok {
				return 0
			}
			if child, ok := L.CheckUserData(2).Value.(*creatures.ShopItem); ok {
				parent.ChildShop = append(parent.ChildShop, *child)
			}
			return 0
		},
	}
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), methods))

	// Shop constructor
	e.setClassConstructor("Shop", func(L *lua.LState) int {
		ud := L.NewUserData()
		ud.Value = &creatures.ShopItem{}
		L.SetMetatable(ud, mt)
		L.Push(ud)
		return 1
	}, methods)
}
