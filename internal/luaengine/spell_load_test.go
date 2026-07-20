package luaengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/spells"
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
