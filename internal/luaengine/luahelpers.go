package luaengine

import (
	"strconv"

	lua "github.com/yuin/gopher-lua"
)

// The Lua binding layer in C++ (Lua::getNumber / Lua::getBoolean) never raises
// on a missing or mistyped argument: it returns the type's zero value and
// tolerates numeric strings. Several spell scripts rely on that leniency (e.g.
// spell:cooldown("2000"), combat:setFormula with only four numbers). These
// helpers reproduce it so scripts load instead of aborting mid-file.

func luaOptNumber(L *lua.LState, n int) float64 {
	v := L.Get(n)
	switch v.Type() {
	case lua.LTNumber:
		return float64(lua.LVAsNumber(v))
	case lua.LTString:
		if f, err := strconv.ParseFloat(v.String(), 64); err == nil {
			return f
		}
	}
	return 0
}

func luaOptInt(L *lua.LState, n int) int { return int(luaOptNumber(L, n)) }

func luaOptBool(L *lua.LState, n int) bool {
	v := L.Get(n)
	switch v.Type() {
	case lua.LTBool:
		return lua.LVAsBool(v)
	case lua.LTNumber:
		return lua.LVAsNumber(v) != 0
	default:
		return false
	}
}
