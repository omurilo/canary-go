package luaengine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestGamestoreCatalogLoads loads the real gamestore Lua module and reports how
// many categories/offers ended up in GameStore.Categories, to diagnose an empty
// store window.
func TestGamestoreCatalogLoads(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	coreDir := filepath.Join(root, "data")
	gs := filepath.Join(coreDir, "modules", "scripts", "gamestore", "gamestore.lua")
	if _, err := os.Stat(gs); err != nil {
		t.Skipf("gamestore module not available: %v", err)
	}

	e := newTestEngine()
	e.L.SetGlobal("CORE_DIRECTORY", lua.LString(coreDir))
	e.L.SetGlobal("DATA_DIRECTORY", lua.LString(filepath.Join(root, "data-otservbr-global")))
	if pkg, ok := e.L.GetGlobal("package").(*lua.LTable); ok {
		e.L.SetField(pkg, "path", lua.LString(fmt.Sprintf(
			"%s/libs/?.lua;%s/libs/?/init.lua;?.lua", coreDir, coreDir)))
	}

	// Mirror the server: load data/libs recursively (defines HIRELING_SKILLS in
	// systems/hireling.lua, which the hireling catalog needs).
	_ = filepath.WalkDir(filepath.Join(coreDir, "libs"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(p) == ".lua" {
			_ = e.DoFile(p)
		}
		return nil
	})
	e.DoString(`print("HIRELING_SKILLS_DEFINED=" .. tostring(HIRELING_SKILLS ~= nil))`)

	if err := e.DoFile(gs); err != nil {
		t.Fatalf("load gamestore module: %v", err)
	}
	e.SyncStoreGlobal()

	// The senders/parsers read the *global* GameStore, so that is what must be
	// populated after SyncStoreGlobal — not just the constants module table.
	if err := e.DoString(`
		if type(_G.GameStore) ~= "table" then error("GameStore global missing") end
		local cats = _G.GameStore.Categories
		if type(cats) ~= "table" or #cats == 0 then
			error("GameStore.Categories not populated on global (store would be empty)")
		end
		local offers = 0
		for _, c in ipairs(cats) do offers = offers + (c.offers and #c.offers or 0) end
		if offers == 0 then error("no offers loaded across categories") end
		print(string.format("STORE_DIAG categories=%d offers=%d", #cats, offers))
	`); err != nil {
		t.Fatalf("catalog not usable by senders: %v", err)
	}
	e.LogStoreCatalogStatus()
}
