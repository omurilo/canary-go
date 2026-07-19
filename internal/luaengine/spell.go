package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/spells"
	lua "github.com/yuin/gopher-lua"
)

const luaSpellTypeName = "Spell"

// registerSpell registers the Spell global constructor and metatable
func (e *Engine) registerSpell() {
	mt := e.L.NewTypeMetatable(luaSpellTypeName)
	e.L.SetGlobal("Spell", e.L.NewFunction(spellConstructor))
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), spellMethods))
	e.L.SetField(mt, "__newindex", e.L.NewFunction(spellNewIndex))
}

func spellConstructor(L *lua.LState) int {
	arg := L.CheckString(1)
	s := &spells.Spell{}
	
	s.Name = arg
	s.Words = arg

	ud := L.NewUserData()
	ud.Value = s
	L.SetMetatable(ud, L.GetTypeMetatable(luaSpellTypeName))
	L.Push(ud)
	return 1
}

var spellMethods = map[string]lua.LGFunction{
	"name":        spellName,
	"words":       spellWords,
	"level":       spellLevel,
	"mana":        spellMana,
	"group":       spellGroup,
	"id":          spellId,
	"cooldown":    spellCooldown,
	"groupCooldown": spellGroupCooldown,
	"isAggressive": spellIsAggressive,
	"needLearn":   spellNeedLearn,
	"vocation":    spellVocation,
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
	L.Push(L.Get(1))
	return 1
}

func spellWords(L *lua.LState) int {
	s := checkSpell(L)
	s.Words = L.CheckString(2)
	L.Push(L.Get(1))
	return 1
}

func spellLevel(L *lua.LState) int {
	s := checkSpell(L)
	s.Level = L.CheckInt(2)
	L.Push(L.Get(1))
	return 1
}

func spellMana(L *lua.LState) int {
	s := checkSpell(L)
	s.Mana = L.CheckInt(2)
	L.Push(L.Get(1))
	return 1
}

func spellGroup(L *lua.LState) int {
	s := checkSpell(L)
	s.Group = L.CheckString(2)
	L.Push(L.Get(1))
	return 1
}

func spellId(L *lua.LState) int {
	_ = checkSpell(L)
	L.Push(L.Get(1))
	return 1
}

func spellCooldown(L *lua.LState) int {
	_ = checkSpell(L)
	L.Push(L.Get(1))
	return 1
}

func spellGroupCooldown(L *lua.LState) int {
	_ = checkSpell(L)
	L.Push(L.Get(1))
	return 1
}

func spellIsAggressive(L *lua.LState) int {
	_ = checkSpell(L)
	L.Push(L.Get(1))
	return 1
}

func spellNeedLearn(L *lua.LState) int {
	s := checkSpell(L)
	s.NeedLearn = L.CheckBool(2)
	L.Push(L.Get(1))
	return 1
}

func spellVocation(L *lua.LState) int {
	s := checkSpell(L)
	s.Vocation = append(s.Vocation, L.CheckString(2))
	L.Push(L.Get(1))
	return 1
}

func spellRegister(L *lua.LState) int {
	s := checkSpell(L)
	spells.Register(s)
	L.Push(lua.LTrue)
	return 1
}

// CallSpell executes the spell's OnCastSpell func.
func (e *Engine) CallSpell(s *spells.Spell, player *game.Player, words string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	L := e.L
	if s.OnCastSpell == nil || s.OnCastSpell.Type() != lua.LTFunction {
		return false
	}

	playerUd := L.NewUserData()
	playerUd.Value = player
	L.SetMetatable(playerUd, L.GetTypeMetatable("Player"))

	param := ""
	prefix := s.Words
	if len(words) > len(prefix) && strings.HasPrefix(strings.ToLower(words), strings.ToLower(prefix)) {
		param = strings.TrimSpace(words[len(prefix):])
	}

	variantUd := lua.LString(param)

	err := L.CallByParam(lua.P{Fn: s.OnCastSpell, NRet: 1, Protect: true}, playerUd, variantUd)
	if err != nil {
		e.log.Error("spell error", "err", err)
		return false
	}

	ret := L.Get(-1)
	L.Pop(1)
	if lua.LVIsFalse(ret) {
		return false
	}
	return true
}
