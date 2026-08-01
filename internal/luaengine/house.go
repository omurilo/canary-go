package luaengine

import (
	lua "github.com/yuin/gopher-lua"

	"github.com/opentibiabr/canary-go/internal/game"
)

const houseTypeName = "House"

// checkHouse returns the *game.House from L.Get(1) or nil.
func checkHouse(L *lua.LState) *game.House {
	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if h, ok := ud.Value.(*game.House); ok {
			return h
		}
	}
	return nil
}

func pushHouse(L *lua.LState, h *game.House) {
	ud := L.NewUserData()
	ud.Value = h
	L.SetMetatable(ud, L.GetTypeMetatable("House"))
	L.Push(ud)
}

func checkHouseArg(L *lua.LState, n int) *game.House {
	ud := L.CheckUserData(n)
	if h, ok := ud.Value.(*game.House); ok {
		return h
	}
	return nil
}

func houseGetOwnerGuid(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(h.OwnerID))
	return 1
}

// houseSetOwner is house:setOwner(guid[, updateDatabase = true]). The second
// argument defaults to TRUE in C++ (house_functions.cpp:159), which is what makes
// `/owner` survive a restart; passing it as false was the old behaviour and lost
// the change on shutdown.
func (e *Engine) houseSetOwner(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		return 0
	}
	ownerID := uint32(lua.LVAsNumber(L.Get(2)))
	h.SetOwner(e.world, ownerID, optBool(L, 3, true), nil)
	return 0
}

// optBool reads an optional boolean argument, mirroring Lua::getBoolean(L, n, def):
// absent or nil means the default, anything else follows Lua truthiness.
func optBool(L *lua.LState, n int, def bool) bool {
	v := L.Get(n)
	if v == lua.LNil || v == nil {
		return def
	}
	return lua.LVAsBool(v)
}

func houseGetPrice(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	price := h.Rent * 100
	L.Push(lua.LNumber(price))
	return 1
}

func houseGetExitPosition(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		L.Push(lua.LNil)
		return 1
	}
	pushPosition(L, h.Position)
	return 1
}

func houseGetBeds(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(h.Beds))
	return 1
}

func houseGetRent(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(h.Rent))
	return 1
}

func houseGetName(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		L.Push(lua.LString(""))
		return 1
	}
	L.Push(lua.LString(h.Name))
	return 1
}

func houseGetSize(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(h.Size))
	return 1
}

// canEditAccessList checks if a player can edit the given access list type.
// GUEST_LIST (0) can be edited by owner and sub-owners.
// SUBOWNER_LIST (1) can only be edited by the owner.
func houseCanEditAccessList(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	listType := L.CheckInt(2)
	p := checkPlayer(L)
	if h == nil || p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	if h.IsOwner(p.DBID) {
		L.Push(lua.LTrue)
		return 1
	}
	// GUEST_LIST can be edited by sub-owners too
	if listType == 0 && h.IsSubOwner(p.Name) {
		L.Push(lua.LTrue)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func (e *Engine) registerHouseMetatable() {
	// Methods that need the engine closure.
	methods := map[string]lua.LGFunction{
		"getOwnerGuid":    houseGetOwnerGuid,
		"setOwner":        e.houseSetOwner,
		"getPrice":        houseGetPrice,
		"getExitPosition": houseGetExitPosition,
		"getBeds":         houseGetBeds,
		"getRent":         houseGetRent,
		"getName":         houseGetName,
		"getSize":         houseGetSize,
		"canEditAccessList": houseCanEditAccessList,
		// setHouseOwner is the name the datapack actually uses (compat.lua:1230,
		// buy_house.lua:58, house_owner.lua:15,25). It mirrors setOwner but
		// returns a boolean, which those callers rely on.
		"setHouseOwner": func(L *lua.LState) int {
			h := checkHouseArg(L, 1)
			if h == nil {
				L.Push(lua.LFalse)
				return 1
			}
			h.SetOwner(e.world, uint32(lua.LVAsNumber(L.Get(2))), optBool(L, 3, true), nil)
			L.Push(lua.LTrue)
			return 1
		},
		// getDoorIdByPosition returns nil when the door is unknown. game.HouseDoor
		// carries no position yet (internal/game/house.go:43), so this cannot be
		// resolved properly; callers guard with `if ... then`, so nil is the safe
		// answer. TODO: needs door positions on House.DoorList.
		"getDoorIdByPosition": func(L *lua.LState) int {
			L.Push(lua.LNil)
			return 1
		},
		// hasNewOwnership reports whether a transfer is pending.
		"hasNewOwnership": func(L *lua.LState) int {
			h := checkHouseArg(L, 1)
			if h == nil {
				L.Push(lua.LFalse)
				return 1
			}
			L.Push(lua.LBool(h.TransferAccept != 0 || h.TransferToName != ""))
			return 1
		},
		// setNewOwnerGuid(guid) with guid 0 clears the pending transfer.
		"setNewOwnerGuid": func(L *lua.LState) int {
			h := checkHouseArg(L, 1)
			if h == nil {
				return 0
			}
			guid := uint32(lua.LVAsNumber(L.Get(2)))
			h.TransferAccept = guid
			if guid == 0 {
				h.TransferToName = ""
				h.TransferPrice = 0
			}
			return 0
		},
		// startTrade is not implemented: House::startTrade in C++
		// (src/map/house/house.cpp:629) needs the HouseTransferItem/onTradeEvent
		// machinery, which has no Go counterpart yet. Returns RETURNVALUE_NOTPOSSIBLE.
		"startTrade": func(L *lua.LState) int {
			L.Push(lua.LNumber(1)) // RETURNVALUE_NOTPOSSIBLE
			return 1
		},
		"getId": func(L *lua.LState) int {
			h := checkHouse(L)
			if h == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(h.ID))
			return 1
		},
		"getOwnerName": func(L *lua.LState) int {
			L.Push(lua.LString(""))
			return 1
		},
		"getDoors": func(L *lua.LState) int {
			L.Push(L.NewTable())
			return 1
		},
		"getTiles": func(L *lua.LState) int {
			L.Push(L.NewTable())
			return 1
		},
		"getItems": func(L *lua.LState) int {
			L.Push(L.NewTable())
			return 1
		},
		"hasItemOnTile": func(L *lua.LState) int {
			h := checkHouseArg(L, 1)
			if h == nil || len(h.HouseTiles) == 0 {
				L.Push(lua.LFalse)
				return 1
			}
			world := e.world
			if world == nil {
				L.Push(lua.LTrue)
				return 1
			}
			for _, pos := range h.HouseTiles {
				tile := world.Map.GetTile(pos)
				if tile == nil {
					continue
				}
				if len(tile.Items) > 0 {
					L.Push(lua.LTrue)
					return 1
				}
			}
			L.Push(lua.LFalse)
			return 1
		},
	}

	mt := e.L.NewTypeMetatable(houseTypeName)
	idx := e.L.NewTable()
	e.L.SetFuncs(idx, methods)
	e.L.SetField(mt, "__index", idx)

	// House(id) is used 9 times in the datapack. This global used to be supplied by
	// the mockClass block, so removing that mock without adding a real constructor
	// would break those call sites.
	e.setClassConstructor(houseTypeName, func(L *lua.LState) int {
		if e.world == nil {
			L.Push(lua.LNil)
			return 1
		}
		h := e.world.GetHouse(uint32(L.CheckInt(1)))
		if h == nil {
			L.Push(lua.LNil)
			return 1
		}
		pushHouse(L, h)
		return 1
	}, methods)
}
