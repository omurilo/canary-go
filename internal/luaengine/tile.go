package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const tileTypeName = "Tile"

func (e *Engine) registerTile() {
	mt := e.L.NewTypeMetatable(tileTypeName)
	methods := e.tileMethods()
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), methods))
	e.setClassConstructor("Tile", e.tileCreate, methods)
}

func (e *Engine) tileCreate(L *lua.LState) int {
	var pos game.Position
	if L.GetTop() >= 4 { // Arg 1 is class
		x := L.CheckInt(2)
		y := L.CheckInt(3)
		z := L.CheckInt(4)
		pos = game.Position{X: uint16(x), Y: uint16(y), Z: uint8(z)}
	} else if L.GetTop() >= 2 {
		pos = checkPosition(L, 2)
	} else {
		L.ArgError(2, "Position or X, Y, Z expected")
		return 0
	}

	tile := e.world.Map.GetTile(pos)
	if tile == nil {
		L.Push(lua.LNil)
		return 1
	}

	pushTile(L, tile, pos)
	return 1
}

type luaTile struct {
	tile *game.Tile
	pos  game.Position
}

func pushTile(L *lua.LState, t *game.Tile, pos game.Position) {
	if t == nil {
		L.Push(lua.LNil)
		return
	}
	ud := L.NewUserData()
	ud.Value = luaTile{tile: t, pos: pos}
	L.SetMetatable(ud, L.GetTypeMetatable(tileTypeName))
	L.Push(ud)
}

func checkTile(L *lua.LState, n int) luaTile {
	ud := L.CheckUserData(n)
	if v, ok := ud.Value.(luaTile); ok {
		return v
	}
	L.ArgError(n, "Tile expected")
	return luaTile{}
}

func (e *Engine) tileMethods() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"isTile": func(L *lua.LState) int { L.Push(lua.LTrue); return 1 },
		"hasFlag": func(L *lua.LState) int {
			t := checkTile(L, 1)
			flag := uint32(L.CheckInt(2))
			L.Push(lua.LBool((uint32(t.tile.Flags) & flag) != 0))
			return 1
		},
		"getPosition": func(L *lua.LState) int {
			t := checkTile(L, 1)
			pushPosition(L, t.pos)
			return 1
		},
		"getItemByType": func(L *lua.LState) int {
			t := checkTile(L, 1)
			itemID := uint16(L.CheckInt(2))
			
			if t.tile.Ground != nil && t.tile.Ground.ID == itemID {
				e.pushItem(L, t.tile.Ground)
				return 1
			}
			for _, it := range t.tile.Items {
				if it.ID == itemID {
					e.pushItem(L, it)
					return 1
				}
			}
			L.Push(lua.LNil)
			return 1
		},
		"getTopCreature": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if len(t.tile.Creatures) > 0 {
				e.pushCreature(L, t.tile.Creatures[0])
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
		"queryAdd": func(L *lua.LState) int {
			L.Push(lua.LNumber(0)) // RETURNVALUE_NOERROR
			return 1
		},
	}
}
