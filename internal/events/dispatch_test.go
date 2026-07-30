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

// allUpstreamHooks is the EventCallback_t inventory
// (src/lua/callbacks/callbacks_definitions.hpp:21), cross-checked against the
// return types documented in data/scripts/eventcallbacks/README.md. Register must
// recognise every one of them, or a script that assigns it is silently inert.
var allUpstreamHooks = []string{
	"creatureOnChangeOutfit", "creatureOnAreaCombat", "creatureOnTargetCombat",
	"creatureOnDrainHealth", "creatureOnCombat",
	"partyOnJoin", "partyOnLeave", "partyOnDisband", "partyOnShareExperience",
	"playerOnBrowseField", "playerOnLook", "playerOnLookInBattleList",
	"playerOnLookInTrade", "playerOnLookInShop", "playerOnMoveItem",
	"playerOnItemMoved", "playerOnChangeZone", "playerOnChangeHazard",
	"playerOnMoveCreature", "playerOnReportRuleViolation", "playerOnReportBug",
	"playerOnTurn", "playerOnTradeRequest", "playerOnTradeAccept",
	"playerOnGainExperience", "playerOnLoseExperience", "playerOnGainSkillTries",
	"playerOnRemoveCount", "playerOnRequestQuestLog", "playerOnRequestQuestLine",
	"playerOnStorageUpdate", "playerOnCombat", "playerOnInventoryUpdate",
	"playerOnRotateItem", "playerOnWalk", "playerOnThink",
	"monsterOnDropLoot", "monsterPostDropLoot",
	"zoneBeforeCreatureEnter", "zoneBeforeCreatureLeave",
	"zoneAfterCreatureEnter", "zoneAfterCreatureLeave",
	"mapOnLoad",
}

func TestAllUpstreamHooksAreRecognised(t *testing.T) {
	if len(allUpstreamHooks) != 43 {
		t.Fatalf("the upstream inventory has 43 hooks, this list has %d", len(allUpstreamHooks))
	}

	known := make(map[string]bool, len(allCallbackFields))
	for _, cf := range allCallbackFields {
		known[cf.Field] = true
	}
	for _, h := range allUpstreamHooks {
		if !known[h] {
			t.Errorf("Register does not recognise %q, so a script assigning it is inert", h)
		}
	}
}

// Every hook must reach Lua. This registers all 43 at once and fires each
// dispatcher, so a missing or mis-wired Execute* shows up as a hook that never ran.
func TestEveryHookDispatches(t *testing.T) {
	var script string
	script = "FIRED = {}\ncb = {\n"
	for _, h := range allUpstreamHooks {
		script += "  " + h + " = function(...) FIRED[\"" + h + "\"] = true return true end,\n"
	}
	script += "}\n"

	e := newTestEngine(t, script)

	p := &game.Player{Name: "Hooked"}
	pos := game.Position{X: 1, Y: 2, Z: 7}

	// Creature
	e.ExecuteCreatureOnChangeOutfit(nil, game.Outfit{LookType: 128})
	e.ExecuteCreatureOnAreaCombat(nil, nil, true)
	e.ExecuteCreatureOnTargetCombat(nil, nil)
	e.ExecuteCreatureOnDrainHealth(nil, nil, 1, 10, 0, 0, 1, 0)
	e.ExecuteCreatureOnCombat(nil, nil, 5)
	// Party
	e.ExecutePartyOnJoin(nil, p)
	e.ExecutePartyOnLeave(nil, p)
	e.ExecutePartyOnDisband(nil)
	e.ExecutePartyOnShareExperience(nil, 100)
	// Player
	e.ExecutePlayerOnBrowseField(p, pos)
	e.ExecuteOnLook(p, nil, pos, 1)
	e.ExecutePlayerOnLookInBattleList(p, nil, 1)
	e.ExecutePlayerOnLookInTrade(p, p, nil, 1)
	e.ExecutePlayerOnLookInShop(p, 3031, 1)
	e.ExecuteOnMoveItem(p, nil, 1, pos, pos)
	e.ExecutePlayerOnItemMoved(p, nil, 1, pos, pos)
	e.ExecutePlayerOnChangeZone(p, 1)
	e.ExecutePlayerOnChangeHazard(p, true)
	e.ExecutePlayerOnMoveCreature(p, nil, pos, pos)
	e.ExecutePlayerOnReportRuleViolation(p, "someone", 1, 2, "c", "")
	e.ExecutePlayerOnReportBug(p, "bug", pos, 1)
	e.ExecutePlayerOnTurn(p, 0)
	e.ExecutePlayerOnTradeRequest(p, p, nil)
	e.ExecutePlayerOnTradeAccept(p, p, nil, nil)
	e.ExecuteOnGainExperience(p, nil, 10, 10)
	e.ExecutePlayerOnLoseExperience(p, 10)
	e.ExecutePlayerOnGainSkillTries(p, 1, 5)
	e.ExecutePlayerOnRemoveCount(p, nil)
	e.ExecutePlayerOnRequestQuestLog(p)
	e.ExecutePlayerOnRequestQuestLine(p, 1)
	e.ExecutePlayerOnStorageUpdate(p, 1, 2, -1, 0)
	e.ExecutePlayerOnCombat(p, nil, nil, 3)
	e.ExecutePlayerOnInventoryUpdate(p, nil, 1, true)
	e.ExecutePlayerOnRotateItem(p, nil, pos)
	e.ExecutePlayerOnWalk(p, 0)
	e.ExecutePlayerOnThink(p, 500)
	// Monster
	e.ExecuteMonsterOnDropLoot(nil, nil)
	e.ExecuteMonsterPostDropLoot(nil, nil)
	// Zone
	e.ExecuteZoneBeforeCreatureEnter(1, nil)
	e.ExecuteZoneBeforeCreatureLeave(1, nil)
	e.ExecuteZoneAfterCreatureEnter(1, nil)
	e.ExecuteZoneAfterCreatureLeave(1, nil)
	// Map
	e.ExecuteMapOnLoad("world.otbm")

	fired, ok := e.L.GetGlobal("FIRED").(*lua.LTable)
	if !ok {
		t.Fatal("FIRED is not a table")
	}
	for _, h := range allUpstreamHooks {
		if e.L.GetField(fired, h) == lua.LNil {
			t.Errorf("%s never reached Lua", h)
		}
	}
}
