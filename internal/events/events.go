package events

import (
	"log/slog"
	"reflect"
	"sync"

	"github.com/omurilo/canary-go/internal/game"
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
	EventPartyOnJoin            EventCallbackType = "partyOnJoin"
	EventPartyOnLeave           EventCallbackType = "partyOnLeave"

	// The remaining EventCallback_t entries, completing the 43 upstream hooks
	// (src/lua/callbacks/callbacks_definitions.hpp:21). Return types are the ones
	// documented in data/scripts/eventcallbacks/README.md: a (bool) hook can veto
	// the action, a (void) hook cannot.
	EventCreatureOnChangeOutfit EventCallbackType = "creatureOnChangeOutfit"
	EventCreatureOnCombat       EventCallbackType = "creatureOnCombat"
	EventCreatureOnDrainHealth  EventCallbackType = "creatureOnDrainHealth"

	EventPlayerOnLookInBattleList    EventCallbackType = "playerOnLookInBattleList"
	EventPlayerOnItemMoved           EventCallbackType = "playerOnItemMoved"
	EventPlayerOnChangeZone          EventCallbackType = "playerOnChangeZone"
	EventPlayerOnChangeHazard        EventCallbackType = "playerOnChangeHazard"
	EventPlayerOnMoveCreature        EventCallbackType = "playerOnMoveCreature"
	EventPlayerOnReportRuleViolation EventCallbackType = "playerOnReportRuleViolation"
	EventPlayerOnReportBug           EventCallbackType = "playerOnReportBug"
	EventPlayerOnTurn                EventCallbackType = "playerOnTurn"
	EventPlayerOnTradeRequest        EventCallbackType = "playerOnTradeRequest"
	EventPlayerOnLoseExperience      EventCallbackType = "playerOnLoseExperience"
	EventPlayerOnGainSkillTries      EventCallbackType = "playerOnGainSkillTries"
	EventPlayerOnCombat              EventCallbackType = "playerOnCombat"
	EventPlayerOnInventoryUpdate     EventCallbackType = "playerOnInventoryUpdate"
	EventPlayerOnWalk                EventCallbackType = "playerOnWalk"
	EventPlayerOnThink               EventCallbackType = "playerOnThink"

	EventZoneBeforeCreatureEnter EventCallbackType = "zoneBeforeCreatureEnter"
	EventZoneBeforeCreatureLeave EventCallbackType = "zoneBeforeCreatureLeave"
	EventZoneAfterCreatureEnter  EventCallbackType = "zoneAfterCreatureEnter"
	EventZoneAfterCreatureLeave  EventCallbackType = "zoneAfterCreatureLeave"

	EventMapOnLoad EventCallbackType = "mapOnLoad"
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
	{"partyOnJoin", EventPartyOnJoin},
	{"partyOnLeave", EventPartyOnLeave},
	{"creatureOnChangeOutfit", EventCreatureOnChangeOutfit},
	{"creatureOnCombat", EventCreatureOnCombat},
	{"creatureOnDrainHealth", EventCreatureOnDrainHealth},
	{"playerOnLookInBattleList", EventPlayerOnLookInBattleList},
	{"playerOnItemMoved", EventPlayerOnItemMoved},
	{"playerOnChangeZone", EventPlayerOnChangeZone},
	{"playerOnChangeHazard", EventPlayerOnChangeHazard},
	{"playerOnMoveCreature", EventPlayerOnMoveCreature},
	{"playerOnReportRuleViolation", EventPlayerOnReportRuleViolation},
	{"playerOnReportBug", EventPlayerOnReportBug},
	{"playerOnTurn", EventPlayerOnTurn},
	{"playerOnTradeRequest", EventPlayerOnTradeRequest},
	{"playerOnGainExperience", EventPlayerOnGainExperience},
	{"playerOnLoseExperience", EventPlayerOnLoseExperience},
	{"playerOnGainSkillTries", EventPlayerOnGainSkillTries},
	{"playerOnCombat", EventPlayerOnCombat},
	{"playerOnInventoryUpdate", EventPlayerOnInventoryUpdate},
	{"playerOnWalk", EventPlayerOnWalk},
	{"playerOnThink", EventPlayerOnThink},
	{"playerOnDeath", EventPlayerOnDeath},
	{"zoneBeforeCreatureEnter", EventZoneBeforeCreatureEnter},
	{"zoneBeforeCreatureLeave", EventZoneBeforeCreatureLeave},
	{"zoneAfterCreatureEnter", EventZoneAfterCreatureEnter},
	{"zoneAfterCreatureLeave", EventZoneAfterCreatureLeave},
	{"mapOnLoad", EventMapOnLoad},
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

// ── Dispatchers completing the 43 upstream hooks.
//
// (bool) hooks return whether the action may proceed; (void) hooks ignore the
// result and are typed bool only for uniformity with executeCallbacks. Argument
// lists follow data/scripts/eventcallbacks/README.md.

// --- Creature ---

// ExecuteCreatureOnChangeOutfit fires creatureOnChangeOutfit(creature, outfit). (bool)
func (e *Engine) ExecuteCreatureOnChangeOutfit(creature game.Creature, outfit game.Outfit) bool {
	tbl := e.L.NewTable()
	e.L.SetField(tbl, "lookType", lua.LNumber(outfit.LookType))
	e.L.SetField(tbl, "lookHead", lua.LNumber(outfit.Head))
	e.L.SetField(tbl, "lookBody", lua.LNumber(outfit.Body))
	e.L.SetField(tbl, "lookLegs", lua.LNumber(outfit.Legs))
	e.L.SetField(tbl, "lookFeet", lua.LNumber(outfit.Feet))
	e.L.SetField(tbl, "lookAddons", lua.LNumber(outfit.Addons))
	e.L.SetField(tbl, "lookMount", lua.LNumber(outfit.LookMount))
	return e.executeCallbacks(EventCreatureOnChangeOutfit, e.ud(creature, "Creature"), tbl)
}

// ExecuteCreatureOnCombat fires creatureOnCombat(attacker, target, damage). (void)
func (e *Engine) ExecuteCreatureOnCombat(attacker, target game.Creature, damage int32) bool {
	return e.executeCallbacks(EventCreatureOnCombat,
		e.ud(attacker, "Creature"), e.ud(target, "Creature"), lua.LNumber(damage))
}

// ExecuteCreatureOnDrainHealth fires
// creatureOnDrainHealth(creature, attacker, typePrimary, damagePrimary, typeSecondary,
// damageSecondary, effectPrimary, effectSecondary). (void)
func (e *Engine) ExecuteCreatureOnDrainHealth(creature, attacker game.Creature, typePrimary uint8, damagePrimary int32, typeSecondary uint8, damageSecondary int32, effectPrimary, effectSecondary uint8) bool {
	return e.executeCallbacks(EventCreatureOnDrainHealth,
		e.ud(creature, "Creature"), e.ud(attacker, "Creature"),
		lua.LNumber(typePrimary), lua.LNumber(damagePrimary),
		lua.LNumber(typeSecondary), lua.LNumber(damageSecondary),
		lua.LNumber(effectPrimary), lua.LNumber(effectSecondary))
}

// --- Party ---

// ExecutePartyOnJoin fires partyOnJoin(party, player). (bool)
func (e *Engine) ExecutePartyOnJoin(party *game.Party, player *game.Player) bool {
	return e.executeCallbacks(EventPartyOnJoin, e.ud(party, "Party"), e.ud(player, "Player"))
}

// ExecutePartyOnLeave fires partyOnLeave(party, player). (bool)
func (e *Engine) ExecutePartyOnLeave(party *game.Party, player *game.Player) bool {
	return e.executeCallbacks(EventPartyOnLeave, e.ud(party, "Party"), e.ud(player, "Player"))
}

// ExecutePartyOnShareExperience fires partyOnShareExperience(party, exp) and returns
// the (possibly modified) experience. (void upstream, but the value is read back.)
func (e *Engine) ExecutePartyOnShareExperience(party *game.Party, exp uint64) uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	fns := e.callbacks[EventPartyOnShareExperience]
	if len(fns) == 0 {
		return exp
	}
	final := exp
	for _, fn := range fns {
		e.L.Push(fn)
		e.L.Push(e.ud(party, "Party"))
		e.L.Push(lua.LNumber(final))
		if err := e.L.PCall(2, 1, nil); err != nil {
			e.log.Error("event callback error", "type", string(EventPartyOnShareExperience), "err", err)
			continue
		}
		ret := e.L.Get(-1)
		e.L.Pop(1)
		if num, ok := ret.(lua.LNumber); ok {
			final = uint64(num)
		}
	}
	return final
}

// --- Player ---

// ExecutePlayerOnLookInBattleList fires playerOnLookInBattleList(player, creature, distance). (void)
func (e *Engine) ExecutePlayerOnLookInBattleList(player *game.Player, creature game.Creature, distance int) bool {
	return e.executeCallbacks(EventPlayerOnLookInBattleList,
		e.ud(player, "Player"), e.ud(creature, "Creature"), lua.LNumber(distance))
}

// ExecutePlayerOnItemMoved fires
// playerOnItemMoved(player, item, count, fromPosition, toPosition, fromCylinder, toCylinder). (void)
//
// The two cylinders are passed as nil: Go has no Cylinder userdata type.
func (e *Engine) ExecutePlayerOnItemMoved(player *game.Player, item *game.Item, count uint16, fromPos, toPos game.Position) bool {
	return e.executeCallbacks(EventPlayerOnItemMoved,
		e.ud(player, "Player"), e.ud(item, "Item"), lua.LNumber(count),
		e.pos(fromPos), e.pos(toPos), lua.LNil, lua.LNil)
}

// ExecutePlayerOnChangeZone fires playerOnChangeZone(player, zone). (void)
func (e *Engine) ExecutePlayerOnChangeZone(player *game.Player, zone uint8) bool {
	return e.executeCallbacks(EventPlayerOnChangeZone,
		e.ud(player, "Player"), lua.LNumber(zone))
}

// ExecutePlayerOnChangeHazard fires playerOnChangeHazard(player, isHazard). (void)
func (e *Engine) ExecutePlayerOnChangeHazard(player *game.Player, isHazard bool) bool {
	return e.executeCallbacks(EventPlayerOnChangeHazard,
		e.ud(player, "Player"), lua.LBool(isHazard))
}

// ExecutePlayerOnMoveCreature fires
// playerOnMoveCreature(player, creature, fromPosition, toPosition). (bool)
func (e *Engine) ExecutePlayerOnMoveCreature(player *game.Player, creature game.Creature, fromPos, toPos game.Position) bool {
	return e.executeCallbacks(EventPlayerOnMoveCreature,
		e.ud(player, "Player"), e.ud(creature, "Creature"), e.pos(fromPos), e.pos(toPos))
}

// ExecutePlayerOnReportRuleViolation fires
// playerOnReportRuleViolation(player, targetName, reportType, reportReason, comment, translation). (void)
func (e *Engine) ExecutePlayerOnReportRuleViolation(player *game.Player, targetName string, reportType, reportReason uint8, comment, translation string) bool {
	return e.executeCallbacks(EventPlayerOnReportRuleViolation,
		e.ud(player, "Player"), lua.LString(targetName),
		lua.LNumber(reportType), lua.LNumber(reportReason),
		lua.LString(comment), lua.LString(translation))
}

// ExecutePlayerOnReportBug fires playerOnReportBug(player, message, position, category). (void)
func (e *Engine) ExecutePlayerOnReportBug(player *game.Player, message string, position game.Position, category uint8) bool {
	return e.executeCallbacks(EventPlayerOnReportBug,
		e.ud(player, "Player"), lua.LString(message), e.pos(position), lua.LNumber(category))
}

// ExecutePlayerOnTurn fires playerOnTurn(player, direction). (bool)
func (e *Engine) ExecutePlayerOnTurn(player *game.Player, direction uint8) bool {
	return e.executeCallbacks(EventPlayerOnTurn,
		e.ud(player, "Player"), lua.LNumber(direction))
}

// ExecutePlayerOnTradeRequest fires playerOnTradeRequest(player, target, item). (bool)
func (e *Engine) ExecutePlayerOnTradeRequest(player, target *game.Player, item *game.Item) bool {
	return e.executeCallbacks(EventPlayerOnTradeRequest,
		e.ud(player, "Player"), e.ud(target, "Player"), e.ud(item, "Item"))
}

// ExecutePlayerOnLoseExperience fires playerOnLoseExperience(player, exp) and returns
// the (possibly modified) amount lost.
func (e *Engine) ExecutePlayerOnLoseExperience(player *game.Player, exp uint64) uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	fns := e.callbacks[EventPlayerOnLoseExperience]
	if len(fns) == 0 {
		return exp
	}
	final := exp
	for _, fn := range fns {
		e.L.Push(fn)
		e.L.Push(e.ud(player, "Player"))
		e.L.Push(lua.LNumber(final))
		if err := e.L.PCall(2, 1, nil); err != nil {
			e.log.Error("event callback error", "type", string(EventPlayerOnLoseExperience), "err", err)
			continue
		}
		ret := e.L.Get(-1)
		e.L.Pop(1)
		if num, ok := ret.(lua.LNumber); ok {
			final = uint64(num)
		}
	}
	return final
}

// ExecutePlayerOnGainSkillTries fires playerOnGainSkillTries(player, skill, tries) and
// returns the (possibly modified) tries.
func (e *Engine) ExecutePlayerOnGainSkillTries(player *game.Player, skill uint8, tries uint64) uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	fns := e.callbacks[EventPlayerOnGainSkillTries]
	if len(fns) == 0 {
		return tries
	}
	final := tries
	for _, fn := range fns {
		e.L.Push(fn)
		e.L.Push(e.ud(player, "Player"))
		e.L.Push(lua.LNumber(skill))
		e.L.Push(lua.LNumber(final))
		if err := e.L.PCall(3, 1, nil); err != nil {
			e.log.Error("event callback error", "type", string(EventPlayerOnGainSkillTries), "err", err)
			continue
		}
		ret := e.L.Get(-1)
		e.L.Pop(1)
		if num, ok := ret.(lua.LNumber); ok {
			final = uint64(num)
		}
	}
	return final
}

// ExecutePlayerOnCombat fires playerOnCombat(player, target, item, damage). (void)
func (e *Engine) ExecutePlayerOnCombat(player *game.Player, target game.Creature, item *game.Item, damage int32) bool {
	return e.executeCallbacks(EventPlayerOnCombat,
		e.ud(player, "Player"), e.ud(target, "Creature"), e.ud(item, "Item"), lua.LNumber(damage))
}

// ExecutePlayerOnInventoryUpdate fires
// playerOnInventoryUpdate(player, item, slot, equip). (void)
func (e *Engine) ExecutePlayerOnInventoryUpdate(player *game.Player, item *game.Item, slot int, equip bool) bool {
	return e.executeCallbacks(EventPlayerOnInventoryUpdate,
		e.ud(player, "Player"), e.ud(item, "Item"), lua.LNumber(slot), lua.LBool(equip))
}

// ExecutePlayerOnWalk fires playerOnWalk(player, direction). (void)
func (e *Engine) ExecutePlayerOnWalk(player *game.Player, direction uint8) bool {
	return e.executeCallbacks(EventPlayerOnWalk,
		e.ud(player, "Player"), lua.LNumber(direction))
}

// ExecutePlayerOnThink fires playerOnThink(player, interval). (void)
func (e *Engine) ExecutePlayerOnThink(player *game.Player, interval uint32) bool {
	return e.executeCallbacks(EventPlayerOnThink,
		e.ud(player, "Player"), lua.LNumber(interval))
}

// --- Zone ---
//
// Zones do not exist in Go yet (the OTBM parser discards the zone ids), so these
// take the numeric zone id the map would carry. They are dispatched from nowhere
// until the zone system lands.

// ExecuteZoneBeforeCreatureEnter fires zoneBeforeCreatureEnter(zone, creature). (bool)
func (e *Engine) ExecuteZoneBeforeCreatureEnter(zone uint16, creature game.Creature) bool {
	return e.executeCallbacks(EventZoneBeforeCreatureEnter,
		lua.LNumber(zone), e.ud(creature, "Creature"))
}

// ExecuteZoneBeforeCreatureLeave fires zoneBeforeCreatureLeave(zone, creature). (bool)
func (e *Engine) ExecuteZoneBeforeCreatureLeave(zone uint16, creature game.Creature) bool {
	return e.executeCallbacks(EventZoneBeforeCreatureLeave,
		lua.LNumber(zone), e.ud(creature, "Creature"))
}

// ExecuteZoneAfterCreatureEnter fires zoneAfterCreatureEnter(zone, creature). (void)
func (e *Engine) ExecuteZoneAfterCreatureEnter(zone uint16, creature game.Creature) bool {
	return e.executeCallbacks(EventZoneAfterCreatureEnter,
		lua.LNumber(zone), e.ud(creature, "Creature"))
}

// ExecuteZoneAfterCreatureLeave fires zoneAfterCreatureLeave(zone, creature). (void)
func (e *Engine) ExecuteZoneAfterCreatureLeave(zone uint16, creature game.Creature) bool {
	return e.executeCallbacks(EventZoneAfterCreatureLeave,
		lua.LNumber(zone), e.ud(creature, "Creature"))
}

// --- Map ---

// ExecuteMapOnLoad fires mapOnLoad(mapPath). (void)
func (e *Engine) ExecuteMapOnLoad(mapPath string) bool {
	return e.executeCallbacks(EventMapOnLoad, lua.LString(mapPath))
}
