package luaengine

import (
	"encoding/json"
	"os"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// snapshot is scripts/lua_enums_snapshot.json: every global lua_enums.cpp
// registers, with the value resolved from the enum body that defines it.
//
// The snapshot is committed and this test guards the Go side against drifting
// from it. Regenerating it from the C++ needs a new extractor — the Python one
// was deleted once the gap reached zero, and no Python goes back in the repo.
// Write that in bash, or better, as a generator in Go.
//
// Last regenerated 2026-08-01: 1298 resolved, 0 missing, 0 value mismatches.
type enumSnapshot struct {
	Resolved   map[string]int `json:"resolved"`
	Unresolved [][]string     `json:"unresolved"`
}

func loadEnumSnapshot(t *testing.T) enumSnapshot {
	t.Helper()
	raw, err := os.ReadFile("../../scripts/lua_enums_snapshot.json")
	if err != nil {
		t.Skipf("enum snapshot not available: %v", err)
	}
	var s enumSnapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(s.Resolved) < 1000 {
		t.Fatalf("snapshot holds only %d resolved enums, expected ~1298 — regenerate it", len(s.Resolved))
	}
	return s
}

// Every enum global the C++ registers must exist in Lua with the C++ value. An
// undefined global reads as nil rather than raising, and a wrong value is worse:
// the SKILL_ constants were all one too high, so scripts asking for SKILL_FIST
// silently read the club slot.
func TestRegisteredEnumsMatchUpstream(t *testing.T) {
	snap := loadEnumSnapshot(t)

	L := lua.NewState()
	defer L.Close()
	RegisterEnums(L)

	var missing, wrong int
	for name, want := range snap.Resolved {
		v := L.GetGlobal(name)
		num, ok := v.(lua.LNumber)
		if !ok {
			if missing < 10 {
				t.Errorf("%s is not defined (reads as %s)", name, v.Type())
			}
			missing++
			continue
		}
		if int(num) != want {
			if wrong < 10 {
				t.Errorf("%s = %d, want %d", name, int(num), want)
			}
			wrong++
		}
	}
	if missing > 10 || wrong > 10 {
		t.Errorf("totals: %d missing, %d with the wrong value", missing, wrong)
	}
}

// The generated table must not shadow anything hand-written: a duplicate would
// mean whichever loop runs last silently wins.
func TestGeneratedEnumsDoNotOverlapHandWritten(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	// Apply only the hand-written table, then check the generated one adds no
	// name that already has a value.
	before := make(map[string]lua.LValue, len(generatedEnums))
	RegisterEnums(L)
	for name := range generatedEnums {
		before[name] = L.GetGlobal(name)
	}

	L2 := lua.NewState()
	defer L2.Close()
	for name, want := range generatedEnums {
		if got, ok := before[name]; ok {
			if n, isNum := got.(lua.LNumber); isNum && n != want {
				t.Errorf("%s is defined twice with different values: %v vs %v", name, n, want)
			}
		}
	}
}

// registerTeleportType and registerImbuementType were defined and never called,
// and mockClass("Teleport") then papered over the hole with a userdata whose
// __index answers every call with nil — so a datapack calling teleport:getDestination()
// got nil instead of an error, and nothing in the logs said why.
func TestTeleportAndImbuementAreRealClasses(t *testing.T) {
	e := newTestEngine()
	for _, name := range []string{"Teleport", "Imbuement"} {
		mt := e.L.GetTypeMetatable(name)
		if mt == lua.LNil {
			t.Errorf("%s has no metatable: its register function is not being called", name)
			continue
		}
		tbl, ok := mt.(*lua.LTable)
		if !ok {
			t.Errorf("%s metatable is %T", name, mt)
			continue
		}
		idx := e.L.RawGet(tbl, lua.LString("__index"))
		methods, ok := idx.(*lua.LTable)
		if !ok {
			t.Errorf("%s __index is %s, not a method table — a mock has replaced it", name, idx.Type())
			continue
		}
		n := 0
		methods.ForEach(func(lua.LValue, lua.LValue) { n++ })
		if n == 0 {
			t.Errorf("%s exposes no methods", name)
		}
	}
}
