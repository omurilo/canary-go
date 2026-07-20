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

	lua "github.com/yuin/gopher-lua"

	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/events"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/spawns"
	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/luaengine"
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
		mapFile    = flag.String("map", "", "path to an OTBM map file (empty = synthetic spawn field)")
		migrate    = flag.Bool("migrate", true, "apply the schema on startup (idempotent)")
		seed       = flag.Bool("seed", false, "seed a test account (god/god, char 'Gm Test')")
	)
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
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

	spawnEngine := game.NewSpawnEngine(world, creatureTypes)
	aiEngine := game.NewAIEngine(world)
	combatEngine := game.NewCombatEngine(world)

	mapFilePath := o.mapFile
	if mapFilePath == "" && cfg.WorldFile != "" {
		mapFilePath = cfg.WorldFile
	}

	var spawn game.Position
	if mapFilePath != "" {
		if catalog == nil {
			return fmt.Errorf("map loading requires a valid appearances.dat (item metadata)")
		}
		res, err := otbm.Load(mapFilePath, catalog, world.Map)
		if err != nil {
			return fmt.Errorf("load map: %w", err)
		}
		log.Info("loaded OTBM map", "file", mapFilePath, "tiles", res.TileCount,
			"items", res.ItemCount, "towns", len(res.Towns))
		
		// Parse spawn files
		mapBase := strings.TrimSuffix(mapFilePath, filepath.Ext(mapFilePath))
		
		monsterFile := mapBase + "-monster.xml"
		if spawnsData, err := spawns.LoadSpawnFile(monsterFile); err == nil {
			spawnEngine.LoadSpawns(spawnsData)
			log.Info("loaded monster spawns", "file", monsterFile)
		} else {
			log.Warn("monster spawn file not loaded", "file", monsterFile, "err", err)
		}
		
		npcFile := mapBase + "-npc.xml"
		if spawnsData, err := spawns.LoadSpawnFile(npcFile); err == nil {
			spawnEngine.LoadSpawns(spawnsData)
			log.Info("loaded npc spawns", "file", npcFile)
		} else {
			log.Warn("npc spawn file not loaded", "file", npcFile, "err", err)
		}

		for _, t := range res.Towns {
			world.Towns[strings.ToLower(t.Name)] = t.Pos
		}
		if len(res.Towns) > 0 {
			spawn = res.Towns[0].Pos
			log.Info("spawn set to town temple", "town", res.Towns[0].Name, "pos", spawn)
		} else {
			spawn = game.Position{X: res.Width / 2, Y: res.Height / 2, Z: 7}
		}
	} else {
		spawn = game.Position{X: 1000, Y: 1000, Z: 7}
		if temple, err := database.TownTemple(ctx, 1); err == nil && (temple.X != 0 || temple.Y != 0) {
			spawn = temple
		}
		world.Map.GenerateFlatField(spawn, 40, 4526) // grass field
		log.Info("generated synthetic spawn field", "center", spawn, "radius", 40)
	}
	world.DefaultSpawn = spawn
	world.OnCreatureMove = func(c game.Creature, oldPos game.Position, newPos game.Position, oldTileIndex int) {
		protocol.BroadcastCreatureMove(world, c, oldPos, newPos, oldTileIndex)
	}
	world.OnCreatureAppear = func(c game.Creature) {
		protocol.BroadcastCreatureAppear(world, c)
	}
	world.OnCreatureRemove = func(c game.Creature) {
		protocol.BroadcastCreatureRemove(world, c)
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
	world.OnTargetLost = func(p *game.Player) {
		protocol.SendCancelTarget(p)
	}
	world.OnPlayerStatsChange = func(p *game.Player) {
		protocol.SendPlayerStats(p)
	}
	world.OnMagicEffect = func(pos game.Position, effect uint16) {
		protocol.BroadcastMagicEffect(world, pos, effect)
	}
	world.OnDistanceEffect = func(from, to game.Position, effect uint16) {
		protocol.BroadcastDistanceEffect(world, from, to, effect)
	}
	// The spell system resolves spell damage/heal through the combat engine.
	world.Combat = combatEngine

	spawnEngine.Start()
	aiEngine.Start()
	combatEngine.Start()

	// Lua engine.
	lengine := luaengine.New(world, log)
	defer lengine.Close()
	lengine.SetGameFunc("getPlayerCount", func(L *lua.LState) int {
		L.Push(lua.LNumber(world.OnlineCount()))
		return 1
	})
	// Core engine data lives in the base `data/` tree (not the world datapack):
	// data/lib + data/npclib define framework classes (e.g. KeywordHandler) that
	// datapack npc scripts require, and data/scripts/spells holds the player
	// vocation spells. Load these (libs first) BEFORE the datapack scripts/npcs.
	coreData := filepath.Dir(filepath.Dir(o.appearances)) // e.g. ../data
	if coreData != "" && coreData != cfg.DataPack {
		for _, sub := range []string{"lib", "npclib", "scripts"} {
			d := filepath.Join(coreData, sub)
			if err := loadScripts(lengine, d, log); err != nil {
				log.Warn("loading core data", "dir", d, "err", err)
			}
		}
	}
	if err := loadScripts(lengine, scriptsDir, log); err != nil {
		log.Warn("loading scripts", "err", err)
	}
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
	if err := loadScripts(lengine, filepath.Join(cfg.DataPack, "npc"), log); err != nil {
		log.Warn("loading npcs", "err", err)
	}

	eventsEngine := events.NewEngine(lengine.L)

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

	errCh := make(chan error, 2)
	go func() { errCh <- loginSvc.Start(ctx) }()
	go func() { errCh <- gameSvc.Start(ctx) }()

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
	libDir := filepath.Join(dir, "lib")
	// load lib first
	filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignore if lib doesn't exist
		}
		if !d.IsDir() && filepath.Ext(path) == ".lua" {
			if err := e.DoFile(path); err != nil {
				log.Warn("script error", "file", path, "err", err)
			} else {
				log.Debug("loaded script", "file", path)
			}
		}
		return nil
	})

	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "lib" && path == libDir {
			return filepath.SkipDir
		}
		if !d.IsDir() && filepath.Ext(path) == ".lua" {
			if err := e.DoFile(path); err != nil {
				log.Warn("script error", "file", path, "err", err)
			} else {
				log.Debug("loaded script", "file", path)
			}
		}
		return nil
	})
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
