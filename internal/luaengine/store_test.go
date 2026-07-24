package luaengine

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	lua "github.com/yuin/gopher-lua"
)

// TestOpenStorePacketOnWire drives the REAL gamestore onRecvbyte(C_OpenStore)
// through the actual Go NetworkMessage/Player bindings and decodes the captured
// S_OpenStore packet exactly as the otclient ProtocolGame::parseStore does. If
// the packet the server emits doesn't line up byte-for-byte with what the client
// reads (leftover bytes or an underrun), the client drops the category tree —
// which is the "only recently added shows" bug. This is the end-to-end guard.
func TestOpenStorePacketOnWire(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	core := filepath.Join(repo, "data")
	gs := filepath.Join(core, "modules", "scripts", "gamestore", "gamestore.lua")
	if _, err := os.Stat(gs); err != nil {
		t.Skipf("gamestore module not available: %v", err)
	}

	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	e.L.SetGlobal("CORE_DIRECTORY", lua.LString(core))
	e.L.SetGlobal("DATA_DIRECTORY", lua.LString(filepath.Join(repo, "data-otservbr-global")))
	if pkg, ok := e.L.GetGlobal("package").(*lua.LTable); ok {
		e.L.SetField(pkg, "path", lua.LString(fmt.Sprintf(
			"%s/libs/?.lua;%s/libs/?/init.lua;?.lua", core, core)))
	}
	for _, sub := range []string{"lib", "libs", "npclib"} {
		walkLoad(t, e, filepath.Join(core, sub))
	}
	if err := e.DoFile(gs); err != nil {
		t.Fatalf("load gamestore module: %v", err)
	}
	e.SyncStoreGlobal()

	player := &game.Player{Name: "Buyer", Level: 20, Vocation: 1, Health: 100, MaxHealth: 100}
	sess := &recordSession{p: player}
	w.AddPlayer(player, sess)

	// Fire onRecvbyte(player, msg, C_OpenStore=0xFA) through real bindings.
	e.mu.Lock()
	L := e.L
	fn := L.GetGlobal("onRecvbyte")
	if fn.Type() != lua.LTFunction {
		e.mu.Unlock()
		t.Fatal("onRecvbyte not defined")
	}
	up := L.NewUserData()
	up.Value = player
	L.SetMetatable(up, L.GetTypeMetatable("Player"))
	msgUd := L.NewUserData()
	msgUd.Value = &luaNetworkMessage{w: netmsg.NewWriter(), r: netmsg.NewReader([]byte{0})}
	L.SetMetatable(msgUd, L.GetTypeMetatable(networkMessageTypeName))
	L.Push(fn)
	L.Push(up)
	L.Push(msgUd)
	L.Push(lua.LNumber(0xFA))
	err := L.PCall(3, 0, nil)
	e.mu.Unlock()
	if err != nil {
		t.Fatalf("onRecvbyte(C_OpenStore) error: %v", err)
	}

	// Find the S_OpenStore (0xFB) packet among what was sent.
	var pkt []byte
	for _, p := range sess.sent {
		if len(p) > 0 && p[0] == 0xFB {
			pkt = p
			break
		}
	}
	if pkt == nil {
		t.Fatalf("no S_OpenStore (0xFB) packet emitted; sent %d packets", len(sess.sent))
	}

	// Decode exactly like otclient parseStore at protocol >= 1332.
	const clientVersion = 1525
	r := netmsg.NewReader(pkt[1:]) // skip opcode
	categoryCount := int(r.GetU16())
	for i := 0; i < categoryCount; i++ {
		_ = r.GetString() // name
		if clientVersion < 1291 {
			_ = r.GetString() // description
		}
		_ = r.GetByte() // state (GameIngameStoreHighlights assumed on)
		iconCount := int(r.GetByte())
		for j := 0; j < iconCount; j++ {
			_ = r.GetString()
		}
		_ = r.GetString() // parent
	}
	if clientVersion >= 1332 {
		_ = r.GetByte()
		_ = r.GetByte()
	}
	leftover := r.Remaining()
	t.Logf("S_OpenStore: total=%d bytes, categories=%d, leftover-after-parse=%d", len(pkt), categoryCount, leftover)
	if categoryCount == 0 {
		t.Fatalf("S_OpenStore carried 0 categories (store tree empty)")
	}
	if leftover != 0 {
		t.Fatalf("S_OpenStore parse desync: %d leftover bytes (client would misparse/underrun)", leftover)
	}
}

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
