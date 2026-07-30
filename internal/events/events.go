package events

import (
	"log/slog"
	"reflect"
	"sync"

	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// EventCallbackType identifies the type of event callback.
type EventCallbackType string

const (
	// Creature callbacks
	EventCreatureOnAreaCombat EventCallbackType = "creatureOnAreaCombat"
	EventCreatureOnTargetCombat EventCallbackType = "creatureOnTargetCombat"

	// Player callbacks
	EventPlayerOnLogin          EventCallbackType = "onLogin"
	EventPlayerOnLook           EventCallbackType = "playerOnLook"
	EventPlayerOnLookInShop     EventCallbackType = "playerOnLookInShop"
	EventPlayerOnLookInTrade    EventCallbackType = "playerOnLookInTrade"
	EventPlayerOnMoveItem       EventCallbackType = "onMoveItem"
	EventPlayerOnGainExperience EventCallbackType = "onGainExperience"
	EventPlayerOnDeath          EventCallbackType = "onDeath"
	EventPlayerOnTradeAccept    EventCallbackType = "playerOnTradeAccept"
	EventPlayerOnBrowseField    EventCallbackType = "playerOnBrowseField"
	EventPlayerOnRotateItem     EventCallbackType = "playerOnRotateItem"
	EventPlayerOnRemoveCount    EventCallbackType = "playerOnRemoveCount"
	EventPlayerOnRequestQuestLog   EventCallbackType = "playerOnRequestQuestLog"
	EventPlayerOnRequestQuestLine  EventCallbackType = "playerOnRequestQuestLine"
	EventPlayerOnStorageUpdate  EventCallbackType = "playerOnStorageUpdate"

	// Monster callbacks
	EventMonsterOnDropLoot     EventCallbackType = "monsterOnDropLoot"
	EventMonsterPostDropLoot   EventCallbackType = "monsterPostDropLoot"

	// Party callbacks
	EventPartyOnDisband         EventCallbackType = "partyOnDisband"
	EventPartyOnShareExperience EventCallbackType = "partyOnShareExperience"
)

// callbackField maps a Lua table field name to an EventCallbackType.
type callbackField struct {
	Field string
	Type  EventCallbackType
}

// allCallbackFields lists all known Lua table field names and their corresponding
// EventCallbackType. Used by Register to populate the callbacks map.
var allCallbackFields = []callbackField{
	{"onLogin", EventPlayerOnLogin},
	{"playerOnLook", EventPlayerOnLook},
	{"onLook", EventPlayerOnLook}, // legacy alias
	{"playerOnLookInShop", EventPlayerOnLookInShop},
	{"playerOnLookInTrade", EventPlayerOnLookInTrade},
	{"onMoveItem", EventPlayerOnMoveItem},
	{"playerOnMoveItem", EventPlayerOnMoveItem},
	{"onGainExperience", EventPlayerOnGainExperience},
	{"onDeath", EventPlayerOnDeath},
	{"playerOnBrowseField", EventPlayerOnBrowseField},
	{"playerOnRotateItem", EventPlayerOnRotateItem},
	{"playerOnRemoveCount", EventPlayerOnRemoveCount},
	{"playerOnTradeAccept", EventPlayerOnTradeAccept},
	{"playerOnRequestQuestLog", EventPlayerOnRequestQuestLog},
	{"playerOnRequestQuestLine", EventPlayerOnRequestQuestLine},
	{"playerOnStorageUpdate", EventPlayerOnStorageUpdate},
	{"creatureOnAreaCombat", EventCreatureOnAreaCombat},
	{"creatureOnTargetCombat", EventCreatureOnTargetCombat},
	{"monsterOnDropLoot", EventMonsterOnDropLoot},
	{"monsterPostDropLoot", EventMonsterPostDropLoot},
	{"partyOnDisband", EventPartyOnDisband},
	{"partyOnShareExperience", EventPartyOnShareExperience},
}

// Engine stores and invokes Lua event callbacks registered by EventCallback scripts.
type Engine struct {
	mu        sync.Mutex
	L         *lua.LState
	log       *slog.Logger
	callbacks map[EventCallbackType][]lua.LValue

	// WrapContainer builds the Container userdata for an item. The payload shape is
	// internal to the Lua engine (a luaContainer struct, not a *game.Item), and
	// events cannot import luaengine because luaengine imports events — so the Lua
	// engine sets this at startup. Without it the corpse would arrive as the wrong
	// userdata and Container methods would reject it.
	WrapContainer func(*game.Item) lua.LValue
}

// GlobalEngine is the process-wide event callback engine.
var GlobalEngine *Engine

// NewEngine creates a new event callback engine and sets it as GlobalEngine.
func NewEngine(L *lua.LState, log *slog.Logger) *Engine {
	e := &Engine{
		L:         L,
		log:       log,
		callbacks: make(map[EventCallbackType][]lua.LValue),
	}
	GlobalEngine = e
	return e
}

// Register reads all known callback field names from the Lua table and registers
// each function value under its corresponding EventCallbackType.
func (e *Engine) Register(callbackTable *lua.LTable) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, cf := range allCallbackFields {
		if val := callbackTable.RawGetString(cf.Field); val != lua.LNil {
			e.callbacks[cf.Type] = append(e.callbacks[cf.Type], val)
		}
	}
}

// executeCallbacks runs all callbacks registered for the given type.
// Each callback receives the provided args and should return a boolean.
// Returns false if any callback returned false (short-circuits).
func (e *Engine) executeCallbacks(typ EventCallbackType, args ...lua.LValue) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	fns, ok := e.callbacks[typ]
	if !ok || len(fns) == 0 {
		return true
	}

	L := e.L
	for _, fn := range fns {
		L.Push(fn)
		for _, arg := range args {
			L.Push(arg)
		}
		n := len(args)
		if err := L.PCall(n, 1, nil); err != nil {
			e.log.Error("event callback error", "type", string(typ), "err", err)
			continue
		}
		ret := L.Get(-1)
		L.Pop(1)
		if luaBool, ok := ret.(lua.LBool); ok && !bool(luaBool) {
			return false
		}
	}
	return true
}

// ExecuteOnLogin fires onLogin callbacks for the given player.
func (e *Engine) ExecuteOnLogin(player *game.Player) bool {
	L := e.L
	ud := L.NewUserData()
	ud.Value = player
	L.SetMetatable(ud, L.GetTypeMetatable("Player"))
	return e.executeCallbacks(EventPlayerOnLogin, ud)
}

// ExecuteOnLook fires playerOnLook/onLook callbacks.
func (e *Engine) ExecuteOnLook(player *game.Player, thing interface{}, position game.Position, distance int) bool {
	L := e.L

	pUd := L.NewUserData()
	pUd.Value = player
	L.SetMetatable(pUd, L.GetTypeMetatable("Player"))

	tUd := L.NewUserData()
	tUd.Value = thing
	if _, ok := thing.(*game.Item); ok {
		L.SetMetatable(tUd, L.GetTypeMetatable("Item"))
	} else {
		L.SetMetatable(tUd, L.GetTypeMetatable("Thing"))
	}

	posUd := L.NewUserData()
	posUd.Value = position
	L.SetMetatable(posUd, L.GetTypeMetatable("Position"))

	return e.executeCallbacks(EventPlayerOnLook, pUd, tUd, posUd, lua.LNumber(distance))
}

// ExecuteOnMoveItem fires onMoveItem callbacks.
func (e *Engine) ExecuteOnMoveItem(player *game.Player, item *game.Item, count uint16, fromPos game.Position, toPos game.Position) bool {
	L := e.L

	pUd := L.NewUserData()
	pUd.Value = player
	L.SetMetatable(pUd, L.GetTypeMetatable("Player"))

	iUd := L.NewUserData()
	iUd.Value = item
	L.SetMetatable(iUd, L.GetTypeMetatable("Item"))

	fPosUd := L.NewUserData()
	fPosUd.Value = fromPos
	L.SetMetatable(fPosUd, L.GetTypeMetatable("Position"))

	tPosUd := L.NewUserData()
	tPosUd.Value = toPos
	L.SetMetatable(tPosUd, L.GetTypeMetatable("Position"))

	return e.executeCallbacks(EventPlayerOnMoveItem, pUd, iUd, lua.LNumber(count), fPosUd, tPosUd)
}

// ExecuteOnGainExperience fires onGainExperience callbacks and returns the
// modified experience value.
func (e *Engine) ExecuteOnGainExperience(player *game.Player, source game.Creature, exp uint64, rawExp uint64) uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	fns, ok := e.callbacks[EventPlayerOnGainExperience]
	if !ok || len(fns) == 0 {
		return exp
	}

	L := e.L
	finalExp := exp
	for _, fn := range fns {
		L.Push(fn)

		pUd := L.NewUserData()
		pUd.Value = player
		L.SetMetatable(pUd, L.GetTypeMetatable("Player"))
		L.Push(pUd)

		if source != nil {
			sUd := L.NewUserData()
			sUd.Value = source
			L.SetMetatable(sUd, L.GetTypeMetatable("Creature"))
			L.Push(sUd)
		} else {
			L.Push(lua.LNil)
		}

		L.Push(lua.LNumber(finalExp))
		L.Push(lua.LNumber(rawExp))

		if err := L.PCall(4, 1, nil); err != nil {
			e.log.Error("event callback error", "type", string(EventPlayerOnGainExperience), "err", err)
			continue
		}

		ret := L.Get(-1)
		L.Pop(1)

		if num, ok := ret.(lua.LNumber); ok {
			finalExp = uint64(num)
		}
	}
	return finalExp
}

// ExecuteOnDeath fires onDeath callbacks.
func (e *Engine) ExecuteOnDeath(player *game.Player, killer game.Creature) bool {
	L := e.L

	pUd := L.NewUserData()
	pUd.Value = player
	L.SetMetatable(pUd, L.GetTypeMetatable("Player"))

	if killer != nil {
		kUd := L.NewUserData()
		kUd.Value = killer
		L.SetMetatable(kUd, L.GetTypeMetatable("Creature"))
		return e.executeCallbacks(EventPlayerOnDeath, pUd, kUd)
	}
	return e.executeCallbacks(EventPlayerOnDeath, pUd, lua.LNil)
}

// ── Dispatchers added for hooks that were being parsed and stored but never
// fired. Signatures come from the datapack scripts in data/scripts/eventcallbacks,
// which are the same argument lists EventCallback:: builds in C++.

// ud wraps a value as Lua userdata of the named metatable, or nil.
//
// The reflect check is load-bearing: a typed nil pointer such as
// (*game.Item)(nil) stored in an `any` is NOT == nil, so a plain comparison would
// wrap it in userdata and Lua would see a non-nil value that panics on first use.
// Several call sites legitimately pass a nil item or player.
func (e *Engine) ud(v any, metatable string) lua.LValue {
	if v == nil || isNilValue(v) {
		return lua.LNil
	}
	u := e.L.NewUserData()
	u.Value = v
	e.L.SetMetatable(u, e.L.GetTypeMetatable(metatable))
	return u
}

// pos wraps a position as Position userdata.
func (e *Engine) pos(p game.Position) lua.LValue {
	u := e.L.NewUserData()
	u.Value = p
	e.L.SetMetatable(u, e.L.GetTypeMetatable("Position"))
	return u
}

// ExecutePlayerOnBrowseField fires playerOnBrowseField(player, position).
func (e *Engine) ExecutePlayerOnBrowseField(player *game.Player, position game.Position) bool {
	return e.executeCallbacks(EventPlayerOnBrowseField,
		e.ud(player, "Player"), e.pos(position))
}

// ExecutePlayerOnLookInShop fires playerOnLookInShop(player, itemType, count).
// itemType is the item id: the Lua side only reads it through ItemType helpers.
func (e *Engine) ExecutePlayerOnLookInShop(player *game.Player, itemID uint16, count uint16) bool {
	return e.executeCallbacks(EventPlayerOnLookInShop,
		e.ud(player, "Player"), lua.LNumber(itemID), lua.LNumber(count))
}

// ExecutePlayerOnLookInTrade fires playerOnLookInTrade(player, partner, item, distance).
func (e *Engine) ExecutePlayerOnLookInTrade(player *game.Player, partner *game.Player, item *game.Item, distance int) bool {
	return e.executeCallbacks(EventPlayerOnLookInTrade,
		e.ud(player, "Player"), e.ud(partner, "Player"), e.ud(item, "Item"), lua.LNumber(distance))
}

// ExecutePlayerOnRotateItem fires playerOnRotateItem(player, item, position).
func (e *Engine) ExecutePlayerOnRotateItem(player *game.Player, item *game.Item, position game.Position) bool {
	return e.executeCallbacks(EventPlayerOnRotateItem,
		e.ud(player, "Player"), e.ud(item, "Item"), e.pos(position))
}

// ExecutePlayerOnRemoveCount fires playerOnRemoveCount(player, item).
func (e *Engine) ExecutePlayerOnRemoveCount(player *game.Player, item *game.Item) bool {
	return e.executeCallbacks(EventPlayerOnRemoveCount,
		e.ud(player, "Player"), e.ud(item, "Item"))
}

// ExecutePlayerOnRequestQuestLog fires playerOnRequestQuestLog(player).
func (e *Engine) ExecutePlayerOnRequestQuestLog(player *game.Player) bool {
	return e.executeCallbacks(EventPlayerOnRequestQuestLog, e.ud(player, "Player"))
}

// ExecutePlayerOnRequestQuestLine fires playerOnRequestQuestLine(player, questId).
func (e *Engine) ExecutePlayerOnRequestQuestLine(player *game.Player, questID uint16) bool {
	return e.executeCallbacks(EventPlayerOnRequestQuestLine,
		e.ud(player, "Player"), lua.LNumber(questID))
}

// ExecutePlayerOnStorageUpdate fires
// playerOnStorageUpdate(player, key, value, oldValue, currentFrameTime).
func (e *Engine) ExecutePlayerOnStorageUpdate(player *game.Player, key uint32, value, oldValue int32, frameTime int64) bool {
	return e.executeCallbacks(EventPlayerOnStorageUpdate,
		e.ud(player, "Player"), lua.LNumber(key), lua.LNumber(value),
		lua.LNumber(oldValue), lua.LNumber(frameTime))
}

// ExecutePlayerOnTradeAccept fires playerOnTradeAccept(player, target, item, targetItem).
func (e *Engine) ExecutePlayerOnTradeAccept(player *game.Player, target *game.Player, item, targetItem *game.Item) bool {
	return e.executeCallbacks(EventPlayerOnTradeAccept,
		e.ud(player, "Player"), e.ud(target, "Player"),
		e.ud(item, "Item"), e.ud(targetItem, "Item"))
}

// ExecutePartyOnDisband fires partyOnDisband(party).
func (e *Engine) ExecutePartyOnDisband(party *game.Party) bool {
	return e.executeCallbacks(EventPartyOnDisband, e.ud(party, "Party"))
}

// ExecuteCreatureOnAreaCombat fires creatureOnAreaCombat(creature, tile, isAggressive).
func (e *Engine) ExecuteCreatureOnAreaCombat(creature game.Creature, tile *game.Tile, aggressive bool) bool {
	return e.executeCallbacks(EventCreatureOnAreaCombat,
		e.ud(creature, "Creature"), e.ud(tile, "Tile"), lua.LBool(aggressive))
}

// ExecuteCreatureOnTargetCombat fires creatureOnTargetCombat(creature, target).
func (e *Engine) ExecuteCreatureOnTargetCombat(creature, target game.Creature) bool {
	return e.executeCallbacks(EventCreatureOnTargetCombat,
		e.ud(creature, "Creature"), e.ud(target, "Creature"))
}

// ExecuteMonsterPostDropLoot fires monsterPostDropLoot(monster, corpse).
//
// Only the POST hook is wired. The main monsterOnDropLoot event generates the loot
// itself in the datapack (data/scripts/eventcallbacks/monster/ondroploot__base.lua
// calls mType:generateLootRoll and corpse:addLoot), and neither generateLootRoll nor
// player:calculateLootFactor is bound in Go yet — dispatching it while Go still
// generates loot inline would double every drop. See Monster::dropLoot
// (monster.cpp:3414), which delegates both.
func (e *Engine) ExecuteMonsterPostDropLoot(monster *game.Monster, corpse *game.Item) bool {
	return e.executeCallbacks(EventMonsterPostDropLoot,
		e.ud(monster, "Monster"), e.container(corpse))
}

// container wraps a corpse through WrapContainer, or nil when unset.
func (e *Engine) container(it *game.Item) lua.LValue {
	if e.WrapContainer == nil || it == nil {
		return lua.LNil
	}
	return e.WrapContainer(it)
}

// isNilValue reports whether v holds a typed nil (pointer, map, slice, interface or
// func), which an `any` comparison against nil misses.
func isNilValue(v any) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}

// ExecuteMonsterOnDropLoot fires monsterOnDropLoot(monster, corpse).
//
// This is where the loot actually comes from: Monster::dropLoot (monster.cpp:3431)
// delegates the whole roll to the callback, and the datapack's base script
// (data/scripts/eventcallbacks/monster/ondroploot__base.lua) calls
// mType:generateLootRoll followed by corpse:addLoot. The corpse is passed as a
// Container so addLoot's self:addItem works on it.
func (e *Engine) ExecuteMonsterOnDropLoot(monster *game.Monster, corpse *game.Item) bool {
	return e.executeCallbacks(EventMonsterOnDropLoot,
		e.ud(monster, "Monster"), e.container(corpse))
}
