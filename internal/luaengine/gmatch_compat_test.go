package luaengine

import (
	"testing"
)

// TestGmatchSingleBindingRegression is the store.lua bug: `local m = s:gmatch(pat)`
// keeps only the iterator and drops the state that gopher-lua's iterator needs,
// so `for v in m do` calls it with nil and fails with
// "userdata expected, got nil". registerLuaCompat wraps gmatch so the state is
// captured as an upvalue.
func TestGmatchSingleBindingRegression(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		-- exact shape from canary data/libs/gamestore/store.lua:242-246
		local name = "Some Name Here"
		local match = name:gmatch("%s+")
		local count = 0
		for v in match do
			count = count + 1
		end
		assert(count == 2, "expected 2 whitespace runs, got " .. count)
	`); err != nil {
		t.Fatalf("single-binding gmatch still fails: %v", err)
	}
}

// TestGmatchDirectIteration ensures the wrapped gmatch is fully compatible with
// the normal direct-for idiom and with capture patterns.
func TestGmatchDirectIteration(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		-- direct-for over a simple match
		local words = {}
		for w in ("a b c"):gmatch("%a+") do
			words[#words + 1] = w
		end
		assert(#words == 3, "expected 3 words, got " .. #words)
		assert(words[1] == "a" and words[3] == "c")

		-- capture pattern: values flow through the wrapper
		local iter2 = ("k1=v1,k2=v2"):gmatch("(%w+)=(%w+)")
		local k, v = iter2()
		assert(k == "k1" and v == "v1", "captures lost: " .. tostring(k) .. " " .. tostring(v))

		-- manual iteration: the wrapped iterator ignores its args, so
		-- local it = s:gmatch(pat); it(nil) behaves like real Lua
		local it = ("x y"):gmatch("%S+")
		local a = it(nil)
		local b = it(nil)
		local c = it(nil)
		assert(a == "x" and b == "y" and c == nil, "manual iteration broken")
	`); err != nil {
		t.Fatalf("wrapped gmatch diverged: %v", err)
	}
}

// TestGmatchOriginalPreserved checks that calling the wrapper with all
// arguments (as the for loop does) still yields the same results.
func TestGmatchOriginalPreserved(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		local n = 0
		for _ in ("  a b  "):gmatch("%s+") do
			n = n + 1
		end
		assert(n == 3, "expected 3 whitespace runs, got " .. n)
	`); err != nil {
		t.Fatalf("wrapped gmatch failed on leading/trailing spaces: %v", err)
	}
}
