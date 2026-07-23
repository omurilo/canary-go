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

	// Store methods DIRECTLY on the metatable. The datapack's revscriptsys.lua
	// overwrites Item.__index with an ItemIndex(self,key) that resolves methods
	// via getmetatable(self)[key] (e.g. itemid -> methods.getId(self)); if the
	// methods live only behind a custom __index function, that lookup returns
	// nil and `item.itemid` crashes (revscriptsys.lua:66). See the same contract
	// for Player/Monster/Npc.
	e.L.SetFuncs(mt, methods)

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
	id := L.OptInt(2, 0)
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

// itemRemove consumes `count` from the item's stack (default the whole stack),
// mirroring Item::remove used by food/rune scripts. It only mutates the model
// count here; the protocol layer that invoked the action reconciles the client
// (removing the node and refreshing the container/slot) once the action
// returns. Count 0 marks the item as fully consumed.
func itemRemove(L *lua.LState) int {
	it := checkItem(L)
	if it.item == nil {
		L.Push(lua.LFalse)
		return 1
	}
	count := 1
	if L.GetTop() >= 2 && L.Get(2).Type() == lua.LTNumber {
		count = luaOptInt(L, 2)
	}
	if count < 1 {
		count = 1
	}
	if int(it.item.Count) > count {
		it.item.Count -= uint16(count)
	} else {
		it.item.Count = 0
	}
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) itemMethods() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"isItem": func(L *lua.LState) int { L.Push(lua.LTrue); return 1 },
		"hasProperty": func(L *lua.LState) int {
			it := checkItem(L)
			_ = L.OptInt(2, 0)
			var has bool
			if cat := e.itemCatalog(); cat != nil {
				if ct := cat.Get(it.item.ID); ct != nil {
					has = ct.BlockSolid
				}
			}
			L.Push(lua.LBool(has))
			return 1
		},
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
		// getSubType/getUniqueId back revscriptsys ItemIndex's "type"/"uid" keys.
		"getSubType": func(L *lua.LState) int {
			it := checkItem(L)
			L.Push(lua.LNumber(it.item.Count))
			return 1
		},
		"getUniqueId": func(L *lua.LState) int {
			it := checkItem(L)
			if it.item.Attr != nil && it.item.Attr.UniqueID != nil {
				L.Push(lua.LNumber(*it.item.Attr.UniqueID))
			} else {
				L.Push(lua.LNumber(0))
			}
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
			if it.item != nil {
				for _, p := range e.world.Players() {
					found := false
					p.WalkInventory(func(inventoryItem *game.Item) {
						if inventoryItem == it.item {
							found = true
						}
					})
					if found {
						pushPosition(L, p.GetPosition())
						return 1
					}
				}
			}
			pushPosition(L, it.pos)
			return 1
		},
		"getTile": stubItemMethod,
		"getContainer": stubItemMethod,
		"getParent": stubItemMethod,
		"clone": stubItemMethod,
		"split": stubItemMethod,
		"remove": itemRemove,
		"getTier": func(L *lua.LState) int {
			it := checkItem(L)
			L.Push(lua.LNumber(it.item.GetTier()))
			return 1
		},
		"setTier": func(L *lua.LState) int {
			it := checkItem(L)
			tier := uint8(L.CheckInt(2))
			it.item.SetTier(tier)
			L.Push(lua.LBool(true))
			return 1
		},
		"getActionId": func(L *lua.LState) int {
			it := checkItem(L)
			if it.item.Attr != nil && it.item.Attr.ActionID != nil {
				L.Push(lua.LNumber(*it.item.Attr.ActionID))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		},
		"setActionId": func(L *lua.LState) int {
			it := checkItem(L)
			actionId := uint16(L.CheckInt(2))
			if it.item.Attr == nil {
				it.item.Attr = &game.ItemAttributes{}
			}
			it.item.Attr.ActionID = &actionId
			L.Push(lua.LBool(true))
			return 1
		},
		"hasAttribute": func(L *lua.LState) int {
			it := checkItem(L)
			attrId := L.CheckInt(2)
			has := false
			if it.item.Attr != nil {
				switch attrId {
				case 1: // ACTIONID
					has = it.item.Attr.ActionID != nil
				case 2: // UNIQUEID
					has = it.item.Attr.UniqueID != nil
				case 3: // DESCRIPTION
					has = it.item.Attr.Description != nil
				case 4: // TEXT
					has = it.item.Attr.Text != nil
				case 5: // DATE
					has = it.item.Attr.WrittenDate != nil
				case 6: // WRITER
					has = it.item.Attr.WrittenBy != nil
				case 7: // NAME
					has = it.item.Attr.Name != nil
				case 8: // ARTICLE
					has = it.item.Attr.Article != nil
				case 9: // PLURALNAME
					has = it.item.Attr.PluralName != nil
				case 10: // WEIGHT
					has = it.item.Attr.Weight != nil
				case 11: // ATTACK
					has = it.item.Attr.Attack != nil
				case 12: // DEFENSE
					has = it.item.Attr.Defense != nil
				case 13: // EXTRADEFENSE
					has = it.item.Attr.ExtraDefense != nil
				case 14: // ARMOR
					has = it.item.Attr.Armor != nil
				case 15: // HITCHANCE
					has = it.item.Attr.HitChance != nil
				case 16: // SHOOTRANGE
					has = it.item.Attr.ShootRange != nil
				case 17: // OWNER
					has = it.item.Attr.Owner != nil
				case 18: // DURATION
					has = it.item.Attr.Duration != nil
				case 19: // DECAYSTATE
					has = it.item.Attr.DecayState != nil
				case 21: // CHARGES
					has = it.item.Attr.Charges != nil
				case 29: // AMOUNT
					has = it.item.Attr.Amount != nil
				case 30: // TIER
					has = it.item.Attr.Tier != nil
				}
			}
			if !has && attrId == 21 { // Fallback for CHARGES
				if catalogItem := e.world.Items.Get(it.item.ID); catalogItem != nil && catalogItem.Charges > 0 {
					has = true
				}
			}
			L.Push(lua.LBool(has))
			return 1
		},
		"getAttribute": func(L *lua.LState) int {
			it := checkItem(L)
			attrId := L.CheckInt(2)
			if it.item.Attr != nil {
				switch attrId {
				case 1: // ACTIONID
					if it.item.Attr.ActionID != nil {
						L.Push(lua.LNumber(*it.item.Attr.ActionID))
						return 1
					}
				case 2: // UNIQUEID
					if it.item.Attr.UniqueID != nil {
						L.Push(lua.LNumber(*it.item.Attr.UniqueID))
						return 1
					}
				case 3: // DESCRIPTION
					if it.item.Attr.Description != nil {
						L.Push(lua.LString(*it.item.Attr.Description))
						return 1
					}
				case 4: // TEXT
					if it.item.Attr.Text != nil {
						L.Push(lua.LString(*it.item.Attr.Text))
						return 1
					}
				case 5: // DATE
					if it.item.Attr.WrittenDate != nil {
						L.Push(lua.LNumber(*it.item.Attr.WrittenDate))
						return 1
					}
				case 6: // WRITER
					if it.item.Attr.WrittenBy != nil {
						L.Push(lua.LString(*it.item.Attr.WrittenBy))
						return 1
					}
				case 7: // NAME
					if it.item.Attr.Name != nil {
						L.Push(lua.LString(*it.item.Attr.Name))
						return 1
					}
				case 8: // ARTICLE
					if it.item.Attr.Article != nil {
						L.Push(lua.LString(*it.item.Attr.Article))
						return 1
					}
				case 9: // PLURALNAME
					if it.item.Attr.PluralName != nil {
						L.Push(lua.LString(*it.item.Attr.PluralName))
						return 1
					}
				case 10: // WEIGHT
					if it.item.Attr.Weight != nil {
						L.Push(lua.LNumber(*it.item.Attr.Weight))
						return 1
					}
				case 11: // ATTACK
					if it.item.Attr.Attack != nil {
						L.Push(lua.LNumber(*it.item.Attr.Attack))
						return 1
					}
				case 12: // DEFENSE
					if it.item.Attr.Defense != nil {
						L.Push(lua.LNumber(*it.item.Attr.Defense))
						return 1
					}
				case 13: // EXTRADEFENSE
					if it.item.Attr.ExtraDefense != nil {
						L.Push(lua.LNumber(*it.item.Attr.ExtraDefense))
						return 1
					}
				case 14: // ARMOR
					if it.item.Attr.Armor != nil {
						L.Push(lua.LNumber(*it.item.Attr.Armor))
						return 1
					}
				case 15: // HITCHANCE
					if it.item.Attr.HitChance != nil {
						L.Push(lua.LNumber(*it.item.Attr.HitChance))
						return 1
					}
				case 16: // SHOOTRANGE
					if it.item.Attr.ShootRange != nil {
						L.Push(lua.LNumber(*it.item.Attr.ShootRange))
						return 1
					}
				case 17: // OWNER
					if it.item.Attr.Owner != nil {
						L.Push(lua.LNumber(*it.item.Attr.Owner))
						return 1
					}
				case 18: // DURATION
					if it.item.Attr.Duration != nil {
						L.Push(lua.LNumber(*it.item.Attr.Duration))
						return 1
					}
				case 19: // DECAYSTATE
					if it.item.Attr.DecayState != nil {
						L.Push(lua.LNumber(*it.item.Attr.DecayState))
						return 1
					}
				case 21: // CHARGES
					if it.item.Attr.Charges != nil {
						L.Push(lua.LNumber(*it.item.Attr.Charges))
						return 1
					}
				case 29: // AMOUNT
					if it.item.Attr.Amount != nil {
						L.Push(lua.LNumber(*it.item.Attr.Amount))
						return 1
					}
				case 30: // TIER
					if it.item.Attr.Tier != nil {
						L.Push(lua.LNumber(*it.item.Attr.Tier))
						return 1
					}
				}
			}
			if attrId == 21 { // Fallback for CHARGES
				if catalogItem := e.world.Items.Get(it.item.ID); catalogItem != nil && catalogItem.Charges > 0 {
					L.Push(lua.LNumber(catalogItem.Charges))
					return 1
				}
			}
			L.Push(lua.LNil)
			return 1
		},
		"setAttribute": func(L *lua.LState) int {
			it := checkItem(L)
			attrId := L.CheckInt(2)
			if it.item.Attr == nil {
				it.item.Attr = &game.ItemAttributes{}
			}
			switch attrId {
			case 1: // ACTIONID
				n := uint16(L.CheckInt(3))
				it.item.Attr.ActionID = &n
			case 2: // UNIQUEID
				n := uint16(L.CheckInt(3))
				it.item.Attr.UniqueID = &n
			case 3: // DESCRIPTION
				s := L.CheckString(3)
				it.item.Attr.Description = &s
			case 4: // TEXT
				s := L.CheckString(3)
				it.item.Attr.Text = &s
			case 5: // DATE
				n := uint64(L.CheckInt(3))
				it.item.Attr.WrittenDate = &n
			case 6: // WRITER
				s := L.CheckString(3)
				it.item.Attr.WrittenBy = &s
			case 7: // NAME
				s := L.CheckString(3)
				it.item.Attr.Name = &s
			case 8: // ARTICLE
				s := L.CheckString(3)
				it.item.Attr.Article = &s
			case 9: // PLURALNAME
				s := L.CheckString(3)
				it.item.Attr.PluralName = &s
			case 10: // WEIGHT
				n := uint32(L.CheckInt(3))
				it.item.Attr.Weight = &n
			case 11: // ATTACK
				n := int32(L.CheckInt(3))
				it.item.Attr.Attack = &n
			case 12: // DEFENSE
				n := int32(L.CheckInt(3))
				it.item.Attr.Defense = &n
			case 13: // EXTRADEFENSE
				n := int32(L.CheckInt(3))
				it.item.Attr.ExtraDefense = &n
			case 14: // ARMOR
				n := int32(L.CheckInt(3))
				it.item.Attr.Armor = &n
			case 15: // HITCHANCE
				n := int8(L.CheckInt(3))
				it.item.Attr.HitChance = &n
			case 16: // SHOOTRANGE
				n := uint8(L.CheckInt(3))
				it.item.Attr.ShootRange = &n
			case 17: // OWNER
				n := uint32(L.CheckInt(3))
				it.item.Attr.Owner = &n
			case 18: // DURATION
				n := int32(L.CheckInt(3))
				it.item.Attr.Duration = &n
			case 19: // DECAYSTATE
				n := uint8(L.CheckInt(3))
				it.item.Attr.DecayState = &n
			case 21: // CHARGES
				n := uint16(L.CheckInt(3))
				it.item.Attr.Charges = &n
			case 29: // AMOUNT
				n := uint16(L.CheckInt(3))
				it.item.Attr.Amount = &n
			case 30: // TIER
				n := uint8(L.CheckInt(3))
				it.item.Attr.Tier = &n
			}
			L.Push(lua.LBool(true))
			return 1
		},
		"removeAttribute": func(L *lua.LState) int {
			it := checkItem(L)
			attrId := L.CheckInt(2)
			if it.item.Attr == nil {
				L.Push(lua.LBool(false))
				return 1
			}
			removed := false
			switch attrId {
			case 1: // ACTIONID
				if it.item.Attr.ActionID != nil {
					it.item.Attr.ActionID = nil
					removed = true
				}
			case 2: // UNIQUEID
				if it.item.Attr.UniqueID != nil {
					it.item.Attr.UniqueID = nil
					removed = true
				}
			case 3: // DESCRIPTION
				if it.item.Attr.Description != nil {
					it.item.Attr.Description = nil
					removed = true
				}
			case 4: // TEXT
				if it.item.Attr.Text != nil {
					it.item.Attr.Text = nil
					removed = true
				}
			case 5: // DATE
				if it.item.Attr.WrittenDate != nil {
					it.item.Attr.WrittenDate = nil
					removed = true
				}
			case 6: // WRITER
				if it.item.Attr.WrittenBy != nil {
					it.item.Attr.WrittenBy = nil
					removed = true
				}
			case 7: // NAME
				if it.item.Attr.Name != nil {
					it.item.Attr.Name = nil
					removed = true
				}
			case 8: // ARTICLE
				if it.item.Attr.Article != nil {
					it.item.Attr.Article = nil
					removed = true
				}
			case 9: // PLURALNAME
				if it.item.Attr.PluralName != nil {
					it.item.Attr.PluralName = nil
					removed = true
				}
			case 10: // WEIGHT
				if it.item.Attr.Weight != nil {
					it.item.Attr.Weight = nil
					removed = true
				}
			case 11: // ATTACK
				if it.item.Attr.Attack != nil {
					it.item.Attr.Attack = nil
					removed = true
				}
			case 12: // DEFENSE
				if it.item.Attr.Defense != nil {
					it.item.Attr.Defense = nil
					removed = true
				}
			case 13: // EXTRADEFENSE
				if it.item.Attr.ExtraDefense != nil {
					it.item.Attr.ExtraDefense = nil
					removed = true
				}
			case 14: // ARMOR
				if it.item.Attr.Armor != nil {
					it.item.Attr.Armor = nil
					removed = true
				}
			case 15: // HITCHANCE
				if it.item.Attr.HitChance != nil {
					it.item.Attr.HitChance = nil
					removed = true
				}
			case 16: // SHOOTRANGE
				if it.item.Attr.ShootRange != nil {
					it.item.Attr.ShootRange = nil
					removed = true
				}
			case 17: // OWNER
				if it.item.Attr.Owner != nil {
					it.item.Attr.Owner = nil
					removed = true
				}
			case 18: // DURATION
				if it.item.Attr.Duration != nil {
					it.item.Attr.Duration = nil
					removed = true
				}
			case 19: // DECAYSTATE
				if it.item.Attr.DecayState != nil {
					it.item.Attr.DecayState = nil
					removed = true
				}
			case 21: // CHARGES
				if it.item.Attr.Charges != nil {
					it.item.Attr.Charges = nil
					removed = true
				}
			case 29: // AMOUNT
				if it.item.Attr.Amount != nil {
					it.item.Attr.Amount = nil
					removed = true
				}
			case 30: // TIER
				if it.item.Attr.Tier != nil {
					it.item.Attr.Tier = nil
					removed = true
				}
			}
			L.Push(lua.LBool(removed))
			return 1
		},
		"canBeMoved": stubItemMethod,
		"transform": e.itemTransform,
		"decay": stubItemMethod,
		"setDuration": stubItemMethod,
		"stopDecay": stubItemMethod,
		"getDescription": e.itemGetDescription,
		"isInsideDepot": stubItemMethod,
		"isContainer": stubItemMethod,
		"actor": func(L *lua.LState) int {
			it := checkItem(L)
			if L.GetTop() == 1 {
				L.Push(lua.LBool(it.item.Actor))
			} else {
				it.item.Actor = L.CheckBool(2)
				L.Push(lua.LTrue)
			}
			return 1
		},
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
