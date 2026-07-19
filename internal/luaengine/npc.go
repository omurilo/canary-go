package luaengine

import (
	lua "github.com/yuin/gopher-lua"
	"github.com/opentibiabr/canary-go/internal/game"
)

func checkNpc(L *lua.LState) *game.Npc {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*game.Npc); ok {
		return v
	}
	L.ArgError(1, "Npc expected")
	return nil
}

// registerNpcType registers the Npc userdata type.
func (e *Engine) registerNpcType() {
	mt := e.L.NewTypeMetatable("Npc")
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), npcMethods))
}

var npcMethods = map[string]lua.LGFunction{
	"isNpc": npcIsnpc,
	"setMasterPos": npcSetmasterpos,
	"getCurrency": npcGetcurrency,
	"setCurrency": npcSetcurrency,
	"getSpeechBubble": npcGetspeechbubble,
	"setSpeechBubble": npcSetspeechbubble,
	"getId": npcGetid,
	"getName": npcGetname,
	"setName": npcSetname,
	"place": npcPlace,
	"say": npcSay,
	"turnToCreature": npcTurntocreature,
	"setPlayerInteraction": npcSetplayerinteraction,
	"removePlayerInteraction": npcRemoveplayerinteraction,
	"isInteractingWithPlayer": npcIsinteractingwithplayer,
	"isInTalkRange": npcIsintalkrange,
	"isPlayerInteractingOnTopic": npcIsplayerinteractingontopic,
	"openShopWindow": npcOpenshopwindow,
	"openShopWindowTable": npcOpenshopwindowtable,
	"closeShopWindow": npcCloseshopwindow,
	"getShopItem": npcGetshopitem,
	"isMerchant": npcIsmerchant,
	"move": npcMove,
	"turn": npcTurn,
	"follow": npcFollow,
	"sellItem": npcSellitem,
	"getDistanceTo": npcGetdistanceto,
}

func npcCloseshopwindow(L *lua.LState) int {
	// TODO: implement closeShopWindow
	return 0
}

func npcFollow(L *lua.LState) int {
	// TODO: implement follow
	return 0
}

func npcGetcurrency(L *lua.LState) int {
	// TODO: implement getCurrency
	return 0
}

func npcGetdistanceto(L *lua.LState) int {
	// TODO: implement getDistanceTo
	return 0
}

func npcGetid(L *lua.LState) int {
	// TODO: implement getId
	return 0
}

func npcGetname(L *lua.LState) int {
	// TODO: implement getName
	return 0
}

func npcGetshopitem(L *lua.LState) int {
	// TODO: implement getShopItem
	return 0
}

func npcGetspeechbubble(L *lua.LState) int {
	// TODO: implement getSpeechBubble
	return 0
}

func npcIsintalkrange(L *lua.LState) int {
	// TODO: implement isInTalkRange
	return 0
}

func npcIsinteractingwithplayer(L *lua.LState) int {
	// TODO: implement isInteractingWithPlayer
	return 0
}

func npcIsmerchant(L *lua.LState) int {
	// TODO: implement isMerchant
	return 0
}

func npcIsnpc(L *lua.LState) int {
	// TODO: implement isNpc
	return 0
}

func npcIsplayerinteractingontopic(L *lua.LState) int {
	// TODO: implement isPlayerInteractingOnTopic
	return 0
}

func npcMove(L *lua.LState) int {
	// TODO: implement move
	return 0
}

func npcOpenshopwindow(L *lua.LState) int {
	// TODO: implement openShopWindow
	return 0
}

func npcOpenshopwindowtable(L *lua.LState) int {
	// TODO: implement openShopWindowTable
	return 0
}

func npcPlace(L *lua.LState) int {
	// TODO: implement place
	return 0
}

func npcRemoveplayerinteraction(L *lua.LState) int {
	// TODO: implement removePlayerInteraction
	return 0
}

func npcSay(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		return 0
	}
	text := L.CheckString(2)
	game.GlobalDispatcher.AddEvent(0, func() {
		n.Say(text)
	})
	return 0
}

func npcSellitem(L *lua.LState) int {
	// TODO: implement sellItem
	return 0
}

func npcSetcurrency(L *lua.LState) int {
	// TODO: implement setCurrency
	return 0
}

func npcSetmasterpos(L *lua.LState) int {
	// TODO: implement setMasterPos
	return 0
}

func npcSetname(L *lua.LState) int {
	// TODO: implement setName
	return 0
}

func npcSetplayerinteraction(L *lua.LState) int {
	// TODO: implement setPlayerInteraction
	return 0
}

func npcSetspeechbubble(L *lua.LState) int {
	// TODO: implement setSpeechBubble
	return 0
}

func npcTurn(L *lua.LState) int {
	// TODO: implement turn
	return 0
}

func npcTurntocreature(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		return 0
	}
	ud := L.CheckUserData(2)
	if c, ok := ud.Value.(game.Creature); ok {
		game.GlobalDispatcher.AddEvent(0, func() {
			n.TurnToCreature(c)
		})
	}
	return 0
}

