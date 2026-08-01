package luaengine

import (
	"reflect"

	lua "github.com/yuin/gopher-lua"
)

// Userdata equality, ported from Lua::luaUserdataCompare
// (src/lua/functions/lua_functions_loader.cpp:885-888):
//
//	pushBoolean(L, getUserdata<void>(L, 1) == getUserdata<void>(L, 2));
//
// C++ registers this as __eq on 29 classes. Go had it on two, and without it
// gopher-lua compares the userdata BOXES, not what they point at — so two
// lookups of the same object come back unequal.
//
// This is a wide, quiet class of bug. It surfaced as an exercise dummy inside a
// house refusing to work:
//
//	local playerHouse = player:getTile():getHouse()
//	local targetHouse = Tile(targetPos):getHouse()
//	if playerHouse ~= targetHouse then ... "You must be inside the house" end
//
// Both calls return the same *game.House. Both wrap it in a fresh LUserData.
// The comparison was therefore always true, and the branch always taken —
// standing in the right house made no difference. Any datapack script comparing
// two items, creatures, tiles or towns had the same problem.

// eqClasses is the C++ list, from
// `grep 'registerMetaMethod(L, "...", "__eq"' src/lua/`. Names Go has no
// metatable for are skipped; keeping them listed makes the diff against
// upstream readable when one of them gets a binding.
var eqClasses = []string{
	"Charm", "Combat", "Condition", "Container", "Creature", "Group", "Guild",
	"House", "Imbuement", "Item", "ItemClassification", "ItemType", "ModalWindow",
	"Monster", "MonsterType", "Mount", "NetworkMessage", "Npc", "NpcType", "Party",
	"Player", "Position", "Spell", "Teleport", "Tile", "Town", "Vocation", "Zone",
}

// registerUserdataEquality installs __eq on every class that has one upstream.
// It must run AFTER all the e.register*() calls, since it looks the metatables
// up by name.
func (e *Engine) registerUserdataEquality() {
	fn := e.L.NewFunction(luaUserdataCompare)
	for _, name := range eqClasses {
		mt, ok := e.L.GetTypeMetatable(name).(*lua.LTable)
		if !ok {
			continue // no binding in Go yet
		}
		// Position and Zone already define their own __eq, and Position's compares
		// coordinates rather than identity — which is what a value type needs and
		// what the datapack expects from `pos == otherPos`. Do not overwrite either.
		if mt.RawGetString("__eq") != lua.LNil {
			continue
		}
		e.L.SetField(mt, "__eq", fn)
	}
}

func luaUserdataCompare(L *lua.LState) int {
	a, aok := L.Get(1).(*lua.LUserData)
	b, bok := L.Get(2).(*lua.LUserData)
	if !aok || !bok {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LBool(sameUserdataTarget(a.Value, b.Value)))
	return 1
}

// sameUserdataTarget answers the C++ question — do these two userdata point at
// the same object — for Go's wrappers.
func sameUserdataTarget(a, b any) bool {
	a, b = unwrapUserdata(a), unwrapUserdata(b)
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ra, rb := reflect.ValueOf(a), reflect.ValueOf(b)
	if ra.Type() != rb.Type() {
		return false
	}
	if ra.Kind() == reflect.Pointer {
		return ra.Pointer() == rb.Pointer()
	}
	// A value wrapper (luaTile and friends). Comparing interfaces holding an
	// uncomparable type panics, so check first rather than find out at runtime
	// inside a script.
	if !ra.Type().Comparable() {
		return false
	}
	return a == b
}

// unwrapUserdata reduces Go's wrapper structs to the thing C++ would have
// compared. luaItem carries a position alongside the item pointer; two handles
// on one item obtained from different places would otherwise differ on the
// position alone, which is not what `itemA == itemB` asks.
func unwrapUserdata(v any) any {
	switch t := v.(type) {
	case luaItem:
		return t.item
	case luaContainer:
		return t.item
	}
	return v
}
