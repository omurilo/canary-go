package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

// registerBitLib installs standard bitwise operation functions (bit / bit32 tables) in Lua.
func registerBitLib(L *lua.LState) {
	bit := L.NewTable()

	L.SetField(bit, "band", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		res := uint32(0xFFFFFFFF)
		if n > 0 {
			res = uint32(L.CheckNumber(1))
			for i := 2; i <= n; i++ {
				res &= uint32(L.CheckNumber(i))
			}
		} else {
			res = 0
		}
		L.Push(lua.LNumber(res))
		return 1
	}))

	L.SetField(bit, "bor", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		res := uint32(0)
		for i := 1; i <= n; i++ {
			res |= uint32(L.CheckNumber(i))
		}
		L.Push(lua.LNumber(res))
		return 1
	}))

	L.SetField(bit, "bxor", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		res := uint32(0)
		if n > 0 {
			res = uint32(L.CheckNumber(1))
			for i := 2; i <= n; i++ {
				res ^= uint32(L.CheckNumber(i))
			}
		}
		L.Push(lua.LNumber(res))
		return 1
	}))

	L.SetField(bit, "bnot", L.NewFunction(func(L *lua.LState) int {
		val := uint32(L.CheckNumber(1))
		L.Push(lua.LNumber(^val))
		return 1
	}))

	L.SetField(bit, "lshift", L.NewFunction(func(L *lua.LState) int {
		val := uint32(L.CheckNumber(1))
		shift := uint32(L.CheckNumber(2)) & 31
		L.Push(lua.LNumber(val << shift))
		return 1
	}))

	L.SetField(bit, "rshift", L.NewFunction(func(L *lua.LState) int {
		val := uint32(L.CheckNumber(1))
		shift := uint32(L.CheckNumber(2)) & 31
		L.Push(lua.LNumber(val >> shift))
		return 1
	}))

	L.SetField(bit, "arshift", L.NewFunction(func(L *lua.LState) int {
		val := int32(L.CheckNumber(1))
		shift := uint32(L.CheckNumber(2)) & 31
		L.Push(lua.LNumber(val >> shift))
		return 1
	}))

	L.SetGlobal("bit", bit)
	L.SetGlobal("bit32", bit)
}
