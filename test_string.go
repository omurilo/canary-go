package main

import (
	"fmt"
	lua "github.com/yuin/gopher-lua"
)

func main() {
	L := lua.NewState()
	defer L.Close()
	err := L.DoString(`
		local s = "hello"
		s:getId()
	`)
	fmt.Println(err)
}
