package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
	lua "github.com/yuin/gopher-lua"
)

// Item custom attributes, the port of ItemFunctions::luaItemSetCustomAttribute.
// The datapack keeps podium looks, unwrap ids and hireling state here, so a missing
// binding loses that state silently.
func TestItemCustomAttributes(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	e.world.Items = items.NewCatalog(&items.ItemType{ID: 1234, Name: "thing"})

	it := &game.Item{ID: 1234}
	e.pushItem(e.L, it)
	e.L.SetGlobal("it", e.L.Get(-1))
	e.L.Pop(1)

	if err := e.L.DoString(`
		assert(it:setCustomAttribute("name", "Bob") == true)
		assert(it:setCustomAttribute("count", 7) == true)
		assert(it:setCustomAttribute("ratio", 1.5) == true)
		assert(it:setCustomAttribute("flag", true) == true)
		-- a numeric key is stringified, so 42 and "42" are the same slot
		assert(it:setCustomAttribute(42, "answer") == true)

		assert(it:getCustomAttribute("name") == "Bob", "string round trip")
		assert(it:getCustomAttribute("count") == 7, "integer round trip")
		assert(it:getCustomAttribute("ratio") == 1.5, "double round trip")
		assert(it:getCustomAttribute("flag") == true, "boolean round trip")
		assert(it:getCustomAttribute("42") == "answer", "numeric key normalises to a string")
		assert(it:getCustomAttribute(42) == "answer", "and reads back either way")

		assert(it:getCustomAttribute("nope") == nil, "an unset key is nil")
		-- an unsupported value type is rejected, as upstream
		assert(it:setCustomAttribute("bad", {}) == nil)

		assert(it:removeCustomAttribute("name") == true)
		assert(it:getCustomAttribute("name") == nil, "removed key must read as nil")
		assert(it:removeCustomAttribute("name") == false, "removing twice is false")
	`); err != nil {
		t.Fatalf("%v", err)
	}

	// A whole number must come back as an integer, not a float: C++ only stores a
	// double when the value has a real fraction, and scripts compare with ==.
	if v, _ := it.GetCustomAttribute("count"); v != int64(7) {
		t.Errorf("count stored as %T(%v), want int64(7)", v, v)
	}
	if v, _ := it.GetCustomAttribute("ratio"); v != 1.5 {
		t.Errorf("ratio stored as %T(%v), want float64(1.5)", v, v)
	}
}

// ItemType:isCorpse reads the appearance flag, which the catalog now carries.
func TestItemTypeIsCorpse(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	e.world.Items = items.NewCatalog(
		&items.ItemType{ID: 3058, Name: "gold ring"},
		&items.ItemType{ID: 5964, Name: "dead rat", IsCorpse: true},
	)
	if err := e.L.DoString(`
		assert(ItemType(5964):isCorpse() == true, "a corpse type must report true")
		assert(ItemType(3058):isCorpse() == false, "a ring is not a corpse")
	`); err != nil {
		t.Fatalf("%v", err)
	}
}

// isSummonable/isIllusionable/isConvinceable are get/set in C++: no argument reads,
// a boolean assigns. The monster .lua files use the setter form while loading.
func TestMonsterTypeBoolFlagsGetAndSet(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	mt := &creatures.MonsterType{Name: "Rat"}
	e.world.TypeRegistry.Monsters["rat"] = mt

	if err := e.L.DoString(`
		local m = MonsterType("Rat")
		assert(m:isSummonable() == false, "defaults to false")
		assert(m:isSummonable(true) == true, "the setter returns true")
		assert(m:isSummonable() == true, "and the value sticks")
		m:isIllusionable(true)
		m:isConvinceable(true)
		m:BestiaryStars(4)
		m:Bestiaryrace(BESTY_RACE_MAMMAL)
		assert(m:BestiaryStars() == 4, "BestiaryStars reads back")
	`); err != nil {
		t.Fatalf("%v", err)
	}

	if !mt.Flags.Summonable || !mt.Flags.Illusionable || !mt.Flags.Convinceable {
		t.Errorf("flags did not reach MonsterType: %+v", mt.Flags)
	}
	if mt.BestiaryStars != 4 {
		t.Errorf("BestiaryStars = %d, want 4", mt.BestiaryStars)
	}
	if mt.BestiaryRace == 0 {
		t.Errorf("Bestiaryrace did not reach MonsterType")
	}
}

// A miss on any of these must be nil rather than a panic, so a script guarding with
// `if not x then` behaves.
func TestNewBindingsHandleNilReceivers(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	e.world.Items = items.NewCatalog()
	if err := e.L.DoString(`
		local t = ItemType(60000)
		assert(t:isCorpse() == false)
	`); err != nil {
		t.Fatalf("%v", err)
	}
	if got := e.L.GetGlobal("nothing"); got != lua.LNil {
		t.Fatalf("sanity check failed")
	}
}
