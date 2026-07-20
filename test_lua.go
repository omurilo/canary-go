package main

import (
	"fmt"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/luaengine"
	lua "github.com/yuin/gopher-lua"
)

func main() {
	e := luaengine.New(nil, nil)
	L := e.L
	
	err := L.DoString(`
		function test(npc, player, type, message)
			player:getId()
		end
	`)
	if err != nil {
		fmt.Println("DoString error:", err)
		return
	}
	
	fn := L.GetGlobal("test")
	L.Push(fn)
	
	udNpc := L.NewUserData()
	udNpc.Value = &game.Npc{}
	L.SetMetatable(udNpc, L.GetTypeMetatable("Npc"))
	L.Push(udNpc)
	
	udPlayer := L.NewUserData()
	udPlayer.Value = &game.Player{}
	L.SetMetatable(udPlayer, L.GetTypeMetatable("Player"))
	L.Push(udPlayer)
	
	L.Push(lua.LNumber(1))
	L.Push(lua.LString("hi"))
	
	if err := L.PCall(4, 0, nil); err != nil {
		fmt.Println("PCall error:", err)
	} else {
		fmt.Println("PCall success!")
	}
}
