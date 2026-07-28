package luaengine

import (
	"fmt"

	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const positionTypeName = "Position"

func (e *Engine) registerPosition() {
	mt := e.L.NewTypeMetatable(positionTypeName)

	positionMethods := map[string]lua.LGFunction{
		"isPosition": func(L *lua.LState) int { L.Push(lua.LTrue); return 1 },
		"sendSingleSoundEffect": func(L *lua.LState) int {
			return 0
		},
		"sendMagicEffect": func(L *lua.LState) int {
			p := checkPosition(L, 1)
			var effect uint16
			if L.GetTop() >= 2 && L.Get(2) != lua.LNil {
				effect = uint16(L.OptInt(2, 0))
			}
			if e.world.OnMagicEffect != nil {
				e.world.OnMagicEffect(p, effect)
			}
			return 0
		},
		"sendDistanceEffect": func(L *lua.LState) int {
			from := checkPosition(L, 1)
			to := checkPosition(L, 2)
			effect := uint16(L.CheckInt(3))
			if e.world.OnDistanceEffect != nil {
				e.world.OnDistanceEffect(from, to, effect)
			}
			return 0
		},
		"getDistance": func(L *lua.LState) int {
			p1 := checkPosition(L, 1)
			p2 := checkPosition(L, 2)
			if p1.Z != p2.Z {
				L.Push(lua.LNumber(0xFFFF))
				return 1
			}
			dx := int(p1.X) - int(p2.X)
			if dx < 0 {
				dx = -dx
			}
			dy := int(p1.Y) - int(p2.Y)
			if dy < 0 {
				dy = -dy
			}
			dist := dx
			if dy > dx {
				dist = dy
			}
			L.Push(lua.LNumber(dist))
			return 1
		},
		"toString": func(L *lua.LState) int {
			p := checkPosition(L, 1)
			L.Push(lua.LString(fmt.Sprintf("%d, %d, %d", p.X, p.Y, p.Z)))
			return 1
		},
		"__tostring": func(L *lua.LState) int {
			p := checkPosition(L, 1)
			L.Push(lua.LString(fmt.Sprintf("%d, %d, %d", p.X, p.Y, p.Z)))
			return 1
		},
		"isSightClear": func(L *lua.LState) int {
			from := checkPosition(L, 1)
			target := checkPosition(L, 2)
			_ = target
			_ = from
			L.Push(lua.LTrue)
			return 1
		},
		"getTile": func(L *lua.LState) int {
			pos := checkPosition(L, 1)
			_ = pos
			L.Push(lua.LNil)
			return 1
		},
		"getZones": func(L *lua.LState) int {
			L.Push(L.NewTable())
			return 1
		},
		"removeMagicEffect": func(L *lua.LState) int {
			return 0
		},
		"sendDoubleSoundEffect": func(L *lua.LState) int {
			return 0
		},
	}

	methods := e.L.SetFuncs(e.L.NewTable(), positionMethods)
	e.L.SetField(mt, "methods", methods)

	e.L.SetField(mt, "__index", e.L.NewFunction(positionIndex))
	e.L.SetField(mt, "__newindex", e.L.NewFunction(positionNewIndex))
	e.L.SetField(mt, "__eq", e.L.NewFunction(positionEq))

	e.setClassConstructor("Position", positionCreate, positionMethods)
}

func positionCreate(L *lua.LState) int {
	var x, y, z int
	arg2 := L.Get(2)
	switch arg2.Type() {
	case lua.LTTable:
		t := L.ToTable(2)
		x = int(lua.LVAsNumber(L.GetField(t, "x")))
		y = int(lua.LVAsNumber(L.GetField(t, "y")))
		z = int(lua.LVAsNumber(L.GetField(t, "z")))
	case lua.LTUserData:
		if pos, ok := arg2.(*lua.LUserData).Value.(game.Position); ok {
			x, y, z = int(pos.X), int(pos.Y), int(pos.Z)
		} else if posPtr, ok := arg2.(*lua.LUserData).Value.(*game.Position); ok && posPtr != nil {
			x, y, z = int(posPtr.X), int(posPtr.Y), int(posPtr.Z)
		}
	default:
		x = L.OptInt(2, 0)
		y = L.OptInt(3, 0)
		z = L.OptInt(4, 7)
	}

	p := game.Position{
		X: uint16(x),
		Y: uint16(y),
		Z: uint8(z),
	}

	pushPosition(L, p)
	return 1
}

func parsePosition(L *lua.LState, n int) (game.Position, bool) {
	v := L.Get(n)
	switch v.Type() {
	case lua.LTUserData:
		ud := v.(*lua.LUserData)
		if p, ok := ud.Value.(game.Position); ok {
			return p, true
		}
		if posPtr, ok := ud.Value.(*game.Position); ok && posPtr != nil {
			return *posPtr, true
		}
	case lua.LTTable:
		t := v.(*lua.LTable)
		return game.Position{
			X: uint16(lua.LVAsNumber(L.GetField(t, "x"))),
			Y: uint16(lua.LVAsNumber(L.GetField(t, "y"))),
			Z: uint8(lua.LVAsNumber(L.GetField(t, "z"))),
		}, true
	}
	return game.Position{}, false
}

func checkPosition(L *lua.LState, n int) game.Position {
	if pos, ok := parsePosition(L, n); ok {
		return pos
	}
	L.ArgError(n, "Position expected")
	return game.Position{}
}

func PushPosition(L *lua.LState, p game.Position) {
	ud := L.NewUserData()
	ud.Value = p
	L.SetMetatable(ud, L.GetTypeMetatable(positionTypeName))
	L.Push(ud)
}

func pushPosition(L *lua.LState, p game.Position) {
	PushPosition(L, p)
}

func positionIndex(L *lua.LState) int {
	ud := L.CheckUserData(1)
	key := L.CheckString(2)
	if p, ok := ud.Value.(game.Position); ok {
		switch key {
		case "x":
			L.Push(lua.LNumber(p.X))
			return 1
		case "y":
			L.Push(lua.LNumber(p.Y))
			return 1
		case "z":
			L.Push(lua.LNumber(p.Z))
			return 1
		}
	}
	mt := L.GetTypeMetatable(positionTypeName)
	methods := L.GetField(mt, "methods")
	if methods.Type() == lua.LTTable {
		val := L.GetField(methods, key)
		if val.Type() != lua.LTNil {
			L.Push(val)
			return 1
		}
	}
	// Fallback to searching the global "Position" table for Lua-defined methods
	if gPos := L.GetGlobal("Position"); gPos.Type() == lua.LTTable {
		val := L.GetField(gPos, key)
		if val.Type() != lua.LTNil {
			L.Push(val)
			return 1
		}
	}
	L.Push(lua.LNil)
	return 1
}

func positionNewIndex(L *lua.LState) int {
	ud := L.CheckUserData(1)
	if p, ok := ud.Value.(game.Position); ok {
		key := L.CheckString(2)
		val := L.CheckNumber(3)
		switch key {
		case "x":
			p.X = uint16(val)
			ud.Value = p
		case "y":
			p.Y = uint16(val)
			ud.Value = p
		case "z":
			p.Z = uint8(val)
			ud.Value = p
		}
	}
	return 0
}

func positionEq(L *lua.LState) int {
	p1 := checkPosition(L, 1)
	p2 := checkPosition(L, 2)
	L.Push(lua.LBool(p1.X == p2.X && p1.Y == p2.Y && p1.Z == p2.Z))
	return 1
}


