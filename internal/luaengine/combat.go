package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game/combat"
	lua "github.com/yuin/gopher-lua"
)

const combatTypeName = "Combat"
const combatAreaTypeName = "CombatArea"

func (e *Engine) registerCombat() {
	// createCombatArea(area[, extArea]) builds an AreaCombat from Lua matrices.
	e.L.SetGlobal("createCombatArea", e.L.NewFunction(combatCreateArea))
	e.L.NewTypeMetatable(combatAreaTypeName)

	mt := e.L.NewTypeMetatable(combatTypeName)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), e.combatMethods()))
	e.L.SetGlobal("Combat", e.L.NewFunction(combatCreate))
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
		return combat.CombatUndefined
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
				if cond, ok := ud.Value.(*luaCondition); ok && cond.condType != combat.ConditionNone {
					c.AddCondition(&combat.ConditionGeneric{
						Type:  cond.condType,
						Ticks: cond.ticks,
					})
				}
			}
			return 0
		},
		// setCallback captures a Lua callback (e.g. onGetPlayerMinMaxValues). Full
		// callback support is not ported; the formula path covers instant spells.
		// TODO(spells): invoke onGetPlayerMinMaxValues / onTargetTile callbacks.
		"setCallback": func(L *lua.LState) int { return 0 },
		"execute":     e.combatExecute,
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
