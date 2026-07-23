// Package config loads the server configuration from a Lua config file (the same
// config.lua the C++ server uses) by executing it and reading its globals.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// Active is a package-level variable holding the currently loaded configuration.
var Active *Config = Default()

// Config holds the subset of settings the Go server currently uses. Unknown
// keys in config.lua are simply ignored.
type Config struct {
	ServerName string
	IP         string

	LoginPort  int
	GamePort   int
	StatusPort int

	DataPack string // dataPackDirectory
	Core     string // coreDirectory

	// MariaDB/MySQL connection (shared with MyAAC and the login-server).
	DBHost     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPort     int

	MOTD          string
	AllowOldProto bool
	AutoBank      bool

	RSAKeyFile string
	WorldFile  string

	Custom     map[string]lua.LValue
}

// Default returns a config with sane defaults for local development.
func Default() *Config {
	return &Config{
		ServerName:    "Canary-Go",
		IP:            "127.0.0.1",
		LoginPort:     7171,
		GamePort:      7172,
		StatusPort:    7171,
		DataPack:      "data-otservbr-global",
		Core:          "data",
		DBHost:        "127.0.0.1",
		DBUser:        "canary",
		DBPassword:    "canary",
		DBName:        "canary",
		DBPort:        3306,
		MOTD:          "Welcome to Canary-Go!",
		AllowOldProto: true,
		RSAKeyFile:    "key.pem",
		Custom:        make(map[string]lua.LValue),
	}
}

// Load executes a Lua config file and overlays its values on the defaults, then
// applies environment overrides (default < config.lua < env). A missing file is
// not fatal: defaults + env still apply.
func Load(path string) (*Config, error) {
	cfg := Default()
	
	// Support sharing the configuration with the C++ server in the root directory.
	// If the default "config.lua" is used, check if there's a "../config.lua" first.
	resolvedPath := path
	if path == "config.lua" {
		if _, err := os.Stat("../config.lua"); err == nil {
			resolvedPath = "../config.lua"
		} else if _, err := os.Stat(path); err != nil {
			// fallback/error handled below
		}
	}

	if _, err := os.Stat(resolvedPath); err != nil {
		applyEnv(cfg)
		return cfg, fmt.Errorf("config: %w", err)
	}

	L := lua.NewState()
	defer L.Close()
	if err := L.DoFile(resolvedPath); err != nil {
		return nil, fmt.Errorf("config: executing %s: %w", resolvedPath, err)
	}
	g := L.Get(lua.GlobalsIndex).(*lua.LTable)

	str := func(key, def string) string {
		if v, ok := g.RawGetString(key).(lua.LString); ok {
			return string(v)
		}
		return def
	}
	num := func(key string, def int) int {
		if v, ok := g.RawGetString(key).(lua.LNumber); ok {
			return int(v)
		}
		return def
	}
	boolean := func(key string, def bool) bool {
		if v, ok := g.RawGetString(key).(lua.LBool); ok {
			return bool(v)
		}
		return def
	}

	cfg.ServerName = str("serverName", cfg.ServerName)
	cfg.IP = str("ip", cfg.IP)
	cfg.LoginPort = num("loginProtocolPort", cfg.LoginPort)
	cfg.GamePort = num("gameProtocolPort", cfg.GamePort)
	cfg.StatusPort = num("statusProtocolPort", cfg.StatusPort)
	cfg.DataPack = str("dataPackDirectory", cfg.DataPack)
	cfg.Core = str("coreDirectory", cfg.Core)

	cfg.DBHost = str("mysqlHost", cfg.DBHost)
	cfg.DBUser = str("mysqlUser", cfg.DBUser)
	cfg.DBPassword = str("mysqlPass", cfg.DBPassword)
	cfg.DBName = str("mysqlDatabase", cfg.DBName)
	cfg.DBPort = num("mysqlPort", cfg.DBPort)

	// Fallback to serverMotd if motd is not defined (to support config.lua.dist)
	cfg.MOTD = str("serverMotd", str("motd", cfg.MOTD))
	cfg.AllowOldProto = boolean("allowOldProtocol", cfg.AllowOldProto)
	cfg.AutoBank = boolean("autoBank", cfg.AutoBank)
	cfg.RSAKeyFile = str("rsaKeyFile", cfg.RSAKeyFile)

	// Fallback to constructing the world path from dataPackDirectory + mapName
	cfg.WorldFile = str("worldFile", "")
	if cfg.WorldFile == "" {
		mapName := str("mapName", "otservbr")
		cfg.WorldFile = fmt.Sprintf("%s/world/%s.otbm", cfg.DataPack, mapName)
	}

	applyEnv(cfg)

	cfg.Custom = make(map[string]lua.LValue)
	g.ForEach(func(k, v lua.LValue) {
		key := strings.ToLower(k.String())
		key = strings.ReplaceAll(key, "_", "")
		cfg.Custom[key] = v
	})
	Active = cfg

	return cfg, nil
}

// applyEnv overlays CANARY_* environment variables (used by the docker stack).
func applyEnv(cfg *Config) {
	if v := os.Getenv("CANARY_DB_HOST"); v != "" {
		cfg.DBHost = v
	}
	if v := os.Getenv("CANARY_DB_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DBPort = n
		}
	}
	if v := os.Getenv("CANARY_DB_NAME"); v != "" {
		cfg.DBName = v
	}
	if v := os.Getenv("CANARY_DB_USER"); v != "" {
		cfg.DBUser = v
	}
	if v := os.Getenv("CANARY_DB_PASSWORD"); v != "" {
		cfg.DBPassword = v
	}
	if v := os.Getenv("CANARY_SERVER_IP"); v != "" {
		cfg.IP = v
	}
	if v := os.Getenv("CANARY_SERVER_NAME"); v != "" {
		cfg.ServerName = v
	}
}
