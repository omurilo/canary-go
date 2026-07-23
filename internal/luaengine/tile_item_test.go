package luaengine

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
)

func TestTileAndItemLuaMethods(t *testing.T) {
	e := newTestEngine()
	w := e.world

	pos := game.Position{X: 100, Y: 100, Z: 7}
	ground := &game.Item{ID: 101, Count: 1}
	dummy := &game.Item{ID: 28558, Count: 1}
	
	tile := &game.Tile{
		Ground: ground,
		Items:  []*game.Item{dummy},
	}
	w.Map.SetTile(pos, tile)

	// Register tile and items in engine world context
	e.registerItem()
	e.registerTile()

	// Push Tile as global "myTile"
	pushTile(e.L, tile, pos)
	tileVal := e.L.Get(-1)
	e.L.Pop(1)
	e.L.SetGlobal("myTile", tileVal)

	// Pushing dummy item as "myDummy"
	e.pushItem(e.L, dummy)
	dummyVal := e.L.Get(-1)
	e.L.Pop(1)
	e.L.SetGlobal("myDummy", dummyVal)

	// Test 1: Item:actor() and Item:actor(bool)
	scriptItem := `
		assert(myDummy:actor() == false)
		assert(myDummy:actor(true) == true)
		assert(myDummy:actor() == true)
		assert(myDummy:actor(false) == true)
		assert(myDummy:actor() == false)
	`
	if err := e.DoString(scriptItem); err != nil {
		t.Fatalf("item actor tests failed: %v", err)
	}

	// Test 2: Tile:getItemById()
	scriptTile := `
		local foundGround = myTile:getItemById(101)
		assert(foundGround ~= nil)
		
		local foundDummy = myTile:getItemById(28558)
		assert(foundDummy ~= nil)

		local nonExistent = myTile:getItemById(9999)
		assert(nonExistent == nil)
	`
	if err := e.DoString(scriptTile); err != nil {
		t.Fatalf("tile getItemById tests failed: %v", err)
	}

	// Test 3: Item:setDestination() and Item:getDestination()
	scriptTeleport := `
		local dest = Position(123, 456, 7)
		myDummy:setDestination(dest)
		local gotDest = myDummy:getDestination()
		assert(gotDest.x == 123 and gotDest.y == 456 and gotDest.z == 7)
	`
	if err := e.DoString(scriptTeleport); err != nil {
		t.Fatalf("item setDestination tests failed: %v", err)
	}

	// Test 4: Tile:getTopVisibleThing() and Tile:getGround()
	scriptTopThing := `
		local topThing = myTile:getTopVisibleThing()
		assert(topThing ~= nil)
		assert(topThing:isItem() == true)
		local ground = myTile:getGround()
		assert(ground ~= nil)
	`
	if err := e.DoString(scriptTopThing); err != nil {
		t.Fatalf("tile getTopVisibleThing tests failed: %v", err)
	}
}
