// Package config loads the server configuration from a Lua config file (the same
// config.lua the C++ server uses) by executing it and reading its globals.
package config

import (
	"fmt"
	"os"
	"strconv"

	lua "github.com/yuin/gopher-lua"
)

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

	RSAKeyFile string
	WorldFile  string
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
	}
}

// Load executes a Lua config file and overlays its values on the defaults, then
// applies environment overrides (default < config.lua < env). A missing file is
// not fatal: defaults + env still apply.
func Load(path string) (*Config, error) {
	cfg := Default()
	if _, err := os.Stat(path); err != nil {
		applyEnv(cfg)
		return cfg, fmt.Errorf("config: %w", err)
	}

	L := lua.NewState()
	defer L.Close()
	if err := L.DoFile(path); err != nil {
		return nil, fmt.Errorf("config: executing %s: %w", path, err)
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

	cfg.MOTD = str("motd", cfg.MOTD)
	cfg.AllowOldProto = boolean("allowOldProtocol", cfg.AllowOldProto)
	cfg.RSAKeyFile = str("rsaKeyFile", cfg.RSAKeyFile)
	cfg.WorldFile = str("worldFile", cfg.WorldFile)

	applyEnv(cfg)
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
