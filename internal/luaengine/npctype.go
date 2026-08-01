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
				n.Health = n.MaxHealth
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

			if val := table.RawGetString("description"); val.Type() == lua.LTString {
				n.Description = val.String()
			}
			if val := table.RawGetString("speechBubble"); val.Type() == lua.LTNumber {
				n.SpeechBubble = uint8(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("currency"); val.Type() == lua.LTNumber {
				n.CurrencyID = uint16(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("walkInterval"); val.Type() == lua.LTNumber {
				n.WalkInterval = uint32(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("walkRadius"); val.Type() == lua.LTNumber {
				n.WalkRadius = int32(lua.LVAsNumber(val))
			}

			// npcConfig.flags = { floorchange =, pushable =, canPushItems =, ... }
			if flagsVal := table.RawGetString("flags"); flagsVal.Type() == lua.LTTable {
				flags := flagsVal.(*lua.LTable)
				boolFlag := func(key string, dst *bool) {
					if v := flags.RawGetString(key); v.Type() == lua.LTBool {
						*dst = lua.LVAsBool(v)
					}
				}
				boolFlag("floorchange", &n.FloorChange)
				boolFlag("pushable", &n.IsPushable)
				boolFlag("canPushItems", &n.CanPushItems)
				boolFlag("canPushCreatures", &n.CanPushCreatures)
				if v := flags.RawGetString("profession"); v.Type() == lua.LTString {
					n.Profession = v.String()
				}
			}

			// npcConfig.voices = { interval =, chance =, { text =, yell = }, ... }
			// interval/chance are named keys; the voices themselves are the array
			// part of the same table, mirroring how the datapack writes it.
			if voicesVal := table.RawGetString("voices"); voicesVal.Type() == lua.LTTable {
				voices := voicesVal.(*lua.LTable)
				if v := voices.RawGetString("interval"); v.Type() == lua.LTNumber {
					n.YellInterval = uint32(lua.LVAsNumber(v))
				}
				if v := voices.RawGetString("chance"); v.Type() == lua.LTNumber {
					n.YellChance = uint32(lua.LVAsNumber(v))
				}
				n.Voices = nil
				for i := 1; ; i++ {
					entry := voices.RawGetInt(i)
					if entry.Type() != lua.LTTable {
						break
					}
					tb := entry.(*lua.LTable)
					voice := creatures.NpcVoice{}
					if v := tb.RawGetString("text"); v.Type() == lua.LTString {
						voice.Text = v.String()
					}
					if v := tb.RawGetString("yell"); v.Type() == lua.LTBool {
						voice.Yell = lua.LVAsBool(v)
					}
					if voice.Text != "" {
						n.Voices = append(n.Voices, voice)
					}
				}
			}

			if rt := table.RawGetString("respawnType"); rt.Type() == lua.LTTable {
				tb := rt.(*lua.LTable)
				if v := tb.RawGetString("period"); v.Type() == lua.LTNumber {
					n.RespawnType.Period = int32(lua.LVAsNumber(v))
				}
				if v := tb.RawGetString("underground"); v.Type() == lua.LTBool {
					n.RespawnType.Underground = lua.LVAsBool(v)
				}
			}

			// Defaults matching NpcInfo's initializers.
			if n.SpeechBubble == 0 {
				n.SpeechBubble = creatures.SpeechBubbleNormal
			}
			if n.CurrencyID == 0 {
				n.CurrencyID = creatures.DefaultNpcCurrency
			}

			// npcConfig.shop = { { itemName=, clientId=, buy=, sell=, subType= }, ... }
			// Most merchant NPCs declare their catalog this way (rather than via
			// npcType:addShopItem), so parse it into ShopItems — isMerchant() and
			// openShopWindow() read from here.
			if shopVal := table.RawGetString("shop"); shopVal.Type() == lua.LTTable {
				n.ShopItems = nil
				shopVal.(*lua.LTable).ForEach(func(_, v lua.LValue) {
					entry, ok := v.(*lua.LTable)
					if !ok {
						return
					}
					item := creatures.ShopItem{}
					if x := entry.RawGetString("clientId"); x.Type() == lua.LTNumber {
						item.ID = uint16(lua.LVAsNumber(x))
					}
					if x := entry.RawGetString("itemId"); x.Type() == lua.LTNumber && item.ID == 0 {
						item.ID = uint16(lua.LVAsNumber(x))
					}
					if x := entry.RawGetString("itemName"); x.Type() == lua.LTString {
						item.Name = x.String()
					}
					if x := entry.RawGetString("name"); x.Type() == lua.LTString && item.Name == "" {
						item.Name = x.String()
					}
					if x := entry.RawGetString("subType"); x.Type() == lua.LTNumber {
						item.SubType = uint8(lua.LVAsNumber(x))
					}
					if x := entry.RawGetString("count"); x.Type() == lua.LTNumber && item.SubType == 0 {
						item.SubType = uint8(lua.LVAsNumber(x))
					}
					if x := entry.RawGetString("buy"); x.Type() == lua.LTNumber {
						item.BuyPrice = uint32(lua.LVAsNumber(x))
					}
					if x := entry.RawGetString("sell"); x.Type() == lua.LTNumber {
						item.SellPrice = uint32(lua.LVAsNumber(x))
					}
					n.ShopItems = append(n.ShopItems, item)
				})
			}

			if e != nil && e.world != nil && e.world.TypeRegistry != nil {
				e.world.TypeRegistry.Npcs[strings.ToLower(n.Name)] = n
			}

			L.Push(lua.LTrue)
			return 1
		},
		"addShopItem": func(L *lua.LState) int {
			n := checkNpcType(L)
			ud := L.CheckUserData(2)
			if shopItem, ok := ud.Value.(*creatures.ShopItem); ok {
				n.ShopItems = append(n.ShopItems, *shopItem)
			}
			L.Push(lua.LTrue)
			return 1
		},
		"isPushable": func(L *lua.LState) int {
			n := checkNpcType(L)
			if n == nil { L.Push(lua.LFalse); return 1 }
			L.Push(lua.LBool(n.IsPushable))
			return 1
		},
		"health": func(L *lua.LState) int {
			n := checkNpcType(L)
			if n == nil { L.Push(lua.LNumber(0)); return 1 }
			L.Push(lua.LNumber(n.Health))
			return 1
		},
		"maxHealth": func(L *lua.LState) int {
			n := checkNpcType(L)
			if n == nil { L.Push(lua.LNumber(0)); return 1 }
			L.Push(lua.LNumber(n.MaxHealth))
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

	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), npcTypeMethods))

	// Populate methods onto the global class table so they are discoverable via pairs()
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
			nType := &creatures.NpcType{
				Name:      name,
				Speed:     200,
				Health:    100,
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
		"addChildShop": func(L *lua.LState) int {
			// Actually we don't need to do anything here because addShopItem does it
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
