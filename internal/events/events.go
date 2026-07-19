package events

import (
	"log/slog"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/luaengine"
	lua "github.com/yuin/gopher-lua"
)

// Engine bridges the server and the Lua Events system.
type Engine struct {
	lua *luaengine.Engine
	log *slog.Logger
}

// New creates a new events engine.
func New(luaEngine *luaengine.Engine, log *slog.Logger) *Engine {
	return &Engine{
		lua: luaEngine,
		log: log,
	}
}

// pushThing pushes a game object (Player, Item, etc.) as userdata.
func (e *Engine) pushThing(L *lua.LState, thing interface{}) lua.LValue {
	if thing == nil {
		return lua.LNil
	}
	ud := L.NewUserData()
	ud.Value = thing
	switch thing.(type) {
	case *game.Item:
		L.SetMetatable(ud, L.GetTypeMetatable("Item"))
	case *game.Player:
		L.SetMetatable(ud, L.GetTypeMetatable("Player"))
	// Add other creature types like Monster, Npc if they exist in game package
	default:
		L.SetMetatable(ud, L.GetTypeMetatable("Creature"))
	}
	return ud
}

// pushPosition pushes a position instance.
func (e *Engine) pushPosition(L *lua.LState, pos game.Position) lua.LValue {
	ud := L.NewUserData()
	ud.Value = pos
	L.SetMetatable(ud, L.GetTypeMetatable("Position"))
	return ud
}

// OnLook triggers the Player:onLook(thing, position, distance) event.
func (e *Engine) OnLook(player *game.Player, thing interface{}, position game.Position, distance int32) {
	var lPlayer, lThing, lPos lua.LValue
	e.lua.Execute(func(L *lua.LState) {
		lPlayer = e.pushThing(L, player)
		lThing = e.pushThing(L, thing)
		lPos = e.pushPosition(L, position)
	})

	_, _ = e.lua.CallEvent("Player", "onLook", lPlayer, lThing, lPos, lua.LNumber(distance))
}

// OnMoveItem triggers the Player:onMoveItem(item, count, fromPosition, toPosition) event.
// Returns true if the item move is allowed by Lua events.
func (e *Engine) OnMoveItem(player *game.Player, item *game.Item, count uint16, fromPosition, toPosition game.Position) bool {
	var lPlayer, lItem, lFrom, lTo lua.LValue
	e.lua.Execute(func(L *lua.LState) {
		lPlayer = e.pushThing(L, player)
		lItem = e.pushThing(L, item)
		lFrom = e.pushPosition(L, fromPosition)
		lTo = e.pushPosition(L, toPosition)
	})

	allowed, err := e.lua.CallEvent("Player", "onMoveItem", lPlayer, lItem, lua.LNumber(count), lFrom, lTo)
	if err != nil {
		return false
	}
	return allowed
}

// OnGainExperience triggers the Player:onGainExperience(source, exp, rawExp) event.
func (e *Engine) OnGainExperience(player *game.Player, source interface{}, exp uint64, rawExp uint64) bool {
	var lPlayer, lSource lua.LValue
	e.lua.Execute(func(L *lua.LState) {
		lPlayer = e.pushThing(L, player)
		lSource = e.pushThing(L, source)
	})

	allowed, err := e.lua.CallEvent("Player", "onGainExperience", lPlayer, lSource, lua.LNumber(exp), lua.LNumber(rawExp))
	if err != nil {
		return false
	}
	return allowed
}
