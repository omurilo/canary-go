package luaengine

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
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
	e.L.SetField(mt, "sellItem", e.L.NewFunction(e.npcSellItem))
	e.L.SetField(mt, "teleportTo", e.L.NewFunction(e.creatureTeleportto))
	e.L.SetField(mt, "changeSpeed", e.L.NewFunction(e.creatureChangespeed))
	e.L.SetField(mt, "setSpeed", e.L.NewFunction(e.creatureSetspeed))
	e.L.SetField(mt, "getParent", e.L.NewFunction(e.creatureGetparent))
	e.L.SetField(mt, "getTile", e.L.NewFunction(e.creatureGettile))
	e.L.SetField(mt, "remove", e.L.NewFunction(e.creatureRemove))
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
	"getDistanceTo": npcGetdistanceto,
}

func npcCloseshopwindow(L *lua.LState) int { return 0 }

func npcFollow(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func npcGetcurrency(L *lua.LState) int {
	// Reads the type's currency (npcConfig.currency), defaulting to ITEM_GOLD_COIN.
	if n := checkNpc(L); n != nil {
		L.Push(lua.LNumber(n.CurrencyID()))
		return 1
	}
	L.Push(lua.LNumber(creatures.DefaultNpcCurrency))
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
	// Reads the type's npcConfig.speechBubble, defaulting to SPEECHBUBBLE_NORMAL.
	if n := checkNpc(L); n != nil {
		L.Push(lua.LNumber(n.SpeechBubble()))
		return 1
	}
	L.Push(lua.LNumber(creatures.SpeechBubbleNormal))
	return 1
}

func npcIsintalkrange(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		L.Push(lua.LFalse)
		return 1
	}
	targetPos, ok := parsePosition(L, 2)
	if !ok {
		L.Push(lua.LFalse)
		return 1
	}
	talkRange := 4
	if L.GetTop() >= 3 {
		if r := L.OptInt(3, 4); r > 0 {
			talkRange = r
		}
	}
	npcPos := n.GetPosition()
	if npcPos.Z != targetPos.Z {
		L.Push(lua.LFalse)
		return 1
	}
	dx := int(npcPos.X) - int(targetPos.X)
	if dx < 0 {
		dx = -dx
	}
	dy := int(npcPos.Y) - int(targetPos.Y)
	if dy < 0 {
		dy = -dy
	}
	L.Push(lua.LBool(dx <= talkRange && dy <= talkRange))
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

// npcSellItem implements
// npc:sellItem(player, itemId, amount, subType=1, actionId=0, ignoreCap=false, inBackpacks=false)
// porting luaNpcSellItem (npc_functions.cpp:569).
//
// This is the delivery half of an NPC purchase and the datapack depends on it: 306
// npc scripts define onBuyItem, and every one of them just calls
// npc:sellItem(...). It used to be a no-op, so nothing was ever delivered through
// the Lua path.
//
// actionId is accepted and ignored: Go's InternalAddItem cannot stamp an action id
// on creation yet, and no shop in the datapack passes a non-zero one.
func (e *Engine) npcSellItem(L *lua.LState) int {
	n := checkNpc(L)
	if n == nil {
		L.Push(lua.LFalse)
		return 1
	}
	p := checkPlayerArg(L, 2)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}

	itemID := uint16(L.CheckInt(3))
	amount := uint16(L.CheckInt(4))
	subType := 1
	if L.GetTop() >= 5 && L.Get(5).Type() == lua.LTNumber {
		subType = L.CheckInt(5)
	}
	// arg 6 is actionId; see the note above.
	inBackpacks := false
	if L.GetTop() >= 8 {
		inBackpacks = lua.LVAsBool(L.Get(8))
	}

	if e.world == nil || e.world.Items == nil {
		L.Push(lua.LFalse)
		return 1
	}

	result, ok := n.SellItemTo(p, e.world.Items, itemID, amount, subType, inBackpacks)
	if !ok {
		p.SendTextMessage(messageFailure, "You do not have enough room to carry this item.")
		L.Push(lua.LFalse)
		return 1
	}

	name := itemDisplayName(e.world.Items, itemID)
	if result.BagsCost > 0 {
		p.SendTextMessage(messageTrade, fmt.Sprintf(
			"Bought %dx %s and shopping bags for %d gold coins.",
			result.Delivered, name, result.Charged))
	} else {
		p.SendTextMessage(messageTrade, fmt.Sprintf(
			"Bought %dx %s for %d gold coins.", result.Delivered, name, result.Charged))
	}

	L.Push(lua.LTrue)
	return 1
}

// Message classes used by the shop replies (MESSAGE_FAILURE / MESSAGE_TRADE).
const (
	messageFailure = 0x13
	messageTrade   = 0x14
)

// itemDisplayName resolves an item's name for a shop message, falling back to the
// numeric id when the catalog has no entry.
func itemDisplayName(catalog *items.Catalog, itemID uint16) string {
	if catalog != nil {
		if it := catalog.Get(itemID); it != nil && it.Name != "" {
			return it.Name
		}
	}
	return fmt.Sprintf("item %d", itemID)
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


// CallNpcCloseChannel fires the NPC's onCloseChannel callback (if defined) when
// a player closes the shop/trade window, mirroring Npc::onPlayerCloseChannel so
// the dialogue module can reset its per-player topic state.
func (e *Engine) CallNpcCloseChannel(npc *game.Npc, player *game.Player) {
	if npc == nil || player == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	e.npcCallbacksMu.Lock()
	if e.npcCallbacks == nil {
		e.npcCallbacksMu.Unlock()
		return
	}
	callbacks, ok := e.npcCallbacks[strings.ToLower(npc.Name)]
	if !ok || callbacks["onCloseChannel"] == nil {
		e.npcCallbacksMu.Unlock()
		return
	}
	fn := callbacks["onCloseChannel"]
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

	if err := L.PCall(2, 0, nil); err != nil {
		e.log.Error("lua npc onCloseChannel", "npc", npc.Name, "err", err)
	}
}

func (e *Engine) CallNpcOnCreatureSay(npc *game.Npc, player *game.Player, talkType byte, text string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.npcCallbacksMu.Lock()
	if e.npcCallbacks == nil {
		e.npcCallbacksMu.Unlock()
		return false
	}
	callbacks, ok := e.npcCallbacks[strings.ToLower(npc.Name)]
	if !ok || callbacks["onSay"] == nil {
		e.npcCallbacksMu.Unlock()
		return false
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

	if err := L.PCall(4, 1, nil); err != nil {
		e.log.Error("lua npc onCreatureSay", "npc", npc.Name, "err", err)
		return false
	}

	ret := L.Get(-1)
	L.Pop(1)
	return lua.LVAsBool(ret)
}

// CallNpcOnThink dispatches the NpcType onThink(npc, interval) callback.
//
// This is what makes npcHandler:onThink run, and with it the whole FocusModule
// lifecycle: greeting timeouts, the farewell when a player walks away, and the
// NpcHandler message queue. The callback was being registered by every npc script
// and never called.
func (e *Engine) CallNpcOnThink(npc *game.Npc, interval uint32) {
	if npc == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.npcCallbacksMu.Lock()
	if e.npcCallbacks == nil {
		e.npcCallbacksMu.Unlock()
		return
	}
	callbacks, ok := e.npcCallbacks[strings.ToLower(npc.Name)]
	if !ok || callbacks["onThink"] == nil {
		e.npcCallbacksMu.Unlock()
		return
	}
	fn := callbacks["onThink"]
	e.npcCallbacksMu.Unlock()

	L := e.L
	L.Push(fn)

	udNpc := L.NewUserData()
	udNpc.Value = npc
	L.SetMetatable(udNpc, L.GetTypeMetatable("Npc"))
	L.Push(udNpc)
	L.Push(lua.LNumber(interval))

	if err := L.PCall(2, 0, nil); err != nil {
		e.log.Error("lua npc onThink", "npc", npc.Name, "err", err)
	}
}

// CallNpcOnBuyItem dispatches
// npc:onBuyItem(player, itemId, subType, amount, ignore, inBackpacks, totalCost),
// mirroring the callback at the end of Npc::onPlayerBuyItem (npc.cpp:775).
//
// The core only validates and prices a purchase; the callback is what performs it,
// by calling npc:sellItem. Returns false when the NPC defines no onBuyItem, so the
// caller can tell "nothing happened" from "the script handled it".
func (e *Engine) CallNpcOnBuyItem(npc *game.Npc, p *game.Player, itemID uint16, subType uint8, amount uint16, ignore, inBackpacks bool, totalCost uint64) bool {
	fn := e.npcCallback(npc, "onBuyItem")
	if fn == nil {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	L := e.L
	L.Push(fn)
	e.pushNpcUserdata(npc)
	e.pushPlayerUserdata(p)
	L.Push(lua.LNumber(itemID))
	L.Push(lua.LNumber(subType))
	L.Push(lua.LNumber(amount))
	L.Push(lua.LBool(ignore))
	L.Push(lua.LBool(inBackpacks))
	L.Push(lua.LNumber(totalCost))

	if err := L.PCall(8, 0, nil); err != nil {
		e.log.Error("lua npc onBuyItem", "npc", npc.Name, "err", err)
		return false
	}
	return true
}

// CallNpcOnSellItem dispatches
// npc:onSellItem(player, itemId, subType, amount, ignore, itemName, totalCost),
// mirroring the callback at the end of Npc::onPlayerSellItem (npc.cpp:989).
//
// Unlike onBuyItem this is a NOTIFICATION: the core has already removed the items
// and credited the proceeds by the time it fires, and the datapack scripts only use
// it to send the "Sold Nx ..." message.
func (e *Engine) CallNpcOnSellItem(npc *game.Npc, p *game.Player, itemID uint16, subType uint8, amount uint32, ignore bool, itemName string, totalCost uint64) bool {
	fn := e.npcCallback(npc, "onSellItem")
	if fn == nil {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	L := e.L
	L.Push(fn)
	e.pushNpcUserdata(npc)
	e.pushPlayerUserdata(p)
	L.Push(lua.LNumber(itemID))
	L.Push(lua.LNumber(subType))
	L.Push(lua.LNumber(amount))
	L.Push(lua.LBool(ignore))
	L.Push(lua.LString(itemName))
	L.Push(lua.LNumber(totalCost))

	if err := L.PCall(8, 0, nil); err != nil {
		e.log.Error("lua npc onSellItem", "npc", npc.Name, "err", err)
		return false
	}
	return true
}

// npcCallback looks up one of an NPC type's registered callbacks, or nil.
func (e *Engine) npcCallback(npc *game.Npc, name string) *lua.LFunction {
	if npc == nil {
		return nil
	}
	e.npcCallbacksMu.Lock()
	defer e.npcCallbacksMu.Unlock()
	if e.npcCallbacks == nil {
		return nil
	}
	callbacks, ok := e.npcCallbacks[strings.ToLower(npc.Name)]
	if !ok {
		return nil
	}
	return callbacks[name]
}

// pushNpcUserdata and pushPlayerUserdata must be called with e.mu held.
func (e *Engine) pushNpcUserdata(npc *game.Npc) {
	ud := e.L.NewUserData()
	ud.Value = npc
	e.L.SetMetatable(ud, e.L.GetTypeMetatable("Npc"))
	e.L.Push(ud)
}

func (e *Engine) pushPlayerUserdata(p *game.Player) {
	ud := e.L.NewUserData()
	ud.Value = p
	e.L.SetMetatable(ud, e.L.GetTypeMetatable("Player"))
	e.L.Push(ud)
}
