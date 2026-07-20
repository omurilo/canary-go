package luaengine

import (
	"strings"
	
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

// registerNpc registers the Npc userdata type.
func (e *Engine) registerNpc() {
	mt := e.L.NewTypeMetatable("Npc")
	// Npc IS-A Creature: expose all creature methods, npc-specific win.
	idx := e.L.NewTable()
	e.L.SetFuncs(idx, creatureMethods)
	e.L.SetFuncs(idx, npcMethods)
	e.L.SetField(mt, "__index", idx)

	// Override methods that need the engine/world instance
	e.L.SetField(idx, "say", e.L.NewFunction(e.npcSay))
	e.L.SetField(idx, "openShopWindow", e.L.NewFunction(e.npcOpenshopwindow))
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
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) npcOpenshopwindow(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		return 0
	}
	p, _ := L.CheckUserData(2).Value.(*game.Player)
	if p == nil {
		return 0
	}
	
	if e.world != nil {
		nType := e.world.TypeRegistry.Npcs[strings.ToLower(n.Name)]
		if nType != nil && len(nType.ShopItems) > 0 {
			p.SendOpenShop(n, nType.ShopItems)
		}
	}
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
	return 0
}

func (e *Engine) npcSay(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		return 0
	}
	text := L.CheckString(2)
	talkType := byte(1) // SAY
	if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTNumber {
		talkType = byte(L.ToNumber(3))
	}
	
	game.GlobalDispatcher.AddEvent(0, func() {
		if e.world != nil && e.world.OnCreatureSay != nil {
			e.world.OnCreatureSay(n, talkType, text)
		}
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


func (e *Engine) CallNpcOnCreatureSay(npc *game.Npc, player *game.Player, talkType byte, text string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.npcCallbacksMu.Lock()
	if e.npcCallbacks == nil {
		e.npcCallbacksMu.Unlock()
		return
	}
	callbacks, ok := e.npcCallbacks[strings.ToLower(npc.Name)]
	if !ok || callbacks["onSay"] == nil {
		e.npcCallbacksMu.Unlock()
		return
	}
	fn := callbacks["onSay"]
	e.npcCallbacksMu.Unlock()

	L := e.L
	L.Push(fn)

	udNpc := L.NewUserData()
	udNpc.Value = npc
	L.SetMetatable(udNpc, L.GetTypeMetatable("Npc"))
	L.Push(udNpc)

	udPlayer := L.NewUserData()
	udPlayer.Value = player
	L.SetMetatable(udPlayer, L.GetTypeMetatable("Player"))
	L.Push(udPlayer)

	L.Push(lua.LNumber(talkType))
	L.Push(lua.LString(text))

	if err := L.PCall(4, 0, nil); err != nil {
		e.log.Error("lua npc onCreatureSay", "npc", npc.Name, "err", err)
	}
}
