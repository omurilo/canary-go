// Package luaengine embeds a Lua 5.1 VM (gopher-lua) and exposes a starter slice
// of the Canary scripting API so existing Lua content can run against the Go
// server. The full ~1300-function surface is ported incrementally.
package luaengine

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/globalevents"
	"github.com/opentibiabr/canary-go/internal/spells"
	lua "github.com/yuin/gopher-lua"
)

// reBackslashZ matches \z followed by optional whitespace (including newlines).
// This is a Lua 5.3 string escape that gopher-lua (5.1) doesn't support.
var reBackslashZ = regexp.MustCompile(`\\z\s*`)

// preprocessLuaSource converts Lua 5.3 \z continuation to Lua 5.1 compatible code.
func preprocessLuaSource(src string) string {
	return reBackslashZ.ReplaceAllString(src, "")
}

// Engine owns a Lua state guarded by a mutex (gopher-lua states are not
// goroutine-safe).
type Engine struct {
	mu       sync.Mutex
	L        *lua.LState
	log      *slog.Logger
	world    *game.World
	database *db.DB

	npcCallbacksMu sync.Mutex
	npcCallbacks   map[string]map[string]*lua.LFunction

	// Scheduled Lua events (addEvent/stopEvent). Guarded by eventMu.
	eventMu  sync.Mutex
	eventSeq int
	events   map[int]*time.Timer

	creatureEventsOnLogin  []*lua.LFunction
	creatureEventsOnLogout []*lua.LFunction

	// GlobalEvents is the scheduling engine for server-wide startup/think/time
	// events. It is created alongside the Lua engine and started after all
	// scripts have loaded.
	GlobalEvents *globalevents.Engine
}

// New creates an engine with the base libraries loaded.
func New(world *game.World, log *slog.Logger) *Engine {
	L := lua.NewState()
	e := &Engine{L: L, log: log, world: world}
	e.registerAPI()
	e.overrideFileLoaders()
	e.registerScheduler()
	e.registerLuaCompat()
	e.overrideMathRandom()

	if world != nil {
		world.OnCastSpell = func(name string, caster game.Creature, target game.Creature) bool {
			sp := spells.FindByName(name)
			if sp == nil {
				return false
			}
			targetID := uint32(0)
			pos := game.Position{}
			vtype := VariantPosition
			if target != nil {
				targetID = target.GetID()
				pos = target.GetPosition()
				vtype = VariantNumber
			} else if caster != nil {
				pos = caster.GetPosition()
				vtype = VariantPosition
			}
			return e.RunSpell(sp, caster, vtype, targetID, pos)
		}
		world.OnTargetTile = func(funcName string, caster game.Creature, pos game.Position) {
			fn := e.L.GetGlobal(funcName)
			if fn.Type() != lua.LTFunction {
				return
			}
			casterUD := e.L.NewUserData()
			casterUD.Value = caster
			if p, ok := caster.(*game.Player); ok && p != nil {
				e.L.SetMetatable(casterUD, e.L.GetTypeMetatable("Player"))
			} else if m, ok := caster.(*game.Monster); ok && m != nil {
				e.L.SetMetatable(casterUD, e.L.GetTypeMetatable("Monster"))
			} else {
				e.L.SetMetatable(casterUD, e.L.GetTypeMetatable("Creature"))
			}
			posUD := e.L.NewUserData()
			posUD.Value = pos
			e.L.SetMetatable(posUD, e.L.GetTypeMetatable("Position"))

			if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, casterUD, posUD); err != nil {
				e.log.Error("OnTargetTile error", "func", funcName, "err", err)
			}
		}
		world.OnTargetCreature = func(funcName string, caster game.Creature, target game.Creature) {
			fn := e.L.GetGlobal(funcName)
			if fn.Type() != lua.LTFunction {
				return
			}
			casterUD := e.L.NewUserData()
			casterUD.Value = caster
			if p, ok := caster.(*game.Player); ok && p != nil {
				e.L.SetMetatable(casterUD, e.L.GetTypeMetatable("Player"))
			} else if m, ok := caster.(*game.Monster); ok && m != nil {
				e.L.SetMetatable(casterUD, e.L.GetTypeMetatable("Monster"))
			} else {
				e.L.SetMetatable(casterUD, e.L.GetTypeMetatable("Creature"))
			}
			targetUD := e.L.NewUserData()
			targetUD.Value = target
			if p, ok := target.(*game.Player); ok && p != nil {
				e.L.SetMetatable(targetUD, e.L.GetTypeMetatable("Player"))
			} else if m, ok := target.(*game.Monster); ok && m != nil {
				e.L.SetMetatable(targetUD, e.L.GetTypeMetatable("Monster"))
			} else {
				e.L.SetMetatable(targetUD, e.L.GetTypeMetatable("Creature"))
			}

			if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, casterUD, targetUD); err != nil {
				e.log.Error("OnTargetCreature error", "func", funcName, "err", err)
			}
		}
	}

	e.GlobalEvents = globalevents.NewEngine(log)
	return e
}

// registerLuaCompat patches standard-library incompatibilities between
// gopher-lua and the Lua 5.1 the datapack targets. Currently: string.gsub
// rejects a numeric replacement, but real Lua coerces it to a string (used by
// NPC message parsing, e.g. |BLESSCOST| → a number). Wrap gsub to coerce arg 3.
func (e *Engine) registerLuaCompat() {
	strTbl, ok := e.L.GetGlobal("string").(*lua.LTable)
	if !ok {
		return
	}
	orig, ok := e.L.GetField(strTbl, "gsub").(*lua.LFunction)
	if !ok {
		return
	}
	e.L.SetField(strTbl, "gsub", e.L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		args := make([]lua.LValue, 0, n)
		for i := 1; i <= n; i++ {
			a := L.Get(i)
			if i == 3 && a.Type() == lua.LTNumber {
				a = lua.LString(a.String())
			}
			args = append(args, a)
		}
		// gsub returns (string, count); forward both.
		if err := L.CallByParam(lua.P{Fn: orig, NRet: 2, Protect: true}, args...); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		return 2
	}))
}

// overrideMathRandom replaces gopher-lua's math.random with a version that
// handles edge cases (max ≤ min) gracefully. The default implementation panics
// via rand.Intn(0) when the range is empty. This matches LuaJIT behaviour of
// returning 0 rather than crashing.
func (e *Engine) overrideMathRandom() {
	mathTbl, ok := e.L.GetGlobal("math").(*lua.LTable)
	if !ok {
		return
	}
	mathTbl.RawSetString("random", e.L.NewFunction(safeRandom))
}

func safeRandom(L *lua.LState) int {
	top := L.GetTop()
	switch top {
	case 0:
		L.Push(lua.LNumber(rand.Float64()))
	case 1:
		n := L.CheckInt(1)
		if n <= 0 {
			L.Push(lua.LNumber(0))
		} else {
			L.Push(lua.LNumber(rand.Intn(n) + 1))
		}
	default:
		min := L.CheckInt(1)
		max := L.CheckInt(2)
		if max < min {
			L.Push(lua.LNumber(0))
		} else {
			L.Push(lua.LNumber(rand.Intn(max-min+1) + min))
		}
	}
	return 1
}

// registerScheduler installs the global addEvent/stopEvent scheduling functions
// (g_dispatcher/g_scheduler in C++). addEvent(callback, delayMs, ...args)
// queues callback(...args) after delayMs and returns a cancellable event id;
// stopEvent(id) cancels it. Used pervasively by NPC dialogue, spells, quests.
func (e *Engine) registerScheduler() {
	e.L.SetGlobal("addEvent", e.L.NewFunction(e.luaAddEvent))
	e.L.SetGlobal("stopEvent", e.L.NewFunction(e.luaStopEvent))
}

func (e *Engine) luaAddEvent(L *lua.LState) int {
	fn, ok := L.Get(1).(*lua.LFunction)
	if !ok {
		L.RaiseError("addEvent: first argument must be a function")
		return 0
	}
	delay := time.Duration(L.CheckInt(2)) * time.Millisecond
	if delay < 0 {
		delay = 0
	}
	// Capture the remaining arguments to forward to the callback.
	var args []lua.LValue
	for i := 3; i <= L.GetTop(); i++ {
		args = append(args, L.Get(i))
	}

	e.eventMu.Lock()
	e.eventSeq++
	id := e.eventSeq
	if e.events == nil {
		e.events = make(map[int]*time.Timer)
	}
	e.events[id] = time.AfterFunc(delay, func() {
		e.eventMu.Lock()
		delete(e.events, id)
		e.eventMu.Unlock()

		e.mu.Lock()
		defer e.mu.Unlock()
		if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, args...); err != nil {
			e.log.Error("addEvent callback", "err", err)
		}
	})
	e.eventMu.Unlock()

	L.Push(lua.LNumber(id))
	return 1
}

func (e *Engine) luaStopEvent(L *lua.LState) int {
	id := L.OptInt(1, 0)
	e.eventMu.Lock()
	if t, ok := e.events[id]; ok {
		t.Stop()
		delete(e.events, id)
	}
	e.eventMu.Unlock()
	return 0
}

// overrideFileLoaders replaces the builtin dofile/loadfile so nested dofile()
// chains (e.g. data/npclib/load.lua → npc_system/modules.lua) get the same
// \z-continuation preprocessing as top-level DoFile loads. gopher-lua's builtin
// dofile skips it and chokes on Lua 5.3 string continuations ("unterminated
// string"), which aborts the whole ordered load chain. These run while the
// engine lock is already held by the outer DoFile/DoString, so they must NOT
// re-lock.
func (e *Engine) overrideFileLoaders() {
	load := func(L *lua.LState, path string) (*lua.LFunction, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return L.Load(strings.NewReader(preprocessLuaSource(string(data))), path)
	}
	e.L.SetGlobal("dofile", e.L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		fn, err := load(L, path)
		if err != nil {
			// Compile error: log and continue rather than aborting the caller.
			// The datapack bootstrap is one long dofile chain (global.lua →
			// lib.lua → ...); a hard abort on one un-ported file would drop every
			// later definition (e.g. global.lua's IsTravelFree, defined after its
			// own dofiles). Resilience beats fidelity during the incremental port.
			e.log.Warn("dofile compile error", "file", path, "err", err)
			return 0
		}
		top := L.GetTop()
		L.Push(fn)
		if perr := L.PCall(0, lua.MultRet, nil); perr != nil {
			// Runtime error inside the loaded chunk: log and continue so the
			// parent chunk keeps executing (nested require/dofile failures in
			// not-yet-ported libs must not cascade).
			e.log.Warn("dofile runtime error", "file", path, "err", perr)
			return 0
		}
		return L.GetTop() - top
	}))
	e.L.SetGlobal("loadfile", e.L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		fn, err := load(L, path)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(fn)
		return 1
	}))
}

// Close releases the Lua state.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.GlobalEvents != nil {
		e.GlobalEvents.Stop()
	}
	e.L.Close()
}

// StartGlobalEventScheduler begins the background tick loop for think and time
// global events. Must be called after all Lua scripts have been loaded and
// RunStartupGlobalEvents has completed.
func (e *Engine) StartGlobalEventScheduler(ctx context.Context) {
	if e.GlobalEvents != nil {
		e.GlobalEvents.Start(ctx)
	}
}

// DoFile executes a Lua script file under the engine lock.
// It preprocesses \z continuation sequences (Lua 5.3) that gopher-lua (5.1)
// doesn't support, and names the chunk after its path so runtime tracebacks
// point at the real file:line instead of an opaque "<string>".
func (e *Engine) DoFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	src := preprocessLuaSource(string(data))
	e.mu.Lock()
	defer e.mu.Unlock()
	fn, err := e.L.Load(strings.NewReader(src), path)
	if err != nil {
		return err
	}
	e.L.Push(fn)
	return e.L.PCall(0, lua.MultRet, nil)
}

// DoString executes a Lua chunk under the engine lock.
func (e *Engine) DoString(src string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.L.DoString(src)
}

// Call invokes a global Lua function by name with string args (best-effort;
// used by event hooks). Missing functions are ignored.
func (e *Engine) Call(fn string, args ...string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	v := e.L.GetGlobal(fn)
	if v.Type() != lua.LTFunction {
		return nil
	}
	largs := make([]lua.LValue, len(args))
	for i, a := range args {
		largs[i] = lua.LString(a)
	}
	return e.L.CallByParam(lua.P{Fn: v, NRet: 0, Protect: true}, largs...)
}

// Execute runs the given function under the engine lock with access to the Lua state.
func (e *Engine) Execute(fn func(L *lua.LState)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn(e.L)
}

// CallEvent calls a method on a Lua class/metatable (e.g., "Player", "onLook"),
// passing the caller (self) and args. Returns true if the event allowed the action.
func (e *Engine) CallEvent(className, methodName string, self lua.LValue, args ...lua.LValue) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	L := e.L
	// Usually the class is exposed globally or we can get it from the TypeMetatable
	class := L.GetTypeMetatable(className)
	if class == lua.LNil {
		// Fallback: try global if not registered as a type metatable
		class = L.GetGlobal(className)
	}
	if class == lua.LNil || class.Type() != lua.LTTable {
		return true, nil // No class/metatable found, allow by default
	}

	index := L.GetField(class, "__index")
	var method lua.LValue
	if index != lua.LNil && index.Type() == lua.LTTable {
		method = L.GetField(index, methodName)
	} else {
		method = L.GetField(class, methodName)
	}

	if method.Type() != lua.LTFunction {
		return true, nil // No such method, allow by default
	}

	largs := make([]lua.LValue, 0, len(args)+1)
	largs = append(largs, self)
	largs = append(largs, args...)

	err := L.CallByParam(lua.P{Fn: method, NRet: 1, Protect: true}, largs...)
	if err != nil {
		e.log.Error("lua event error", "class", className, "method", methodName, "err", err)
		return false, err
	}

	ret := L.Get(-1)
	L.Pop(1)
	if lua.LVIsFalse(ret) {
		return false, nil
	}
	return true, nil
}
