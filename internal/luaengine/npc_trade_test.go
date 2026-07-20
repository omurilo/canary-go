package luaengine

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	lua "github.com/yuin/gopher-lua"
)

// recordSession captures packets a script sends to a player.
type recordSession struct {
	p    *game.Player
	sent [][]byte
}

func (s *recordSession) SendToClient(w *netmsg.Writer) { s.sent = append(s.sent, append([]byte(nil), w.Bytes()...)) }
func (s *recordSession) Player() *game.Player          { return s.p }

// TestNpcTradeOpensShop drives the full post-greeting interaction: greet the
// merchant, then say "trade", and assert the shop-open packet (0x7A) is sent.
// This guards the interaction-state fix (isInteractingWithPlayer) plus the
// shop-item parsing and merchant detection — without them, "trade" silently
// does nothing after the greeting (the bug the user reported).
func TestNpcTradeOpensShop(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	datapack := filepath.Join(repo, "data-otservbr-global")
	core := filepath.Join(repo, "data")
	benjamin := filepath.Join(datapack, "npc", "benjamin.lua")
	if _, err := os.Stat(benjamin); err != nil {
		t.Skip("datapack not available")
	}

	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	e.L.SetGlobal("DATA_DIRECTORY", lua.LString(datapack))
	e.L.SetGlobal("CORE_DIRECTORY", lua.LString(core))
	walkLoad(t, e, filepath.Join(datapack, "lib"))
	for _, sub := range []string{"lib", "libs", "npclib"} {
		walkLoad(t, e, filepath.Join(core, sub))
	}
	if err := e.DoFile(benjamin); err != nil {
		t.Fatalf("load benjamin: %v", err)
	}

	npc := game.NewNpc(100001, "Benjamin", w.TypeRegistry.Npcs["benjamin"])
	w.AddCreature(npc)
	player := &game.Player{Name: "Trader", Level: 8, Health: 100, MaxHealth: 100}
	sess := &recordSession{p: player}
	w.AddPlayer(player, sess)

	// Sanity: the datapack shop must have been parsed into the type registry.
	if nt := w.TypeRegistry.Npcs["benjamin"]; nt == nil || len(nt.ShopItems) == 0 {
		t.Fatalf("benjamin shop items not parsed (got %v)", nt)
	}

	say := func(talkType byte, text string) {
		e.npcCallbacksMu.Lock()
		fn := e.npcCallbacks["benjamin"]["onSay"]
		e.npcCallbacksMu.Unlock()
		e.mu.Lock()
		L := e.L
		L.Push(fn)
		un := L.NewUserData()
		un.Value = npc
		L.SetMetatable(un, L.GetTypeMetatable("Npc"))
		L.Push(un)
		up := L.NewUserData()
		up.Value = player
		L.SetMetatable(up, L.GetTypeMetatable("Player"))
		L.Push(up)
		L.Push(lua.LNumber(talkType))
		L.Push(lua.LString(text))
		err := L.PCall(4, 0, nil)
		e.mu.Unlock()
		if err != nil {
			t.Fatalf("onSay(%q) error: %v", text, err)
		}
	}

	say(1, "hi") // TALKTYPE_SAY greeting
	if !npc.IsInteractingWithPlayer(player.ID) {
		t.Fatalf("npc not interacting with player after greet")
	}
	say(12, "trade") // TALKTYPE_PRIVATE_PN

	// Look for a 0x7A (open shop) packet among what the NPC sent the player.
	shopOpened := false
	for _, pkt := range sess.sent {
		if len(pkt) > 0 && pkt[0] == 0x7A {
			shopOpened = true
		}
	}
	if !shopOpened {
		t.Errorf("no shop-open (0x7A) packet sent after 'trade'; sent %d packets", len(sess.sent))
	}
}
