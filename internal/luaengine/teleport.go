package luaengine

import (
	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const luaTeleportTypeName = "Teleport"

// Teleport, from src/lua/functions/map/teleport_functions.cpp:17
//
//	Lua::registerSharedClass(L, "Teleport", "Item", TeleportFunctions::luaTeleportCreate);
//
// A shared class is a global TABLE whose metatable carries __call for the
// constructor, and whose instance metatable's __index is that same table
// (lua_functions_loader.cpp:747-786). "Item" is the base class, so anything
// Teleport does not define falls through to Item's methods.
//
// The global used to be a bare function returning nil. Two entries in the log,
// one cause:
//
//	data/libs/functions/teleport.lua:1: attempt to index a non-table
//	object(function) with key 'isTeleport'
//
// Line 1 of that lib is `function Teleport.isTeleport(self)`, so the file
// aborted on its first statement — and SimpleTeleport, defined on line 5 of the
// same file, therefore never existed:
//
//	oskayaat.lua:1: attempt to call a non-function object
func (e *Engine) registerTeleportType() {
	mt := e.L.NewTypeMetatable(luaTeleportTypeName)
	methods := map[string]lua.LGFunction{
		"getDestination": teleportGetDestination,
		"setDestination": teleportSetDestination,
	}

	// The global table with __call, and the instance metatable pointing at it so
	// a method the datapack adds in Lua is visible on every instance.
	classTable := e.setClassConstructor(luaTeleportTypeName, e.teleportCreate, methods)
	e.L.SetField(mt, "__index", classTable)

	// Base class Item: fall back to its methods for everything Teleport does not
	// define, which is what registerSharedClass's baseClass argument sets up.
	if itemMt, ok := e.L.GetTypeMetatable(itemTypeName).(*lua.LTable); ok {
		if idx, ok := e.L.GetField(itemMt, "__index").(*lua.LTable); ok {
			classMt := e.L.NewTable()
			e.L.SetField(classMt, "__index", idx)
			e.L.SetMetatable(classTable, classMt)
		}
	}
}

// teleportCreate is luaTeleportCreate: Teleport(uid) hands back the item with
// the Teleport metatable, or nil when it is not one.
//
// The script environment's uid table is not modelled here, so this only
// resolves an Item userdata the caller already holds. A numeric uid returns nil
// rather than pretending — the datapack reaches teleports through Tile and
// MoveEvent, not by uid.
func (e *Engine) teleportCreate(L *lua.LState) int {
	if L.GetTop() >= 2 {
		if ud, ok := L.Get(2).(*lua.LUserData); ok {
			if it, ok := ud.Value.(luaItem); ok && it.item != nil {
				out := L.NewUserData()
				out.Value = it
				L.SetMetatable(out, L.GetTypeMetatable(luaTeleportTypeName))
				L.Push(out)
				return 1
			}
		}
	}
	L.Push(lua.LNil)
	return 1
}

// teleportDestinationKey holds the destination until Teleport is backed by a
// real game type. ATTR_TELEPORT_DEST is the upstream attribute; keeping it in
// the item's custom attributes at least survives a set/get round trip instead
// of being dropped, which is what the previous no-op pair did.
const teleportDestinationKey = "teleport-destination"

func teleportGetDestination(L *lua.LState) int {
	it := checkItem(L)
	if it.item == nil || it.item.Attr == nil || it.item.Attr.Custom == nil {
		L.Push(lua.LNil)
		return 1
	}
	packed, ok := it.item.Attr.Custom[teleportDestinationKey].(int64)
	if !ok {
		L.Push(lua.LNil)
		return 1
	}
	pushPosition(L, game.Position{
		X: uint16(packed >> 24 & 0xFFFF),
		Y: uint16(packed >> 8 & 0xFFFF),
		Z: uint8(packed & 0xFF),
	})
	return 1
}

func teleportSetDestination(L *lua.LState) int {
	it := checkItem(L)
	if it.item == nil {
		L.Push(lua.LNil)
		return 1
	}
	pos := checkPosition(L, 2)
	if it.item.Attr == nil {
		it.item.Attr = &game.ItemAttributes{}
	}
	if it.item.Attr.Custom == nil {
		it.item.Attr.Custom = map[string]any{}
	}
	it.item.Attr.Custom[teleportDestinationKey] = int64(pos.X)<<24 | int64(pos.Y)<<8 | int64(pos.Z)
	L.Push(lua.LTrue)
	return 1
}
