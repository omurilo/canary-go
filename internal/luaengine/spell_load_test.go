package luaengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/spells"
	lua "github.com/yuin/gopher-lua"
)

// spellDataDir locates the otservbr spell scripts relative to the repo, or ""
// when not present (minimal checkouts).
func spellDataDir() string {
	c := filepath.Join("..", "..", "..", "data-otservbr-global", "scripts", "spells", "monster")
	if fi, err := os.Stat(c); err == nil && fi.IsDir() {
		return c
	}
	return ""
}

// TestLoadRealSpellScripts loads every real instant-spell script and asserts the
// large majority register cleanly (the loader must tolerate the full effect /
// condition / area surface the scripts use).
func TestLoadRealSpellScripts(t *testing.T) {
	dir := spellDataDir()
	if dir == "" {
		t.Skip("otservbr spell data not available")
	}
	e := newTestEngine()

	before := spells.Count()
	var files, errs int
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".lua" {
			return nil
		}
		files++
		if derr := e.DoFile(path); derr != nil {
			errs++
			if errs <= 5 {
				t.Logf("script error %s: %v", filepath.Base(path), derr)
			}
		}
		return nil
	})

	registered := spells.Count() - before
	t.Logf("spell scripts: files=%d errors=%d registered=%d", files, errs, registered)

	if registered == 0 {
		t.Fatalf("no spells registered from %d files", files)
	}
	// Allow a small number of scripts to fail (e.g. ones depending on unported
	// APIs) but the vast majority must load.
	if errs > files/10 {
		t.Errorf("too many spell script errors: %d/%d", errs, files)
	}
}

func TestConfigManager_UnderscoreStripping(t *testing.T) {
	e := newTestEngine()
	config.Active = &config.Config{
		Custom: map[string]lua.LValue{
			"rateofflinetrainingspeed": lua.LNumber(2.5),
			"allowoldprotocol":         lua.LBool(true),
		},
	}
	defer func() { config.Active = nil }()

	script := `
		local speed = configManager.getFloat("RATE_OFFLINE_TRAINING_SPEED")
		local oldProto = configManager.getBoolean("ALLOW_OLD_PROTOCOL")
		return speed, oldProto
	`
	if err := e.DoString(script); err != nil {
		t.Fatalf("failed to execute config script: %v", err)
	}

	speed := e.L.Get(-2)
	oldProto := e.L.Get(-1)

	if speed.String() != "2.5" {
		t.Errorf("expected 2.5, got %s", speed)
	}
	if oldProto.String() != "true" {
		t.Errorf("expected true, got %s", oldProto)
	}
}

