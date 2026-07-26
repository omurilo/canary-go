package luaengine

import (
	lua "github.com/yuin/gopher-lua"

	"github.com/opentibiabr/canary-go/internal/game"
)

const houseTypeName = "House"

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

func houseSetOwner(L *lua.LState) int {
	h := checkHouseArg(L, 1)
	if h == nil {
		return 0
	}
	ownerID := uint32(lua.LVAsNumber(L.Get(2)))
	h.SetOwner(ownerID)
	return 0
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
		"setOwner":        houseSetOwner,
		"getPrice":        houseGetPrice,
		"getExitPosition": houseGetExitPosition,
		"getBeds":         houseGetBeds,
		"getRent":         houseGetRent,
		"getName":         houseGetName,
		"getSize":         houseGetSize,
		"canEditAccessList": houseCanEditAccessList,
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
}
