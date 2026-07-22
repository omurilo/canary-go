package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/spells"
	lua "github.com/yuin/gopher-lua"
)

const luaSpellTypeName = "Spell"

// registerSpell registers the Spell global constructor and metatable, mirroring
// the Lua Spell bindings (src/lua/functions/creatures/combat/spell_functions.cpp).
func (e *Engine) registerSpell() {
	mt := e.L.NewTypeMetatable(luaSpellTypeName)
	e.setClassConstructor("Spell", spellConstructor, spellMethods)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), spellMethods))
	e.L.SetField(mt, "__newindex", e.L.NewFunction(spellNewIndex))
}

// spellConstructor mirrors SpellFunctions::luaSpellCreate: the argument selects
// the spell TYPE ("instant"/"rune" or SPELL_* enum), not the name. Name/words
// are set later via spell:name()/spell:words().
func spellConstructor(L *lua.LState) int {
	s := spells.NewSpell("")
	ud := L.NewUserData()
	ud.Value = s
	L.SetMetatable(ud, L.GetTypeMetatable(luaSpellTypeName))
	L.Push(ud)
	return 1
}

var spellMethods = map[string]lua.LGFunction{
	"name":                        spellName,
	"words":                       spellWords,
	"level":                       spellLevel,
	"magicLevel":                  spellMagicLevel,
	"mana":                        spellMana,
	"manaPercent":                 spellManaPercent,
	"soul":                        spellSoul,
	"group":                       spellGroup,
	"secondaryGroup":              spellSecondaryGroup,
	"id":                          spellNoop,
	"cooldown":                    spellCooldown,
	"groupCooldown":               spellGroupCooldown,
	"isAggressive":                spellIsAggressive,
	"isSelfTarget":                spellIsSelfTarget,
	"needTarget":                  spellNeedTarget,
	"needDirection":               spellNeedDirection,
	"blockWalls":                  spellBlockWalls,
	"needLearn":                   spellNeedLearn,
	"needWeapon":                  spellNeedWeapon,
	"isPremium":                   spellIsPremium,
	"allowOnSelf":                 spellAllowOnSelf,
	"pzLock":                      spellPzLock,
	"hasParams":                   spellHasParams,
	"range":                       spellRange,
	"vocation":                    spellVocation,
	"castSound":                   spellNoop,
	"impactSound":                 spellNoop,
	"isBlocking":                  spellNoop,
	"isDisabled":                  spellNoop,
	"needCasterTargetOrDirection": spellCasterTargetOrDirection,
	// Rune-spell setters. runeId/charges/allowFarUse have no gameplay effect in
	// this slice yet (rune items aren't wired to cast), but every rune script
	// calls them, so accept-and-ignore keeps them loading. setPzLocked is an
	// alias for pzLock and maps to the real field.
	"runeId":      spellNoop,
	"charges":     spellNoop,
	"allowFarUse": spellNoop,
	"setPzLocked": spellPzLock,
	"monkSpellType": spellNoop,
	"hasPlayerNameParam": spellNoop,
	"register":    spellRegister,
}

func checkSpell(L *lua.LState) *spells.Spell {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*spells.Spell); ok {
		return v
	}
	L.ArgError(1, "Spell expected")
	return nil
}

// spellSelf returns the spell userdata (arg 1) so setters can be chained.
func spellSelf(L *lua.LState) int {
	L.Push(L.Get(1))
	return 1
}

func spellNewIndex(L *lua.LState) int {
	s := checkSpell(L)
	key := L.CheckString(2)
	val := L.CheckAny(3)
	if key == "onCastSpell" {
		s.OnCastSpell = val
	}
	return 0
}

func spellName(L *lua.LState) int {
	s := checkSpell(L)
	s.Name = L.CheckString(2)
	return spellSelf(L)
}

func spellWords(L *lua.LState) int {
	s := checkSpell(L)
	s.Words = L.CheckString(2)
	// A second argument declares the spell takes a parameter (hasParams).
	if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTBool {
		s.HasParam = lua.LVAsBool(L.Get(3))
	}
	return spellSelf(L)
}

func spellLevel(L *lua.LState) int {
	s := checkSpell(L)
	s.Level = luaOptInt(L, 2)
	return spellSelf(L)
}

func spellMagicLevel(L *lua.LState) int {
	s := checkSpell(L)
	s.MagicLevel = luaOptInt(L, 2)
	return spellSelf(L)
}

func spellMana(L *lua.LState) int {
	s := checkSpell(L)
	s.Mana = luaOptInt(L, 2)
	return spellSelf(L)
}

func spellManaPercent(L *lua.LState) int {
	s := checkSpell(L)
	s.ManaPercent = luaOptInt(L, 2)
	return spellSelf(L)
}

func spellSoul(L *lua.LState) int {
	s := checkSpell(L)
	s.Soul = luaOptInt(L, 2)
	return spellSelf(L)
}

// spellGroupValue resolves a group argument that may be a numeric SPELLGROUP_*
// enum or a group name string.
func spellGroupValue(L *lua.LState, n int) spells.SpellGroup {
	v := L.Get(n)
	if v.Type() == lua.LTNumber {
		return spells.SpellGroup(lua.LVAsNumber(v))
	}
	switch strings.ToLower(v.String()) {
	case "attack":
		return spells.SpellGroupAttack
	case "healing":
		return spells.SpellGroupHealing
	case "support":
		return spells.SpellGroupSupport
	case "special":
		return spells.SpellGroupSpecial
	default:
		return spells.SpellGroupNone
	}
}

func spellGroup(L *lua.LState) int {
	s := checkSpell(L)
	s.Group = spellGroupValue(L, 2)
	if L.GetTop() >= 3 {
		s.SecondaryGroup = spellGroupValue(L, 3)
	}
	return spellSelf(L)
}

func spellSecondaryGroup(L *lua.LState) int {
	s := checkSpell(L)
	s.SecondaryGroup = spellGroupValue(L, 2)
	return spellSelf(L)
}

func spellCooldown(L *lua.LState) int {
	s := checkSpell(L)
	s.Cooldown = uint32(luaOptInt(L, 2))
	return spellSelf(L)
}

func spellGroupCooldown(L *lua.LState) int {
	s := checkSpell(L)
	s.GroupCooldown = uint32(luaOptInt(L, 2))
	if L.GetTop() >= 3 {
		s.SecondaryGroupCooldown = uint32(luaOptInt(L, 3))
	}
	return spellSelf(L)
}

func spellIsAggressive(L *lua.LState) int {
	s := checkSpell(L)
	s.Aggressive = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellIsSelfTarget(L *lua.LState) int {
	s := checkSpell(L)
	s.SelfTarget = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellNeedTarget(L *lua.LState) int {
	s := checkSpell(L)
	s.NeedTarget = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellNeedDirection(L *lua.LState) int {
	s := checkSpell(L)
	s.NeedDirection = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellCasterTargetOrDirection(L *lua.LState) int {
	s := checkSpell(L)
	s.CasterTargetOrDirection = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellBlockWalls(L *lua.LState) int {
	s := checkSpell(L)
	s.BlockWalls = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellNeedLearn(L *lua.LState) int {
	s := checkSpell(L)
	s.NeedLearn = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellNeedWeapon(L *lua.LState) int {
	s := checkSpell(L)
	s.NeedWeapon = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellIsPremium(L *lua.LState) int {
	s := checkSpell(L)
	s.NeedPremium = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellAllowOnSelf(L *lua.LState) int {
	s := checkSpell(L)
	s.AllowOnSelf = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellPzLock(L *lua.LState) int {
	s := checkSpell(L)
	s.PzLock = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellHasParams(L *lua.LState) int {
	s := checkSpell(L)
	s.HasParam = luaOptBool(L, 2)
	return spellSelf(L)
}

func spellRange(L *lua.LState) int {
	s := checkSpell(L)
	s.Range = luaOptInt(L, 2)
	return spellSelf(L)
}

func spellVocation(L *lua.LState) int {
	s := checkSpell(L)
	// vocation("name") or vocation("name;true") — take the leading name.
	for i := 2; i <= L.GetTop(); i++ {
		if L.Get(i).Type() != lua.LTString {
			continue
		}
		name := L.Get(i).String()
		if idx := strings.IndexByte(name, ';'); idx >= 0 {
			name = name[:idx]
		}
		s.VocationNames = append(s.VocationNames, strings.ToLower(strings.TrimSpace(name)))
	}
	return spellSelf(L)
}

// spellNoop accepts and ignores its argument, returning self (used for setters
// with no gameplay effect yet, e.g. castSound/impactSound/id).
func spellNoop(L *lua.LState) int {
	_ = checkSpell(L)
	return spellSelf(L)
}

func spellRegister(L *lua.LState) int {
	s := checkSpell(L)
	ok := spells.Register(s)
	L.Push(lua.LBool(ok))
	return 1
}

// RunSpell executes a spell's onCastSpell(creature, var) closure with the given
// variant, mirroring InstantSpell::executeCastSpell (spells.cpp:1322). It does
// NOT run the cast checks or spend mana/cooldowns — the protocol layer does that
// before calling this. Returns the boolean the Lua function returned.
func (e *Engine) RunSpell(sp *spells.Spell, caster *game.Player, vtype LuaVariantType, targetID uint32, pos game.Position) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	L := e.L
	if sp.OnCastSpell == nil {
		e.log.Warn("spell OnCastSpell is nil (spell registered but callback function not found)", "spell", sp.Name)
		return false
	}
	if sp.OnCastSpell.Type() != lua.LTFunction {
		e.log.Warn("spell OnCastSpell is not a function", "spell", sp.Name, "type", sp.OnCastSpell.Type().String())
		return false
	}

	casterUD := L.NewUserData()
	casterUD.Value = caster
	L.SetMetatable(casterUD, L.GetTypeMetatable("Player"))

	v := &luaVariant{vtype: vtype, number: targetID, pos: pos, instantName: sp.Name}
	varUD := L.NewUserData()
	varUD.Value = v
	L.SetMetatable(varUD, L.GetTypeMetatable(variantTypeName))

	if err := L.CallByParam(lua.P{Fn: sp.OnCastSpell, NRet: 1, Protect: true}, casterUD, varUD); err != nil {
		e.log.Error("spell onCastSpell error", "spell", sp.Name, "err", err)
		return false
	}
	ret := L.Get(-1)
	L.Pop(1)
	return !lua.LVIsFalse(ret)
}
