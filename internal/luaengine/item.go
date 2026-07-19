package luaengine

import lua "github.com/yuin/gopher-lua"

const itemTypeName = "Item"

// Item represents a game item.
type Item struct {
	ID uint32
}

func (e *Engine) registerItem() {
	mt := e.L.NewTypeMetatable(itemTypeName)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), itemMethods))

	e.L.SetGlobal("Item", e.L.NewFunction(itemCreate))
}

func itemCreate(L *lua.LState) int {
	id := L.OptInt(1, 0)
	item := &Item{ID: uint32(id)}
	ud := L.NewUserData()
	ud.Value = item
	L.SetMetatable(ud, L.GetTypeMetatable(itemTypeName))
	L.Push(ud)
	return 1
}

func checkItem(L *lua.LState) *Item {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*Item); ok {
		return v
	}
	// Container also inherits from Item
	if v, ok := ud.Value.(*Container); ok {
		return &v.Item
	}
	L.ArgError(1, "Item expected")
	return nil
}

func stubItemMethod(L *lua.LState) int {
	return 0
}

var itemMethods = map[string]lua.LGFunction{
	"isItem":            func(L *lua.LState) int { L.Push(lua.LTrue); return 1 },
	"getContainer":      stubItemMethod,
	"getParent":         stubItemMethod,
	"getTopParent":      stubItemMethod,
	"getId":             func(L *lua.LState) int { item := checkItem(L); L.Push(lua.LNumber(item.ID)); return 1 },
	"clone":             stubItemMethod,
	"split":             stubItemMethod,
	"remove":            stubItemMethod,
	"getUniqueId":       stubItemMethod,
	"getActionId":       stubItemMethod,
	"setActionId":       stubItemMethod,
	"getCount":          stubItemMethod,
	"getCharges":        stubItemMethod,
	"getFluidType":      stubItemMethod,
	"getWeight":         stubItemMethod,
	"getSubType":        stubItemMethod,
	"getName":           stubItemMethod,
	"getPluralName":     stubItemMethod,
	"getArticle":        stubItemMethod,
	"getPosition":       stubItemMethod,
	"getTile":           stubItemMethod,
	"hasAttribute":      stubItemMethod,
	"getAttribute":      stubItemMethod,
	"setAttribute":      stubItemMethod,
	"removeAttribute":   stubItemMethod,
	"getCustomAttribute": stubItemMethod,
	"setCustomAttribute": stubItemMethod,
	"removeCustomAttribute": stubItemMethod,
	"canBeMoved":        stubItemMethod,
	"moveTo":            stubItemMethod,
	"transform":         stubItemMethod,
	"decay":             stubItemMethod,
	"serializeAttributes": stubItemMethod,
	"moveToSlot":        stubItemMethod,
	"getDescription":    stubItemMethod,
	"hasProperty":       stubItemMethod,
	"getImbuementSlot":  stubItemMethod,
	"getImbuement":      stubItemMethod,
	"setDuration":       stubItemMethod,
	"isInsideDepot":     stubItemMethod,
	"isContainer":       stubItemMethod,
	"getTier":           stubItemMethod,
	"setTier":           stubItemMethod,
	"getClassification": stubItemMethod,
	"canReceiveAutoCarpet": stubItemMethod,
	"setOwner":          stubItemMethod,
	"getOwnerId":        stubItemMethod,
	"isOwner":           stubItemMethod,
	"getOwnerName":      stubItemMethod,
	"hasOwner":          stubItemMethod,
	"actor":             stubItemMethod,
	"setShader":         stubItemMethod,
	"getShader":         stubItemMethod,
	"hasShader":         stubItemMethod,
}
