package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// LuaVariantType mirrors LuaVariantType_t (src/enums/lua_variant_type.hpp).
type LuaVariantType int

const (
	VariantNone           LuaVariantType = 0
	VariantNumber         LuaVariantType = 1
	VariantPosition       LuaVariantType = 2
	VariantTargetPosition LuaVariantType = 3
	VariantString         LuaVariantType = 4
)

// luaVariant mirrors the C++ LuaVariant (src/lua/global/lua_variant.hpp) passed
// as the second argument to a spell's onCastSpell(creature, var).
type luaVariant struct {
	vtype       LuaVariantType
	number      uint32
	pos         game.Position
	text        string
	instantName string
}

const variantTypeName = "Variant"

// registerVariant installs the Variant metatable and the global Variant(...)
// constructor, matching variant_functions.cpp: Variant(number|position|string).
func (e *Engine) registerVariant() {
	mt := e.L.NewTypeMetatable(variantTypeName)
	e.setClassConstructor("Variant", variantConstructor, variantMethods)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), variantMethods))
}

func pushVariant(L *lua.LState, v *luaVariant) {
	ud := L.NewUserData()
	ud.Value = v
	L.SetMetatable(ud, L.GetTypeMetatable(variantTypeName))
	L.Push(ud)
}

func checkVariant(L *lua.LState, n int) *luaVariant {
	ud := L.CheckUserData(n)
	if v, ok := ud.Value.(*luaVariant); ok {
		return v
	}
	L.ArgError(n, "Variant expected")
	return nil
}

func variantConstructor(L *lua.LState) int {
	v := &luaVariant{}
	arg := L.Get(2)
	switch arg.Type() {
	case lua.LTNumber:
		v.vtype = VariantNumber
		v.number = uint32(lua.LVAsNumber(arg))
	case lua.LTString:
		v.vtype = VariantString
		v.text = arg.String()
	case lua.LTUserData:
		if p, ok := arg.(*lua.LUserData).Value.(game.Position); ok {
			v.vtype = VariantPosition
			v.pos = p
		}
	}
	pushVariant(L, v)
	return 1
}

var variantMethods = map[string]lua.LGFunction{
	"getNumber": func(L *lua.LState) int {
		v := checkVariant(L, 1)
		L.Push(lua.LNumber(v.number))
		return 1
	},
	"getString": func(L *lua.LState) int {
		v := checkVariant(L, 1)
		L.Push(lua.LString(v.text))
		return 1
	},
	"getPosition": func(L *lua.LState) int {
		v := checkVariant(L, 1)
		pushPosition(L, v.pos)
		return 1
	},
	"getInstantName": func(L *lua.LState) int {
		v := checkVariant(L, 1)
		L.Push(lua.LString(v.instantName))
		return 1
	},
}
