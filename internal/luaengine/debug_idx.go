package luaengine

import (
	"fmt"
	lua "github.com/yuin/gopher-lua"
)

func dumpTableKeys(name string, t *lua.LTable) {
	fmt.Printf("--- DUMP %s ---\n", name)
	t.ForEach(func(k, v lua.LValue) {
		fmt.Printf("%s: %s\n", k.String(), v.Type().String())
	})
	fmt.Printf("--- END %s ---\n", name)
}
