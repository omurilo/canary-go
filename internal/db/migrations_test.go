package db

import (
	"path/filepath"
	"testing"
)

func TestExtractMigrationVersion(t *testing.T) {
	cases := map[string]int{
		"1.lua": 1, "58.lua": 58, "10.lua": 10,
		"README.md": -1, "notes.txt": -1, "": -1,
	}
	for name, want := range cases {
		if got := extractMigrationVersion(name); got != want {
			t.Errorf("%q: got %d want %d", name, got, want)
		}
	}
}

func TestCollectMigrationsRealDatapack(t *testing.T) {
	dir := filepath.Join("..", "..", "data-otservbr-global", "migrations")
	ms, err := CollectMigrations(dir)
	if err != nil {
		t.Skipf("datapack not available: %v", err)
	}
	var applied, skipped int
	prev := -999
	for _, m := range ms {
		if m.Version < prev {
			t.Fatalf("not sorted: %d after %d", m.Version, prev)
		}
		prev = m.Version
		if m.Version > 0 {
			applied++
		} else {
			skipped++
		}
	}
	t.Logf("total=%d aplicáveis(>0)=%d ignorados(<=0)=%d maior=%d", len(ms), applied, skipped, prev)
	if applied != 58 {
		t.Errorf("esperava 58 migrations aplicáveis, obtive %d", applied)
	}
	if skipped != 1 {
		t.Errorf("esperava 1 ignorado (README.md), obtive %d", skipped)
	}
}
