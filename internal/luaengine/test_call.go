package luaengine

import (
	"fmt"
	lua "github.com/yuin/gopher-lua"
)

func TestGopherLuaCall() {
	L := lua.NewState()
	defer L.Close()

	classTable := L.NewTable()
	mt := L.NewTypeMetatable("Test_ClassCtor")
	L.SetField(mt, "__call", L.NewFunction(func(L *lua.LState) int {
		fmt.Println("Constructor called!")
		return 0
	}))
	L.SetMetatable(classTable, mt)
	L.SetGlobal("TestClass", classTable)

	err := L.DoString(`
		TestClass()
	`)
	if err != nil {
		fmt.Println("Error:", err)
	}
}
