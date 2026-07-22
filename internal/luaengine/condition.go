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

// luaCondition wraps a real, concrete combat.Condition and tracks raw Lua type.
type luaCondition struct {
	cond        combat.Condition
	rawType     int
	boundPlayer *game.Player
}

func (e *Engine) registerCondition() {
	mt := e.L.NewTypeMetatable(luaConditionTypeName)
	e.setClassConstructor("Condition", conditionConstructor, conditionMethods)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), conditionMethods))
}

func conditionConstructor(L *lua.LState) int {
	rawType := 0
	if L.GetTop() >= 1 && L.Get(2).Type() == lua.LTNumber {
		rawType = luaOptInt(L, 2)
	}
	condType := luaToConditionType(rawType)

	conditionId := combat.ConditionId(0)
	if L.GetTop() >= 3 {
		conditionId = combat.ConditionId(luaOptInt(L, 3))
	}
	subId := uint32(0)
	if L.GetTop() >= 4 {
		subId = uint32(luaOptInt(L, 4))
	}
	isPersistent := false
	if L.GetTop() >= 5 {
		isPersistent = L.ToBool(5)
	}

	c := &luaCondition{
		rawType: rawType,
		cond:    combat.CreateCondition(conditionId, condType, 0, subId, isPersistent),
	}

	ud := L.NewUserData()
	ud.Value = c
	L.SetMetatable(ud, L.GetTypeMetatable(luaConditionTypeName))
	L.Push(ud)
	return 1
}

func (c *luaCondition) getTicks() int32 {
	if c.boundPlayer != nil && c.rawType == conditionRegeneration {
		return c.boundPlayer.RegenTicks
	}
	if c.cond != nil {
		return c.cond.GetTicks()
	}
	return 0
}

func (c *luaCondition) setTicks(t int32) {
	if c.cond != nil {
		c.cond.SetTicks(t)
	}
	if c.boundPlayer != nil && c.rawType == conditionRegeneration {
		c.boundPlayer.RegenTicks = t
		if c.boundPlayer.Session != nil {
			c.boundPlayer.Session.SendStats() // refresh client food timer
		}
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
		key := int32(luaOptInt(L, 2))
		var value int32
		if L.Get(3).Type() == lua.LTBool {
			if L.ToBool(3) {
				value = 1
			} else {
				value = 0
			}
		} else {
			value = int32(luaOptInt(L, 3))
		}
		if c.cond != nil {
			c.cond.SetParam(key, value)
		}
		// Special case for ticks parameter
		if key == 2 {
			c.setTicks(value)
		}
		L.Push(lua.LTrue)
		return 1
	},
	"setTicks": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		c.setTicks(int32(luaOptInt(L, 2)))
		L.Push(lua.LTrue)
		return 1
	},
	"getTicks": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		L.Push(lua.LNumber(c.getTicks()))
		return 1
	},
	"getType": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		if c.cond != nil {
			L.Push(lua.LNumber(c.cond.GetType()))
		} else {
			L.Push(lua.LNumber(0))
		}
		return 1
	},
	"setFormula": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		mina := float32(L.CheckNumber(2))
		minb := float32(L.CheckNumber(3))
		maxa := float32(L.CheckNumber(4))
		maxb := float32(L.CheckNumber(5))
		if c.cond != nil {
			if speedCond, ok := c.cond.(*combat.ConditionSpeedStruct); ok {
				speedCond.SetFormulaVars(mina, minb, maxa, maxb)
				L.Push(lua.LTrue)
				return 1
			}
		}
		L.Push(lua.LFalse)
		return 1
	},
	"setOutfit": func(L *lua.LState) int {
		// outfit conditions are currently simplified to return true
		L.Push(lua.LTrue)
		return 1
	},
	"addDamage": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		rounds := int32(L.CheckNumber(2))
		time := int32(L.CheckNumber(3))
		value := int32(L.CheckNumber(4))
		if c.cond != nil {
			if dmgCond, ok := c.cond.(*combat.ConditionDamageStruct); ok {
				dmgCond.AddDamage(rounds, time, value)
				L.Push(lua.LTrue)
				return 1
			}
		}
		L.Push(lua.LFalse)
		return 1
	},
	"setTickInterval": func(L *lua.LState) int {
		c := checkCondition(L, 1)
		value := int32(L.CheckNumber(2))
		if c.cond != nil {
			if dmgCond, ok := c.cond.(*combat.ConditionDamageStruct); ok {
				dmgCond.TickInterval = value
				L.Push(lua.LTrue)
				return 1
			}
		}
		L.Push(lua.LFalse)
		return 1
	},
	"register": func(L *lua.LState) int {
		L.Push(lua.LTrue)
		return 1
	},
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
	case 5:
		return combat.ConditionHaste
	case 6:
		return combat.ConditionParalyze
	case 7:
		return combat.ConditionOutfit
	case 8:
		return combat.ConditionInvisible
	case 9:
		return combat.ConditionLight
	case 10:
		return combat.ConditionManaShield
	case 11:
		return combat.ConditionInFight
	case 12:
		return combat.ConditionDrunk
	case 13:
		return combat.ConditionExhaust
	case 14:
		return combat.ConditionRegeneration
	case 15:
		return combat.ConditionSoul
	case 17:
		return combat.ConditionMuted
	case 18:
		return combat.ConditionChannelMutedCondition
	case 19:
		return combat.ConditionYellTicks
	case 20:
		return combat.ConditionAttributes
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
