package luaengine

import (
	"fmt"
	"github.com/omurilo/canary-go/internal/items"
	"math"
	"strconv"

	"github.com/omurilo/canary-go/internal/game"
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
	L.Push(e.itemValue(L, it))
}

// itemValue wraps an item as a Lua value without pushing it, for the call sites
// that store items in a table instead of returning them directly.
func (e *Engine) itemValue(L *lua.LState, it *game.Item) lua.LValue {
	if it == nil {
		return lua.LNil
	}
	ud := L.NewUserData()
	ud.Value = luaItem{item: it}
	L.SetMetatable(ud, L.GetTypeMetatable(itemTypeName))
	return ud
}

func checkItem(L *lua.LState) luaItem {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(luaItem); ok {
		return v
	}
	if v, ok := ud.Value.(*game.Item); ok {
		return luaItem{item: v}
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
		// item:hasProperty(prop), a port of Item::hasProperty (item.cpp). The property
		// argument used to be discarded and BlockSolid answered every question, so
		// asking about BLOCKPROJECTILE, BLOCKPATH or any of the immovable/nofield
		// variants got the wrong answer for every item that is not simply solid.
		"hasProperty": func(L *lua.LState) int {
			it := checkItem(L)
			prop := items.ItemProperty(L.OptInt(2, 0))
			var has bool
			if cat := e.itemCatalog(); cat != nil && it.item != nil {
				has = cat.Get(it.item.ID).HasProperty(prop)
			}
			L.Push(lua.LBool(has))
			return 1
		},
		"getId": func(L *lua.LState) int {
			it := checkItem(L)
			L.Push(lua.LNumber(it.item.ID))
			return 1
		},
		"getName": func(L *lua.LState) int {
			it := checkItem(L)
			if it.item != nil {
				if cat := e.itemCatalog(); cat != nil {
					if ct := cat.Get(it.item.ID); ct != nil && ct.Name != "" {
						L.Push(lua.LString(ct.Name))
						return 1
					}
				}
			}
			L.Push(lua.LString(""))
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
			if L.GetTop() < 2 {
				L.Push(lua.LFalse)
				return 1
			}

			// Try as Item (Container) first
			ud := L.CheckUserData(2)
			if destItem, ok := ud.Value.(*luaItem); ok {
				if it.pos.X != 0 || it.pos.Y != 0 {
					e.world.RemoveMapItem(it.pos, it.item)
					it.pos = game.Position{}
				} else {
					// We need to detach it from the player's inventory if it's there
					for _, p := range e.world.Players() {
						p.WalkInventory(func(inventoryItem *game.Item) {
							if inventoryItem == it.item {
								// In a full ECS, we'd remove it from the slot properly.
								// For now, if we are moving it, we could remove it from parent
								if it.item.Parent != nil {
									for i, child := range it.item.Parent.Contents {
										if child == it.item {
											it.item.Parent.Contents = append(it.item.Parent.Contents[:i], it.item.Parent.Contents[i+1:]...)
											break
										}
									}
								} else {
									// It's in a slot, we'd need to clear the slot
									for i, slotItem := range p.Inventory {
										if slotItem == it.item {
											p.Inventory[i] = nil
										}
									}
								}
							}
						})
					}
				}
				destItem.item.Contents = append(destItem.item.Contents, it.item)
				it.item.Parent = destItem.item
				L.Push(lua.LTrue)
				return 1
			}

			// Otherwise, it must be a Position
			if dest, ok := ud.Value.(game.Position); ok {
				if it.pos.X != 0 || it.pos.Y != 0 {
					e.world.RemoveMapItem(it.pos, it.item)
				}
				ok := e.world.AddItem(dest, it.item)
				if ok {
					it.pos = dest
				}
				L.Push(lua.LBool(ok))
				return 1
			}

			L.Push(lua.LFalse)
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
		"getTile":      stubItemMethod,
		"getContainer": stubItemMethod,
		"getParent":    stubItemMethod,
		"clone":        stubItemMethod,
		"split":        stubItemMethod,
		"remove": func(L *lua.LState) int {
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
			// Item::remove goes through internalRemoveItem, which removes from
			// whatever cylinder holds the item — a tile, a container or the
			// inventory. Keying only off it.pos meant an item that was not lying on
			// that tile (a potion in the hand, loot inside a bag) was never
			// decremented: the tile removal silently found nothing and the item kept
			// its count, so potions, food and fluid containers were never consumed.
			removed := false
			if it.pos.X != 0 && e.world != nil && e.world.Map != nil {
				if tile := e.world.Map.GetTile(it.pos); tile != nil {
					for _, on := range tile.Items {
						if on == it.item {
							e.world.InternalRemoveItem(it.pos, it.item, uint16(count))
							removed = true
							break
						}
					}
				}
			}
			// Not on a tile: it is in a container or an inventory slot, and it has to
			// come OUT of that holder. Setting Count to 0 and leaving the object in
			// place made a mystic bag survive item:remove(1) — still in the backpack,
			// still usable, handing out a prize on every click.
			if !removed && e.world != nil {
				holder := e.playerHoldingItem(it.item)
				removed = e.world.RemoveItemFromHolder(holder, it.item, uint16(count))
				if removed && holder != nil && holder.Session != nil {
					holder.Session.SendInventoryIds()
					if parent := it.item.Parent; parent != nil {
						holder.Session.RefreshContainer(parent)
					} else {
						for _, oc := range holder.OpenContainersSnapshot() {
							if oc.Container != nil {
								holder.Session.RefreshContainer(oc.Container)
							}
						}
					}
				}
			}
			if !removed {
				// Nothing owns it — decrement so a caller that holds a bare Item still
				// sees the count fall.
				if int(it.item.Count) > count {
					it.item.Count -= uint16(count)
				} else {
					it.item.Count = 0
				}
			}
			L.Push(lua.LTrue)
			return 1
		},
		"addItem": func(L *lua.LState) int {
			it := checkItem(L)
			itemID, ok := e.resolveItemID(L, 2)
			if !ok {
				L.Push(lua.LNil)
				return 1
			}
			count := uint16(luaOptInt(L, 3))
			if count == 0 {
				count = 1
			}
			newItem := &game.Item{ID: itemID, Count: count, Parent: it.item}
			it.item.Contents = append(it.item.Contents, newItem)
			e.pushItem(L, newItem)
			return 1
		},
		// item:setCustomAttribute(key, value) / getCustomAttribute(key) /
		// removeCustomAttribute(key), ports of ItemFunctions::luaItemSetCustomAttribute
		// and friends (src/lua/functions/items/item_functions.cpp). A numeric key is
		// stringified, and the value may be a number, string or boolean — anything else
		// yields nil, as upstream.
		"setCustomAttribute": func(L *lua.LState) int {
			it := checkItem(L)
			key, ok := customAttributeKey(L, 2)
			if !ok || it.item == nil {
				L.Push(lua.LNil)
				return 1
			}
			switch v := L.Get(3); v.Type() {
			case lua.LTNumber:
				n := float64(lua.LVAsNumber(v))
				// C++ keeps whole numbers as int64 and only falls back to double for a
				// real fraction, so getCustomAttribute hands scripts an integer back.
				if n == math.Trunc(n) {
					it.item.SetCustomAttribute(key, int64(n))
				} else {
					it.item.SetCustomAttribute(key, n)
				}
			case lua.LTString:
				it.item.SetCustomAttribute(key, v.String())
			case lua.LTBool:
				it.item.SetCustomAttribute(key, v == lua.LTrue)
			default:
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LTrue)
			return 1
		},
		"getCustomAttribute": func(L *lua.LState) int {
			it := checkItem(L)
			key, ok := customAttributeKey(L, 2)
			if !ok || it.item == nil {
				L.Push(lua.LNil)
				return 1
			}
			v, found := it.item.GetCustomAttribute(key)
			if !found {
				L.Push(lua.LNil)
				return 1
			}
			switch tv := v.(type) {
			case int64:
				L.Push(lua.LNumber(tv))
			case float64:
				L.Push(lua.LNumber(tv))
			case string:
				L.Push(lua.LString(tv))
			case bool:
				L.Push(lua.LBool(tv))
			default:
				L.Push(lua.LNil)
			}
			return 1
		},
		"removeCustomAttribute": func(L *lua.LState) int {
			it := checkItem(L)
			key, ok := customAttributeKey(L, 2)
			if !ok || it.item == nil {
				L.Push(lua.LFalse)
				return 1
			}
			L.Push(lua.LBool(it.item.RemoveCustomAttribute(key)))
			return 1
		},
		// item:getFluidType returns the fluid subtype, which Item stores in Count for
		// splashes and fluid containers (ItemFunctions::luaItemGetFluidType).
		"getFluidType": func(L *lua.LState) int {
			it := checkItem(L)
			if it.item == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(it.item.Count))
			return 1
		},
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
		"setDestination": func(L *lua.LState) int {
			it := checkItem(L)
			pos := checkPosition(L, 2)
			if it.item.Attr == nil {
				it.item.Attr = &game.ItemAttributes{}
			}
			it.item.Attr.TeleDest = &pos
			L.Push(lua.LBool(true))
			return 1
		},
		"getDestination": func(L *lua.LState) int {
			it := checkItem(L)
			if it.item.Attr != nil && it.item.Attr.TeleDest != nil {
				pushPosition(L, *it.item.Attr.TeleDest)
				return 1
			}
			pushPosition(L, game.Position{})
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
				case 20: // TELEPORT_DESTINATION
					has = it.item.Attr.TeleDest != nil
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
				case 20: // TELEPORT_DESTINATION
					if it.item.Attr.TeleDest != nil {
						pushPosition(L, *it.item.Attr.TeleDest)
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
			case 20: // TELEPORT_DESTINATION
				pos := checkPosition(L, 3)
				it.item.Attr.TeleDest = &pos
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
		"setOwner": func(L *lua.LState) int {
			// item:setOwner(creature|guid) — binds the item to a character (C++
			// Item::setOwner via getGUID()). Used by store delivery to mark
			// non-movable purchases. Accepts a Player/Creature userdata or a
			// numeric GUID.
			it := checkItem(L)
			var guid uint32
			switch v := L.Get(2).(type) {
			case *lua.LUserData:
				if p, ok := v.Value.(*game.Player); ok {
					guid = p.DBID
				} else if c, ok := v.Value.(game.Creature); ok {
					guid = c.GetID()
				}
			case lua.LNumber:
				guid = uint32(v)
			}
			if guid != 0 {
				if it.item.Attr == nil {
					it.item.Attr = &game.ItemAttributes{}
				}
				it.item.Attr.Owner = &guid
			}
			L.Push(lua.LBool(true))
			return 1
		},
		"getOwner": func(L *lua.LState) int {
			it := checkItem(L)
			if it.item.Attr != nil && it.item.Attr.Owner != nil {
				L.Push(lua.LNumber(*it.item.Attr.Owner))
			} else {
				L.Push(lua.LNumber(0))
			}
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
		"canBeMoved": func(L *lua.LState) int {
			L.Push(lua.LTrue)
			return 1
		},
		"transform":      e.itemTransform,
		"decay":          stubItemMethod,
		"setDuration":    stubItemMethod,
		"stopDecay":      stubItemMethod,
		"getDescription": e.itemGetDescription,
		"isInsideDepot":  stubItemMethod,
		"isContainer":    stubItemMethod,
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
	if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTNumber {
		subType := uint16(L.CheckNumber(3))
		li.item.Count = subType
	}
	L.Push(lua.LBool(true))
	return 1
}

func checkItemAt(L *lua.LState, index int) luaItem {
	ud := L.CheckUserData(index)
	if v, ok := ud.Value.(luaItem); ok {
		return v
	}
	if v, ok := ud.Value.(*game.Item); ok {
		return luaItem{item: v}
	}
	if v, ok := ud.Value.(luaContainer); ok {
		return luaItem{item: v.item, pos: v.pos}
	}
	L.ArgError(index, "Item expected")
	return luaItem{}
}

// customAttributeKey normalises the key argument of the custom-attribute methods.
// C++ accepts a number or a string and stringifies the number, so 42 and "42" are
// the same key; anything else is rejected (luaItemSetCustomAttribute).
func customAttributeKey(L *lua.LState, n int) (string, bool) {
	switch v := L.Get(n); v.Type() {
	case lua.LTNumber:
		return strconv.FormatInt(int64(lua.LVAsNumber(v)), 10), true
	case lua.LTString:
		return v.String(), true
	}
	return "", false
}

// playerHoldingItem finds the online player whose inventory ultimately holds the
// item. luaItem carries only the item and a map position, so when the item is not
// on a tile the holder has to be located — Item::getHoldingPlayer walks the parent
// chain in C++, which is the same idea from the other end.
func (e *Engine) playerHoldingItem(item *game.Item) *game.Player {
	if item == nil || e.world == nil {
		return nil
	}
	// Climb to the outermost container; that is what sits in an inventory slot.
	root := item
	for root.Parent != nil {
		root = root.Parent
	}
	for _, p := range e.world.Players() {
		for _, inv := range p.Inventory {
			if inv == root {
				return p
			}
		}
	}
	return nil
}
