// Command canary is the Go migration of the Canary MMORPG server. It serves the
// Tibia 13.x login (7171) and game (7172) protocols against PostgreSQL, with an
// embedded Lua scripting engine.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/opentibiabr/canary-go/internal/actions"
	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/events"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/imbuements"
	"github.com/opentibiabr/canary-go/internal/game/spawns"
	"github.com/opentibiabr/canary-go/internal/game/vocations"
	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/luaengine"
	"github.com/opentibiabr/canary-go/internal/mounts"
	"github.com/opentibiabr/canary-go/internal/network"
	"github.com/opentibiabr/canary-go/internal/otbm"
	"github.com/opentibiabr/canary-go/internal/protocol"
	"github.com/opentibiabr/canary-go/internal/spells"
	"github.com/opentibiabr/canary-go/internal/tibcrypto"
)

func main() {
	var (
		configPath = flag.String("config", "config.lua", "path to the Lua config file")
		schemaPath = flag.String("schema", "schema/mysql.sql", "path to the MySQL schema (canary-go extras)")
		scriptsDir = flag.String("scripts", "scripts", "directory of Lua scripts to load at startup")
		appearances = flag.String("appearances", "../data/items/appearances.dat", "path to appearances.dat (item metadata)")
		mapFile     = flag.String("map", "", "path to an OTBM map file (empty = synthetic spawn field)")
		migrate     = flag.Bool("migrate", true, "apply the schema on startup (idempotent)")
		seed        = flag.Bool("seed", false, "seed a test account (god/god, char 'Gm Test')")
		logLevelStr = flag.String("logLevel", "info", "log level (debug, info, warn, error)")
	)
	flag.Parse()

	var level slog.Level
	switch strings.ToLower(*logLevelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	opts := runOpts{
		configPath: *configPath, schemaPath: *schemaPath, scriptsDir: *scriptsDir,
		appearances: *appearances, mapFile: *mapFile, migrate: *migrate, seed: *seed,
	}
	if err := run(opts, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

type runOpts struct {
	configPath  string
	schemaPath  string
	scriptsDir  string
	appearances string
	mapFile     string
	migrate     bool
	seed        bool
}

func run(o runOpts, log *slog.Logger) error {
	configPath, schemaPath, scriptsDir := o.configPath, o.schemaPath, o.scriptsDir
	migrate, seed := o.migrate, o.seed
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Warn("using default config", "err", err)
	}

	if v, ok := cfg.Custom["logLevel"]; ok {
		if lv := v.String(); lv != "" {
			var lvl slog.Level
			switch strings.ToLower(lv) {
			case "debug":
				lvl = slog.LevelDebug
			case "warn", "warning":
				lvl = slog.LevelWarn
			case "error":
				lvl = slog.LevelError
			default:
				lvl = slog.LevelInfo
			}
			log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
			slog.SetDefault(log)
		}
	}
	
	if scriptsDir == "scripts" && cfg.DataPack != "" {
		scriptsDir = filepath.Join(cfg.DataPack, "scripts")
	}
	log.Info("configuration loaded", "server", cfg.ServerName, "loginPort", cfg.LoginPort,
		"gamePort", cfg.GamePort, "db", cfg.DBName)

	rsa, err := tibcrypto.LoadRSAFromPEM(cfg.RSAKeyFile)
	if err != nil {
		return fmt.Errorf("load RSA key %q: %w", cfg.RSAKeyFile, err)
	}

	database, err := db.Connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	if migrate {
		if err := database.ApplySchema(ctx, cfg, schemaPath); err != nil {
			return err
		}
		log.Info("schema applied", "path", schemaPath)
	}
	if seed {
		if err := database.SeedTestAccount(ctx, "god", "god@canary.local", "god", "Gm Test"); err != nil {
			return err
		}
		log.Info("seeded test account", "account", "god", "password", "god", "character", "Gm Test")
	}

	// Item metadata catalog (client appearances).
	var catalog *items.Catalog
	if cat, err := items.Load(o.appearances); err != nil {
		log.Warn("item catalog not loaded (AddItem will send bare ids)", "path", o.appearances, "err", err)
	} else {
		catalog = cat
		xmlPath := filepath.Join(filepath.Dir(o.appearances), "items.xml")
		if err := catalog.LoadXML(xmlPath); err != nil {
			log.Warn("items.xml not loaded (missing xml attributes)", "path", xmlPath, "err", err)
		}
		log.Info("item catalog loaded", "types", cat.Len())
	}

	// World: real OTBM map if provided, else a synthetic spawn field.
	world := game.NewWorld()
	world.Items = catalog

	if bc, err := database.GetBoostedCreature(ctx); err == nil && bc != "" && bc != "default" {
		world.BoostedCreature = bc
	}
		if bb, err := database.GetBoostedBoss(ctx); err == nil && bb != "" && bb != "default" {
			world.BoostedBoss = bb
		}

		// Market table and pre-load existing offers.
		if err := database.EnsureMarketTable(ctx); err != nil {
			log.Warn("ensure market table", "err", err)
		} else if err := database.LoadMarketOffers(ctx, world.Market); err != nil {
			log.Warn("load market offers", "err", err)
		}

		// Familiars table.
		if err := database.EnsureFamiliarsTable(ctx); err != nil {
			log.Warn("ensure familiars table", "err", err)
		}

		// Hazard system table.
		if err := database.EnsureHazardTable(ctx); err != nil {
			log.Warn("ensure hazard table", "err", err)
		}

		// Concoctions system table.
		if err := database.EnsureConcoctionsTable(ctx); err != nil {
			log.Warn("ensure concoctions table", "err", err)
		}

			// Houses table.
			if err := database.EnsureHousesTables(ctx); err != nil {
				log.Warn("ensure houses table", "err", err)
			}

		imbPath := filepath.Join(filepath.Dir(filepath.Dir(o.appearances)), "XML", "imbuements.xml")
	if imbReg, err := imbuements.LoadRegistry(imbPath); err != nil {
		log.Warn("imbuements not loaded", "path", imbPath, "err", err)
	} else {
		world.Imbuements = imbReg
		log.Info("imbuements loaded", "imbuements", len(imbReg.GetAllImbuements()))
	}

	creatureTypes := creatures.NewTypeRegistry()
	world.TypeRegistry = creatureTypes
	if cfg.DataPack != "" {
		// XML fallback loader (legacy format). The otservbr data pack is Lua, so
		// this typically loads nothing; the real load happens later via the Lua
		// engine (Game.createMonsterType/register) in loadScripts.
		if err := creatureTypes.LoadMonsters(filepath.Join(cfg.DataPack, "monster")); err != nil {
			log.Warn("loading monster types (xml fallback)", "err", err)
		} else if n := len(creatureTypes.Monsters); n > 0 {
			log.Info("loaded monster types (xml fallback)", "count", n)
		}
		if err := creatureTypes.LoadNpcs(filepath.Join(cfg.DataPack, "npc")); err != nil {
			log.Warn("loading npc types", "err", err)
		} else {
			log.Info("loaded npc types", "count", len(creatureTypes.Npcs))
		}
	}

	// Load vocations (base speed, attack speed, HP/mana/cap gains). Without this
	// the registry is empty and players fall back to defaults (base speed 110).
	if cfg.Core != "" {
		vocFile := filepath.Join(cfg.Core, "XML", "vocations.xml")
		if err := vocations.LoadVocations(vocFile); err != nil {
			log.Warn("vocations not loaded", "path", vocFile, "err", err)
		} else {
			log.Info("loaded vocations", "path", vocFile)
		}

		mountsFile := filepath.Join(cfg.Core, "XML", "mounts.xml")
		if err := mounts.Load(mountsFile); err != nil {
			log.Warn("mounts not loaded", "path", mountsFile, "err", err)
		} else {
			log.Info("loaded mounts", "path", mountsFile, "count", len(mounts.All()))
		}
	}

	spawnEngine := game.NewSpawnEngine(world, creatureTypes)
	aiEngine := game.NewAIEngine(world)
	combatEngine := game.NewCombatEngine(world)

	mapFilePath := o.mapFile
	if mapFilePath == "" && cfg.WorldFile != "" {
		mapFilePath = cfg.WorldFile
	}

	// generateSyntheticField is the playable fallback used when no OTBM map is
	// configured, or when the configured map can't be loaded (e.g. the datapack
	// ships without the main otservbr.otbm). It mirrors the C++ "spawn field".
	generateSyntheticField := func() game.Position {
		spawn := game.Position{X: 1000, Y: 1000, Z: 7}
		if temple, err := database.TownTemple(ctx, 1); err == nil && (temple.X != 0 || temple.Y != 0) {
			spawn = temple
		}
		world.Map.GenerateFlatField(spawn, 40, 4526) // grass field
		log.Info("generated synthetic spawn field", "center", spawn, "radius", 40)
		return spawn
	}

	var spawn game.Position
	var loadedMap bool
	var monsterSpawnFile string
	var npcSpawnFile string
	if mapFilePath != "" {
		if catalog == nil {
			return fmt.Errorf("map loading requires a valid appearances.dat (item metadata)")
		}
		// Retry the load a few times: on Docker Desktop (macOS) a large
		// bind-mounted OTBM can be briefly unavailable right after container
		// start, which would otherwise silently drop the server onto the
		// synthetic field (sending every player/temple to 1000,1000).
		var res *otbm.Result
		var err error
		for attempt := 1; attempt <= 5; attempt++ {
			res, err = otbm.Load(mapFilePath, catalog, world.Map)
			if err == nil {
				break
			}
			log.Warn("OTBM load attempt failed; retrying", "file", mapFilePath, "attempt", attempt, "err", err)
			time.Sleep(1 * time.Second)
		}
		if err != nil {
			// A missing or unreadable map must not crash the server: the working
			// vertical slice falls back to the synthetic spawn field so login and
			// gameplay still work without the (large, unshipped) OTBM map.
			log.Warn("OTBM map not loaded; falling back to synthetic spawn field",
				"file", mapFilePath, "err", err)
			spawn = generateSyntheticField()
		} else {
			loadedMap = true
			log.Info("loaded OTBM map", "file", mapFilePath, "tiles", res.TileCount,
				"items", res.ItemCount, "towns", len(res.Towns))

			// Parse spawn files (use OTBM header attributes if present, otherwise mapBase fallback)
			mapDir := filepath.Dir(mapFilePath)
			mapBase := strings.TrimSuffix(mapFilePath, filepath.Ext(mapFilePath))

			monsterSpawnFile = mapBase + "-monster.xml"
			if res.SpawnMonFile != "" {
				monsterSpawnFile = filepath.Join(mapDir, res.SpawnMonFile)
			}
			npcSpawnFile = mapBase + "-npc.xml"
			if res.SpawnNPCFile != "" {
				npcSpawnFile = filepath.Join(mapDir, res.SpawnNPCFile)
			}

			for _, t := range res.Towns {
				world.Towns[strings.ToLower(t.Name)] = t.Pos
				world.TownsByID[uint16(t.ID)] = t.Pos
				world.TownNames[uint16(t.ID)] = t.Name
			}

			// Load houses from the house XML file (OTBM reference).
			if res.HouseFile != "" {
				housePath := filepath.Join(mapDir, res.HouseFile)
				houses, err := database.ParseHouseFile(housePath)
				if err != nil {
					log.Warn("parse house file", "path", housePath, "err", err)
				} else {
					for _, h := range houses {
						// Insert/update the house in DB and register in world.
						if err := database.SaveHouse(ctx, &h); err != nil {
							log.Warn("save house", "id", h.ID, "err", err)
							continue
						}
						w := h
						world.RegisterHouse(&w)
					}
					log.Info("loaded houses from XML", "count", len(houses))
				}
			}
			if len(res.Towns) > 0 {
				spawn = res.Towns[0].Pos
				log.Info("spawn set to town temple", "town", res.Towns[0].Name, "pos", spawn)
			} else {
				spawn = game.Position{X: res.Width / 2, Y: res.Height / 2, Z: 7}
			}
		}
	}
	if !loadedMap && mapFilePath == "" {
		spawn = generateSyntheticField()
	}
	world.DefaultSpawn = spawn
	world.StartDecayingMap()

	// Load houses from DB and register their map tiles.
	if err := database.LoadHouses(ctx, world); err != nil {
		log.Warn("load houses", "err", err)
	} else {
		world.RegisterHouseTiles()
		log.Info("houses loaded and tiles registered", "count", len(world.Houses))
	}

		// Load houses from DB and register their map tiles.

	var lengine *luaengine.Engine

	world.OnCreatureMove = func(c game.Creature, oldPos game.Position, newPos game.Position, oldTileIndex int) {
		protocol.BroadcastCreatureMove(world, c, oldPos, newPos, oldTileIndex)
		if lengine != nil {
			if player, ok := c.(*game.Player); ok {
				for _, cr := range world.Creatures() {
					if npc, ok := cr.(*game.Npc); ok && npc.IsInteractingWithPlayer(player.ID) {
						dist := player.GetPosition().MaxDistance(npc.GetPosition())
						if dist < 0 || dist > 3 {
							targetNpc, targetPlayer := npc, player
							game.GlobalDispatcher.AddEvent(0, func() {
								lengine.CallNpcCloseChannel(targetNpc, targetPlayer)
							})
							npc.RemovePlayerInteraction(player.ID)
						}
					}
				}
			} else if npc, ok := c.(*game.Npc); ok {
				for _, playerID := range npc.InteractingPlayers() {
					if player := world.PlayerByID(playerID); player != nil {
						dist := player.GetPosition().MaxDistance(npc.GetPosition())
						if dist < 0 || dist > 3 {
							targetNpc, targetPlayer := npc, player
							game.GlobalDispatcher.AddEvent(0, func() {
								lengine.CallNpcCloseChannel(targetNpc, targetPlayer)
							})
							npc.RemovePlayerInteraction(player.ID)
						}
					}
				}
			}
		}
	}
	world.OnCreatureAppear = func(c game.Creature) {
		protocol.BroadcastCreatureAppear(world, c)
	}
	world.OnCreatureRemove = func(c game.Creature) {
		protocol.BroadcastCreatureRemove(world, c)
	}
	world.OnGhostModeChange = func(p *game.Player) {
		protocol.BroadcastGhostModeChange(world, p)
	}
	world.OnCreatureHealthChange = func(c game.Creature) {
		protocol.BroadcastCreatureHealth(world, c)
	}
	world.OnCombatHit = func(attacker, victim game.Creature, damage int32, effect uint16) {
		protocol.BroadcastCombatHit(world, attacker, victim, damage, effect)
	}
	world.OnItemAppear = func(pos game.Position, item *game.Item) {
		protocol.BroadcastAddItem(world, pos, item)
	}
	world.OnItemDecay = func(pos game.Position, stackPos uint8, oldItem, newItem *game.Item) {
		protocol.BroadcastItemDecay(world, pos, stackPos, oldItem, newItem)
	}
	world.OnTargetLost = func(p *game.Player) {
		protocol.SendCancelTarget(p)
	}
	world.OnChangeSpeed = func(c game.Creature) {
		go protocol.BroadcastChangeSpeed(world, c)
	}
	world.OnIconsUpdate = func(p *game.Player) {
		if p.Session != nil {
			p.Session.SendIcons()
		}
	}
	world.OnBosstiaryEntryChanged = func(p *game.Player, bossRaceID uint16) {
		if gp, ok := p.Session.(*protocol.GameProtocol); ok {
			gp.SendBosstiaryEntryChanged(uint32(bossRaceID))
		}
	}
	world.OnBestiaryEntryChanged = func(p *game.Player, raceID uint16) {
		if gp, ok := p.Session.(*protocol.GameProtocol); ok {
			gp.SendBestiaryEntryChanged(raceID)
		}
	}
	world.OnPlayerStatsChange = func(p *game.Player) {
		protocol.SendPlayerStats(p)
	}
	world.OnTextMessage = func(p *game.Player, class uint8, value uint64, text string) {
		protocol.SendExpMessage(p, value, text)
	}
	world.OnPlayerDeath = func(p *game.Player, killer game.Creature) {
		if events.GlobalEngine != nil {
			events.GlobalEngine.ExecuteOnDeath(p, killer)
		}
		protocol.HandlePlayerDeath(world, p, killer)
		// Persist the penalty immediately so a crash/relog can't revert it.
		if err := database.SavePlayer(context.Background(), p); err != nil {
			log.Warn("save on death failed", "player", p.Name, "err", err)
		}
	}
	world.OnGainExperience = func(p *game.Player, source game.Creature, exp uint64, rawExp uint64) uint64 {
		if events.GlobalEngine != nil {
			return events.GlobalEngine.ExecuteOnGainExperience(p, source, exp, rawExp)
		}
		return exp
	}
	world.OnShieldUpdate = func(viewer, target *game.Player) {
		protocol.SendPartyShield(viewer, target)
	}
	world.OnMagicEffect = func(pos game.Position, effect uint16) {
		protocol.BroadcastMagicEffect(world, pos, effect)
	}
	world.OnDistanceEffect = func(from, to game.Position, effect uint16) {
		protocol.BroadcastDistanceEffect(world, from, to, effect)
	}
	// The spell system resolves spell damage/heal through the combat engine.
	world.Combat = combatEngine


	// Lua engine.
	lengine = luaengine.New(world, log)
	lengine.SetDB(database)
	defer lengine.Close()
	lengine.SetGameFunc("getPlayerCount", func(L *lua.LState) int {
		L.Push(lua.LNumber(world.OnlineCount()))
		return 1
	})
	// Core engine data lives in the base `data/` tree (not the world datapack):
	// data/lib + data/npclib define framework classes (e.g. KeywordHandler) that
	
	world.OnCreatureSay = func(speaker game.Creature, talkType byte, text string) {
		protocol.BroadcastCreatureSay(world, speaker, talkType, text)
		
		if player, ok := speaker.(*game.Player); ok {
			spectators := world.SpectatorCreatures(speaker.GetPosition())
			for _, spec := range spectators {
				if npc, ok := spec.(*game.Npc); ok {
					targetNpc, targetPlayer, tType, txt := npc, player, talkType, text
					game.GlobalDispatcher.AddEvent(0, func() {
						lengine.CallNpcOnCreatureSay(targetNpc, targetPlayer, tType, txt)
					})
				}
			}
		}
	}
	// The datapack ships its own lib/ (data-otservbr-global/lib) that defines
	// Storage, Storage.Quest and the quest/boss constants that the core npclib
	// (e.g. data/npclib/npc_system/custom_modules.lua) AND the datapack scripts
	// reference. In the C++ flow data/global.lua dofiles it via DATA_DIRECTORY
	// before the rest; the Go loader bypasses that bootstrap, so load it FIRST
	// (with DATA_DIRECTORY set for the scripts' own dofile chains) — before the
	// core npclib and the datapack scripts/monsters/npcs that depend on Storage.
	if cfg.DataPack != "" {
		lengine.L.SetGlobal("DATA_DIRECTORY", lua.LString(cfg.DataPack))
		if err := loadScripts(lengine, filepath.Join(cfg.DataPack, "lib"), log); err != nil {
			log.Warn("loading datapack lib", "err", err)
		}
	}
	// datapack npc scripts require, and data/scripts/spells holds the player
	// vocation spells. Load these (libs first) BEFORE the datapack scripts/npcs.
	coreData := filepath.Dir(filepath.Dir(o.appearances)) // e.g. ../data
	if coreData != "" && coreData != cfg.DataPack {
		lengine.L.SetGlobal("CORE_DIRECTORY", lua.LString(coreData))
		// data/global.lua + stages.lua are the C++ bootstrap (dofiled by
		// data/core.lua). They define base helpers/constants — IsTravelFree,
		// IsRetroPVP, NORTH/EAST direction aliases, rate globals — that the
		// npclib modules rely on (e.g. StdModule.say calls IsTravelFree()).
		// Without them those globals are nil and every keyword reply errors.
		for _, f := range []string{"global.lua", "stages.lua"} {
			p := filepath.Join(coreData, f)
			if _, err := os.Stat(p); err == nil {
				if err := lengine.DoFile(p); err != nil {
					log.Warn("loading core bootstrap", "file", p, "err", err)
				}
			}
		}
		// Load core module scripts that are required by action scripts (e.g.
		// daily_reward module defines the DailyReward global used by the shrine
		// action). These must be loaded BEFORE the core scripts directory.
		modulesDir := filepath.Join(coreData, "modules", "scripts")
		if entries, err := os.ReadDir(modulesDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() && entry.Name() != "gamestore" {
					modFile := filepath.Join(modulesDir, entry.Name(), entry.Name()+".lua")
					if _, statErr := os.Stat(modFile); statErr == nil {
						if err := lengine.DoFile(modFile); err != nil {
							log.Warn("loading module script", "module", entry.Name(), "err", err)
						}
					}
				}
			}
		} else {
			log.Warn("modules scripts dir not found", "dir", modulesDir)
		}
		for _, sub := range []string{"lib", "libs", "npclib", "scripts"} {
			d := filepath.Join(coreData, sub)
			if err := loadScripts(lengine, d, log); err != nil {
				log.Warn("loading core data", "dir", d, "err", err)
			}
		}
	}
	if err := loadScripts(lengine, scriptsDir, log); err != nil {
		log.Warn("loading scripts", "err", err)
	}
	// In-game store: the gamestore Lua module lives under the CORE data dir
	// (data/modules/scripts/gamestore), not the datapack. It bootstraps its own
	// libs via dofile/require, so load its entry point explicitly.
	storeBase := coreData
	if storeBase == "" {
		storeBase = cfg.Core
	}
	gs := filepath.Join(storeBase, "modules", "scripts", "gamestore", "gamestore.lua")
	if _, statErr := os.Stat(gs); statErr == nil {
		if err := lengine.DoFile(gs); err != nil {
			log.Warn("loading gamestore module", "err", err)
		} else {
			lengine.SyncStoreGlobal()
			log.Info("loaded in-game store module", "path", gs)
			lengine.LogStoreCatalogStatus()
		}
	} else {
		log.Warn("gamestore module not found", "path", gs)
	}
	log.Info("registered actions (lua)", "count", actions.Count())
	log.Info("registered instant spells (lua)", "count", spells.Count())
	if err := loadScripts(lengine, filepath.Join(cfg.DataPack, "monster"), log); err != nil {
		log.Warn("loading monsters", "err", err)
	}
	// The real monster data is Lua (executed above via createMonsterType/register),
	// not the XML the earlier LoadMonsters call scans. Resolve loot entries given
	// by name to item ids now that both the catalog and the registry are loaded,
	// then log the real count.
	resolveMonsterLoot(creatureTypes, catalog, log)
	log.Info("loaded monster types (lua)", "count", len(creatureTypes.Monsters))

	// Daily boosted boss & monster: pick (date-seeded random) + persist to the
	// DB once per day now that the monster/boss types are loaded. Same date =>
	// same pick, shared via the DB with MyAAC and other sessions.
	if bb, err := database.RotateBoostedBoss(ctx, creatureTypes); err == nil && bb != "" && bb != "default" {
		world.BoostedBoss = bb

		log.Info("boosted boss", "name", bb)
	}
	if bc, err := database.RotateBoostedCreature(ctx, creatureTypes); err == nil && bc != "" && bc != "default" {
		world.BoostedCreature = bc
		log.Info("boosted creature", "name", bc)
	}

	if err := loadScripts(lengine, filepath.Join(cfg.DataPack, "npc"), log); err != nil {
		log.Warn("loading npcs", "err", err)
	}

	// Now that Lua scripts have populated the monster and npc type registries,
	// load the spawns into the spawn engine to generate the initial map creatures.
	if loadedMap && mapFilePath != "" {
		if monsterSpawnFile != "" {
			if spawnsData, err := spawns.LoadSpawnFile(monsterSpawnFile); err == nil {
				spawnEngine.LoadSpawns(spawnsData)
				log.Info("loaded monster spawns", "file", monsterSpawnFile)
			} else {
				log.Warn("monster spawn file not loaded", "file", monsterSpawnFile, "err", err)
			}
		}
		if npcSpawnFile != "" {
			if spawnsData, err := spawns.LoadSpawnFile(npcSpawnFile); err == nil {
				spawnEngine.LoadSpawns(spawnsData)
				log.Info("loaded npc spawns", "file", npcSpawnFile)
			} else {
				log.Warn("npc spawn file not loaded", "file", npcSpawnFile, "err", err)
			}
		}
	}

	lengine.RunStartupGlobalEvents()

	// Start the background globalevent scheduler (think/time events).
	lengine.StartGlobalEventScheduler(ctx)

	eventsEngine := events.NewEngine(lengine.L)

	spawnEngine.Start()
	aiEngine.Start()
	combatEngine.Start()

	deps := &protocol.Deps{
		Cfg: cfg, DB: database, RSA: rsa, World: world, Items: catalog, Lua: lengine, Events: eventsEngine, Log: log,
	}

	// Async job worker (PostgreSQL LISTEN/NOTIFY queue).
	go func() {
		if err := database.RunJobWorker(ctx, log, jobHandler(database, log)); err != nil {
			log.Warn("job worker stopped", "err", err)
		}
	}()

	// Services.
		loginSvc := network.NewService("login", fmt.Sprintf(":%d", cfg.LoginPort),
			cfg.ServerName, protocol.NewLoginFactory(deps), log)
		gameSvc := network.NewService("game", fmt.Sprintf(":%d", cfg.GamePort),
			cfg.ServerName, protocol.NewGameFactory(deps), log)

		errCh := make(chan error, 3)
		go func() { errCh <- loginSvc.Start(ctx) }()
		go func() { errCh <- gameSvc.Start(ctx) }()
		if cfg.StatusPort != cfg.LoginPort {
			statusSvc := network.NewService("status", fmt.Sprintf(":%d", cfg.StatusPort),
				cfg.ServerName, protocol.NewStatusFactory(deps), log)
			go func() { errCh <- statusSvc.Start(ctx) }()
		}

	log.Info("canary-go is running", "login", cfg.LoginPort, "game", cfg.GamePort)

	select {
	case <-ctx.Done():
		log.Info("shutting down")
		return nil
	case err := <-errCh:
		return err
	}
}

// loadScripts runs every .lua file in dir (recursive) through the engine.
func loadScripts(e *luaengine.Engine, dir string, log *slog.Logger) error {
	absDir, _ := filepath.Abs(dir)
	log.Info("loadScripts starting walkthrough", "dir", dir, "absDir", absDir)

	libDir := filepath.Join(dir, "lib")
	// load lib first
	_ = filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignore if lib doesn't exist
		}
		if !d.IsDir() && filepath.Ext(path) == ".lua" {
			if err := e.DoFile(path); err != nil {
				log.Warn("script error", "file", path, "err", err)
			}
		}
		return nil
	})

	// Special case for npclib to ensure npc_handler and modules are loaded in order before custom_modules
	if filepath.Base(dir) == "npclib" {
		for _, primary := range []string{
			filepath.Join(dir, "npc_system", "npc_handler.lua"),
			filepath.Join(dir, "npc_system", "modules.lua"),
			filepath.Join(dir, "npc_system", "custom_modules.lua"),
		} {
			if _, err := os.Stat(primary); err == nil {
				if err := e.DoFile(primary); err != nil {
					log.Warn("script error", "file", primary, "err", err)
				}
			}
		}
	}

	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			log.Warn("walking script directory entry failed; skipping", "path", path, "err", err)
			return nil
		}
		if d.IsDir() && d.Name() == "lib" && path == libDir {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			if filepath.Ext(path) == ".lua" {
				// Skip pre-loaded core npclib files
				if filepath.Base(dir) == "npclib" && (strings.HasSuffix(path, "npc_handler.lua") || strings.HasSuffix(path, "modules.lua") || strings.HasSuffix(path, "custom_modules.lua")) {
					return nil
				}
				if err := e.DoFile(path); err != nil {
					log.Warn("script error", "file", path, "err", err)
				}
			}
		}
		return nil
	})

	if walkErr != nil {
		log.Warn("loadScripts WalkDir finished with error", "dir", dir, "err", walkErr)
	} else {
		log.Info("loadScripts WalkDir finished successfully", "dir", dir)
	}
	return walkErr
}

// resolveMonsterLoot resolves loot entries declared by name (e.g. "gold coin")
// to item ids using the item catalog, mirroring the name lookup the C++ loot
// loader performs. Entries with an explicit id are left as-is.
func resolveMonsterLoot(reg *creatures.TypeRegistry, catalog *items.Catalog, log *slog.Logger) {
	if reg == nil || catalog == nil {
		return
	}
	var unresolved int
	var walk func(loot []creatures.LootBlock)
	walk = func(loot []creatures.LootBlock) {
		for i := range loot {
			lb := &loot[i]
			if lb.ID == 0 && lb.Name != "" {
				if id, ok := catalog.IDByName(lb.Name); ok {
					lb.ID = id
				} else {
					unresolved++
				}
			}
			if len(lb.ChildLoot) > 0 {
				walk(lb.ChildLoot)
			}
		}
	}
	for _, mt := range reg.Monsters {
		walk(mt.Loot)
	}
	if unresolved > 0 {
		log.Warn("some monster loot names could not be resolved to item ids", "count", unresolved)
	}
}

// jobHandler processes async jobs pulled from the PostgreSQL queue.
func jobHandler(_ *db.DB, log *slog.Logger) db.JobHandler {
	return func(ctx context.Context, kind string, payload json.RawMessage) error {
		log.Info("processing async job", "kind", kind, "payload", string(payload))
		return nil
	}
}
