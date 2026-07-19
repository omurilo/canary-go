// Package luaengine embeds a Lua 5.1 VM (gopher-lua) and exposes a starter slice
// of the Canary scripting API so existing Lua content can run against the Go
// server. The full ~1300-function surface is ported incrementally.
package luaengine

import (
	"log/slog"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// Engine owns a Lua state guarded by a mutex (gopher-lua states are not
// goroutine-safe).
type Engine struct {
	mu  sync.Mutex
	L   *lua.LState
	log *slog.Logger
}

// New creates an engine with the base libraries loaded.
func New(log *slog.Logger) *Engine {
	L := lua.NewState()
	e := &Engine{L: L, log: log}
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
