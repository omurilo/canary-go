package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const containerTypeName = "Container"

type luaContainer struct {
	item *game.Item
	pos  game.Position
}

func (e *Engine) registerContainer() {
	mt := e.L.NewTypeMetatable(containerTypeName)
	// Combine itemMethods and containerMethods for the __index table
	indexTbl := e.L.NewTable()
	for k, v := range e.itemMethods() {
		e.L.SetField(indexTbl, k, e.L.NewFunction(v))
	}
	for k, v := range e.containerMethods() {
		e.L.SetField(indexTbl, k, e.L.NewFunction(v))
	}
	e.L.SetField(mt, "__index", indexTbl)

	e.L.SetGlobal("Container", e.L.NewFunction(e.containerCreate))
}

func (e *Engine) containerCreate(L *lua.LState) int {
	id := L.OptInt(1, 0)
	c := &game.Item{ID: uint16(id)}
	e.pushContainer(L, c)
	return 1
}

func (e *Engine) pushContainer(L *lua.LState, it *game.Item) {
	if it == nil {
		L.Push(lua.LNil)
		return
	}
	ud := L.NewUserData()
	ud.Value = luaContainer{item: it}
	L.SetMetatable(ud, L.GetTypeMetatable(containerTypeName))
	L.Push(ud)
}

func checkContainer(L *lua.LState) luaContainer {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(luaContainer); ok {
		return v
	}
	L.ArgError(1, "Container expected")
	return luaContainer{}
}

func stubContainerMethod(L *lua.LState) int {
	return 0
}

func (e *Engine) containerMethods() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"getSize": func(L *lua.LState) int { 
			c := checkContainer(L)
			L.Push(lua.LNumber(len(c.item.Contents)))
			return 1 
		},
		"addItem": func(L *lua.LState) int {
			c := checkContainer(L)
			itemID := uint16(L.CheckInt(2))
			count := uint16(L.OptInt(3, 1))
			
			newItem := &game.Item{ID: itemID, Count: count}
			c.item.Contents = append(c.item.Contents, newItem)
			
			e.pushItem(L, newItem)
			return 1
		},
		"getMaxCapacity":         stubContainerMethod,
		"getCapacity":            stubContainerMethod,
		"getEmptySlots":          stubContainerMethod,
		"getContentDescription":  stubContainerMethod,
		"getItems":               stubContainerMethod,
		"getItemHoldingCount":    stubContainerMethod,
		"getItemCountById":       stubContainerMethod,
		"getItem":                stubContainerMethod,
		"hasItem":                stubContainerMethod,
		"addItemEx":              stubContainerMethod,
		"getCorpseOwner":         stubContainerMethod,
		"registerReward":         stubContainerMethod,
		"removeAllItems":         stubContainerMethod,
		"removeItemById":         stubContainerMethod,
	}
}
