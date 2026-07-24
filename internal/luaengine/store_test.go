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

	// Drive the real openStore() through mock NetworkMessage/Player and decode
	// the resulting byte stream exactly like the otclient parseStore() does. A
	// 1525 client reads two trailing bytes after the category list; if openStore
	// is short, the real client underruns and drops the whole category tree.
	if err := e.DoString(`
		-- Mock NetworkMessage that just tallies bytes written, matching the
		-- wire sizes of each adder.
		local total = 0
		local fakeMsg = {}
		fakeMsg.__index = fakeMsg
		function fakeMsg:addByte(v) total = total + 1 end
		function fakeMsg:addU16(v) total = total + 2 end
		function fakeMsg:addU32(v) total = total + 4 end
		function fakeMsg:addString(s) total = total + 2 + #tostring(s) end
		function fakeMsg:sendToPlayer(p) self.sent = true end
		_G.NetworkMessage = function() return setmetatable({}, fakeMsg) end

		_G.Player = function()
			return {
				getClient = function() return { version = 1525, os = 2 } end,
				getVocation = function() return { getId = function() return 1 end } end,
				getId = function() return 1 end,
			}
		end
		_G.sendStoreBalanceUpdating = function() end

		if type(openStore) ~= "function" then error("openStore global not exposed") end
		openStore(1)

		-- Expected minimum: opcode(1) + count(2) + per category
		-- [name str + state(1) + iconCount(1) + icons + parent str] + 2 trailing.
		local cats = _G.GameStore.Categories
		local expected = 1 + 2 + 2 -- opcode + u16 count + 2 trailing
		for _, c in ipairs(cats) do
			expected = expected + (2 + #c.name) + 1 + 1
			for _, ic in ipairs(c.icons or {}) do expected = expected + 2 + #ic end
			expected = expected + (c.parent and (2 + #c.parent) or 2)
		end
		if total ~= expected then
			error(string.format("openStore packet size mismatch: got %d want %d (trailing bytes missing?)", total, expected))
		end
		print(string.format("STORE_OPEN_OK bytes=%d categories=%d", total, #cats))
	`); err != nil {
		t.Fatalf("openStore layout: %v", err)
	}
}
