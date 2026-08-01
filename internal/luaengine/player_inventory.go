package luaengine

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
	lua "github.com/yuin/gopher-lua"
)

// Inventory bindings that need the item catalog (e.world.Items) for name->id
// resolution, stack sizes, and container capacity. They are registered as
// engine-method closures in registerPlayerType (like teleportTo) so they can
// reach e; the package-level stubs of the same name remain as fallbacks.

// resolveItemID reads arg n as either a numeric item id or an item name and
// resolves it to a client id. Returns (id, true) on success.
func (e *Engine) resolveItemID(L *lua.LState, n int) (uint16, bool) {
	v := L.Get(n)
	switch v.Type() {
	case lua.LTNumber:
		return uint16(lua.LVAsNumber(v)), true
	case lua.LTString:
		if e.world != nil && e.world.Items != nil {
			if id, ok := e.world.Items.IDByName(v.String()); ok {
				return id, true
			}
		}
	}
	return 0, false
}

// itemCatalog returns the world item catalog, or nil when no world is attached
// (tests). The game inventory helpers tolerate a nil catalog.
func (e *Engine) itemCatalog() *items.Catalog {
	if e.world == nil {
		return nil
	}
	return e.world.Items
}

// refreshInventory re-pushes the player's equipment slots, stats, and open
// backpack window after an inventory mutation, so the client reflects the new
// state without a full relog.
func (e *Engine) refreshInventory(p *game.Player) {
	if p.Session == nil {
		return
	}
	if e.world != nil {
		p.UpdateInventoryWeight(e.world.Items)
	}
	for slot := game.ConstSlotFirst; slot <= game.ConstSlotLast; slot++ {
		if it := p.Inventory[slot]; it != nil {
			p.Session.SendInventoryItem(uint8(slot), it)
		} else {
			p.Session.SendInventoryEmpty(uint8(slot))
		}
	}
	if bp := p.Inventory[game.ConstSlotBackpack]; bp != nil {
		p.Session.RefreshContainer(bp)
	}
	p.Session.SendStats()
}

// playerSay broadcasts player speech (e.g. the "Munch." from eating food) to
// spectators via the world, mirroring npcSay. Without this, player:say was a
// no-op stub so scripted player speech never appeared.
func (e *Engine) playerSay(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	text := L.CheckString(2)
	talkType := byte(1) // TALKTYPE_SAY
	if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTNumber {
		talkType = byte(L.ToNumber(3))
	}
	game.GlobalDispatcher.AddEvent(0, func() {
		if e.world != nil && e.world.OnCreatureSay != nil {
			e.world.OnCreatureSay(p, talkType, text)
		}
	})
	return 0
}

func (e *Engine) playerGetitemcount(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	id, ok := e.resolveItemID(L, 2)
	if !ok {
		L.Push(lua.LNumber(0))
		return 1
	}
	subType := -1
	if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTNumber {
		subType = luaOptInt(L, 3)
	}
	var cat = e.itemCatalog()
	L.Push(lua.LNumber(p.GetItemTypeCount(cat, id, subType)))
	return 1
}

func (e *Engine) playerGetitembyid(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	id, ok := e.resolveItemID(L, 2)
	if !ok {
		L.Push(lua.LNil)
		return 1
	}
	deep := luaOptBool(L, 3)
	subType := -1
	if L.GetTop() >= 4 && L.Get(4).Type() == lua.LTNumber {
		subType = luaOptInt(L, 4)
	}
	it := p.FindItemOfType(e.itemCatalog(), id, deep, subType)
	if it == nil {
		L.Push(lua.LNil)
		return 1
	}
	e.pushItem(L, it)
	return 1
}

func (e *Engine) playerAdditem(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	id, ok := e.resolveItemID(L, 2)
	if !ok {
		L.Push(lua.LNil)
		return 1
	}
	cat := e.itemCatalog()
	count := 1
	if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTNumber {
		count = luaOptInt(L, 3)
	}
	// arg 4 canDropOnMap (unused: we never drop on map here), arg 5 subType,
	// arg 6 slot.
	subType := 1
	if L.GetTop() >= 5 && L.Get(5).Type() == lua.LTNumber {
		subType = luaOptInt(L, 5)
	}
	slot := game.ConstSlotWhereever
	if L.GetTop() >= 6 && L.Get(6).Type() == lua.LTNumber {
		slot = luaOptInt(L, 6)
	}
	if count < 1 {
		count = 1
	}
	placed, _ := p.InternalAddItem(cat, id, uint32(count), subType, slot)
	e.refreshInventory(p)
	if len(placed) == 0 {
		L.Push(lua.LNil)
		return 1
	}
	if len(placed) == 1 {
		e.pushItem(L, placed[0])
		return 1
	}
	tbl := L.NewTable()
	for i, it := range placed {
		ud := L.NewUserData()
		ud.Value = luaItem{item: it}
		L.SetMetatable(ud, L.GetTypeMetatable(itemTypeName))
		tbl.RawSetInt(i+1, ud)
	}
	L.Push(tbl)
	return 1
}

func (e *Engine) playerAdditemex(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(4)) // RETURNVALUE_NOTPOSSIBLE
		return 1
	}
	it := checkItemAt(L, 2)
	if it.item == nil {
		L.Push(lua.LNumber(1))
		return 1
	}
	slot := game.ConstSlotWhereever
	if L.GetTop() >= 4 && L.Get(4).Type() == lua.LTNumber {
		slot = luaOptInt(L, 4)
	}
	cat := e.itemCatalog()
	count := uint32(it.item.Count)
	if count == 0 {
		count = 1
	}
	placed, ok := p.InternalAddItem(cat, it.item.ID, count, int(it.item.Count), slot)
	e.refreshInventory(p)
	if !ok || len(placed) == 0 {
		L.Push(lua.LNumber(5)) // RETURNVALUE_NOTENOUGHROOM
		return 1
	}
	L.Push(lua.LNumber(0)) // RETURNVALUE_NOERROR
	return 1
}

func (e *Engine) playerRemoveitem(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	id, ok := e.resolveItemID(L, 2)
	if !ok {
		L.Push(lua.LFalse)
		return 1
	}
	count := luaOptInt(L, 3)
	subType := -1
	if L.GetTop() >= 4 && L.Get(4).Type() == lua.LTNumber {
		subType = luaOptInt(L, 4)
	}
	ignoreEquipped := luaOptBool(L, 5)
	removed := p.RemoveItemOfType(e.itemCatalog(), id, uint32(count), subType, ignoreEquipped)
	if removed {
		e.refreshInventory(p)
	}
	L.Push(lua.LBool(removed))
	return 1
}

func (e *Engine) playerGetfreebackpackslots(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.GetFreeBackpackSlots(e.itemCatalog())))
	return 1
}
