package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

// registerAPI installs the starter global API. This is intentionally small; it
// establishes the bridge pattern (Go closures registered as Lua globals/tables)
// that the remaining Canary API surface will follow.
func (e *Engine) registerAPI() {
	L := e.L

	// Global Tibia enums (COMBAT_*, BESTY_RACE_*, CONST_SLOT_*, ...) must exist
	// before content scripts (monsters, spells) reference them.
	RegisterEnums(L)

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

	e.registerGame()
	e.registerCreatureType()
	e.registerPlayerType()
	e.registerMonster()
	e.registerNpc()
	e.registerPosition()
	e.registerItem()
	e.registerContainer()
	e.registerTile()
	e.registerAction()
	e.registerMoveEvent()
	e.registerMonsterType()
	e.registerLootClass()
	e.registerNpcType()
	e.registerNetworkMessage()
	e.registerBank()
	e.registerParty()
	e.registerTown()

	linkClasses := func(child, parent string) {
		childMt, _ := L.GetTypeMetatable(child).(*lua.LTable)
		parentMt, _ := L.GetTypeMetatable(parent).(*lua.LTable)
		if childMt == nil || parentMt == nil {
			return
		}
		
		childIdx, _ := L.RawGet(childMt, lua.LString("__index")).(*lua.LTable)
		parentIdx, _ := L.RawGet(parentMt, lua.LString("__index")).(*lua.LTable)
		
		if childIdx != nil && parentIdx != nil {
			idxMt := L.NewTable()
			L.SetField(idxMt, "__index", parentIdx)
			L.SetMetatable(childIdx, idxMt)
		}
	}

	linkClasses("Player", "Creature")
	linkClasses("Monster", "Creature")
	linkClasses("Npc", "Creature")

	// Mock constructors for unused revscriptsys classes so scripts don't crash
	mockClass := func(name string) {
		mt := L.NewTypeMetatable(name)
		
		idxTable := L.NewTable()
		idxMt := L.NewTable()
		L.SetField(idxMt, "__index", L.NewFunction(func(L *lua.LState) int {
			L.Push(L.NewFunction(func(L *lua.LState) int { return 0 }))
			return 1
		}))
		L.SetMetatable(idxTable, idxMt)
		
		L.SetField(mt, "__index", idxTable)
		L.SetField(mt, "__newindex", L.NewFunction(func(L *lua.LState) int { return 0 }))
		
		// The constructor (__call) returns a new userdata
		L.SetField(mt, "__call", L.NewFunction(func(L *lua.LState) int {
			ud := L.NewUserData()
			ud.Value = name
			L.SetMetatable(ud, mt)
			L.Push(ud)
			return 1
		}))

		classTable := L.NewTable()
		L.SetMetatable(classTable, mt)
		L.SetGlobal(name, classTable)
	}
	mockClass("CreatureEvent")
	mockClass("GlobalEvent")
	mockClass("Weapon")
	mockClass("Result")
	mockClass("Achievement")
	mockClass("BestiaryCharm")
	mockClass("ItemTier")
	mockClass("Spawns")
	mockClass("BedItem")
	mockClass("DropLoot")

	// rawgetmetatable allows scripts (like revscriptsys) to retrieve the type metatable
	L.SetGlobal("rawgetmetatable", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		mt := L.GetTypeMetatable(name)
		if mt != lua.LNil {
			L.Push(mt)
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))
	
	// Ensure these class tables exist so scripts can inject methods into them (e.g. Player.feed = ...)
	ensureClassTable := func(name string) {
		if L.GetGlobal(name) == lua.LNil {
			var classTable *lua.LTable
			if typeMt, ok := L.GetTypeMetatable(name).(*lua.LTable); ok {
				if idx := L.RawGet(typeMt, lua.LString("__index")); idx.Type() == lua.LTTable {
					classTable = idx.(*lua.LTable)
				}
			}
			if classTable == nil {
				classTable = L.NewTable()
			}
			mt := L.NewTypeMetatable(name + "_ClassDummy")
			// Dummy __call returning nil so scripts don't crash when calling Player(cid)
			L.SetField(mt, "__call", L.NewFunction(func(L *lua.LState) int { return 0 }))
			
			// Dummy __index returning a dummy function so arbitrary method calls don't crash
			L.SetField(mt, "__index", L.NewFunction(func(L *lua.LState) int {
				L.Push(L.NewFunction(func(L *lua.LState) int { return 0 }))
				return 1
			}))

			L.SetMetatable(classTable, mt)
			L.SetGlobal(name, classTable)
		}
	}
	// Player/Creature/Npc/Monster are also callable constructors: Player(x)
	// resolves x (a creature userdata, or a numeric creature id) to the matching
	// userdata, mirroring the C++ Player(cid)/Creature(cid) lookups. NPC dialog
	// scripts rely on Player(creature):getPosition() etc. A dummy __call here
	// would return nil and crash those scripts.
	setCreatureConstructor := func(name string) {
		var classTable *lua.LTable
		if typeMt, ok := L.GetTypeMetatable(name).(*lua.LTable); ok {
			if idx := L.RawGet(typeMt, lua.LString("__index")); idx.Type() == lua.LTTable {
				classTable = idx.(*lua.LTable)
			}
		}
		if classTable == nil {
			classTable = L.NewTable()
		}

		mt := L.NewTypeMetatable(name + "_ClassCtor")
		L.SetField(mt, "__call", L.NewFunction(func(L *lua.LState) int {
			return e.creatureConstructorCall(L, name)
		}))
		// Keep field injection working (e.g. Player.feed = ...) on the table.
		L.SetMetatable(classTable, mt)
		L.SetGlobal(name, classTable)
	}
	setCreatureConstructor("Player")
	setCreatureConstructor("Monster")
	setCreatureConstructor("Npc")
	setCreatureConstructor("Creature")
	ensureClassTable("ItemType")
	ensureClassTable("MonsterType")
	ensureClassTable("Teleport")
	ensureClassTable("Vocation")
	ensureClassTable("Party")
	ensureClassTable("GemAtelier")
	ensureClassTable("Guild")
	ensureClassTable("Group")
	ensureClassTable("Town")
	ensureClassTable("House")
	ensureClassTable("Variant")
	ensureClassTable("Condition")
	ensureClassTable("Combat")
	ensureClassTable("Zone")

	// configManager / configKeys mirror the C++ globals that expose config.lua.
	// The full server reads real config values here; this slice provides safe
	// defaults so datapack scripts that read config (e.g. boss cooldowns via
	// configKeys.*) degrade gracefully instead of erroring on a nil table.
	configManagerTbl := L.NewTable()
	L.SetField(configManagerTbl, "getNumber", L.NewFunction(func(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }))
	L.SetField(configManagerTbl, "getFloat", L.NewFunction(func(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }))
	L.SetField(configManagerTbl, "getString", L.NewFunction(func(L *lua.LState) int { L.Push(lua.LString("")); return 1 }))
	L.SetField(configManagerTbl, "getBoolean", L.NewFunction(func(L *lua.LState) int { L.Push(lua.LFalse); return 1 }))
	L.SetGlobal("configManager", configManagerTbl)

	// configKeys.X resolves to the key name itself (never nil), so callers can
	// pass it straight to configManager.get*.
	configKeysTbl := L.NewTable()
	configKeysMeta := L.NewTable()
	L.SetField(configKeysMeta, "__index", L.NewFunction(func(L *lua.LState) int {
		L.Push(L.Get(2))
		return 1
	}))
	L.SetMetatable(configKeysTbl, configKeysMeta)
	L.SetGlobal("configKeys", configKeysTbl)

	e.registerShop()
	e.registerEventCallback()
	e.registerTalkAction()
	e.registerSpell()
	e.registerCombat()
	e.registerVariant()
	e.registerCondition()
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

// setClassConstructor registers a global table with a __call metamethod so that
// scripts can use `Class(args)` to construct new instances. This matches how Canary C++
// exports constructors like Action, MoveEvent, Spell, etc.
func (e *Engine) setClassConstructor(name string, constructor lua.LGFunction, methods map[string]lua.LGFunction) *lua.LTable {
	L := e.L
	var classTable *lua.LTable
	if typeMt, ok := L.GetTypeMetatable(name).(*lua.LTable); ok {
		if idx := L.RawGet(typeMt, lua.LString("__index")); idx.Type() == lua.LTTable {
			classTable = idx.(*lua.LTable)
		}
	}
	if classTable == nil {
		classTable = L.NewTable()
	}

	mt := L.NewTypeMetatable(name + "_ClassCtor")
	call := func(L *lua.LState) int {
		return constructor(L)
	}
	L.SetField(mt, "__call", L.NewFunction(call))

	if methods != nil {
		L.SetFuncs(classTable, methods)
	}
	L.SetMetatable(classTable, mt)
	L.SetGlobal(name, classTable)
	return classTable
}
