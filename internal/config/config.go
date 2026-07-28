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

// normalizeKey matches the key transform applied when populating Custom
// (lower-cased, underscores stripped), so callers can use the config.lua name.
func normalizeKey(key string) string {
	return strings.ReplaceAll(strings.ToLower(key), "_", "")
}

// Number returns the integer value of a config.lua key (by its config.lua name),
// or def when the key is absent or not numeric. Mirrors g_configManager().getNumber.
func (c *Config) Number(key string, def int64) int64 {
	if c == nil || c.Custom == nil {
		return def
	}
	if v, ok := c.Custom[normalizeKey(key)]; ok {
		if n, ok := v.(lua.LNumber); ok {
			return int64(n)
		}
	}
	return def
}

// Bool returns the boolean value of a config.lua key, or def when absent.
func (c *Config) Bool(key string, def bool) bool {
	if c == nil || c.Custom == nil {
		return def
	}
	if v, ok := c.Custom[normalizeKey(key)]; ok {
		if b, ok := v.(lua.LBool); ok {
			return bool(b)
		}
	}
	return def
}

// Number reads an integer config value from the active configuration.
func Number(key string, def int64) int64 { return Active.Number(key, def) }

// Bool reads a boolean config value from the active configuration.
func Bool(key string, def bool) bool { return Active.Bool(key, def) }

// Config holds the subset of settings the Go server currently uses. Unknown
// keys in config.lua are simply ignored.
type Config struct {
	ServerName string
	IP         string

	LoginPort    int
	GamePort     int
	StatusPort   int
	Legacy1100Port int // legacy 11.00 game protocol port (0 = disabled)
	Legacy860Port  int // legacy 8.60 game protocol port (0 = disabled)

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

	// MapDownloadURL is the URL to download the OTBM map file from when it
	// is not found locally. Mirrors CANARY_MAP_URL from the C++ docker stack.
	// Default: "https://github.com/opentibiabr/canary/releases/download/v3.6.1/otservbr.otbm"
	MapDownloadURL string

	// MapDownloadEnabled controls whether the server auto-downloads a missing
	// OTBM map file on startup. Default true.
	MapDownloadEnabled bool

	Custom map[string]lua.LValue
}

// Default returns a config with sane defaults for local development.
func Default() *Config {
	return &Config{
		ServerName:    "Canary-Go",
		IP:            "127.0.0.1",
		LoginPort:     7171,
		GamePort:      7172,
		StatusPort:    7171,
		Legacy1100Port: 0, // disabled by default; enable via config.lua or env
		Legacy860Port:  0, // disabled by default; enable via config.lua or env
		DataPack:      "data-otservbr-global",
		Core:          "data",
		DBHost:        "127.0.0.1",
		DBUser:        "canary",
		DBPassword:    "canary",
		DBName:        "canary",
		DBPort:        3306,
		MOTD:               "Welcome to Canary-Go!",
		AllowOldProto:      true,
		RSAKeyFile:         "key.pem",
		MapDownloadURL:     "https://github.com/opentibiabr/canary/releases/download/v3.6.1/otservbr.otbm",
		MapDownloadEnabled: true,
		Custom:             make(map[string]lua.LValue),
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
	cfg.Legacy1100Port = num("legacy1100GameProtocolPort", cfg.Legacy1100Port)
	cfg.Legacy860Port = num("legacy860GameProtocolPort", cfg.Legacy860Port)
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

	// Map download settings (matches CANARY_MAP_URL in the docker stack).
	cfg.MapDownloadURL = str("mapDownloadUrl", cfg.MapDownloadURL)
	cfg.MapDownloadEnabled = boolean("mapDownloadEnabled", cfg.MapDownloadEnabled)

	applyEnv(cfg)

	// Build the world file path after env overrides so that CANARY_DATA_PACK
	// takes effect on the derived path when worldFile is not explicitly set.
	cfg.WorldFile = str("worldFile", "")
	if cfg.WorldFile == "" {
		mapName := str("mapName", "otservbr")
		cfg.WorldFile = fmt.Sprintf("%s/world/%s.otbm", cfg.DataPack, mapName)
	}

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
	if v := os.Getenv("CANARY_DATA_PACK"); v != "" {
		cfg.DataPack = v
	}
	if v := os.Getenv("CANARY_LEGACY_1100_GAME_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Legacy1100Port = n
		}
	}
	if v := os.Getenv("CANARY_LEGACY_860_GAME_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Legacy860Port = n
		}
	}
	if v := os.Getenv("CANARY_MAP_URL"); v != "" {
		cfg.MapDownloadURL = v
	}
	if v := os.Getenv("CANARY_MAP_DOWNLOAD"); v != "" {
		cfg.MapDownloadEnabled = v == "true" || v == "1"
	}
}
