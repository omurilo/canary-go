package luaengine

import lua "github.com/yuin/gopher-lua"

const containerTypeName = "Container"

// Container represents a game container item.
type Container struct {
	Item
	Size uint32
}

func (e *Engine) registerContainer() {
	mt := e.L.NewTypeMetatable(containerTypeName)
	// Combine itemMethods and containerMethods for the __index table
	indexTbl := e.L.NewTable()
	for k, v := range itemMethods {
		e.L.SetField(indexTbl, k, e.L.NewFunction(v))
	}
	for k, v := range containerMethods {
		e.L.SetField(indexTbl, k, e.L.NewFunction(v))
	}
	e.L.SetField(mt, "__index", indexTbl)

	e.L.SetGlobal("Container", e.L.NewFunction(containerCreate))
}

func containerCreate(L *lua.LState) int {
	id := L.OptInt(1, 0)
	c := &Container{
		Item: Item{ID: uint32(id)},
	}
	ud := L.NewUserData()
	ud.Value = c
	L.SetMetatable(ud, L.GetTypeMetatable(containerTypeName))
	L.Push(ud)
	return 1
}

func checkContainer(L *lua.LState) *Container {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*Container); ok {
		return v
	}
	L.ArgError(1, "Container expected")
	return nil
}

func stubContainerMethod(L *lua.LState) int {
	return 0
}

var containerMethods = map[string]lua.LGFunction{
	"getSize":                func(L *lua.LState) int { c := checkContainer(L); L.Push(lua.LNumber(c.Size)); return 1 },
	"getMaxCapacity":         stubContainerMethod,
	"getCapacity":            stubContainerMethod,
	"getEmptySlots":          stubContainerMethod,
	"getContentDescription":  stubContainerMethod,
	"getItems":               stubContainerMethod,
	"getItemHoldingCount":    stubContainerMethod,
	"getItemCountById":       stubContainerMethod,
	"getItem":                stubContainerMethod,
	"hasItem":                stubContainerMethod,
	"addItem":                stubContainerMethod,
	"addItemEx":              stubContainerMethod,
	"getCorpseOwner":         stubContainerMethod,
	"registerReward":         stubContainerMethod,
	"removeAllItems":         stubContainerMethod,
	"removeItemById":         stubContainerMethod,
}
