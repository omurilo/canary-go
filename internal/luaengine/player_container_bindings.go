package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// Player container bindings. Open-container state lives on game.Player
// (openContainers) and is shared with the protocol layer, so getContainerId/
// ById/Index reflect the client's real open windows.

func (e *Engine) playerGetcontainerid(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(-1))
		return 1
	}
	c := checkContainer(L)
	if c.item == nil {
		L.Push(lua.LNumber(-1))
		return 1
	}
	L.Push(lua.LNumber(p.GetContainerID(c.item)))
	return 1
}

func (e *Engine) playerGetcontainerbyid(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	cid := uint8(luaOptInt(L, 2))
	c := p.GetContainerByID(cid)
	if c == nil {
		L.Push(lua.LNil)
		return 1
	}
	e.pushContainer(L, c)
	return 1
}

func (e *Engine) playerGetcontainerindex(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	cid := uint8(luaOptInt(L, 2))
	L.Push(lua.LNumber(p.GetContainerIndex(cid)))
	return 1
}

func (e *Engine) playerSendcontainer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	c := checkContainer(L)
	if c.item != nil && p.Session != nil {
		p.Session.OpenContainer(c.item)
	}
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) playerSendupdatecontainer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	c := checkContainer(L)
	if c.item != nil && p.Session != nil {
		p.Session.RefreshContainer(c.item)
	}
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) playerAdditembatchtopaginedcontainer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	c := checkContainer(L)
	if c.item == nil || !c.item.Pagination {
		L.Push(lua.LNumber(0)) // only paginated containers accept batch adds
		return 1
	}
	id, ok := e.resolveItemID(L, 3)
	if !ok {
		L.Push(lua.LNumber(0))
		return 1
	}
	count := luaOptInt(L, 4)
	if count < 1 {
		count = 1
	}
	cat := e.itemCatalog()
	capacity := int(c.item.ContainerCapacity(cat))
	actuallyAdded := 0
	for actuallyAdded < count && len(c.item.Contents) < capacity {
		c.item.Contents = append(c.item.Contents, &game.Item{ID: id, Count: 1, Parent: c.item})
		actuallyAdded++
	}
	if actuallyAdded > 0 && p.Session != nil {
		p.Session.RefreshContainer(c.item)
	}
	L.Push(lua.LNumber(actuallyAdded))
	return 1
}
