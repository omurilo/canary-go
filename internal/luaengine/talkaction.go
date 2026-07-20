package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/talkactions"
	lua "github.com/yuin/gopher-lua"
)

const luaTalkActionTypeName = "TalkAction"

// registerTalkAction registers the TalkAction global constructor and metatable
func (e *Engine) registerTalkAction() {
	mt := e.L.NewTypeMetatable(luaTalkActionTypeName)
	e.setClassConstructor("TalkAction", talkActionConstructor, talkActionMethods)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), talkActionMethods))
	e.L.SetField(mt, "__newindex", e.L.NewFunction(talkActionNewIndex))
}

func talkActionConstructor(L *lua.LState) int {
	words := L.CheckString(2)
	t := &talkactions.TalkAction{
		Words: words,
	}
	ud := L.NewUserData()
	ud.Value = t
	L.SetMetatable(ud, L.GetTypeMetatable(luaTalkActionTypeName))
	L.Push(ud)
	return 1
}

var talkActionMethods = map[string]lua.LGFunction{
	"separator": talkActionSeparator,
	"groupType": talkActionGroupType,
	"register":  talkActionRegister,
}

func checkTalkAction(L *lua.LState) *talkactions.TalkAction {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*talkactions.TalkAction); ok {
		return v
	}
	L.ArgError(1, "TalkAction expected")
	return nil
}

func talkActionNewIndex(L *lua.LState) int {
	t := checkTalkAction(L)
	key := L.CheckString(2)
	val := L.CheckAny(3)

	if key == "onSay" {
		t.OnSayFunc = val
	}
	return 0
}

func talkActionSeparator(L *lua.LState) int {
	t := checkTalkAction(L)
	t.Separator = L.CheckString(2)
	L.Push(L.Get(1))
	return 1
}

func talkActionGroupType(L *lua.LState) int {
	t := checkTalkAction(L)
	t.GroupType = L.CheckString(2)
	L.Push(L.Get(1))
	return 1
}

func talkActionRegister(L *lua.LState) int {
	t := checkTalkAction(L)
	talkactions.Register(t)
	L.Push(lua.LTrue)
	return 1
}

// CallTalkAction executes the talkaction's OnSayFunc.
func (e *Engine) CallTalkAction(t *talkactions.TalkAction, player *game.Player, typeID byte, words string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	L := e.L
	if t.OnSayFunc == nil || t.OnSayFunc.Type() != lua.LTFunction {
		return false
	}

	// args: (player, words, param, type)
	playerUd := L.NewUserData()
	playerUd.Value = player
	L.SetMetatable(playerUd, L.GetTypeMetatable("Player"))

	param := ""
	prefix := t.Words
	if len(words) > len(prefix) && strings.HasPrefix(strings.ToLower(words), strings.ToLower(prefix)) {
		param = strings.TrimSpace(words[len(prefix):])
	}

	err := L.CallByParam(lua.P{Fn: t.OnSayFunc, NRet: 1, Protect: true},
		playerUd, lua.LString(words), lua.LString(param), lua.LNumber(typeID))
	if err != nil {
		e.log.Error("talkaction error", "err", err)
		return false
	}

	ret := L.Get(-1)
	L.Pop(1)
	if lua.LVIsFalse(ret) {
		return false
	}
	return true
}
