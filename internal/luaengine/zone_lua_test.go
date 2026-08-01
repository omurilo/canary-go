package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
)

// The Zone class used to sit behind mockClass, so every one of these calls returned
// a no-op userdata. zone:addArea alone is used 45 times in the datapack.
func TestZoneLuaAPI(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		local z = Zone("arena")
		assert(z ~= nil, "Zone(name) must construct")
		assert(z:getName() == "arena")
		-- the same name gives back the same zone, it does not create a second
		assert(Zone("arena"):getName() == "arena")
		assert(Zone.getByName("arena") ~= nil)
		assert(Zone.getByName("nosuchzone") == nil)

		assert(z:addArea(Position(100, 100, 7), Position(101, 100, 7)) == true)
		local positions = z:getPositions()
		assert(#positions == 2, "getPositions returned " .. #positions)

		local at = Zone.getByPosition(Position(100, 100, 7))
		assert(#at == 1, "getByPosition returned " .. #at)
		assert(at[1]:getName() == "arena")

		assert(z:subtractArea(Position(100, 100, 7), Position(100, 100, 7)) == true)
		assert(#z:getPositions() == 1)
		assert(#Zone.getByPosition(Position(100, 100, 7)) == 0)

		assert(z:setRemoveDestination(Position(50, 50, 7)) == true)
		assert(z:setMonsterVariant("arena-variant") == true)
		assert(z:refresh() == true)
		-- the member queries return tables even when empty
		assert(#z:getPlayers() == 0)
		assert(#z:getMonsters() == 0)
		assert(#z:getNpcs() == 0)
		assert(#z:getCreatures() == 0)
		assert(#z:getItems() == 0)
		assert(z:removePlayers() == true)
		assert(z:removeMonsters() == true)
		assert(z:removeNpcs() == true)

		assert(#Zone.getAll() >= 1)
	`); err != nil {
		t.Fatalf("%v", err)
	}

	if z := e.world.Zones.ByName("arena"); z == nil {
		t.Fatal("the zone did not reach the registry")
	} else if z.MonsterVariant() != "arena-variant" {
		t.Errorf("MonsterVariant = %q, want arena-variant", z.MonsterVariant())
	}
}

// A zone the map created is reachable from Lua by name once the XML has named it,
// and its positions are the OTBM's.
func TestZoneLuaSeesMapZones(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	e.world.Zones.ApplyZonePositions(map[uint16][]game.Position{
		3: {{X: 10, Y: 10, Z: 7}},
	})
	if _, err := e.world.Zones.Add("from-map", 3); err != nil {
		t.Fatal(err)
	}
	if err := e.L.DoString(`
		local z = Zone.getByName("from-map")
		assert(z ~= nil, "a map zone must be visible to scripts")
		assert(#z:getPositions() == 1)
		local p = z:getPositions()[1]
		assert(p.x == 10 and p.y == 10 and p.z == 7, "position did not survive")
	`); err != nil {
		t.Fatalf("%v", err)
	}
}

// position:getTile() was a stub that returned nil for every position, so every
// caller took its "no tile here" branch even on a fully loaded map. That is what
// made Zone:randomPosition (data/libs/systems/zones.lua) report "no valid
// positions" for a zone whose addArea had worked perfectly: it filters positions
// with `tile and ...` and the tile was always nil.
func TestPositionGetTileAndGetZones(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	here := game.Position{X: 500, Y: 500, Z: 7}
	e.world.Map.SetTile(here, &game.Tile{Ground: &game.Item{ID: 1}})

	z, _ := e.world.Zones.Add("plaza", 0)
	z.AddArea(game.Area{From: here, To: here})

	if err := e.L.DoString(`
		local tile = Position(500, 500, 7):getTile()
		assert(tile ~= nil, "a loaded tile must be returned, not nil")
		local p = tile:getPosition()
		assert(p.x == 500 and p.y == 500 and p.z == 7, "the tile must carry its position")

		-- No tile at all is still nil, which is what the datapack guards on.
		assert(Position(1, 1, 7):getTile() == nil)

		local zones = Position(500, 500, 7):getZones()
		assert(zones ~= nil, "getZones on a real tile must return a table")
		assert(#zones == 1, "expected one zone, got " .. #zones)
		assert(zones[1]:getName() == "plaza")

		-- C++ returns nil rather than an empty table when there is no tile.
		assert(Position(1, 1, 7):getZones() == nil)
	`); err != nil {
		t.Fatalf("%v", err)
	}
}

// The end-to-end shape of the raid script that was failing: build a zone from two
// areas, then filter its positions by whether a tile exists there.
func TestZonePositionsFilterByTile(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	// Two of the four positions have tiles.
	for _, p := range []game.Position{{X: 10, Y: 10, Z: 7}, {X: 11, Y: 10, Z: 7}} {
		e.world.Map.SetTile(p, &game.Tile{Ground: &game.Item{ID: 1}})
	}
	if err := e.L.DoString(`
		local z = Zone("raid-area")
		z:addArea(Position(10, 10, 7), Position(11, 10, 7))
		z:addArea(Position(20, 20, 7), Position(21, 20, 7))
		local positions = z:getPositions()
		assert(#positions == 4, "zone should span 4 positions, got " .. #positions)
		local withTiles = 0
		for _, pos in ipairs(positions) do
			if pos:getTile() then withTiles = withTiles + 1 end
		end
		assert(withTiles == 2, "expected 2 positions with tiles, got " .. withTiles)
	`); err != nil {
		t.Fatalf("%v", err)
	}
}

// The datapack defines Tile:isWalkable itself (data/libs/functions/tile.lua:39) and
// Go's class-table sharing makes it reachable, so there was never a Go method to
// add. What it needs underneath is a hasProperty that answers the property it was
// actually asked about — this exercises the real thing end to end.
func TestDatapackIsWalkableRunsAgainstRealProperties(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	e.world.Items = items.NewCatalog(
		&items.ItemType{ID: 1, Name: "grass", Movable: false},
		&items.ItemType{ID: 2, Name: "wall", BlockSolid: true, BlockProjectile: true, Movable: false},
	)

	open := game.Position{X: 10, Y: 10, Z: 7}
	blocked := game.Position{X: 11, Y: 10, Z: 7}
	noGround := game.Position{X: 12, Y: 10, Z: 7}
	e.world.Map.SetTile(open, &game.Tile{Ground: &game.Item{ID: 1}})
	e.world.Map.SetTile(blocked, &game.Tile{Ground: &game.Item{ID: 1}, Items: []*game.Item{{ID: 2}}})
	e.world.Map.SetTile(noGround, &game.Tile{})

	// The subset of data/libs/functions/tile.lua the callers actually reach, with
	// the same argument list the datapack uses: (pz, creature, floorchange, block, proj).
	if err := e.L.DoString(`
		function Tile:isWalkable(pz, creature, floorchange, block, proj)
			if not self then return false end
			if not self:getGround() then return false end
			if self:hasProperty(CONST_PROP_BLOCKSOLID) or self:hasProperty(CONST_PROP_BLOCKPROJECTILE) then
				return false
			end
			if self:hasProperty(CONST_PROP_IMMOVABLEBLOCKSOLID) then return false end
			if creature and self:getTopCreature() ~= nil then return false end
			return true
		end

		local openTile = Position(10, 10, 7):getTile()
		assert(openTile ~= nil, "the open tile must exist")
		assert(openTile:isWalkable(false, false, false, false, true) == true, "plain ground is walkable")

		local blockedTile = Position(11, 10, 7):getTile()
		assert(blockedTile ~= nil, "the blocked tile must exist")
		assert(blockedTile:isWalkable(false, false, false, false, true) == false,
			"a tile holding a solid wall must not be walkable")

		local groundless = Position(12, 10, 7):getTile()
		assert(groundless ~= nil)
		assert(groundless:isWalkable(false, false, false, false, true) == false, "no ground, not walkable")
	`); err != nil {
		t.Fatalf("%v", err)
	}
}

// hasProperty must answer the question asked. With the argument discarded, every
// property returned whatever BlockSolid was, so a tile that only blocks projectiles
// looked walkable.
func TestTileHasPropertyDistinguishesProperties(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	e.world.Items = items.NewCatalog(
		&items.ItemType{ID: 1, Name: "grass"},
		// Blocks projectiles and pathfinding, but is NOT solid.
		&items.ItemType{ID: 3, Name: "bush", BlockProjectile: true, BlockPathFind: true},
	)
	pos := game.Position{X: 5, Y: 5, Z: 7}
	e.world.Map.SetTile(pos, &game.Tile{Ground: &game.Item{ID: 1}, Items: []*game.Item{{ID: 3}}})

	if err := e.L.DoString(`
		local tile = Position(5, 5, 7):getTile()
		assert(tile:hasProperty(CONST_PROP_BLOCKSOLID) == false, "the bush is not solid")
		assert(tile:hasProperty(CONST_PROP_BLOCKPROJECTILE) == true, "but it does block projectiles")
		assert(tile:hasProperty(CONST_PROP_BLOCKPATH) == true, "and pathfinding")
		assert(tile:hasProperty(CONST_PROP_HASHEIGHT) == false)
	`); err != nil {
		t.Fatalf("%v", err)
	}
}
