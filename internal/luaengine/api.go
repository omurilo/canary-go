package luaengine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/mounts"
	lua "github.com/yuin/gopher-lua"
)

// registerAPI installs the starter global API. This is intentionally small; it
// establishes the bridge pattern (Go closures registered as Lua globals/tables)
// that the remaining Canary API surface will follow.
func (e *Engine) registerAPI() {
	L := e.L

	// Set global directory paths required by Lua scripts
	cfg := config.Active
	dataDir := "data-otservbr-global"
	coreDir := "data"
	if cfg != nil {
		if cfg.DataPack != "" {
			dataDir = cfg.DataPack
		}
		if cfg.Core != "" {
			coreDir = cfg.Core
		}
	}

	// Resolve directories relative to current working directory if needed (e.g. running from package subdirs in tests)
	resolveDir := func(dir string) string {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		for _, up := range []string{"..", filepath.Join("..", ".."), filepath.Join("..", "..", "..")} {
			if _, err := os.Stat(filepath.Join(up, dir)); err == nil {
				return filepath.Join(up, dir)
			}
		}
		return dir
	}

	dataDirResolved := resolveDir(dataDir)
	coreDirResolved := resolveDir(coreDir)

	L.SetGlobal("DATA_DIRECTORY", lua.LString(dataDirResolved))
	L.SetGlobal("CORE_DIRECTORY", lua.LString(coreDirResolved))

	registerBitLib(L)

	// Set package.path so require(...) can find scripts across the datapack and core libs
	// The datapack requires modules by a path rooted ABOVE the datapack directory:
	// data-otservbr-global/lib/core/load.lua:3 does
	// require("data-otservbr-global.lib.core.quests"), which resolves to
	// <parent>/data-otservbr-global/lib/core/quests.lua. Without the parent
	// directories here the lookup doubled the datapack name
	// (data-otservbr-global/data-otservbr-global/...) and every require failed,
	// aborting lib/core/load.lua and the quests library it pulls in.
	pkgPath := strings.Join([]string{
		filepath.Dir(dataDirResolved) + "/?.lua",
		filepath.Dir(dataDirResolved) + "/?/init.lua",
		filepath.Dir(coreDirResolved) + "/?.lua",
		filepath.Dir(coreDirResolved) + "/?/init.lua",
		coreDirResolved + "/libs/?.lua",
		coreDirResolved + "/libs/?/init.lua",
		dataDirResolved + "/?.lua",
		dataDirResolved + "/?/init.lua",
		"?.lua",
		"?/init.lua",
	}, ";")
	if pkg, ok := L.GetGlobal("package").(*lua.LTable); ok {
		L.SetField(pkg, "path", lua.LString(pkgPath))
	}

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
	logFunc := func(level string) lua.LGFunction {
		return func(L *lua.LState) int {
			top := L.GetTop()
			if top == 0 {
				return 0
			}
			msg := L.ToString(1)
			if top > 1 {
				for i := 2; i <= top; i++ {
					val := L.ToString(i)
					if strings.Contains(msg, "{}") {
						msg = strings.Replace(msg, "{}", val, 1)
					} else {
						msg += " " + val
					}
				}
			}
			switch level {
			case "trace", "debug":
				e.log.Debug("lua", "msg", msg)
			case "info":
				e.log.Info("lua", "msg", msg)
			case "warn":
				e.log.Warn("lua", "msg", msg)
			case "error":
				e.log.Error("lua", "msg", msg)
			}
			return 0
		}
	}
	L.SetField(logger, "trace", L.NewFunction(logFunc("trace")))
	L.SetField(logger, "debug", L.NewFunction(logFunc("debug")))
	L.SetField(logger, "info", L.NewFunction(logFunc("info")))
	L.SetField(logger, "warn", L.NewFunction(logFunc("warn")))
	L.SetField(logger, "error", L.NewFunction(logFunc("error")))
	L.SetGlobal("logger", logger)

	L.SetGlobal("SERVER_NAME", lua.LString("Canary"))

	// Server save stubs
	L.SetGlobal("saveServer", L.NewFunction(func(L *lua.LState) int {
		return 0
	}))
	L.SetGlobal("SaveHirelings", L.NewFunction(func(L *lua.LState) int {
		return 0
	}))
	// Inject a custom loader to map "data.*" to "data-otservbr-global/*"
	pkg := L.GetGlobal("package")
	if tbl, ok := pkg.(*lua.LTable); ok {
		loaders := L.GetField(tbl, "loaders")
		if loadersTbl, ok := loaders.(*lua.LTable); ok {
			customLoader := L.NewFunction(func(L *lua.LState) int {
				mod := L.CheckString(1)
				if strings.HasPrefix(mod, "data.") {
					// Replace "data." with "data-otservbr-global/"
					path := strings.Replace(mod, ".", "/", -1)
					path = strings.Replace(path, "data/", "data-otservbr-global/", 1)
					path += ".lua"

					data, err := os.ReadFile(path)
					if err != nil {
						L.Push(lua.LString("no file '" + path + "'"))
						return 1
					}
					fn, err := L.Load(strings.NewReader(preprocessLuaSource(string(data))), path)
					if err != nil {
						L.Push(lua.LString("error loading module '" + mod + "' from file '" + path + "':\n\t" + err.Error()))
						return 1
					}
					L.Push(fn)
					return 1
				}
				L.Push(lua.LString(""))
				return 1
			})
			// Insert at index 2 (after preload loader)
			loadersTbl.Insert(2, customLoader)
		}
	}

	// Item and fluid enums used across Lua scripts
	L.SetGlobal("FLUID_NONE", lua.LNumber(0))
	L.SetGlobal("FLUID_WATER", lua.LNumber(1))
	L.SetGlobal("FLUID_WINE", lua.LNumber(2))
	L.SetGlobal("FLUID_BEER", lua.LNumber(3))
	L.SetGlobal("FLUID_MUD", lua.LNumber(4))
	L.SetGlobal("FLUID_BLOOD", lua.LNumber(5))
	L.SetGlobal("FLUID_SLIME", lua.LNumber(6))
	L.SetGlobal("FLUID_OIL", lua.LNumber(7))
	L.SetGlobal("FLUID_URINE", lua.LNumber(8))
	L.SetGlobal("FLUID_MILK", lua.LNumber(9))
	L.SetGlobal("FLUID_MANA", lua.LNumber(10))
	L.SetGlobal("FLUID_LIFE", lua.LNumber(11))
	L.SetGlobal("FLUID_LEMONADE", lua.LNumber(12))
	L.SetGlobal("FLUID_RUM", lua.LNumber(13))
	L.SetGlobal("FLUID_FRUITJUICE", lua.LNumber(14))
	L.SetGlobal("FLUID_COCONUTMILK", lua.LNumber(15))
	L.SetGlobal("FLUID_MEAD", lua.LNumber(16))
	L.SetGlobal("FLUID_TEA", lua.LNumber(17))
	L.SetGlobal("FLUID_INK", lua.LNumber(18))
	L.SetGlobal("FLUID_CANDY", lua.LNumber(19))
	L.SetGlobal("FLUID_CHOCOLATE", lua.LNumber(20))

	// Daily reward status enums

	// Player sex enums

	e.registerGame()
	e.registerCreatureType()
	e.registerPlayerType()
	e.registerGuildType()
	e.registerMonster()
	e.registerNpc()
	e.registerPosition()
	e.registerItem()
	e.registerItemType()
	e.registerContainer()
	e.registerTile()
	e.registerAction()
	e.registerMoveEvent()
	e.registerMonsterType()
	e.registerMonsterSpellClass()
	e.registerLootClass()
	e.registerNpcType()
	e.registerNetworkMessage()
	e.registerBank()
	e.registerParty()
	e.registerHouseMetatable()
	e.registerTown()
	e.registerCreatureEvent()
	e.registerGlobalEventClass()
	e.registerVocation()
	e.registerModalWindowType()
	e.registerWeaponType()
	e.registerWebhookType()
	e.registerMetrics()

	// HirelingsInit is a no-op stub called by server_initialization.lua.
	// The full hireling system is not ported to Go yet.
	L.SetGlobal("HirelingsInit", L.NewFunction(func(L *lua.LState) int {
		return 0
	}))

	// Mock constructors for unused revscriptsys classes so scripts don't crash
	mockClass := func(name string) {
		mt := L.NewTypeMetatable(name)

		idxTable := L.NewTable()
		idxMt := L.NewTable()
		L.SetField(idxMt, "__index", L.NewFunction(func(L *lua.LState) int {
			key := L.CheckString(2)
			L.Push(L.NewFunction(func(L *lua.LState) int {
				if strings.HasPrefix(key, "count") {
					L.Push(lua.LNumber(0))
					return 1
				}
				if key == "getPositions" || key == "getZones" || key == "getCreatures" || key == "getPlayers" || key == "getMonsters" || key == "getItems" {
					L.Push(L.NewTable())
					return 1
				}
				ud := L.NewUserData()
				ud.Value = name
				L.SetMetatable(ud, mt)
				L.Push(ud)
				return 1
			}))
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
	// EventsScheduler global table
	eventsScheduler := L.NewTable()
	hundredFunc := L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(100))
		return 1
	})
	L.SetField(eventsScheduler, "getEventSLoot", hundredFunc)
	L.SetField(eventsScheduler, "getEventSBossLoot", hundredFunc)
	L.SetField(eventsScheduler, "getEventSSkill", hundredFunc)
	L.SetField(eventsScheduler, "getEventSExp", hundredFunc)
	L.SetField(eventsScheduler, "getSpawnMonsterSchedule", hundredFunc)
	L.SetGlobal("EventsScheduler", eventsScheduler)

	L.SetGlobal("AUTH_TYPE", lua.LString("password"))

	// IMPORTANT: mockClass installs a catch-all __index on the type metatable.
	// Because gopher-lua's NewTypeMetatable returns the *existing* table when the
	// name is already registered, mocking a name that has a real binding silently
	// destroys that binding. Never mock a name that appears in the e.register*()
	// block above.
	//
	// Guild, House, Party and Weapon used to be mocked here even though they have
	// real bindings, which made every guild:/house:/party: call return a mock
	// userdata instead of the real value.
	mockClass("Result")
	mockClass("Achievement")
	mockClass("BestiaryCharm")
	mockClass("ItemTier")
	mockClass("Spawns")
	mockClass("BedItem")
	mockClass("DropLoot")
	mockClass("Charm")
	mockClass("GemAtelier")
	mockClass("Group")
	mockClass("Hazard")
	mockClass("ZoneEvent")
	mockClass("HazardMonster")
	// Zone and Teleport stay mocked on purpose. registerZone/registerTeleportType
	// exist but are NOT drop-in replacements: their constructors return nil, and
	// their method sets miss what the datapack actually calls (zone:addArea is
	// used 32 times and is absent, likewise setRemoveDestination/removePlayers/
	// removeMonsters/register/randomPosition). Wiring them today would turn 33
	// zone scripts from silently-wrong into hard "index a nil value" errors.
	// Same for Imbuement: registerImbuementType is intentionally left unwired.
	mockClass("Teleport")
	mockClass("Zone")

	// ItemClassification has a real binding (not a mock) so it can be
	// populated from data/scripts/systems/item_tiers.lua.
	e.registerItemClassificationType()

	L.SetGlobal("WEBHOOK_COLOR_GREEN", lua.LNumber(0x00FF00))
	L.SetGlobal("WEBHOOK_COLOR_RED", lua.LNumber(0xFF0000))
	L.SetGlobal("WEBHOOK_COLOR_YELLOW", lua.LNumber(0xFFFF00))
	L.SetGlobal("WEBHOOK_COLOR_BLUE", lua.LNumber(0x0000FF))
	L.SetGlobal("WEBHOOK_COLOR_WARNING", lua.LNumber(0xFFFF00))

	announcementChannels := L.NewTable()
	L.SetField(announcementChannels, "serverAnnouncements", lua.LString(""))
	L.SetField(announcementChannels, "raids", lua.LString(""))
	L.SetField(announcementChannels, "player-kills", lua.LString(""))
	L.SetField(announcementChannels, "player-levels", lua.LString(""))
	L.SetField(announcementChannels, "reports", lua.LString(""))
	L.SetGlobal("announcementChannels", announcementChannels)

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

	linkClasses := func(child, parent string) {
		childMt, _ := L.GetTypeMetatable(child).(*lua.LTable)
		parentMt, _ := L.GetTypeMetatable(parent).(*lua.LTable)
		if childMt == nil || parentMt == nil {
			return
		}

		childIdx, _ := L.RawGet(childMt, lua.LString("__index")).(*lua.LTable)
		parentIdx, _ := L.RawGet(parentMt, lua.LString("__index")).(*lua.LTable)

		if childIdx != nil && parentIdx != nil {
			idxMt := L.GetMetatable(childIdx)
			if idxMt == nil || idxMt == lua.LNil {
				idxMt = L.NewTable()
				L.SetMetatable(childIdx, idxMt)
			}
			if tbl, ok := idxMt.(*lua.LTable); ok {
				L.SetField(tbl, "__index", parentIdx)
			}
		}
	}

	linkClasses("Player", "Creature")
	linkClasses("Monster", "Creature")
	linkClasses("Npc", "Creature")
	ensureClassTable("MonsterType")
	ensureClassTable("NpcType")
	ensureClassTable("Spell")
	ensureClassTable("Party")
	ensureClassTable("Town")
	ensureClassTable("Variant")
	ensureClassTable("Condition")
	ensureClassTable("Combat")

	// configManager / configKeys mirror the C++ globals that expose config.lua.
	// The full server reads real config values here; this slice provides safe
	// defaults so datapack scripts that read config (e.g. boss cooldowns via
	// configKeys.*) degrade gracefully instead of erroring on a nil table.
	configManagerTbl := L.NewTable()
	L.SetField(configManagerTbl, "getNumber", L.NewFunction(func(L *lua.LState) int {
		key := strings.ToLower(L.OptString(1, ""))
		key = strings.ReplaceAll(key, "_", "")
		if config.Active != nil && config.Active.Custom != nil {
			if val, exists := config.Active.Custom[key]; exists {
				if num, ok := val.(lua.LNumber); ok {
					L.Push(num)
					return 1
				}
				if str, ok := val.(lua.LString); ok {
					if f, err := strconv.ParseFloat(string(str), 64); err == nil {
						L.Push(lua.LNumber(f))
						return 1
					}
				}
			}
		}
		if key == "maxallowedonadummy" {
			L.Push(lua.LNumber(1))
		} else {
			L.Push(lua.LNumber(0))
		}
		return 1
	}))
	L.SetField(configManagerTbl, "getFloat", L.NewFunction(func(L *lua.LState) int {
		key := strings.ToLower(L.OptString(1, ""))
		key = strings.ReplaceAll(key, "_", "")
		if config.Active != nil && config.Active.Custom != nil {
			if val, exists := config.Active.Custom[key]; exists {
				if num, ok := val.(lua.LNumber); ok {
					L.Push(num)
					return 1
				}
				if str, ok := val.(lua.LString); ok {
					if f, err := strconv.ParseFloat(string(str), 64); err == nil {
						L.Push(lua.LNumber(f))
						return 1
					}
				}
			}
		}
		if strings.HasPrefix(key, "rate") || strings.Contains(key, "speed") || strings.Contains(key, "multiplier") {
			L.Push(lua.LNumber(1.0))
		} else {
			L.Push(lua.LNumber(0))
		}
		return 1
	}))
	L.SetField(configManagerTbl, "getString", L.NewFunction(func(L *lua.LState) int {
		key := strings.ToLower(L.OptString(1, ""))
		key = strings.ReplaceAll(key, "_", "")
		if config.Active != nil && config.Active.Custom != nil {
			if val, exists := config.Active.Custom[key]; exists {
				L.Push(lua.LString(val.String()))
				return 1
			}
		}
		if strings.Contains(key, "save_time") || strings.Contains(key, "savetime") {
			L.Push(lua.LString("03:00:00"))
			return 1
		}
		L.Push(lua.LString(""))
		return 1
	}))
	L.SetField(configManagerTbl, "getBoolean", L.NewFunction(func(L *lua.LState) int {
		key := strings.ToLower(L.OptString(1, ""))
		key = strings.ReplaceAll(key, "_", "")
		if config.Active != nil && config.Active.Custom != nil {
			if val, exists := config.Active.Custom[key]; exists {
				if b, ok := val.(lua.LBool); ok {
					L.Push(b)
					return 1
				}
				if str, ok := val.(lua.LString); ok {
					s := strings.ToLower(string(str))
					if s == "true" || s == "yes" || s == "1" {
						L.Push(lua.LTrue)
						return 1
					} else if s == "false" || s == "no" || s == "0" {
						L.Push(lua.LFalse)
						return 1
					}
				}
			}
		}
		if strings.HasPrefix(key, "remove") || key == "removepotioncharges" || key == "removechargesfrompotions" {
			L.Push(lua.LTrue)
			return 1
		}
		L.Push(lua.LFalse)
		return 1
	}))
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
	e.registerDB()
	// registerCharmType runs after the mockClass block so its real Charm type
	// and Game.createBestiaryCharm override the mocks.
	e.registerCharmType()

	// addPlayerEvent global fallback function if modules.lua isn't loaded yet.
	// Mirrors data/modules/lib/modules.lua: addPlayerEvent(callable, delay,
	// playerId, ...) schedules callable(player, ...) after `delay` (the datapack
	// re-checks the player is online first; our store senders resolve
	// Player(playerId) themselves and no-op when nil, so forwarding the id is
	// equivalent). luaAddEvent forwards args 3+ to the callback, so
	// addEvent(fn, delay, target, args...) fires as fn(target, args...) — which
	// is exactly what the callables (sendShowStoreOffers, sendStoreError, …)
	// expect as their first parameter. The previous wrapper dropped the target
	// (used arg 2 as the first parameter), so scheduled store offers were never
	// sent and opening a category hung the client.
	L.SetGlobal("addPlayerEvent", L.NewFunction(func(L *lua.LState) int {
		fn := L.Get(1)
		delay := L.CheckInt(2)
		n := L.GetTop()
		addEv := L.GetGlobal("addEvent")
		addEvFn, ok := addEv.(*lua.LFunction)
		if !ok {
			return 0
		}
		L.Push(addEvFn)
		L.Push(fn)
		L.Push(lua.LNumber(delay))
		for i := 3; i <= n; i++ { // target + trailing args
			L.Push(L.Get(i))
		}
		L.Call(n, 0) // fn, delay, target, args... => n arguments to addEvent
		return 0
	}))

	// systemTime(): milliseconds since epoch (C++ OTSYS_TIME). Used e.g. by
	// Player:addItemStoreInbox to stamp ITEM_ATTRIBUTE_STORE.
	L.SetGlobal("systemTime", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Now().UnixMilli()))
		return 1
	}))

	// BatchUpdate(player): batches container UI refreshes in C++. We update
	// containers eagerly via addItemEx, so a no-op stub (add/delete) is
	// functionally equivalent and lets store delivery (addItemStoreInbox) run.
	L.SetGlobal("BatchUpdate", L.NewFunction(func(L *lua.LState) int {
		t := L.NewTable()
		noop := L.NewFunction(func(L *lua.LState) int { return 0 })
		L.SetField(t, "add", noop)
		L.SetField(t, "delete", noop)
		L.Push(t)
		return 1
	}))

	// Mount(id): returns a mount handle exposing getClientId/getId/getName.
	// The gamestore senders call Mount(offer.id):getClientId() for SHOW_MOUNT
	// offers with no nil-check (senders.lua sendShowStoreOffers), so always
	// return a valid object — fall back to the raw id as the client id when the
	// mount isn't in the registry.
	L.SetGlobal("Mount", L.NewFunction(func(L *lua.LState) int {
		id := uint16(L.CheckInt(1))
		clientID, name := id, ""
		if m, ok := mounts.GetByID(id); ok {
			clientID, name = m.ClientID, m.Name
		}
		t := L.NewTable()
		L.SetField(t, "getClientId", L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LNumber(clientID))
			return 1
		}))
		L.SetField(t, "getId", L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LNumber(id))
			return 1
		}))
		L.SetField(t, "getName", L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LString(name))
			return 1
		}))
		L.Push(t)
		return 1
	}))

	// DailyReward table fallback
	dailyRewardTbl := L.NewTable()
	L.SetField(dailyRewardTbl, "init", L.NewFunction(func(L *lua.LState) int { return 0 }))
	storagesTbl := L.NewTable()
	L.SetField(storagesTbl, "lastServerSave", lua.LNumber(1001))
	L.SetField(dailyRewardTbl, "storages", storagesTbl)
	L.SetGlobal("DailyReward", dailyRewardTbl)

	// QuestTrackerServerConfig fallback
	qtConfigTbl := L.NewTable()
	L.SetField(qtConfigTbl, "kvScope", lua.LString("quest-tracker"))
	L.SetField(qtConfigTbl, "trackedMissionsKey", lua.LString("tracked-missions"))
	L.SetField(qtConfigTbl, "knownQuestsKey", lua.LString("known-quests"))
	L.SetField(qtConfigTbl, "autoTrackNewQuestsKey", lua.LString("auto-track-new-quests"))
	L.SetField(qtConfigTbl, "autoUntrackCompletedQuestsKey", lua.LString("auto-untrack-completed-quests"))
	L.SetField(qtConfigTbl, "completedMissionRemovalDelay", lua.LNumber(5000))
	L.SetField(qtConfigTbl, "loginLoadDelay", lua.LNumber(1000))
	L.SetField(qtConfigTbl, "initialSyncWindow", lua.LNumber(5000))
	L.SetGlobal("QuestTrackerServerConfig", qtConfigTbl)

	// VOCATION table
	vocTbl := L.NewTable()
	baseIdTbl := L.NewTable()
	L.SetField(baseIdTbl, "NONE", lua.LNumber(0))
	L.SetField(baseIdTbl, "SORCERER", lua.LNumber(1))
	L.SetField(baseIdTbl, "DRUID", lua.LNumber(2))
	L.SetField(baseIdTbl, "PALADIN", lua.LNumber(3))
	L.SetField(baseIdTbl, "KNIGHT", lua.LNumber(4))
	L.SetField(baseIdTbl, "MONK", lua.LNumber(5))
	L.SetField(vocTbl, "BASE_ID", baseIdTbl)
	L.SetGlobal("VOCATION", vocTbl)
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
