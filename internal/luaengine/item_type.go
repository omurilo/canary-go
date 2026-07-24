package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/items"
	lua "github.com/yuin/gopher-lua"
)

const itemTypeClassName = "ItemType"

type luaItemType struct {
	id   uint16
	item *items.ItemType
}

func (e *Engine) registerItemType() {
	mt := e.L.NewTypeMetatable(itemTypeClassName)
	methods := map[string]lua.LGFunction{
		"getName": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil && it.item.Name != "" {
				L.Push(lua.LString(it.item.Name))
			} else {
				L.Push(lua.LString("An Item"))
			}
			return 1
		},
		"getId": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			L.Push(lua.LNumber(it.id))
			return 1
		},
		"getClientId": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			L.Push(lua.LNumber(it.id))
			return 1
		},
		"getWeight": func(L *lua.LState) int {
			_ = checkItemType(L, 1)
			L.Push(lua.LNumber(0))
			return 1
		},
		"isStackable": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.Stackable))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isContainer": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.IsContainer()))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isFluidContainer": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.IsFluidContainer()))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"getFluidSource": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LNumber(it.item.FluidSource))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		},
		"getCharges": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LNumber(it.item.Charges))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		},
		"getStackSize": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil && it.item.StackSize > 0 {
				L.Push(lua.LNumber(it.item.StackSize))
			} else {
				L.Push(lua.LNumber(100)) // Tibia default stack size
			}
			return 1
		},
		"isRune": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.TypeName == "rune"))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"getDecayId": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LNumber(it.item.DecayTo))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		},
		"isMovable": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.Pickupable))
			} else {
				L.Push(lua.LBool(true))
			}
			return 1
		},
		"getType": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			L.Push(lua.LNumber(it.id))
			return 1
		},
		"hasProperty": func(L *lua.LState) int {
			_ = checkItemType(L, 1)
			_ = L.OptInt(2, 0)
			L.Push(lua.LFalse)
			return 1
		},
	}

	e.L.SetFuncs(mt, methods)
	methodTable := e.L.SetFuncs(e.L.NewTable(), methods)

	e.L.SetField(mt, "__index", e.L.NewFunction(func(L *lua.LState) int {
		it := checkItemType(L, 1)
		key := L.CheckString(2)
		if val := methodTable.RawGetString(key); val != lua.LNil {
			L.Push(val)
			return 1
		}
		if key == "id" {
			L.Push(lua.LNumber(it.id))
			return 1
		}
		if key == "name" {
			if it.item != nil {
				L.Push(lua.LString(it.item.Name))
			} else {
				L.Push(lua.LString("An Item"))
			}
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	e.setClassConstructor("ItemType", func(L *lua.LState) int {
		var id uint16
		if L.GetTop() >= 1 && L.Get(1).Type() == lua.LTNumber {
			id = uint16(L.CheckInt(1))
		}
		var item *items.ItemType
		if cat := e.itemCatalog(); cat != nil {
			item = cat.Get(id)
		}
		ud := L.NewUserData()
		ud.Value = &luaItemType{id: id, item: item}
		L.SetMetatable(ud, L.GetTypeMetatable(itemTypeClassName))
		L.Push(ud)
		return 1
	}, methods)
}

func checkItemType(L *lua.LState, n ...int) *luaItemType {
	idx := 1
	if len(n) > 0 {
		idx = n[0]
	}
	ud := L.CheckUserData(idx)
	if it, ok := ud.Value.(*luaItemType); ok {
		return it
	}
	L.ArgError(idx, "ItemType expected")
	return nil
}
