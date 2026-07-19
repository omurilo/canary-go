package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game/combat"
	lua "github.com/yuin/gopher-lua"
)

const combatTypeName = "Combat"

func (e *Engine) registerCombat() {
	// register createCombatArea
	e.L.SetGlobal("createCombatArea", e.L.NewFunction(combatCreateArea))

	// register Combat type
	mt := e.L.NewTypeMetatable(combatTypeName)
	methods := e.L.SetFuncs(e.L.NewTable(), combatMethods)
	e.L.SetField(mt, "__index", methods)
	e.L.SetGlobal("Combat", e.L.NewFunction(combatCreate))
}

func combatCreateArea(L *lua.LState) int {
	ud := L.NewUserData()
	ud.Value = struct{}{}
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

var combatMethods = map[string]lua.LGFunction{
	"setParameter": func(L *lua.LState) int {
		c := checkCombat(L, 1)
		key := L.CheckInt(2)
		val := L.CheckInt(3)
		c.SetParam(combat.CombatParam(key), uint32(val))
		return 0
	},
	"setFormula": func(L *lua.LState) int {
		c := checkCombat(L, 1)
		formulaType := L.CheckInt(2)
		mina := L.CheckNumber(3)
		minb := L.CheckNumber(4)
		maxa := L.CheckNumber(5)
		maxb := L.CheckNumber(6)
		c.SetPlayerCombatValues(combat.FormulaType(formulaType), float64(mina), float64(minb), float64(maxa), float64(maxb))
		return 0
	},
	"setArea": func(L *lua.LState) int {
		return 0
	},
	"setCallback": func(L *lua.LState) int {
		return 0
	},
	"execute": func(L *lua.LState) int {
		c := checkCombat(L, 1)
		ud := L.CheckUserData(2)
		creature, ok := ud.Value.(combat.Creature)
		if !ok {
			L.ArgError(2, "Creature expected")
			return 0
		}
		
		// For now, assume target is the caster themselves, or ignored
		dummyDamage := combat.CombatDamage{}
		c.DoCombatHealth(creature, creature, dummyDamage)
		
		L.Push(lua.LTrue)
		return 1
	},
}
