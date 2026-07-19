package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const itemTypeName = "Item"

// luaItem wraps a game.Item along with its current map position if applicable.
// (In a full ECS we'd just have the Item and ask the map, but we need position for moveTo).
type luaItem struct {
	item *game.Item
	pos  game.Position
}

func (e *Engine) registerItem() {
	mt := e.L.NewTypeMetatable(itemTypeName)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), e.itemMethods()))

	e.L.SetGlobal("Item", e.L.NewFunction(e.itemCreate))
}

func (e *Engine) itemCreate(L *lua.LState) int {
	id := L.OptInt(1, 0)
	it := &game.Item{ID: uint16(id)}
	e.pushItem(L, it)
	return 1
}

func (e *Engine) pushItem(L *lua.LState, it *game.Item) {
	if it == nil {
		L.Push(lua.LNil)
		return
	}
	ud := L.NewUserData()
	ud.Value = luaItem{item: it}
	L.SetMetatable(ud, L.GetTypeMetatable(itemTypeName))
	L.Push(ud)
}

func checkItem(L *lua.LState) luaItem {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(luaItem); ok {
		return v
	}
	// Container also inherits from Item
	if v, ok := ud.Value.(luaContainer); ok {
		return luaItem{item: v.item, pos: v.pos}
	}
	L.ArgError(1, "Item expected")
	return luaItem{}
}

func stubItemMethod(L *lua.LState) int {
	return 0
}

func (e *Engine) itemMethods() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"isItem": func(L *lua.LState) int { L.Push(lua.LTrue); return 1 },
		"getId": func(L *lua.LState) int { 
			it := checkItem(L)
			L.Push(lua.LNumber(it.item.ID))
			return 1 
		},
		"getCount": func(L *lua.LState) int {
			it := checkItem(L)
			L.Push(lua.LNumber(it.item.Count))
			return 1
		},
		"moveTo": func(L *lua.LState) int {
			it := checkItem(L)
			dest := checkPosition(L, 2)
			
			// Remove from old pos if we had one (we might not track it properly in luaItem yet,
			// but if it's on map, we should remove it. Since luaItem pos might be empty, 
			// it's partially stubbed for now. Ideally, we search map or use proper entity ID).
			if it.pos.X != 0 || it.pos.Y != 0 {
				e.world.Map.RemoveItemPtr(it.pos, it.item)
			}
			
			ok := e.world.Map.AddItem(dest, it.item)
			if ok {
				it.pos = dest
			}
			
			L.Push(lua.LBool(ok))
			return 1
		},
		"getTile": stubItemMethod,
		"getContainer": stubItemMethod,
		"getParent": stubItemMethod,
		"clone": stubItemMethod,
		"split": stubItemMethod,
		"remove": stubItemMethod,
		"getActionId": stubItemMethod,
		"setActionId": stubItemMethod,
		"hasAttribute": stubItemMethod,
		"getAttribute": stubItemMethod,
		"setAttribute": stubItemMethod,
		"removeAttribute": stubItemMethod,
		"canBeMoved": stubItemMethod,
		"transform": stubItemMethod,
		"decay": stubItemMethod,
		"getDescription": stubItemMethod,
		"isInsideDepot": stubItemMethod,
		"isContainer": stubItemMethod,
	}
}
