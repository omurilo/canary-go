package luaengine

import (
	"fmt"

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
	methods := e.itemMethods()
	
	// Create method table
	methodTable := e.L.SetFuncs(e.L.NewTable(), methods)
	
	e.L.SetField(mt, "__index", e.L.NewFunction(func(L *lua.LState) int {
		it := checkItem(L)
		key := L.CheckString(2)
		
		switch key {
		case "itemid":
			L.Push(lua.LNumber(it.item.ID))
			return 1
		case "actionid":
			if it.item.Attr != nil && it.item.Attr.ActionID != nil {
				L.Push(lua.LNumber(*it.item.Attr.ActionID))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		case "type", "count":
			L.Push(lua.LNumber(it.item.Count))
			return 1
		case "uid":
			if it.item.Attr != nil && it.item.Attr.UniqueID != nil {
				L.Push(lua.LNumber(*it.item.Attr.UniqueID))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		}
		
		// Fallback to method
		val := methodTable.RawGetString(key)
		L.Push(val)
		return 1
	}))

	e.setClassConstructor("Item", e.itemCreate, methods)
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
			
			ok := e.world.AddItem(dest, it.item)
			if ok {
				it.pos = dest
			}
			
			L.Push(lua.LBool(ok))
			return 1
		},
		"getPosition": func(L *lua.LState) int {
			it := checkItem(L)
			pushPosition(L, it.pos)
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
		"transform": e.itemTransform,
		"decay": stubItemMethod,
		"getDescription": e.itemGetDescription,
		"isInsideDepot": stubItemMethod,
		"isContainer": stubItemMethod,
	}
}

func (e *Engine) itemGetDescription(L *lua.LState) int {
	li := checkItem(L)
	if li.item == nil {
		L.Push(lua.LString(""))
		return 1
	}
	
	it := e.world.Items.Get(li.item.ID)
	if it == nil {
		L.Push(lua.LString("an item of type " + fmt.Sprint(li.item.ID)))
		return 1
	}

	name := it.Name
	if name == "" {
		name = "an item of type " + fmt.Sprint(li.item.ID)
	}

	article := it.Article
	if article == "" {
		article = "a"
	}

	var desc string
	if it.Description != "" {
		desc = "\n" + it.Description
	}

	// Just a basic "You see a sword." for now.
	// Weight, attack, armor can be added later.
	text := "You see " + article + " " + name + "." + desc

	L.Push(lua.LString(text))
	return 1
}

func (e *Engine) itemTransform(L *lua.LState) int {
	li := checkItem(L)
	if li.item == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	newID := uint16(L.CheckNumber(2))
	if newID > 0 {
		e.world.TransformItem(li.pos, li.item, newID)
	}
	L.Push(lua.LBool(true))
	return 1
}
