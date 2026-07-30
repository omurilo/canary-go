package events

import (
	"log/slog"
	"os"
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

func newTestEngine(t *testing.T, script string) *Engine {
	t.Helper()
	L := lua.NewState()
	t.Cleanup(L.Close)

	// Minimal metatables so the userdata wrappers resolve.
	for _, name := range []string{"Player", "Item", "Position", "Party", "Creature", "Tile", "Monster", "Container", "Thing"} {
		L.SetGlobal(name, L.NewTypeMetatable(name))
	}

	e := NewEngine(L, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

	if err := L.DoString(script); err != nil {
		t.Fatalf("script: %v", err)
	}
	tbl, ok := L.GetGlobal("cb").(*lua.LTable)
	if !ok {
		t.Fatal("script must define a table named cb")
	}
	e.Register(tbl)
	return e
}

// Each of these hooks was being parsed and stored by Register but had no Execute*
// at all, so a registered script could never run.
func TestNewlyWiredHooksFire(t *testing.T) {
	e := newTestEngine(t, `
		LOG = {}
		local function note(s) LOG[#LOG + 1] = s end
		cb = {
			playerOnBrowseField      = function(p, pos) note("browseField") return true end,
			playerOnLookInShop       = function(p, it, c) note("lookInShop:" .. it .. ":" .. c) return true end,
			playerOnLookInTrade      = function(p, partner, item, d) note("lookInTrade:" .. d) return true end,
			playerOnRotateItem       = function(p, item, pos) note("rotateItem") return true end,
			playerOnRemoveCount      = function(p, item) note("removeCount") return true end,
			playerOnRequestQuestLog  = function(p) note("questLog") return true end,
			playerOnRequestQuestLine = function(p, id) note("questLine:" .. id) return true end,
			playerOnStorageUpdate    = function(p, k, v, old, t) note("storage:" .. k .. ":" .. v .. ":" .. old) return true end,
			playerOnTradeAccept      = function(p, target, item, targetItem) note("tradeAccept") return true end,
			partyOnDisband           = function(party) note("disband") return true end,
			creatureOnAreaCombat     = function(c, tile, aggressive) note("areaCombat:" .. tostring(aggressive)) return true end,
			creatureOnTargetCombat   = function(c, target) note("targetCombat") return true end,
			monsterPostDropLoot      = function(m, corpse) note("postDropLoot") return true end,
		}
	`)

	p := &game.Player{Name: "Tester"}
	pos := game.Position{X: 10, Y: 20, Z: 7}

	e.ExecutePlayerOnBrowseField(p, pos)
	e.ExecutePlayerOnLookInShop(p, 3031, 5)
	e.ExecutePlayerOnLookInTrade(p, p, nil, 3)
	e.ExecutePlayerOnRotateItem(p, nil, pos)
	e.ExecutePlayerOnRemoveCount(p, nil)
	e.ExecutePlayerOnRequestQuestLog(p)
	e.ExecutePlayerOnRequestQuestLine(p, 42)
	e.ExecutePlayerOnStorageUpdate(p, 1000, 7, -1, 123)
	e.ExecutePlayerOnTradeAccept(p, p, nil, nil)
	e.ExecutePartyOnDisband(nil)
	e.ExecuteCreatureOnAreaCombat(nil, nil, true)
	e.ExecuteCreatureOnTargetCombat(nil, nil)
	e.ExecuteMonsterPostDropLoot(nil, nil)

	want := []string{
		"browseField", "lookInShop:3031:5", "lookInTrade:3", "rotateItem",
		"removeCount", "questLog", "questLine:42", "storage:1000:7:-1",
		"tradeAccept", "disband", "areaCombat:true", "targetCombat", "postDropLoot",
	}
	assertLog(t, e.L, want)
}

// A callback returning false must short-circuit, which is how EventCallback gates
// the caller on the C++ side.
func TestFalseReturnShortCircuits(t *testing.T) {
	e := newTestEngine(t, `
		LOG = {}
		cb = {
			playerOnBrowseField = function(p, pos) LOG[#LOG + 1] = "called" return false end,
		}
	`)
	if e.ExecutePlayerOnBrowseField(&game.Player{}, game.Position{}) {
		t.Error("a false return must propagate as false")
	}
	assertLog(t, e.L, []string{"called"})
}

// A hook nobody registered must report success rather than blocking the caller.
func TestUnregisteredHookIsTransparent(t *testing.T) {
	e := newTestEngine(t, `cb = {}`)
	if !e.ExecutePlayerOnBrowseField(&game.Player{}, game.Position{}) {
		t.Error("an unregistered hook must not block the action")
	}
}

// Nil arguments must reach Lua as nil rather than panicking, since several call
// sites cannot resolve every argument yet.
func TestNilArgumentsBecomeLuaNil(t *testing.T) {
	e := newTestEngine(t, `
		LOG = {}
		cb = {
			playerOnLookInTrade = function(p, partner, item, d)
				LOG[#LOG + 1] = tostring(item == nil)
				return true
			end,
		}
	`)
	e.ExecutePlayerOnLookInTrade(&game.Player{}, nil, nil, 1)
	assertLog(t, e.L, []string{"true"})
}

func assertLog(t *testing.T, L *lua.LState, want []string) {
	t.Helper()
	tbl, ok := L.GetGlobal("LOG").(*lua.LTable)
	if !ok {
		t.Fatal("LOG is not a table")
	}
	var got []string
	tbl.ForEach(func(_, v lua.LValue) { got = append(got, v.String()) })

	if len(got) != len(want) {
		t.Fatalf("fired %d hooks, want %d\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hook %d: got %q want %q", i, got[i], want[i])
		}
	}
}
