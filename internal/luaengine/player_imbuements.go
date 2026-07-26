package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) playerApplyscrollimbuement(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}

	item := checkItemAt(L, 2)
	scrollItem := checkItemAt(L, 3)
	if item.item == nil || scrollItem.item == nil {
		L.Push(lua.LFalse)
		return 1
	}

	imbReg := e.world.Imbuements
	imb := imbReg.GetImbuementByScrollID(scrollItem.item.ID)
	if imb == nil {
		L.Push(lua.LFalse)
		return 1
	}

	baseImb := imbReg.GetBaseByID(imb.BaseID)
	if baseImb == nil {
		L.Push(lua.LFalse)
		return 1
	}

	slot := -1
	it := e.world.Items.Get(item.item.ID)
	if it != nil {
		for i := uint8(0); i < 3; i++ {
			if info, ok := item.item.GetImbuementInfo(i); !ok || info.ID == 0 {
				slot = int(i)
				break
			}
		}
	}
	
	if slot < 0 {
		L.Push(lua.LFalse)
		return 1
	}

	if !p.RemoveItemOfType(e.world.Items, scrollItem.item.ID, 1, -1, false) {
		L.Push(lua.LFalse)
		return 1
	}

	item.item.SetImbuement(uint8(slot), imb.ID, baseImb.Duration)

	// Since we removed an item from inventory without notifying the client yet,
	// let's assume the caller will trigger refreshContainers or sendInventoryItem.
	L.Push(lua.LTrue)
	return 1
}
