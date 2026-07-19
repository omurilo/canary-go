// Package luaengine embeds a Lua 5.1 VM (gopher-lua) and exposes a starter slice
// of the Canary scripting API so existing Lua content can run against the Go
// server. The full ~1300-function surface is ported incrementally.
package luaengine

import (
	"log/slog"
	"sync"

	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// Engine owns a Lua state guarded by a mutex (gopher-lua states are not
// goroutine-safe).
type Engine struct {
	mu    sync.Mutex
	L     *lua.LState
	log   *slog.Logger
	world *game.World
}

// New creates an engine with the base libraries loaded.
func New(world *game.World, log *slog.Logger) *Engine {
	L := lua.NewState()
	e := &Engine{L: L, log: log, world: world}
	e.registerAPI()
	return e
}

// Close releases the Lua state.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.L.Close()
}

// DoFile executes a Lua script file under the engine lock.
func (e *Engine) DoFile(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.L.DoFile(path)
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
