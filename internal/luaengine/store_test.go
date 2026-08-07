package luaengine

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
	"github.com/omurilo/canary-go/internal/netmsg"
	lua "github.com/yuin/gopher-lua"
)

// TestAddPlayerEventForwardsTargetAndArgs guards the addPlayerEvent fallback:
// addPlayerEvent(callable, delay, target, ...) must fire callable(target, ...).
// A previous wrapper dropped the target (used arg 2 as the first parameter), so
// deferred store sends like sendShowStoreOffers(playerId, category) ran as
// sendShowStoreOffers(category) — Player(category) failed, no offers packet was
// sent, and opening a store category hung the client.
func TestAddPlayerEventForwardsTargetAndArgs(t *testing.T) {
	e := newTestEngine()
	if err := e.DoString(`
		_R = ""
		function _rec(target, arg) _R = tostring(target) .. "|" .. tostring(arg) end
	`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := e.DoString(`addPlayerEvent(_rec, 5, "PID", "CAT")`); err != nil {
		t.Fatalf("addPlayerEvent call: %v", err)
	}
	var got string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		got = e.L.GetGlobal("_R").String()
		e.mu.Unlock()
		if got != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got != "PID|CAT" {
		t.Fatalf("addPlayerEvent fired callback with wrong args: got %q, want %q (target dropped?)", got, "PID|CAT")
	}
}

// TestStoreInboxDelivery verifies the "receber" path: getStoreInbox returns a
// real Container and addItemEx (with FLAG_NOLIMIT) delivers a created item into
// the player's StoreInbox — the mechanic the datapack's Player:addItemStoreInbox
// uses to deliver in-game store purchases.
func TestStoreInboxDelivery(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	// Gold has to be in the catalog for Game.createItem to read its count as a
	// stack size rather than a plain item count (C++ ItemType::hasSubType).
	w.Items = items.NewCatalog(&items.ItemType{ID: 3031, Stackable: true, StackSize: 100})
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

	player := &game.Player{Name: "Buyer", DBID: 42, Vocation: 1}
	sess := &recordSession{p: player}
	w.AddPlayer(player, sess)

	e.mu.Lock()
	up := e.L.NewUserData()
	up.Value = player
	e.L.SetMetatable(up, e.L.GetTypeMetatable("Player"))
	e.L.SetGlobal("_p", up)
	e.mu.Unlock()

	if err := e.DoString(`
		local inbox = _p:getStoreInbox()
		if not inbox then error("getStoreInbox returned nil") end
		local it = Game.createItem(3031, 5)
		if not it then error("Game.createItem returned nil") end
		it:setOwner(_p)
		local ret = inbox:addItemEx(it, INDEX_WHEREEVER, FLAG_NOLIMIT)
		if ret ~= 0 then error("addItemEx returned " .. tostring(ret)) end
	`); err != nil {
		t.Fatalf("delivery script: %v", err)
	}

	if player.StoreInbox == nil {
		t.Fatal("StoreInbox not created")
	}
	if player.StoreInbox.Container == nil {
		t.Fatalf("StoreInbox has no container")
	}
	if got := len(player.StoreInbox.Container.Contents); got != 1 {
		t.Fatalf("StoreInbox has %d items, want 1", got)
	}
	deliv := player.StoreInbox.Container.Contents[0]
	if deliv.ID != 3031 || deliv.Count != 5 {
		t.Fatalf("delivered item = id %d x%d, want 3031 x5", deliv.ID, deliv.Count)
	}
	if deliv.Attr == nil || deliv.Attr.Owner == nil || *deliv.Attr.Owner != 42 {
		t.Fatalf("delivered item owner not set to player GUID 42")
	}
}

// TestOpenStorePacketOnWire drives the REAL gamestore onRecvbyte(C_OpenStore)
// through the actual Go NetworkMessage/Player bindings and decodes the captured
// S_OpenStore packet exactly as the otclient ProtocolGame::parseStore does. If
// the packet the server emits doesn't line up byte-for-byte with what the client
// reads (leftover bytes or an underrun), the client drops the category tree —
// which is the "only recently added shows" bug. This is the end-to-end guard.
func TestOpenStorePacketOnWire(t *testing.T) {
	// Run the whole flow once per client family. The two trailing bytes after the
	// category list are an OTCLIENT extension — its parseStore reads them from
	// client version 1332 up, the official Tibia client never does — so the same
	// protocol version must produce different packets, and a stock client that
	// receives them reads the leftovers as an opcode and dies with
	// "Unknown Gameserver Message: 0".
	for _, tc := range []struct {
		name         string
		clientOS     uint16
		wantTrailing bool
	}{
		{"stock tibia client", 2, false},
		{"otclient", 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) { runOpenStoreWireCheck(t, tc.clientOS, tc.wantTrailing) })
	}
}

func runOpenStoreWireCheck(t *testing.T, clientOS uint16, wantTrailing bool) {
	repo := filepath.Join("..", "..") // canary-go root — the datapack the server actually uses
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
	sess := &recordSession{p: player, os: clientOS}
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

	// Decode exactly like otclient parseStore at protocol >= 1332, but STRICTLY:
	// the client's InputMessage::getU8 throws on read-past-end (checkRead), so we
	// assert enough bytes remain BEFORE each read. netmsg.Reader is lenient
	// (returns 0 past end), which previously masked the underrun.
	const clientVersion = 1525
	r := netmsg.NewReader(pkt[1:]) // skip opcode
	need := func(n int, what string) {
		if r.Remaining() < n {
			t.Fatalf("S_OpenStore underrun reading %s: need %d, have %d (client parseStore would throw 'eof reached')", what, n, r.Remaining())
		}
	}
	need(2, "categoryCount")
	categoryCount := int(r.GetU16())
	for i := 0; i < categoryCount; i++ {
		need(2, "category name len")
		_ = r.GetString() // name
		if clientVersion < 1291 {
			_ = r.GetString() // description
		}
		need(1, "state")
		_ = r.GetByte() // state (GameIngameStoreHighlights assumed on)
		need(1, "iconCount")
		iconCount := int(r.GetByte())
		for j := 0; j < iconCount; j++ {
			need(2, "icon len")
			_ = r.GetString()
		}
		need(2, "parent len")
		_ = r.GetString() // parent
	}
	// The two trailing bytes are an OTCLIENT extension, not a protocol-version one:
	// otclient's parseStore reads them from client version 1332 up, and the official
	// Tibia client never does. Sending them to a stock client leaves two bytes it
	// reads as the next opcode — "Unknown Gameserver Message: 0" — and it dies.
	if wantTrailing {
		need(2, "two trailing bytes (otclient)")
		_ = r.GetByte()
		_ = r.GetByte()
	}
	leftover := r.Remaining()
	t.Logf("S_OpenStore: total=%d bytes, categories=%d, leftover-after-parse=%d", len(pkt), categoryCount, leftover)
	if categoryCount == 0 {
		t.Fatalf("S_OpenStore carried 0 categories (store tree empty)")
	}
	if leftover != 0 {
		t.Fatalf("S_OpenStore parse desync: %d leftover bytes (client would misparse)", leftover)
	}
}

// TestGamestoreCatalogLoads loads the real gamestore Lua module and reports how
// many categories/offers ended up in GameStore.Categories, to diagnose an empty
// store window.
func TestGamestoreCatalogLoads(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..")) // canary-go root
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
	// Checks the byte budget of openStore for a STOCK client: opcode, count and the
	// categories, with no OTC trailing bytes. Getting this wrong in either direction
	// desyncs the client — short and it underruns, long and it reads the leftovers as
	// an opcode.
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
				-- os 2 is a stock client, so no OTC-only trailing bytes. The real
				-- predicate lives in data/libs/functions/player.lua, which this
				-- harness does not load.
				isUsingOtClient = function() return false end,
				getVocation = function() return { getId = function() return 1 end } end,
				getId = function() return 1 end,
			}
		end
		_G.sendStoreBalanceUpdating = function() end

		if type(openStore) ~= "function" then error("openStore global not exposed") end
		openStore(1)

		-- Expected: opcode(1) + count(2) + per category
		-- [name str + state(1) + iconCount(1) + icons + parent str].
		local cats = _G.GameStore.Categories
		-- No trailing bytes: this harness has no session, so getClient reports a stock
		-- client and isUsingOtClient() is false. The OTC case is covered end to end by
		-- TestOpenStorePacketOnWire.
		local expected = 1 + 2 -- opcode + u16 count
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

// TestRemoveTransferableAndTibiaCoins verifies the combined coin deduction used
// by store purchases (transferable first, then normal). A prior stub returned
// true without deducting, so purchases never reduced the balance.
func TestRemoveTransferableAndTibiaCoins(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	player := &game.Player{Name: "Buyer", CoinBalance: 100, CoinTransferable: 30}
	w.AddPlayer(player, &recordSession{p: player})
	e.mu.Lock()
	up := e.L.NewUserData()
	up.Value = player
	e.L.SetMetatable(up, e.L.GetTypeMetatable("Player"))
	e.L.SetGlobal("_p", up)
	e.mu.Unlock()
	if err := e.DoString(`if not _p:removeTransferableAndTibiaCoins(50) then error("returned false") end`); err != nil {
		t.Fatalf("deduct: %v", err)
	}
	// 50 spent: 30 transferable + 20 normal => transferable 0, normal 80.
	if player.CoinTransferable != 0 || player.CoinBalance != 80 {
		t.Fatalf("after spending 50: transferable=%d normal=%d, want 0/80", player.CoinTransferable, player.CoinBalance)
	}
}

// TestMountGlobal verifies Mount(id):getClientId() is callable — the gamestore
// senders use it for SHOW_MOUNT offers with no nil-check, so a missing Mount
// global crashed the offers packet ("attempt to call a non-function object").
func TestMountGlobal(t *testing.T) {
	e := newTestEngine()
	if err := e.DoString(`
		local m = Mount(5)
		if not m then error("Mount returned nil") end
		local cid = m:getClientId()
		if type(cid) ~= "number" then error("getClientId not a number") end
	`); err != nil {
		t.Fatalf("Mount global: %v", err)
	}
}
