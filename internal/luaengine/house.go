package luaengine

import (
	lua "github.com/yuin/gopher-lua"

	"github.com/omurilo/canary-go/internal/game"
)

const houseTypeName = "House"

// ReturnValue_t values used by house:startTrade, and MESSAGE_EVENT_ADVANCE.
// The datapack turns the number into the message the player reads, so these
// have to be the upstream values and not a local invention.
const (
	returnValueTradePlayerFarAway           = 72
	returnValueYouDontOwnThisHouse          = 73
	returnValueTradePlayerAlreadyOwnsAHouse = 74
	returnValueYouCannotTradeThisHouse      = 76
	msgEventAdvance                         = 19
)

// withinRange is Position::areInRange<dx, dy, 0>: same floor, within a box.
func withinRange(a, b game.Position, dx, dy int) bool {
	if a.Z != b.Z {
		return false
	}
	ax, bx := int(a.X), int(b.X)
	ay, by := int(a.Y), int(b.Y)
	if ax-bx > dx || bx-ax > dx {
		return false
	}
	return !(ay-by > dy || by-ay > dy)
}

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

// houseCanEditAccessList is house:canEditAccessList(listId, player)
// (house_functions.cpp).
//
// It carried its own copy of the rule and compared the list id against 0. The
// Lua GUEST_LIST enum is 0x100 (map_definitions.hpp:14), so the sub-owner
// branch could never be taken: a sub-owner was refused the guest list, which is
// the one list they are supposed to be able to edit.
func houseCanEditAccessList(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	listID := uint32(L.CheckInt(2))
	p := checkPlayerArg(L, 3)
	if h == nil || p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LBool(h.CanEditAccessList(listID, p)))
	return 1
}

// houseGetAccessList is house:getAccessList(listId) (house_functions.cpp). It
// had no Go counterpart at all, so a script could read no list.
func houseGetAccessList(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		L.Push(lua.LNil)
		return 1
	}
	list, ok := h.GetAccessList(uint32(L.CheckInt(2)))
	if !ok {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(list))
	return 1
}

// houseSetAccessList is house:setAccessList(listId, list) (house_functions.cpp).
func (e *Engine) houseSetAccessList(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		L.Push(lua.LFalse)
		return 1
	}
	h.SetAccessList(e.world, uint32(L.CheckInt(2)), L.CheckString(3))
	L.Push(lua.LTrue)
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
		// getDoorIdByPosition is house:getDoorIdByPosition(position)
		// (house_functions.cpp:335). It used to answer a flat nil because
		// HouseDoor carried no position; registerHouseFurniture now fills one in
		// from the door item's HouseDoorID attribute, so the lookup is real.
		"getDoorIdByPosition": func(L *lua.LState) int {
			h := checkHouseArg(L, 1)
			if h == nil {
				L.Push(lua.LNil)
				return 1
			}
			door, ok := h.GetDoorByPosition(checkPosition(L, 2))
			if !ok {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(door.ID))
			return 1
		},
		"getAccessList": houseGetAccessList,
		"setAccessList": e.houseSetAccessList,
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
		// startTrade is house:startTrade(player, tradePartner)
		// (house_functions.cpp:221). It answered a flat NOTPOSSIBLE, so a house
		// owner trying to sell got "sorry, not possible" whatever was actually
		// wrong — including when nothing was.
		//
		// The validation chain below is upstream's, in upstream's order, and its
		// return value is what the datapack turns into the message the player
		// reads. The handoff at the end is not: internalStartTrade needs the
		// player-to-player trade window, and World.PlayerRequestTrade is still
		// empty. So the document is minted, the trade fails to start, and
		// resetTransferItem takes it back — which is exactly the path upstream
		// runs when internalStartTrade returns false.
		"startTrade": func(L *lua.LState) int {
			h := checkHouseArg(L, 1)
			p := checkPlayerArg(L, 2)
			partner := checkPlayerArg(L, 3)
			if h == nil || p == nil || partner == nil {
				L.Push(lua.LNil)
				return 1
			}
			if !withinRange(partner.GetPosition(), p.GetPosition(), 2, 2) {
				L.Push(lua.LNumber(returnValueTradePlayerFarAway))
				return 1
			}
			if !h.IsOwner(p.DBID) {
				L.Push(lua.LNumber(returnValueYouDontOwnThisHouse))
				return 1
			}
			if e.world != nil && e.world.GetHouseByPlayerID(partner.DBID) != nil {
				L.Push(lua.LNumber(returnValueTradePlayerAlreadyOwnsAHouse))
				return 1
			}
			transferItem := h.GetTransferItem()
			if transferItem == nil {
				L.Push(lua.LNumber(returnValueYouCannotTradeThisHouse))
				return 1
			}
			if optBool(L, 4, false) && h.HasNewOwnership() {
				partner.SendTextMessage(msgEventAdvance, "You cannot buy this house. Ownership is already scheduled to be transferred upon the next server restart.")
				p.SendTextMessage(msgEventAdvance, "You cannot sell this house. Ownership is already scheduled to be transferred upon the next server restart.")
				h.ResetTransferItem()
				L.Push(lua.LNumber(returnValueYouCannotTradeThisHouse))
				return 1
			}
			// internalStartTrade has no counterpart yet; it would always fail, and
			// upstream resets the document when it does.
			h.ResetTransferItem()
			L.Push(lua.LNumber(returnValueYouCannotTradeThisHouse))
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
		// hasItemOnTile is house:hasItemOnTile() (house_functions.cpp:188).
		//
		// The inline copy answered true for ANY item on any house tile. Upstream
		// only counts wrapable and pickupable ones (house.cpp:401) — a house with
		// a plain fixed decoration inside is still purchasable, and under the old
		// rule it never was.
		"hasItemOnTile": func(L *lua.LState) int {
			h := checkHouseArg(L, 1)
			if h == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LBool(h.HasItemOnTile(e.world)))
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
