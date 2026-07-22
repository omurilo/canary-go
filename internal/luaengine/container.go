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

	e.setClassConstructor("Container", e.containerCreate, combinedMethods)
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
		if recursive && len(it.Contents) > 0 {
			containerContents(it.Contents, recursive, fn)
		}
	}
}

func (e *Engine) containerMethods() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"getSize": func(L *lua.LState) int {
			c := checkContainer(L)
			L.Push(lua.LNumber(len(c.item.Contents)))
			return 1
		},
		"getCapacity": func(L *lua.LState) int {
			c := checkContainer(L)
			L.Push(lua.LNumber(c.item.ContainerCapacity(e.itemCatalog())))
			return 1
		},
		"getMaxCapacity": func(L *lua.LState) int {
			c := checkContainer(L)
			mc := c.item.MaxItems
			if mc == 0 {
				mc = c.item.ContainerCapacity(e.itemCatalog())
			}
			L.Push(lua.LNumber(mc))
			return 1
		},
		"getEmptySlots": func(L *lua.LState) int {
			c := checkContainer(L)
			recursive := luaOptBool(L, 2)
			free := int(c.item.ContainerCapacity(e.itemCatalog())) - len(c.item.Contents)
			if free < 0 {
				free = 0
			}
			if recursive {
				containerContents(c.item.Contents, false, func(it *game.Item) {
					if it.IsContainer(e.itemCatalog()) {
						f := int(it.ContainerCapacity(e.itemCatalog())) - len(it.Contents)
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
			if idx < 0 || idx >= len(c.item.Contents) {
				L.Push(lua.LNil)
				return 1
			}
			e.pushItem(L, c.item.Contents[idx])
			return 1
		},
		"getItems": func(L *lua.LState) int {
			c := checkContainer(L)
			recursive := luaOptBool(L, 2)
			tbl := L.NewTable()
			i := 1
			containerContents(c.item.Contents, recursive, func(it *game.Item) {
				ud := L.NewUserData()
				ud.Value = luaItem{item: it}
				L.SetMetatable(ud, L.GetTypeMetatable(itemTypeName))
				tbl.RawSetInt(i, ud)
				i++
			})
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
			// Reject when the container is full.
			if int(c.item.ContainerCapacity(cat)) <= len(c.item.Contents) {
				L.Push(lua.LFalse)
				return 1
			}
			newItem := &game.Item{ID: itemID, Count: count, Parent: c.item}
			c.item.Contents = append(c.item.Contents, newItem)
			e.pushItem(L, newItem)
			return 1
		},
		"hasItem": func(L *lua.LState) int {
			c := checkContainer(L)
			target := checkItem(L)
			found := false
			containerContents(c.item.Contents, true, func(it *game.Item) {
				if it == target.item {
					found = true
				}
			})
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
			containerContents(c.item.Contents, true, func(it *game.Item) {
				if it.ID == id {
					if subType == -1 || subType == int(it.Count) {
						total += uint32(it.Count)
						if it.Count == 0 {
							total++
						}
					}
				}
			})
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
			if len(c.item.Contents) == 0 {
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
			remaining := uint32(count)
			out := c.item.Contents[:0]
			for _, it := range c.item.Contents {
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
			c.item.Contents = out
			L.Push(lua.LBool(remaining == 0))
			return 1
		},
		"addItemEx":      stubContainerMethod,
		"getCorpseOwner": stubContainerMethod,
		"registerReward": stubContainerMethod,
		"removeAllItems": func(L *lua.LState) int {
			c := checkContainer(L)
			c.item.Contents = nil
			L.Push(lua.LTrue)
			return 1
		},
	}
}
