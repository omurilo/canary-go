package luaengine

import (
	"strings"

	"github.com/omurilo/canary-go/internal/creatures"
	lua "github.com/yuin/gopher-lua"
)

const luaMonsterSpellTypeName = "MonsterSpell"

type luaMonsterSpell struct {
	Attack creatures.MonsterAttack
}

func (e *Engine) registerMonsterSpellClass() {
	mt := e.L.NewTypeMetatable(luaMonsterSpellTypeName)

	spellMethods := map[string]lua.LGFunction{
		"setType": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.Name = strings.ToLower(L.CheckString(2))
			return 0
		},
		"setAttackValue": func(L *lua.LState) int {
			return 0
		},
		"setCombatValue": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.MinDamage = L.CheckInt(2)
			s.Attack.MaxDamage = L.CheckInt(3)
			return 0
		},
		"setInterval": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.Interval = L.CheckInt(2)
			return 0
		},
		"setCombatEffect": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.Effect = uint16(L.CheckInt(2))
			return 0
		},
		"castSound": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.CastSound = L.CheckString(2)
			return 0
		},
		"impactSound": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.ImpactSound = L.CheckString(2)
			return 0
		},
		"setCombatType": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			// e.g. COMBAT_FIREDAMAGE -> map enum or save raw
			s.Attack.CombatType = L.CheckAny(2).String()
			return 0
		},
		"setConditionType": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.ConditionType = L.CheckAny(2).String()
			return 0
		},
		"setChance": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.Chance = L.CheckInt(2)
			return 0
		},
		"setRange": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.Range = L.CheckInt(2)
			return 0
		},
		"setConditionDuration": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.Duration = L.CheckInt(2)
			return 0
		},
		"setConditionSpeedChange": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.SpeedChange = L.CheckInt(2)
			return 0
		},
		"setNeedTarget": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.NeedTarget = L.CheckBool(2)
			return 0
		},
		"setCombatLength": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.Length = L.CheckInt(2)
			return 0
		},
		"setCombatSpread": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.Spread = L.CheckInt(2)
			return 0
		},
		"setCombatRadius": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.Radius = L.CheckInt(2)
			return 0
		},
		"setOutfitMonster": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.OutfitMonster = L.CheckString(2)
			return 0
		},
		"setOutfitItem": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.OutfitItem = L.CheckInt(2)
			return 0
		},
		"setConditionDamage": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.ConditionDamage = L.CheckInt(2)
			return 0
		},
		"setCombatShootEffect": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.ShootEffect = uint16(L.CheckInt(2))
			return 0
		},
		"setConditionTickInterval": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.TickInterval = L.CheckInt(2)
			return 0
		},
		"setScriptName": func(L *lua.LState) int {
			s := checkMonsterSpell(L)
			s.Attack.ScriptName = L.CheckString(2)
			return 0
		},
	}

	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), spellMethods))

	// Global MonsterSpell constructor
	e.L.SetGlobal("MonsterSpell", e.L.NewFunction(func(L *lua.LState) int {
		s := &luaMonsterSpell{
			Attack: creatures.MonsterAttack{
				Interval: 2000,
				Chance:   100,
			},
		}
		ud := L.NewUserData()
		ud.Value = s
		L.SetMetatable(ud, mt)
		L.Push(ud)
		return 1
	}))
}

func checkMonsterSpell(L *lua.LState) *luaMonsterSpell {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*luaMonsterSpell); ok {
		return v
	}
	L.ArgError(1, "MonsterSpell expected")
	return nil
}
