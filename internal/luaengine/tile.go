package luaengine

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
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
		var ok bool
		pos, ok = parsePosition(L, 2)
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
	} else {
		L.Push(lua.LNil)
		return 1
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
			// Lenient like C++ getNumber: a nil/absent flag (e.g. a TILESTATE_*
			// constant the datapack references but the engine doesn't define) reads
			// as 0 instead of raising, so the calling script keeps running.
			flag := uint32(L.OptInt(2, 0))
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
		"getItemById": func(L *lua.LState) int {
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
		"getItemCountById": func(L *lua.LState) int {
			t := checkTile(L, 1)
			itemID := uint16(L.CheckInt(2))
			var count int
			if t.tile.Ground != nil && t.tile.Ground.ID == itemID {
				count++
			}
			for _, it := range t.tile.Items {
				if it.ID == itemID {
					count++
				}
			}
			L.Push(lua.LNumber(count))
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
		"getBottomCreature": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if len(t.tile.Creatures) > 0 {
				e.pushCreature(L, t.tile.Creatures[len(t.tile.Creatures)-1])
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
		"getTopVisibleCreature": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if len(t.tile.Creatures) > 0 {
				e.pushCreature(L, t.tile.Creatures[0])
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
		"getTopVisibleThing": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if len(t.tile.Creatures) > 0 {
				e.pushCreature(L, t.tile.Creatures[0])
				return 1
			}
			if len(t.tile.Items) > 0 {
				e.pushItem(L, t.tile.Items[len(t.tile.Items)-1])
				return 1
			}
			if t.tile.Ground != nil {
				e.pushItem(L, t.tile.Ground)
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
		"getGround": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if t.tile.Ground != nil {
				e.pushItem(L, t.tile.Ground)
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
		"getTopTopItem": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if len(t.tile.Items) > 0 {
				e.pushItem(L, t.tile.Items[len(t.tile.Items)-1])
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
		"getTopDownItem": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if len(t.tile.Items) > 0 {
				e.pushItem(L, t.tile.Items[0])
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
		"getThing": func(L *lua.LState) int {
			t := checkTile(L, 1)
			idx := L.OptInt(2, 0)
			if idx <= 0 {
				if len(t.tile.Items) > 0 {
					e.pushItem(L, t.tile.Items[len(t.tile.Items)-1])
					return 1
				}
				if t.tile.Ground != nil {
					e.pushItem(L, t.tile.Ground)
					return 1
				}
			} else if idx <= len(t.tile.Items) {
				e.pushItem(L, t.tile.Items[idx-1])
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
		"getCreatureCount": func(L *lua.LState) int {
			t := checkTile(L, 1)
			L.Push(lua.LNumber(len(t.tile.Creatures)))
			return 1
		},
		"getCreatures": func(L *lua.LState) int {
			t := checkTile(L, 1)
			tbl := L.NewTable()
			for i, cr := range t.tile.Creatures {
				ud := L.NewUserData()
				ud.Value = cr
				L.SetMetatable(ud, L.GetTypeMetatable(metatableForCreature(cr)))
				tbl.RawSetInt(i+1, ud)
			}
			L.Push(tbl)
			return 1
		},
		// tile:hasProperty(prop) ports Tile::hasProperty (tile.cpp), which reads the
		// TILESTATE_ flag the tile accumulated as items were added. Go's Tile keeps
		// only the OTBM flags, so the value is derived the way setTileFlags builds it:
		// the tile has a property when any thing on it does.
		//
		// The property argument used to be discarded and BlocksSolid answered
		// everything, which made the datapack's Tile:isWalkable — the one that filters
		// zone positions and teleport destinations — accept tiles that block
		// projectiles or pathfinding.
		"hasProperty": func(L *lua.LState) int {
			t := checkTile(L, 1)
			prop := items.ItemProperty(L.OptInt(2, 0))
			L.Push(lua.LBool(tileHasProperty(t.tile, e.itemCatalog(), prop)))
			return 1
		},
		"getItemCount": func(L *lua.LState) int {
			t := checkTile(L, 1)
			L.Push(lua.LNumber(len(t.tile.Items)))
			return 1
		},
		"getItems": func(L *lua.LState) int {
			t := checkTile(L, 1)
			tbl := L.NewTable()
			for i, it := range t.tile.Items {
				ud := L.NewUserData()
				ud.Value = luaItem{item: it, pos: t.pos}
				L.SetMetatable(ud, L.GetTypeMetatable(itemTypeName))
				tbl.RawSetInt(i+1, ud)
			}
			L.Push(tbl)
			return 1
		},
		"getThingCount": func(L *lua.LState) int {
			t := checkTile(L, 1)
			count := len(t.tile.Items) + len(t.tile.Creatures)
			if t.tile.Ground != nil {
				count++
			}
			L.Push(lua.LNumber(count))
			return 1
		},
		"queryAdd": func(L *lua.LState) int {
			L.Push(lua.LNumber(0)) // RETURNVALUE_NOERROR
			return 1
		},
		"getHouse": func(L *lua.LState) int {
			tile := checkTile(L, 1)
			if tile.tile == nil || tile.tile.HouseID == 0 {
				L.Push(lua.LNil)
				return 1
			}
			world := e.world
			if world == nil {
				L.Push(lua.LNil)
				return 1
			}
			h := world.Houses[tile.tile.HouseID]
			if h == nil {
				L.Push(lua.LNil)
				return 1
			}
			ud := L.NewUserData()
			ud.Value = h
			L.SetMetatable(ud, L.GetTypeMetatable(houseTypeName))
			L.Push(ud)
			return 1
		},
		"getFieldItem": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if t.tile == nil {
				L.Push(lua.LNil)
				return 1
			}
			cat := e.itemCatalog()
			for _, it := range t.tile.Items {
				if cat != nil {
					if typ := cat.Get(it.ID); typ != nil && typ.TypeName == "magicfield" {
						e.pushItem(L, it)
						return 1
					}
				}
			}
			L.Push(lua.LNil)
			return 1
		},
		"getItemByTopOrder": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if t.tile == nil {
				L.Push(lua.LNil)
				return 1
			}
			order := L.CheckInt(2)
			cat := e.itemCatalog()
			for _, it := range t.tile.Items {
				if cat != nil {
					if typ := cat.Get(it.ID); typ != nil && int(typ.AlwaysOnTopOrder) == order {
						e.pushItem(L, it)
						return 1
					}
				}
			}
			L.Push(lua.LNil)
			return 1
		},
		"getBottomVisibleCreature": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if t.tile == nil {
				L.Push(lua.LNil)
				return 1
			}
			if len(t.tile.Creatures) > 0 {
				e.pushCreature(L, t.tile.Creatures[len(t.tile.Creatures)-1])
				return 1
			}
			L.Push(lua.LNil)
			return 1
		},
		"getDownItemCount": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if t.tile == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			count := 0
			cat := e.itemCatalog()
			for _, it := range t.tile.Items {
				if cat != nil {
					if typ := cat.Get(it.ID); typ != nil && typ.AlwaysOnTopOrder == 0 {
						count++
					}
				}
			}
			L.Push(lua.LNumber(count))
			return 1
		},
		"getTopItemCount": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if t.tile == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			count := 0
			cat := e.itemCatalog()
			for _, it := range t.tile.Items {
				if cat != nil {
					if typ := cat.Get(it.ID); typ != nil && typ.AlwaysOnTopOrder > 0 {
						count++
					}
				}
			}
			L.Push(lua.LNumber(count))
			return 1
		},
		"getThingIndex": func(L *lua.LState) int {
			t := checkTile(L, 1)
			if t.tile == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			if L.GetTop() < 2 {
				L.Push(lua.LNumber(0))
				return 1
			}
			ud, ok := L.Get(2).(*lua.LUserData)
			if !ok {
				L.Push(lua.LNumber(0))
				return 1
			}
			// Check for creature
			for i, cr := range t.tile.Creatures {
				if ud.Value == cr {
					L.Push(lua.LNumber(i + 1))
					return 1
				}
			}
			// Check for luaItem
			if li, ok := ud.Value.(luaItem); ok {
				if t.tile.Ground != nil && t.tile.Ground == li.item {
					L.Push(lua.LNumber(0))
					return 1
				}
				for i, it := range t.tile.Items {
					if it == li.item {
						L.Push(lua.LNumber(i + 1))
						return 1
					}
				}
			}
			L.Push(lua.LNumber(0))
			return 1
		},
	}
}

// tileHasProperty reports whether any thing on the tile has the property, which is
// how Tile::setTileFlags accumulates the TILESTATE_ flags that Tile::hasProperty
// then reads back.
func tileHasProperty(t *game.Tile, cat *items.Catalog, prop items.ItemProperty) bool {
	if t == nil || cat == nil {
		return false
	}
	if t.Ground != nil && cat.Get(t.Ground.ID).HasProperty(prop) {
		return true
	}
	for _, it := range t.Items {
		if it != nil && cat.Get(it.ID).HasProperty(prop) {
			return true
		}
	}
	return false
}
