package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/items"
	lua "github.com/yuin/gopher-lua"
)

const itemTypeClassName = "ItemType"

type luaItemType struct {
	id   uint16
	item *items.ItemType
}

func (e *Engine) registerItemType() {
	mt := e.L.NewTypeMetatable(itemTypeClassName)
	methods := map[string]lua.LGFunction{
		"getArticle": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil && it.item.Article != "" {
				L.Push(lua.LString(it.item.Article))
			} else {
				L.Push(lua.LString(""))
			}
			return 1
		},
		"getName": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil && it.item.Name != "" {
				L.Push(lua.LString(it.item.Name))
			} else {
				L.Push(lua.LString("An Item"))
			}
			return 1
		},
		"getId": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			L.Push(lua.LNumber(it.id))
			return 1
		},
		"getClientId": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			L.Push(lua.LNumber(it.id))
			return 1
		},
		"getWeight": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LNumber(it.item.GetWeight()))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		},
		"isStackable": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.Stackable))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isContainer": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.IsContainer()))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isFluidContainer": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.IsFluidContainer()))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"getFluidSource": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LNumber(it.item.FluidSource))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		},
		"getCharges": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LNumber(it.item.Charges))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		},
		"getStackSize": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil && it.item.StackSize > 0 {
				L.Push(lua.LNumber(it.item.StackSize))
			} else {
				L.Push(lua.LNumber(100)) // Tibia default stack size
			}
			return 1
		},
		"isRune": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.TypeName == "rune"))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"getDecayId": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LNumber(it.item.DecayTo))
			} else {
				L.Push(lua.LNumber(0))
			}
			return 1
		},
		"isMovable": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.Pickupable))
			} else {
				L.Push(lua.LBool(true))
			}
			return 1
		},
		"getType": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			L.Push(lua.LNumber(it.id))
			return 1
		},
		"hasProperty": func(L *lua.LState) int {
			_ = checkItemType(L, 1)
			_ = L.OptInt(2, 0)
			L.Push(lua.LFalse)
			return 1
		},
		"getWeaponType": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := 0 // WEAPON_NONE
			if it.item != nil {
				switch it.item.WeaponType {
				case "sword": val = 1
				case "club": val = 2
				case "axe": val = 3
				case "shield": val = 4
				case "distance": val = 5
				case "wand": val = 6
				case "ammunition": val = 7
				}
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getAmmoType": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := 0 // AMMO_NONE
			if it.item != nil {
				switch it.item.AmmoType {
				case "bolt": val = 1
				case "arrow": val = 2
				case "spear": val = 3
				case "throwingstar": val = 4
				case "throwingknife": val = 5
				case "stone": val = 6
				case "snowball": val = 7
				}
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"isQuiver": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.IsQuiver))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		// -- boolean property checks --
		"isDoor": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.IsDoor))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isBlocking": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.BlockSolid))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isGroundTile": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.IsGround()))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isMagicField": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.TypeName == "magicfield"))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isPickupable": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.Pickupable))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isKey": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.TypeName == "key"))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isWeapon": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.WeaponType != ""))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isLadder": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.IsLadder))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isForceUse": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.ForceUse))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"hasHeight": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.HasHeight))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"isPodium": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.Podium))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"getShowDuration": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.ShowDuration))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		"getShowCharges": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			if it.item != nil {
				L.Push(lua.LBool(it.item.ShowCharges))
			} else {
				L.Push(lua.LBool(false))
			}
			return 1
		},
		// -- numeric getters --
		"getAttack": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := int32(0)
			if it.item != nil {
				val = it.item.Attack
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getDefense": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := int32(0)
			if it.item != nil {
				val = it.item.Defense
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getExtraDefense": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := int32(0)
			if it.item != nil {
				val = it.item.ExtraDefense
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getArmor": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := int32(0)
			if it.item != nil {
				val = it.item.Armor
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getHitChance": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := int32(0)
			if it.item != nil {
				val = it.item.HitChance
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getShootRange": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint8(0)
			if it.item != nil {
				val = it.item.ShootRange
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getDecayTime": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint32(0)
			if it.item != nil {
				val = it.item.DecayTime
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getGroundSpeed": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint16(0)
			if it.item != nil {
				val = it.item.GroundSpeed
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getAlwaysOnTopOrder": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint8(0)
			if it.item != nil {
				val = it.item.AlwaysOnTopOrder
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getMaxHitChance": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := int32(0)
			if it.item != nil {
				val = it.item.MaxHitChance
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getCapacity": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint32(0)
			if it.item != nil {
				val = it.item.Capacity
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getUpgradeClassification": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint8(0)
			if it.item != nil {
				val = it.item.UpgradeClassification
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getTransformEquipId": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint16(0)
			if it.item != nil {
				val = it.item.TransformEquipTo
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getTransformDeEquipId": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint16(0)
			if it.item != nil {
				val = it.item.TransformDeEquipTo
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		// -- string getters --
		"getDescription": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := ""
			if it.item != nil {
				val = it.item.Description
			}
			L.Push(lua.LString(val))
			return 1
		},
		"getSlotPosition": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := ""
			if it.item != nil {
				val = it.item.SlotPosition
			}
			L.Push(lua.LString(val))
			return 1
		},
		"getSlotType": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := ""
			if it.item != nil {
				val = it.item.SlotType
			}
			L.Push(lua.LString(val))
			return 1
		},
		"getTypeName": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := ""
			if it.item != nil {
				val = it.item.TypeName
			}
			L.Push(lua.LString(val))
			return 1
		},
		"getFloorChange": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := ""
			if it.item != nil {
				val = it.item.FloorChange
			}
			L.Push(lua.LString(val))
			return 1
		},
		"getShootType": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := ""
			if it.item != nil {
				val = it.item.ShootType
			}
			L.Push(lua.LString(val))
			return 1
		},
		// -- new fields --
		"getPluralName": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := ""
			if it.item != nil {
				val = it.item.PluralName
			}
			L.Push(lua.LString(val))
			return 1
		},
		"getElementType": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint8(0)
			if it.item != nil {
				val = it.item.ElementType
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getElementDamage": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint16(0)
			if it.item != nil {
				val = it.item.ElementDamage
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getImbuementSlot": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint8(0)
			if it.item != nil {
				val = it.item.ImbuementSlot
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getRequiredLevel": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint16(0)
			if it.item != nil {
				val = it.item.MinReqLevel
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getSpeed": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := int32(0)
			if it.item != nil {
				val = it.item.Speed
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getBaseSpeed": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := int32(0)
			if it.item != nil {
				val = it.item.BaseSpeed
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getWrapableTo": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint16(0)
			if it.item != nil {
				val = it.item.WrapableTo
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getDestroyId": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := uint16(0)
			if it.item != nil {
				val = it.item.DestroyID
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"getVocationString": func(L *lua.LState) int {
			it := checkItemType(L, 1)
			val := ""
			if it.item != nil {
				val = it.item.VocationString
			}
			L.Push(lua.LString(val))
			return 1
		},
	}

	e.L.SetFuncs(mt, methods)
	methodTable := e.L.SetFuncs(e.L.NewTable(), methods)

	e.L.SetField(mt, "__index", e.L.NewFunction(func(L *lua.LState) int {
		it := checkItemType(L, 1)
		key := L.CheckString(2)
		if val := methodTable.RawGetString(key); val != lua.LNil {
			L.Push(val)
			return 1
		}
		if key == "id" {
			L.Push(lua.LNumber(it.id))
			return 1
		}
		if key == "name" {
			if it.item != nil {
				L.Push(lua.LString(it.item.Name))
			} else {
				L.Push(lua.LString("An Item"))
			}
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	e.setClassConstructor("ItemType", func(L *lua.LState) int {
		var id uint16
		if L.GetTop() >= 1 && L.Get(1).Type() == lua.LTNumber {
			id = uint16(L.CheckInt(1))
		}
		var item *items.ItemType
		if cat := e.itemCatalog(); cat != nil {
			item = cat.Get(id)
		}
		ud := L.NewUserData()
		ud.Value = &luaItemType{id: id, item: item}
		L.SetMetatable(ud, L.GetTypeMetatable(itemTypeClassName))
		L.Push(ud)
		return 1
	}, methods)
}

func checkItemType(L *lua.LState, n ...int) *luaItemType {
	idx := 1
	if len(n) > 0 {
		idx = n[0]
	}
	ud := L.CheckUserData(idx)
	if it, ok := ud.Value.(*luaItemType); ok {
		return it
	}
	L.ArgError(idx, "ItemType expected")
	return nil
}