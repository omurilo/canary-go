package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

// registerAPI installs the starter global API. This is intentionally small; it
// establishes the bridge pattern (Go closures registered as Lua globals/tables)
// that the remaining Canary API surface will follow.
func (e *Engine) registerAPI() {
	L := e.L

	// print -> structured logger, so script output shows up in server logs.
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		parts := make([]any, 0, n)
		for i := 1; i <= n; i++ {
			parts = append(parts, L.ToStringMeta(L.Get(i)).String())
		}
		e.log.Info("lua", "msg", parts)
		return 0
	}))

	// Minimal Game table.
	game := L.NewTable()
	L.SetGlobal("Game", game)

	// Logger table mirroring the C++ `logger` global.
	logger := L.NewTable()
	L.SetField(logger, "info", L.NewFunction(func(L *lua.LState) int {
		e.log.Info("lua", "msg", L.CheckString(1))
		return 0
	}))
	L.SetField(logger, "warn", L.NewFunction(func(L *lua.LState) int {
		e.log.Warn("lua", "msg", L.CheckString(1))
		return 0
	}))
	L.SetField(logger, "error", L.NewFunction(func(L *lua.LState) int {
		e.log.Error("lua", "msg", L.CheckString(1))
		return 0
	}))
	L.SetGlobal("logger", logger)

	e.registerPosition()
	e.registerTile()
	e.registerItem()
	e.registerContainer()
	e.registerCreatureType()
	e.registerPlayerType()
	e.registerMonsterType()
	e.registerNpcType()
	e.registerGame()
	e.registerAction()
	e.registerCombat()
	e.registerTalkAction()
	e.registerSpell()
}

// SetGameFunc registers a Go function as a field on the global Game table.
func (e *Engine) SetGameFunc(name string, fn lua.LGFunction) {
	e.mu.Lock()
	defer e.mu.Unlock()
	game, ok := e.L.GetGlobal("Game").(*lua.LTable)
	if !ok {
		game = e.L.NewTable()
		e.L.SetGlobal("Game", game)
	}
	e.L.SetField(game, name, e.L.NewFunction(fn))
}
