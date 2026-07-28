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
