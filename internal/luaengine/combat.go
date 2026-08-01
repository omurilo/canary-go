package luaengine

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/game/combat"
	lua "github.com/yuin/gopher-lua"
)

const combatTypeName = "Combat"
const combatAreaTypeName = "CombatArea"

func (e *Engine) registerCombat() {
	// createCombatArea(area[, extArea]) builds an AreaCombat from Lua matrices.
	e.L.SetGlobal("createCombatArea", e.L.NewFunction(combatCreateArea))
	e.L.NewTypeMetatable(combatAreaTypeName)

	mt := e.L.NewTypeMetatable(combatTypeName)
	methods := e.combatMethods()
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), methods))
	e.setClassConstructor("Combat", combatCreate, methods)

	e.L.SetGlobal("doTargetCombatHealth", e.L.NewFunction(e.luaDoTargetCombatHealth))
	e.L.SetGlobal("doTargetCombatMana", e.L.NewFunction(e.luaDoTargetCombatMana))
	e.L.SetGlobal("doAreaCombatHealth", e.L.NewFunction(e.luaDoAreaCombatHealth))
	e.L.SetGlobal("doAreaCombatMana", e.L.NewFunction(e.luaDoAreaCombatMana))
	e.L.SetGlobal("doTargetCombat", e.L.NewFunction(e.luaDoTargetCombatHealth))
	e.L.SetGlobal("doAreaCombat", e.L.NewFunction(e.luaDoAreaCombatHealth))

	// Export setCombatCallback globally to appease tests that don't load compat.lua
	e.L.SetGlobal("setCombatCallback", e.L.NewFunction(func(L *lua.LState) int {
		c := checkCombat(L, 1)
		if c == nil {
			L.Push(lua.LFalse)
			return 1
		}
		key := L.CheckInt(2)
		funcName := L.CheckString(3)
		switch key {
		case 0: // CALLBACK_PARAM_LEVELMAGICVALUE
			c.CallbackLevelMagicValue = funcName
		case 1: // CALLBACK_PARAM_SKILLVALUE
			c.CallbackSkillValue = funcName
		case 2: // CALLBACK_PARAM_TARGETTILE
			c.CallbackTargetTile = funcName
		case 3: // CALLBACK_PARAM_TARGETCREATURE
			c.CallbackTargetCreature = funcName
		case 4: // CALLBACK_PARAM_CHAINVALUE
			c.ChainCallback = funcName
		case 5: // CALLBACK_PARAM_CHAINPICKER
			c.ChainPickerCallback = funcName
		}
		L.Push(lua.LTrue)
		return 1
	}))
}

// combatMatrixFromTable converts a Lua matrix (rows of {0/1/3,...}) into a flat
// list plus row count, matching createArea's input (combat.cpp:2291).
func combatMatrixFromTable(t *lua.LTable) ([]uint32, uint32) {
	var list []uint32
	var rows uint32
	t.ForEach(func(_, rowVal lua.LValue) {
		row, ok := rowVal.(*lua.LTable)
		if !ok {
			return
		}
		rows++
		row.ForEach(func(_, cell lua.LValue) {
			list = append(list, uint32(lua.LVAsNumber(cell)))
		})
	})
	return list, rows
}

func combatCreateArea(L *lua.LState) int {
	var area *combat.AreaCombat
	if t, ok := L.Get(1).(*lua.LTable); ok {
		list, rows := combatMatrixFromTable(t)
		area = combat.NewAreaCombat(list, rows)
		if ext, ok := L.Get(2).(*lua.LTable); ok {
			elist, erows := combatMatrixFromTable(ext)
			area.SetupExtArea(elist, erows)
		}
	} else {
		area = &combat.AreaCombat{}
	}
	ud := L.NewUserData()
	ud.Value = area
	L.SetMetatable(ud, L.GetTypeMetatable(combatAreaTypeName))
	L.Push(ud)
	return 1
}

func combatCreate(L *lua.LState) int {
	c := combat.NewCombat()
	ud := L.NewUserData()
	ud.Value = c
	L.SetMetatable(ud, L.GetTypeMetatable(combatTypeName))
	L.Push(ud)
	return 1
}

func checkCombat(L *lua.LState, n int) *combat.Combat {
	ud := L.CheckUserData(n)
	if v, ok := ud.Value.(*combat.Combat); ok {
		return v
	}
	L.ArgError(n, "Combat expected")
	return nil
}

// luaToCombatType maps the Lua CombatType_t sequential enum value to the
// internal combat.CombatType bitflag used by the Go combat engine.
func luaToCombatType(v int) combat.CombatType {
	switch v {
	case 0:
		return combat.CombatPhysical
	case 1:
		return combat.CombatFire
	case 2:
		return combat.CombatEarth
	case 3:
		return combat.CombatEnergy
	case 5:
		return combat.CombatLifeDrain
	case 6:
		return combat.CombatManaDrain
	case 7:
		return combat.CombatHealing
	case 9:
		return combat.CombatIce
	case 10:
		return combat.CombatHoly
	case 11:
		return combat.CombatDeath
	default:
		return combat.CombatType(v)
	}
}

func (e *Engine) combatMethods() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"setParameter": func(L *lua.LState) int {
			c := checkCombat(L, 1)
			key := luaOptInt(L, 2)
			val := luaOptInt(L, 3)
			// CombatParam_t values (creatures_definitions.hpp).
			switch key {
			case 0: // COMBAT_PARAM_TYPE
				c.Params.CombatType = luaToCombatType(val)
			case 1: // COMBAT_PARAM_EFFECT
				c.Params.ImpactEffect = uint16(val)
			case 2: // COMBAT_PARAM_DISTANCEEFFECT
				c.Params.DistanceEffect = uint16(val)
			case 3: // COMBAT_PARAM_BLOCKSHIELD
				c.Params.BlockedByShield = val != 0
			case 4: // COMBAT_PARAM_BLOCKARMOR
				c.Params.BlockedByArmor = val != 0
			case 7: // COMBAT_PARAM_AGGRESSIVE
				c.Params.Aggressive = val != 0
			case 8: // COMBAT_PARAM_DISPEL
				c.Params.DispelType = combat.ConditionType(val)
			case 12: // COMBAT_PARAM_CHAIN_EFFECT
				c.Params.ChainEffect = uint16(val)
			}
			return 0
		},
		"setFormula": func(L *lua.LState) int {
			c := checkCombat(L, 1)
			// combat:setFormula(type, mina, minb, maxa, maxb). Missing args
			// default to 0 (Lua::getNumber), matching the C++ binding.
			c.SetPlayerCombatValues(
				combat.FormulaType(luaOptInt(L, 2)),
				luaOptNumber(L, 3), luaOptNumber(L, 4),
				luaOptNumber(L, 5), luaOptNumber(L, 6),
			)
			return 0
		},
		"setArea": func(L *lua.LState) int {
			c := checkCombat(L, 1)
			if ud, ok := L.Get(2).(*lua.LUserData); ok {
				if area, ok := ud.Value.(*combat.AreaCombat); ok {
					c.SetArea(area)
				}
			}
			return 0
		},
		"addCondition": func(L *lua.LState) int {
			c := checkCombat(L, 1)
			if ud, ok := L.Get(2).(*lua.LUserData); ok {
				if cond, ok := ud.Value.(*luaCondition); ok && cond.cond != nil {
					c.AddCondition(cond.cond.Clone())
				}
			}
			return 0
		},
		// setCallback captures a Lua callback (e.g. onGetPlayerMinMaxValues).
		"setCallback": func(L *lua.LState) int {
			c := checkCombat(L, 1)
			if c == nil {
				L.Push(lua.LFalse)
				return 1
			}
			key := L.CheckInt(2)
			funcName := L.CheckString(3)
			switch key {
			case 0: // CALLBACK_PARAM_LEVELMAGICVALUE
				c.CallbackLevelMagicValue = funcName
			case 1: // CALLBACK_PARAM_SKILLVALUE
				c.CallbackSkillValue = funcName
			case 2: // CALLBACK_PARAM_TARGETTILE
				c.CallbackTargetTile = funcName
			case 3: // CALLBACK_PARAM_TARGETCREATURE
				c.CallbackTargetCreature = funcName
			case 4: // CALLBACK_PARAM_CHAINVALUE
				c.ChainCallback = funcName
			case 5: // CALLBACK_PARAM_CHAINPICKER
				c.ChainPickerCallback = funcName
			}
			L.Push(lua.LTrue)
			return 1
		},
		"execute": e.combatExecute,
	}
}

// combatExecute mirrors CombatFunctions::luaCombatExecute
// (src/lua/functions/creatures/combat/combat_functions.cpp): dispatch on the
// variant type and resolve the target/area through the live world combat engine.
func (e *Engine) combatExecute(L *lua.LState) int {
	c := checkCombat(L, 1)
	caster := getCreature(L, 2)
	v := checkVariant(L, 3)
	if c == nil || caster == nil || v == nil {
		L.Push(lua.LFalse)
		return 1
	}
	c.InstantSpellName = v.instantName

	if c.CallbackLevelMagicValue != "" {
		level, maglevel := 0, 0
		if p, ok := caster.(*game.Player); ok {
			level = int(p.Level)
			maglevel = int(p.MagLevel)
		}

		fn := e.L.GetGlobal(c.CallbackLevelMagicValue)
		if fn.Type() == lua.LTFunction {
			ud := e.L.NewUserData()
			ud.Value = caster
			if _, isPlayer := caster.(*game.Player); isPlayer {
				e.L.SetMetatable(ud, e.L.GetTypeMetatable("Player"))
			} else if _, isMonster := caster.(*game.Monster); isMonster {
				e.L.SetMetatable(ud, e.L.GetTypeMetatable("Monster"))
			} else {
				e.L.SetMetatable(ud, e.L.GetTypeMetatable("Creature"))
			}

			if err := e.L.CallByParam(lua.P{
				Fn:      fn,
				NRet:    2,
				Protect: true,
			}, ud, lua.LNumber(level), lua.LNumber(maglevel)); err == nil {
				minDamage := int32(e.L.ToNumber(-2))
				maxDamage := int32(e.L.ToNumber(-1))
				e.L.Pop(2)

				e.log.Info("executed formula callback", "func", c.CallbackLevelMagicValue, "min", minDamage, "max", maxDamage)

				c2 := *c
				c2.FormulaType = combat.CombatFormulaDamage
				c2.MinA = float64(minDamage)
				c2.MaxA = float64(maxDamage)
				c = &c2
			} else {
				e.log.Error("combat callback error", "func", c.CallbackLevelMagicValue, "err", err)
			}
		} else {
			e.log.Error("combat callback is not a function", "func", c.CallbackLevelMagicValue, "type", fn.Type().String())
		}
	} else if c.CallbackSkillValue != "" {
		fn := e.L.GetGlobal(c.CallbackSkillValue)
		if fn.Type() == lua.LTFunction {
			ud := e.L.NewUserData()
			ud.Value = caster
			if _, isPlayer := caster.(*game.Player); isPlayer {
				e.L.SetMetatable(ud, e.L.GetTypeMetatable("Player"))
			} else if _, isMonster := caster.(*game.Monster); isMonster {
				e.L.SetMetatable(ud, e.L.GetTypeMetatable("Monster"))
			} else {
				e.L.SetMetatable(ud, e.L.GetTypeMetatable("Creature"))
			}

			skill := int32(0)
			attack := int32(7)
			factor := float32(1.0)

			if p, ok := caster.(*game.Player); ok {
				if e.world != nil {
					catalog := e.world.Items
					tool := p.GetWeapon(catalog, false)
					if tool != nil {
						attack = tool.Attack(catalog)
						if tool.WeaponType(catalog) == "ammo" {
							launcher := p.GetWeapon(catalog, true)
							if launcher != nil {
								attack += launcher.Attack(catalog)
							}
						}
						skill = int32(p.GetWeaponSkillForItem(catalog, tool))
					} else {
						skill = int32(p.GetWeaponSkillForItem(catalog, nil))
					}
				} else {
					skill = int32(p.GetWeaponSkillForItem(nil, nil))
				}
				factor = float32(p.GetAttackFactor())
			}

			if err := e.L.CallByParam(lua.P{
				Fn:      fn,
				NRet:    2,
				Protect: true,
			}, ud, lua.LNumber(skill), lua.LNumber(attack), lua.LNumber(factor)); err == nil {
				minDamage := int32(e.L.ToNumber(-2))
				maxDamage := int32(e.L.ToNumber(-1))
				e.L.Pop(2)

				e.log.Info("executed formula callback (skill)", "func", c.CallbackSkillValue, "min", minDamage, "max", maxDamage)

				c2 := *c
				c2.FormulaType = combat.CombatFormulaDamage
				c2.MinA = float64(minDamage)
				c2.MaxA = float64(maxDamage)
				c = &c2
			} else {
				e.log.Error("combat callback error", "func", c.CallbackSkillValue, "err", err)
			}
		} else {
			e.log.Error("combat callback is not a function", "func", c.CallbackSkillValue, "type", fn.Type().String())
		}
	}

	if e.world == nil || e.world.Combat == nil {
		L.Push(lua.LFalse)
		return 1
	}
	ce := e.world.Combat

	switch v.vtype {
	case VariantNumber:
		target := e.world.CreatureByID(v.number)
		if target == nil {
			L.Push(lua.LFalse)
			return 1
		}
		if c.HasArea() {
			ce.DoCombatArea(c, caster, target.GetPosition())
		} else {
			ce.DoCombatTarget(c, caster, target)
		}
	case VariantPosition, VariantTargetPosition:
		ce.DoCombatArea(c, caster, v.pos)
	case VariantString:
		if target := e.world.PlayerByName(v.text); target != nil {
			ce.DoCombatTarget(c, caster, target)
		}
	default:
		L.Push(lua.LFalse)
		return 1
	}

	L.Push(lua.LTrue)
	return 1
}

func toCreature(L *lua.LState, index int) game.Creature {
	val := L.Get(index)
	if ud, ok := val.(*lua.LUserData); ok {
		if c, ok := ud.Value.(game.Creature); ok {
			return c
		}
	}
	return nil
}

func toCombatArea(L *lua.LState, index int) *combat.AreaCombat {
	val := L.Get(index)
	if ud, ok := val.(*lua.LUserData); ok {
		if a, ok := ud.Value.(*combat.AreaCombat); ok {
			return a
		}
	}
	return nil
}

func (e *Engine) luaDoTargetCombatHealth(L *lua.LState) int {
	caster := toCreature(L, 1)
	target := toCreature(L, 2)
	if target == nil || e.world == nil || e.world.Combat == nil {
		L.Push(lua.LFalse)
		return 1
	}
	combatType := luaToCombatType(L.CheckInt(3))
	min := int32(L.CheckNumber(4))
	max := int32(L.CheckNumber(5))
	effect := uint16(L.OptNumber(6, 0))

	e.world.Combat.DoTargetCombatHealth(caster, target, combatType, min, max, effect)
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) luaDoTargetCombatMana(L *lua.LState) int {
	caster := toCreature(L, 1)
	target := toCreature(L, 2)
	if target == nil || e.world == nil || e.world.Combat == nil {
		L.Push(lua.LFalse)
		return 1
	}
	min := int32(L.CheckNumber(3))
	max := int32(L.CheckNumber(4))
	effect := uint16(L.OptNumber(5, 0))

	e.world.Combat.DoTargetCombatMana(caster, target, min, max, effect)
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) luaDoAreaCombatHealth(L *lua.LState) int {
	caster := toCreature(L, 1)
	if e.world == nil || e.world.Combat == nil {
		L.Push(lua.LFalse)
		return 1
	}
	combatType := luaToCombatType(L.CheckInt(2))
	pos, ok := parsePosition(L, 3)
	if !ok && caster != nil {
		pos = caster.GetPosition()
	}
	area := toCombatArea(L, 4)
	min := int32(L.CheckNumber(5))
	max := int32(L.CheckNumber(6))
	effect := uint16(L.OptNumber(7, 0))

	e.world.Combat.DoAreaCombatHealth(caster, combatType, pos, area, min, max, effect)
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) luaDoAreaCombatMana(L *lua.LState) int {
	caster := toCreature(L, 1)
	if e.world == nil || e.world.Combat == nil {
		L.Push(lua.LFalse)
		return 1
	}
	pos, ok := parsePosition(L, 2)
	if !ok && caster != nil {
		pos = caster.GetPosition()
	}
	area := toCombatArea(L, 3)
	min := int32(L.CheckNumber(4))
	max := int32(L.CheckNumber(5))
	effect := uint16(L.OptNumber(6, 0))

	e.world.Combat.DoAreaCombatMana(caster, pos, area, min, max, effect)
	L.Push(lua.LTrue)
	return 1
}
