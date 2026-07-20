package luaengine

import (
	"strings"
	
	lua "github.com/yuin/gopher-lua"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/creatures"
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
	// Npc IS-A Creature; methods live directly on the metatable (see
	// registerCreatureType) so revscriptsys CreatureIndex finds them.
	e.L.SetFuncs(mt, creatureMethods)
	e.L.SetFuncs(mt, npcMethods)
	// Engine/world-bound overrides.
	e.L.SetField(mt, "say", e.L.NewFunction(e.npcSay))
	e.L.SetField(mt, "openShopWindow", e.L.NewFunction(e.npcOpenshopwindow))
	e.L.SetField(mt, "isMerchant", e.L.NewFunction(e.npcIsmerchant))
	e.L.SetField(mt, "teleportTo", e.L.NewFunction(e.creatureTeleportto))
	e.L.SetField(mt, "__index", mt)
}

// npcIsmerchant reports whether the NPC's type defines shop items. NpcHandler:
// tradeRequest only calls openShopWindow when this is true, so a merchant must
// answer true here for "trade" to open the shop.
func (e *Engine) npcIsmerchant(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		L.Push(lua.LFalse)
		return 1
	}
	merchant := false
	if e.world != nil && e.world.TypeRegistry != nil {
		if nt := e.world.TypeRegistry.Npcs[strings.ToLower(n.Name)]; nt != nil && len(nt.ShopItems) > 0 {
			merchant = true
		}
	}
	L.Push(lua.LBool(merchant))
	return 1
}

var npcMethods = map[string]lua.LGFunction{
	"isNpc": npcIsnpc,
	"setMasterPos": npcSetmasterpos,
	"getCurrency": npcGetcurrency,
	"setCurrency": npcSetcurrency,
	"getSpeechBubble": npcGetspeechbubble,
	"setSpeechBubble": npcSetspeechbubble,
	// getId/getName/move are inherited from creatureMethods (which are now
	// implemented); don't shadow them with stubs here.
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
	"turn": npcTurn,
	"follow": npcFollow,
	"sellItem": npcSellitem,
	"getDistanceTo": npcGetdistanceto,
}

func npcCloseshopwindow(L *lua.LState) int { return 0 }

func npcFollow(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func npcGetcurrency(L *lua.LState) int {
	// Default shop currency: gold coin (client id 3031).
	L.Push(lua.LNumber(3031))
	return 1
}

func npcGetdistanceto(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		L.Push(lua.LNil)
		return 1
	}
	ud, ok := L.Get(2).(*lua.LUserData)
	if !ok {
		L.Push(lua.LNil)
		return 1
	}
	other, ok := ud.Value.(game.Creature)
	if !ok {
		L.Push(lua.LNil)
		return 1
	}
	a, b := n.GetPosition(), other.GetPosition()
	dx := int(a.X) - int(b.X)
	dy := int(a.Y) - int(b.Y)
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	dist := dx
	if dy > dist {
		dist = dy
	}
	L.Push(lua.LNumber(dist))
	return 1
}

func npcGetshopitem(L *lua.LState) int {
	L.Push(lua.LNil)
	return 1
}

func npcGetspeechbubble(L *lua.LState) int {
	// SPEECHBUBBLE_NORMAL (1).
	L.Push(lua.LNumber(1))
	return 1
}

func npcIsintalkrange(L *lua.LState) int {
	// No range check modelled; assume in range so dialogue proceeds.
	L.Push(lua.LTrue)
	return 1
}

func npcIsinteractingwithplayer(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LBool(n.IsInteractingWithPlayer(interactionPlayerID(L, 2))))
	return 1
}

// interactionPlayerID extracts a player creature id from arg n, which may be a
// creature userdata or a numeric id.
func interactionPlayerID(L *lua.LState, n int) uint32 {
	switch v := L.Get(n); v.Type() {
	case lua.LTUserData:
		if c, ok := v.(*lua.LUserData).Value.(game.Creature); ok {
			return c.GetID()
		}
	case lua.LTNumber:
		return uint32(lua.LVAsNumber(v))
	}
	return 0
}


func npcIsnpc(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func npcIsplayerinteractingontopic(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
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
			p.ShopOwnerID = n.ID
			p.SendOpenShop(n, nType.ShopItems)
		}
	}
	return 0
}

func npcOpenshopwindowtable(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		return 0
	}
	p, _ := L.CheckUserData(2).Value.(*game.Player)
	if p == nil {
		return 0
	}
	
	tbl := L.CheckTable(3)
	var shopItems []creatures.ShopItem
	
	tbl.ForEach(func(key lua.LValue, val lua.LValue) {
		if innerTbl, ok := val.(*lua.LTable); ok {
			var si creatures.ShopItem
			
			if idVal := innerTbl.RawGetString("id"); idVal.Type() == lua.LTNumber {
				si.ID = uint16(lua.LVAsNumber(idVal))
			} else if idVal := innerTbl.RawGetString("itemId"); idVal.Type() == lua.LTNumber {
				si.ID = uint16(lua.LVAsNumber(idVal))
			}
			
			if buyVal := innerTbl.RawGetString("buy"); buyVal.Type() == lua.LTNumber {
				si.BuyPrice = uint32(lua.LVAsNumber(buyVal))
			}
			
			if sellVal := innerTbl.RawGetString("sell"); sellVal.Type() == lua.LTNumber {
				si.SellPrice = uint32(lua.LVAsNumber(sellVal))
			}
			
			if nameVal := innerTbl.RawGetString("name"); nameVal.Type() == lua.LTString {
				si.Name = lua.LVAsString(nameVal)
			}
			
			// SubType check if we support it
			if subTypeVal := innerTbl.RawGetString("subType"); subTypeVal.Type() == lua.LTNumber {
				si.SubType = uint8(lua.LVAsNumber(subTypeVal))
			}
			
			if si.ID != 0 {
				shopItems = append(shopItems, si)
			}
		}
	})
	
	p.ShopOwnerID = n.ID
	p.SendOpenShop(n, shopItems)
	
	L.Push(lua.LTrue)
	return 1
}

func npcPlace(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func npcRemoveplayerinteraction(L *lua.LState) int {
	if n := checkNpc(L); n != nil {
		n.RemovePlayerInteraction(interactionPlayerID(L, 2))
	}
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
	// The concrete buy/sell flow is handled by the onBuyItem/onSellItem NPC
	// callbacks; this helper is a no-op in this slice.
	return 0
}

func npcSetcurrency(L *lua.LState) int { return 0 }

func npcSetmasterpos(L *lua.LState) int { return 0 }

func npcSetname(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		return 0
	}
	n.Name = L.CheckString(2)
	return 0
}

func npcSetplayerinteraction(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		return 0
	}
	topic := 0
	if L.GetTop() >= 3 {
		topic = luaOptInt(L, 3)
	}
	n.SetPlayerInteraction(interactionPlayerID(L, 2), topic)
	return 0
}

func npcSetspeechbubble(L *lua.LState) int { return 0 }

func npcTurn(L *lua.LState) int { return 0 }

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
