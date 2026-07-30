package events

import (
	"log/slog"
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
