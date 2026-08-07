package luaengine

import (
	"github.com/omurilo/canary-go/internal/game"
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
	combinedMethods := make(map[string]lua.LGFunction)
	for k, v := range e.itemMethods() {
		combinedMethods[k] = v
	}
	for k, v := range e.containerMethods() {
		combinedMethods[k] = v
	}

	// Methods must live directly on the metatable so revscriptsys.lua's
	// ItemIndex (getmetatable(self)[key]) resolves them — see registerItem.
	e.L.SetFuncs(mt, combinedMethods)

	indexTbl := e.L.NewTable()
	for k, v := range combinedMethods {
		e.L.SetField(indexTbl, k, e.L.NewFunction(v))
	}
	e.L.SetField(mt, "__index", indexTbl)

	// The global class table must BE the metatable, not a separate method table.
	//
	// revscriptsys.lua:77 replaces Container.__index with ItemIndex, which resolves
	// every key as `getmetatable(self)[key]`. The datapack then extends the class
	// with `function Container:addLoot(loot)`
	// (data/libs/functions/container.lua:8) — that assignment has to land on the
	// metatable or the lookup never finds it. C++ has the same identity, because
	// registerClass/registerMethod write the methods onto the shared class table.
	//
	// Without this, corpse:addLoot was nil and the monsterOnDropLoot chain died at
	// the point where it fills the corpse.
	ctorMt := e.L.NewTypeMetatable(containerTypeName + "_ClassCtor")
	e.L.SetField(ctorMt, "__call", e.L.NewFunction(e.containerCreate))
	e.L.SetMetatable(mt, ctorMt)
	e.L.SetGlobal(containerTypeName, mt)
}

func (e *Engine) containerCreate(L *lua.LState) int {
	id := L.OptInt(2, 0)
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

// containerContents recurses a container's contents, invoking fn for each item.
func containerContents(items []*game.Item, recursive bool, fn func(it *game.Item)) {
	for _, it := range items {
		if it == nil {
			continue
		}
		fn(it)
		if recursive && it.Container != nil && len(it.Container.Contents) > 0 {
			containerContents(it.Container.Contents, recursive, fn)
		}
	}
}

func (e *Engine) containerMethods() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"getSize": func(L *lua.LState) int {
			c := checkContainer(L)
			sz := 0
			if c.item.Container != nil {
				sz = len(c.item.Container.Contents)
			}
			L.Push(lua.LNumber(sz))
			return 1
		},
		"getCapacity": func(L *lua.LState) int {
			c := checkContainer(L)
			L.Push(lua.LNumber(c.item.ContainerCapacity(e.itemCatalog())))
			return 1
		},
		"getMaxCapacity": func(L *lua.LState) int {
			c := checkContainer(L)
			mc := uint16(0)
			if c.item.Container != nil {
				mc = c.item.Container.MaxItems
			}
			if mc == 0 {
				mc = c.item.ContainerCapacity(e.itemCatalog())
			}
			L.Push(lua.LNumber(mc))
			return 1
		},
		"getEmptySlots": func(L *lua.LState) int {
			c := checkContainer(L)
			recursive := luaOptBool(L, 2)
			sz := 0
			if c.item.Container != nil {
				sz = len(c.item.Container.Contents)
			}
			free := int(c.item.ContainerCapacity(e.itemCatalog())) - sz
			if free < 0 {
				free = 0
			}
			if recursive && c.item.Container != nil {
				containerContents(c.item.Container.Contents, false, func(it *game.Item) {
					if it.Container != nil && it.IsContainer(e.itemCatalog()) {
						f := int(it.ContainerCapacity(e.itemCatalog())) - len(it.Container.Contents)
						if f > 0 {
							free += f
						}
					}
				})
			}
			L.Push(lua.LNumber(free))
			return 1
		},
		"getItem": func(L *lua.LState) int {
			c := checkContainer(L)
			idx := luaOptInt(L, 2)
			if c.item.Container == nil || idx < 0 || idx >= len(c.item.Container.Contents) {
				L.Push(lua.LNil)
				return 1
			}
			e.pushItem(L, c.item.Container.Contents[idx])
			return 1
		},
		"getItems": func(L *lua.LState) int {
			c := checkContainer(L)
			recursive := luaOptBool(L, 2)
			tbl := L.NewTable()
			i := 1
			if c.item.Container != nil {
				containerContents(c.item.Container.Contents, recursive, func(it *game.Item) {
					ud := L.NewUserData()
				ud.Value = luaItem{item: it}
				L.SetMetatable(ud, L.GetTypeMetatable(itemTypeName))
					tbl.RawSetInt(i, ud)
					i++
				})
			}
			L.Push(tbl)
			return 1
		},
		"addItem": func(L *lua.LState) int {
			c := checkContainer(L)
			itemID, ok := e.resolveItemID(L, 2)
			if !ok {
				L.Push(lua.LFalse)
				return 1
			}
			count := uint16(luaOptInt(L, 3))
			if count == 0 {
				count = 1
			}
			cat := e.itemCatalog()
			if it := cat.Get(itemID); it != nil && it.Stackable && count > 100 {
				count = 100
			}
			if c.item.Container == nil {
				L.Push(lua.LFalse)
				return 1
			}
			// Reject when the container is full.
			if int(c.item.ContainerCapacity(cat)) <= len(c.item.Container.Contents) {
				L.Push(lua.LFalse)
				return 1
			}
			newItem := &game.Item{ID: itemID, Count: count}
			if newItem.IsContainer(cat) {
				newItem.Container = game.NewContainer(0)
				newItem.Container.Parent = c.item
			}
			c.item.Container.Contents = append(c.item.Container.Contents, newItem)
			e.pushItem(L, newItem)
			return 1
		},
		"hasItem": func(L *lua.LState) int {
			c := checkContainer(L)
			target := checkItemAt(L, 2)
			found := false
			if c.item.Container != nil {
				containerContents(c.item.Container.Contents, true, func(it *game.Item) {
					if it == target.item {
						found = true
					}
				})
			}
			L.Push(lua.LBool(found))
			return 1
		},
		"getItemCountById": func(L *lua.LState) int {
			c := checkContainer(L)
			id, ok := e.resolveItemID(L, 2)
			if !ok {
				L.Push(lua.LNumber(0))
				return 1
			}
			subType := -1
			if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTNumber {
				subType = luaOptInt(L, 3)
			}
			var total uint32
			cat := e.itemCatalog()
			if c.item.Container != nil {
				containerContents(c.item.Container.Contents, true, func(it *game.Item) {
					if it.ID == id {
						if subType == -1 || subType == int(it.Count) {
							total += uint32(it.Count)
							if it.Count == 0 {
								total++
							}
						}
					}
				})
			}
			_ = cat
			L.Push(lua.LNumber(total))
			return 1
		},
		"getItemHoldingCount": func(L *lua.LState) int {
			c := checkContainer(L)
			L.Push(lua.LNumber(c.item.HoldingCount()))
			return 1
		},
		"getContentDescription": func(L *lua.LState) int {
			c := checkContainer(L)
			if c.item.Container == nil || len(c.item.Container.Contents) == 0 {
				L.Push(lua.LString("nothing"))
				return 1
			}
			L.Push(lua.LString(""))
			return 1
		},
		"removeItemById": func(L *lua.LState) int {
			c := checkContainer(L)
			id, ok := e.resolveItemID(L, 2)
			if !ok {
				L.Push(lua.LFalse)
				return 1
			}
			count := luaOptInt(L, 3)
			if count <= 0 {
				count = 1
			}
			if c.item.Container == nil {
				L.Push(lua.LFalse)
				return 1
			}
			remaining := uint32(count)
			out := c.item.Container.Contents[:0]
			for _, it := range c.item.Container.Contents {
				if it == nil {
					continue
				}
				if remaining > 0 && it.ID == id {
					if it.Count <= uint16(remaining) {
						remaining -= uint32(it.Count)
						continue
					}
					it.Count -= uint16(remaining)
					remaining = 0
				}
				out = append(out, it)
			}
			c.item.Container.Contents = out
			L.Push(lua.LBool(remaining == 0))
			return 1
		},
		"addItemEx": func(L *lua.LState) int {
			// container:addItemEx(item, index?, flags?) moves an existing Item
			// into this container. Returns RETURNVALUE_NOERROR (0) on success
			// (C++ Cylinder::addItemEx). FLAG_NOLIMIT (bit 0) bypasses the
			// capacity check — used by store delivery (addItemStoreInbox).
			c := checkContainer(L)
			ud, ok := L.Get(2).(*lua.LUserData)
			if !ok {
				L.Push(lua.LNumber(4)) // RETURNVALUE_NOTPOSSIBLE
				return 1
			}
			li, ok := ud.Value.(luaItem)
			if !ok || li.item == nil {
				L.Push(lua.LNumber(4)) // RETURNVALUE_NOTPOSSIBLE
				return 1
			}
			flags := 0
			if L.GetTop() >= 4 {
				flags = luaOptInt(L, 4)
			}
			noLimit := flags&1 != 0
			cat := e.itemCatalog()
			if !noLimit && c.item.Container != nil && int(c.item.ContainerCapacity(cat)) <= len(c.item.Container.Contents) {
				L.Push(lua.LNumber(23)) // RETURNVALUE_CONTAINERNOTENOUGHROOM
				return 1
			}
			if c.item.Container != nil {
				if li.item.Container != nil {
					li.item.Container.Parent = c.item
				}
				c.item.Container.Contents = append(c.item.Container.Contents, li.item)
			}
			L.Push(lua.LNumber(0)) // RETURNVALUE_NOERROR
			return 1
		},
		"getCorpseOwner": func(L *lua.LState) int {
			c := checkContainer(L)
			if c.item == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(0)) // stub - corpse owner tracking not implemented
			return 1
		},
		"registerReward": func(L *lua.LState) int {
			c := checkContainer(L)
			if c.item == nil {
				return 0
			}
			L.Push(lua.LTrue)
			return 1
		},
		"removeAllItems": func(L *lua.LState) int {
			c := checkContainer(L)
			if c.item.Container != nil {
				c.item.Container.Contents = nil
			}
			L.Push(lua.LTrue)
			return 1
		},
	}
}

// ContainerValue returns the Container userdata for an item without pushing it, so
// callers outside this package (the event engine) can pass a corpse to Lua. The
// payload must be a luaContainer, which is why this cannot be built elsewhere.
func (e *Engine) ContainerValue(it *game.Item) lua.LValue {
	if it == nil {
		return lua.LNil
	}
	ud := e.L.NewUserData()
	ud.Value = luaContainer{item: it}
	e.L.SetMetatable(ud, e.L.GetTypeMetatable(containerTypeName))
	return ud
}
