package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/combat"
	lua "github.com/yuin/gopher-lua"
)

// conditionRegeneration is the Lua CONDITION_REGENERATION enum value (enums.go).
const conditionRegeneration = 14

// This is a minimal port of the Lua Condition bindings
// (src/lua/functions/creatures/combat/condition_functions.cpp). It captures the
// condition type and its parameters so combat:addCondition() can attach a
// generic condition, but the full damage-over-time / attribute machinery is not
// yet ported. Attack/heal spells (the priority) do not depend on it.
//
// TODO(conditions): port ConditionDamage/ConditionAttributes formula handling so
// paralyze/DoT/haste spells apply their real effects.

const luaConditionTypeName = "Condition"

// luaCondition wraps the combat condition type plus captured parameters.
// rawType keeps the original Lua CONDITION_* value (needed to recognise
// CONDITION_REGENERATION, which has no combat.ConditionType mapping). When
// boundPlayer is set (a regeneration condition returned by getCondition),
// getTicks/setTicks read and write that player's RegenTicks so the food script's
// accumulate-and-check logic works against live state.
type luaCondition struct {
	condType    combat.ConditionType
	rawType     int
	ticks       int32
	boundPlayer *game.Player
}

func (e *Engine) registerCondition() {
	mt := e.L.NewTypeMetatable(luaConditionTypeName)
	e.setClassConstructor("Condition", conditionConstructor, conditionMethods)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), conditionMethods))
}

func conditionConstructor(L *lua.LState) int {
	c := &luaCondition{}
	if L.GetTop() >= 1 && L.Get(2).Type() == lua.LTNumber {
		c.rawType = luaOptInt(L, 2)
		c.condType = luaToConditionType(c.rawType)
	}
	ud := L.NewUserData()
	ud.Value = c
	L.SetMetatable(ud, L.GetTypeMetatable(luaConditionTypeName))
	L.Push(ud)
	return 1
}

// getTicks/setTicks read/write the bound player's RegenTicks for a regeneration
// condition, else the local ticks value.
func (c *luaCondition) getTicks() int32 {
	if c.boundPlayer != nil && c.rawType == conditionRegeneration {
		return c.boundPlayer.RegenTicks
	}
	return c.ticks
}

func (c *luaCondition) setTicks(t int32) {
	c.ticks = t
	if c.boundPlayer != nil && c.rawType == conditionRegeneration {
		c.boundPlayer.RegenTicks = t
	}
}

func checkCondition(L *lua.LState, n int) *luaCondition {
	ud := L.CheckUserData(n)
	if v, ok := ud.Value.(*luaCondition); ok {
		return v
	}
	L.ArgError(n, "Condition expected")
	return nil
}

var conditionMethods = map[string]lua.LGFunction{
	"setParameter": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		// CONDITION_PARAM_TICKS == 2 (creatures_definitions.hpp).
		if luaOptInt(L, 2) == 2 {
			c.setTicks(int32(luaOptInt(L, 3)))
		}
		return 0
	},
	"setTicks": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		c.setTicks(int32(luaOptInt(L, 2)))
		return 0
	},
	"getTicks": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		L.Push(lua.LNumber(c.getTicks()))
		return 1
	},
	"getType": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		L.Push(lua.LNumber(c.condType))
		return 1
	},
	"setFormula":      conditionNoop,
	"setOutfit":       conditionNoop,
	"addDamage":       conditionNoop,
	"setTickInterval": conditionNoop,
	"register":        conditionNoop,
}

func conditionNoop(L *lua.LState) int { return 0 }

// luaToConditionType maps the Lua CONDITION_* enum (sequential values in
// enums.go) to the internal combat.ConditionType bitflags.
func luaToConditionType(v int) combat.ConditionType {
	switch v {
	case 1:
		return combat.ConditionPoison
	case 2:
		return combat.ConditionFire
	case 3:
		return combat.ConditionEnergy
	case 4:
		return combat.ConditionBleeding
	case 6:
		return combat.ConditionParalyze
	case 21:
		return combat.ConditionFreezing
	case 22:
		return combat.ConditionDazzled
	case 23:
		return combat.ConditionCursed
	default:
		return combat.ConditionNone
	}
}
