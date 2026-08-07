package luaengine

import (
	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const luaCreatureEventTypeName = "CreatureEvent"

type LuaCreatureEvent struct {
	Name          string
	OnLogin       *lua.LFunction
	OnLogout      *lua.LFunction
	OnModalWindow *lua.LFunction
	OnDeath       *lua.LFunction
}

// registerCreatureEvent registers the CreatureEvent global constructor and metatable
func (e *Engine) registerCreatureEvent() {
	mt := e.L.NewTypeMetatable(luaCreatureEventTypeName)
	methods := map[string]lua.LGFunction{
		"register": e.creatureEventRegister,
		"type":     creatureEventType,
		"onLogin": func(L *lua.LState) int {
			ev := checkCreatureEvent(L)
			if fn, ok := L.Get(2).(*lua.LFunction); ok {
				ev.OnLogin = fn
			}
			return 0
		},
		"onLogout": func(L *lua.LState) int {
			ev := checkCreatureEvent(L)
			if fn, ok := L.Get(2).(*lua.LFunction); ok {
				ev.OnLogout = fn
			}
			return 0
		},
		"onThink":        func(L *lua.LState) int { return 0 },
		"onPrepareDeath": func(L *lua.LState) int { return 0 },
		// onDeath was a no-op, so data/scripts/creaturescripts/player/death.lua
		// registered its handler into nothing and player_deaths was never written —
		// the death list stayed empty however many times a character died.
		"onDeath": func(L *lua.LState) int {
			ev := checkCreatureEvent(L)
			if fn, ok := L.Get(2).(*lua.LFunction); ok {
				ev.OnDeath = fn
			}
			return 0
		},
		"onKill":    func(L *lua.LState) int { return 0 },
		"onAdvance": func(L *lua.LState) int { return 0 },
		"onModalWindow": func(L *lua.LState) int {
			ev := checkCreatureEvent(L)
			if fn, ok := L.Get(2).(*lua.LFunction); ok {
				ev.OnModalWindow = fn
			}
			return 0
		},
		"onTextEdit":       func(L *lua.LState) int { return 0 },
		"onHealthChange":   func(L *lua.LState) int { return 0 },
		"onManaChange":     func(L *lua.LState) int { return 0 },
		"onExtendedOpcode": func(L *lua.LState) int { return 0 },
	}
	e.setClassConstructor("CreatureEvent", creatureEventConstructor, methods)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), methods))
	e.L.SetField(mt, "__newindex", e.L.NewFunction(creatureEventNewIndex))
}

func creatureEventConstructor(L *lua.LState) int {
	name := L.CheckString(2) // Arg 1 is the class table, arg 2 is the string name
	ev := &LuaCreatureEvent{
		Name: name,
	}
	ud := L.NewUserData()
	ud.Value = ev
	L.SetMetatable(ud, L.GetTypeMetatable(luaCreatureEventTypeName))
	L.Push(ud)
	return 1
}

func checkCreatureEvent(L *lua.LState) *LuaCreatureEvent {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*LuaCreatureEvent); ok {
		return v
	}
	L.ArgError(1, "CreatureEvent expected")
	return nil
}

func creatureEventNewIndex(L *lua.LState) int {
	ev := checkCreatureEvent(L)
	key := L.CheckString(2)
	val := L.Get(3)

	if key == "onLogin" {
		if fn, ok := val.(*lua.LFunction); ok {
			ev.OnLogin = fn
		}
	} else if key == "onLogout" {
		if fn, ok := val.(*lua.LFunction); ok {
			ev.OnLogout = fn
		}
	} else if key == "onDeath" {
		// The datapack installs this by assignment
		// (`function playerDeath.onDeath(...)`, death.lua:181), not by calling the
		// method, so __newindex is the path that actually matters.
		if fn, ok := val.(*lua.LFunction); ok {
			ev.OnDeath = fn
		}
	} else if key == "onModalWindow" {
		if fn, ok := val.(*lua.LFunction); ok {
			ev.OnModalWindow = fn
		}
	}
	return 0
}

func creatureEventType(L *lua.LState) int {
	L.Push(L.Get(1))
	return 1
}

// creatureEventRegister publishes a CreatureEvent under its name so a creature
// can later bind to it via registerEvent(name). The login/logout/modalWindow
// callbacks keep the old global dispatch (every registered event fires for every
// player); onDeath is deliberately NOT appended to a global slice — it resolves
// per creature at death time, by the names the creature registered
// (creaturescripts/player/death.lua + the monster's `monster.events`).
func (e *Engine) creatureEventRegister(L *lua.LState) int {
	ev := checkCreatureEvent(L)
	if ev.Name != "" {
		e.creatureEventsByName[ev.Name] = ev
	}
	if ev.OnLogin != nil {
		e.creatureEventsOnLogin = append(e.creatureEventsOnLogin, ev.OnLogin)
	}
	if ev.OnLogout != nil {
		e.creatureEventsOnLogout = append(e.creatureEventsOnLogout, ev.OnLogout)
	}
	if ev.OnModalWindow != nil {
		e.creatureEventsOnModalWindow = append(e.creatureEventsOnModalWindow, ev.OnModalWindow)
	}
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) ExecuteCreatureOnLogin(player *game.Player) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, fn := range e.creatureEventsOnLogin {
		pUd := e.L.NewUserData()
		pUd.Value = player
		e.L.SetMetatable(pUd, e.L.GetTypeMetatable("Player"))

		e.L.Push(fn)
		e.L.Push(pUd)

		if err := e.L.PCall(1, 1, nil); err != nil {
			e.log.Warn("Error executing CreatureEvent onLogin", "err", err)
			continue
		}

		ret := e.L.Get(-1)
		e.L.Pop(1)

		if luaBool, ok := ret.(lua.LBool); ok {
			if !bool(luaBool) {
				return false
			}
		}
	}
	return true
}

func (e *Engine) ExecuteCreatureOnLogout(player *game.Player) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, fn := range e.creatureEventsOnLogout {
		pUd := e.L.NewUserData()
		pUd.Value = player
		e.L.SetMetatable(pUd, e.L.GetTypeMetatable("Player"))

		e.L.Push(fn)
		e.L.Push(pUd)

		if err := e.L.PCall(1, 1, nil); err != nil {
			e.log.Warn("Error executing CreatureEvent onLogout", "err", err)
			continue
		}

		ret := e.L.Get(-1)
		e.L.Pop(1)

		if luaBool, ok := ret.(lua.LBool); ok {
			if !bool(luaBool) {
				return false
			}
		}
	}
	return true
}

// ExecuteCreatureOnModalWindow fires all registered creature-event onModalWindow
// callbacks. Each receives (player, modalWindowId, buttonId, choiceId).
func (e *Engine) ExecuteCreatureOnModalWindow(player *game.Player, modalWindowID uint32, buttonID uint8, choiceID uint8) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, fn := range e.creatureEventsOnModalWindow {
		pUd := e.L.NewUserData()
		pUd.Value = player
		e.L.SetMetatable(pUd, e.L.GetTypeMetatable("Player"))

		e.L.Push(fn)
		e.L.Push(pUd)
		e.L.Push(lua.LNumber(modalWindowID))
		e.L.Push(lua.LNumber(buttonID))
		e.L.Push(lua.LNumber(choiceID))

		if err := e.L.PCall(5, 0, nil); err != nil {
			e.log.Warn("Error executing CreatureEvent onModalWindow", "err", err)
		}
	}
}

// ExecuteCreatureOnDeath runs the onDeath handlers the player registered
// (login.lua's player:registerEvent). The datapack's signature is
// onDeath(player, corpse, killer, mostDamageKiller, unjustified,
// mostDamageUnjustified) — death.lua uses every argument to build the
// player_deaths row, so passing fewer would write a row with the wrong killer.
func (e *Engine) ExecuteCreatureOnDeath(player *game.Player, corpse *game.Item, killer, mostDamageKiller game.Creature, unjustified, mostDamageUnjustified bool) bool {
	return e.executeCreatureOnDeath(game.EventRegistrarOf(player), player, corpse, killer, mostDamageKiller, unjustified, mostDamageUnjustified)
}

// ExecuteMonsterOnDeath runs the onDeath handlers bound to this monster — the
// ones named in its type's `monster.events`, copied onto the instance at spawn.
// Quest bosses created by Game.createMonster after the creature's death bind
// their handler to the boss itself, so they fire here (at the death tile) and
// not for every player death at the temple.
func (e *Engine) ExecuteMonsterOnDeath(m *game.Monster, corpse *game.Item, killer, mostDamageKiller game.Creature, unjustified, mostDamageUnjustified bool) bool {
	return e.executeCreatureOnDeath(game.EventRegistrarOf(m), m, corpse, killer, mostDamageKiller, unjustified, mostDamageUnjustified)
}

// executeCreatureOnDeath is the shared per-creature death dispatch: it looks up
// each event name the creature holds in the registry and runs that event's
// onDeath, rather than running every onDeath in the system. The creature is
// bound as the most specific metatable (Player/Monster/Npc) via
// pushCreatureAs(_, "Creature"). A handler returning false vetoes the death
// (aggregated), mirroring the C++ onPrepareDeath semantics.
//
// corpse is passed as nil when the caller has none to hand: the script only
// reads it for the description, and Lua guards it.
func (e *Engine) executeCreatureOnDeath(reg game.EventRegistrar, c game.Creature, corpse *game.Item, killer, mostDamageKiller game.Creature, unjustified, mostDamageUnjustified bool) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := true
	if reg == nil {
		return result
	}
	for _, name := range reg.RegisteredEvents() {
		ev, ok := e.creatureEventsByName[name]
		if !ok || ev.OnDeath == nil {
			continue
		}
		e.L.Push(ev.OnDeath)

		if !e.pushCreatureAs(e.L, c, "Creature") {
			e.L.Push(lua.LNil)
		}

		if corpse != nil {
			e.pushItem(e.L, corpse)
		} else {
			e.L.Push(lua.LNil)
		}
		e.pushCreatureOrNil(killer)
		e.pushCreatureOrNil(mostDamageKiller)
		e.L.Push(lua.LBool(unjustified))
		e.L.Push(lua.LBool(mostDamageUnjustified))

		if err := e.L.PCall(6, 1, nil); err != nil {
			e.log.Warn("Error executing CreatureEvent onDeath", "event", name, "err", err)
			continue
		}
		ret := e.L.Get(-1)
		e.L.Pop(1)
		if luaBool, ok := ret.(lua.LBool); ok && !bool(luaBool) {
			result = false
		}
	}
	return result
}

// pushCreatureOrNil pushes a creature userdata, or nil for an absent killer (a
// player who drowned has no killer at all).
func (e *Engine) pushCreatureOrNil(c game.Creature) {
	if c == nil {
		e.L.Push(lua.LNil)
		return
	}
	e.pushCreature(e.L, c)
}
