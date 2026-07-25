package luaengine

import (
	"math"

	"github.com/opentibiabr/canary-go/internal/game/vocations"
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerVocation() {
	mt := e.L.NewTypeMetatable("Vocation")
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), vocationMethods))
	e.L.SetGlobal("Vocation", mt)
}

var vocationMethods = map[string]lua.LGFunction{
	"getId":                  vocationGetId,
	"getBaseId":              vocationGetBaseId,
	"getBase":                vocationGetBase,
	"getClientId":            vocationGetId, // Same as getId
	"getName":                vocationGetName,
	"getHealthGain":          vocationGetHealthGain,
	"getHealthGainTicks":     vocationGetHealthGainTicks,
	"getHealthGainAmount":    vocationGetHealthGainAmount,
	"getManaGain":            vocationGetManaGain,
	"getManaGainTicks":       vocationGetManaGainTicks,
	"getManaGainAmount":      vocationGetManaGainAmount,
	"getCapacityGain":        vocationGetCapacityGain,
	"getRequiredManaSpent":   vocationGetRequiredManaSpent,
	"getRequiredSkillTries":  vocationGetRequiredSkillTries,
	"getAttackSpeed":         vocationGetAttackSpeed,
	"getBaseAttackSpeed":     vocationGetAttackSpeed,
	"getBaseSpeed":           vocationGetBaseSpeed,
	"getDemotion":            vocationGetDemotion,
	"getPromotion":           vocationGetPromotion,
}

func checkVocation(L *lua.LState) *vocations.Vocation {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*vocations.Vocation); ok {
		return v
	}
	L.ArgError(1, "Vocation expected")
	return nil
}

func pushVocation(L *lua.LState, v *vocations.Vocation) {
	if v == nil {
		L.Push(lua.LNil)
		return
	}
	ud := L.NewUserData()
	ud.Value = v
	L.SetMetatable(ud, L.GetTypeMetatable("Vocation"))
	L.Push(ud)
}

// Vocation:getId()
func vocationGetId(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.ID))
	return 1
}

// Vocation:getBaseId()
func vocationGetBaseId(L *lua.LState) int {
	v := checkVocation(L)
	baseID := v.ID
	if baseID > 4 {
		baseID = ((baseID - 1) % 4) + 1
	}
	L.Push(lua.LNumber(baseID))
	return 1
}

// Vocation:getBase()
func vocationGetBase(L *lua.LState) int {
	v := checkVocation(L)
	baseID := v.ID
	if baseID > 4 {
		baseID = ((baseID - 1) % 4) + 1
	}
	baseVoc := vocations.GetVocation(baseID)
	if baseVoc == nil {
		L.Push(lua.LNil)
	} else {
		pushVocation(L, baseVoc)
	}
	return 1
}

// Vocation:getName()
func vocationGetName(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LString(v.Name))
	return 1
}

// Vocation:getHealthGain()
func vocationGetHealthGain(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.GainHP))
	return 1
}

// Vocation:getHealthGainTicks()
func vocationGetHealthGainTicks(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.GainHPTicks))
	return 1
}

// Vocation:getHealthGainAmount()
func vocationGetHealthGainAmount(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.GainHPAmount))
	return 1
}

// Vocation:getManaGain()
func vocationGetManaGain(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.GainMana))
	return 1
}

// Vocation:getManaGainTicks()
func vocationGetManaGainTicks(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.GainManaTicks))
	return 1
}

// Vocation:getManaGainAmount()
func vocationGetManaGainAmount(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.GainManaAmount))
	return 1
}

// Vocation:getCapacityGain()
func vocationGetCapacityGain(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.GainCap))
	return 1
}

// Vocation:getAttackSpeed()
func vocationGetAttackSpeed(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.AttackSpeed))
	return 1
}

// Vocation:getBaseSpeed()
func vocationGetBaseSpeed(L *lua.LState) int {
	v := checkVocation(L)
	L.Push(lua.LNumber(v.BaseSpeed))
	return 1
}

// Vocation:getDemotion()
func vocationGetDemotion(L *lua.LState) int {
	// Not implemented, return nil for now
	L.Push(lua.LNil)
	return 1
}

// Vocation:getPromotion()
func vocationGetPromotion(L *lua.LState) int {
	// Not implemented, return nil for now
	L.Push(lua.LNil)
	return 1
}


// Vocation:getRequiredManaSpent(magicLevel)
func vocationGetRequiredManaSpent(L *lua.LState) int {
	v := checkVocation(L)
	magicLevel := L.CheckNumber(2)
	// Base formula used in Tibia for mana spent: 1600 * multiplier^(magicLevel-1)
	// Actually Canary uses a complex precalculated or formula. We will implement the standard Tibia formula:
	// req = 1600 * math.Pow(ManaMultiplier, magicLevel-1) (Wait, typically base is 1600)
	if v.ManaMultiplier == 0 || v.ManaMultiplier == 1.0 {
		L.Push(lua.LNumber(0))
		return 1
	}
	req := 1600.0 * math.Pow(v.ManaMultiplier, float64(magicLevel-1))
	L.Push(lua.LNumber(uint64(req)))
	return 1
}

// Vocation:getRequiredSkillTries(skillId, skillLevel)
func vocationGetRequiredSkillTries(L *lua.LState) int {
	v := checkVocation(L)
	skillId := L.CheckInt(2)
	skillLevel := L.CheckNumber(3)
	
	// Base formula: 50 * multiplier^(skillLevel-10) for normal skills?
	// Actually base varies per skill:
	// Fist, Club, Sword, Axe, Distance = 50
	// Shield = 100
	// Fishing = 20
	// Let's get the multiplier for the specific skillId
	var multiplier float64 = 1.0
	for _, s := range v.Skills {
		if s.ID == skillId {
			multiplier = s.Multiplier
			break
		}
	}
	
	if multiplier <= 1.0 {
		L.Push(lua.LNumber(0))
		return 1
	}
	
	if skillLevel <= 10 {
		L.Push(lua.LNumber(0))
		return 1
	}

	var base float64 = 50.0
	if skillId == 5 { // Shielding
		base = 100.0
	} else if skillId == 6 { // Fishing
		base = 20.0
	}
	
	req := base * math.Pow(multiplier, float64(skillLevel-11))
	L.Push(lua.LNumber(uint64(req)))
	return 1
}
