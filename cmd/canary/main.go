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
	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/luaengine"
	"github.com/opentibiabr/canary-go/internal/network"
	"github.com/opentibiabr/canary-go/internal/otbm"
	"github.com/opentibiabr/canary-go/internal/protocol"
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
		log.Info("item catalog loaded", "types", cat.Len())
	}

	// World: real OTBM map if provided, else a synthetic spawn field.
	world := game.NewWorld()
	var spawn game.Position
	if o.mapFile != "" {
		if catalog == nil {
			return fmt.Errorf("map loading requires a valid appearances.dat (item metadata)")
		}
		res, err := otbm.Load(o.mapFile, catalog, world.Map)
		if err != nil {
			return fmt.Errorf("load map: %w", err)
		}
		log.Info("loaded OTBM map", "file", o.mapFile, "tiles", res.TileCount,
			"items", res.ItemCount, "towns", len(res.Towns))
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

	// Lua engine.
	lengine := luaengine.New(log)
	defer lengine.Close()
	lengine.SetGameFunc("getPlayerCount", func(L *lua.LState) int {
		L.Push(lua.LNumber(world.OnlineCount()))
		return 1
	})
	if err := loadScripts(lengine, scriptsDir, log); err != nil {
		log.Warn("loading scripts", "err", err)
	}

	deps := &protocol.Deps{
		Cfg: cfg, DB: database, RSA: rsa, World: world, Items: catalog, Lua: lengine, Log: log,
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

// loadScripts runs every .lua file in dir (non-recursive) through the engine.
func loadScripts(e *luaengine.Engine, dir string, log *slog.Logger) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".lua" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := e.DoFile(path); err != nil {
			log.Warn("script error", "file", path, "err", err)
			continue
		}
		log.Info("loaded script", "file", path)
	}
	return nil
}

// jobHandler processes async jobs pulled from the PostgreSQL queue.
func jobHandler(_ *db.DB, log *slog.Logger) db.JobHandler {
	return func(ctx context.Context, kind string, payload json.RawMessage) error {
		log.Info("processing async job", "kind", kind, "payload", string(payload))
		return nil
	}
}
