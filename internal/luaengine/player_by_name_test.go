package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
)

func TestPlayerConstructorByName(t *testing.T) {
	w := game.NewWorld()
	e := New(w, nil)

	p := &game.Player{ID: 100, Name: "Target Player"}
	w.AddPlayer(p, nil)

	L := e.L
	err := L.DoString(`
		local p = Player("Target Player")
		if not p then
			error("expected Player('Target Player') to return player userdata, got nil")
		end
		if p:getName() ~= "Target Player" then
			error("expected player name 'Target Player', got " .. tostring(p:getName()))
		end

		local missing = Player("NonExistentPlayer")
		if missing ~= nil then
			error("expected Player('NonExistentPlayer') to return nil, got " .. tostring(missing))
		end
	`)
	if err != nil {
		t.Fatalf("Lua execution error: %v", err)
	}
}

func TestPositionConstructorUserdata(t *testing.T) {
	w := game.NewWorld()
	e := New(w, nil)

	L := e.L
	err := L.DoString(`
		local p1 = Position(100, 200, 7)
		local p2 = Position(p1)
		if p2.x ~= 100 or p2.y ~= 200 or p2.z ~= 7 then
			error("expected Position(Position) to copy coordinates (100, 200, 7), got " .. tostring(p2.x) .. ", " .. tostring(p2.y) .. ", " .. tostring(p2.z))
		end
	`)
	if err != nil {
		t.Fatalf("Lua execution error: %v", err)
	}
}

func TestWebhookAndAnnouncementChannels(t *testing.T) {
	w := game.NewWorld()
	e := New(w, nil)

	err := e.L.DoString(`
		assert(Webhook ~= nil, "Webhook must not be nil")
		assert(announcementChannels ~= nil, "announcementChannels must not be nil")
		assert(WEBHOOK_COLOR_WARNING ~= nil, "WEBHOOK_COLOR_WARNING must not be nil")

		Webhook.sendMessage("test title", "test message", WEBHOOK_COLOR_WARNING, announcementChannels["serverAnnouncements"])
		Webhook.sendMessage(":man_wearing_turban: test message", announcementChannels["serverAnnouncements"])
	`)
	if err != nil {
		t.Fatalf("Webhook test failed: %v", err)
	}
}

func TestMonsterTypeByNameAndAddItem(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry.Monsters["dragon"] = &creatures.MonsterType{Name: "Dragon"}
	e := New(w, nil)

	err := e.L.DoString(`
		local mt = Game.getMonsterTypeByName("Dragon")
		assert(mt ~= nil, "Game.getMonsterTypeByName('Dragon') returned nil")
		assert(mt:getName() == "Dragon", "Expected name Dragon")

		local types = Game.getMonsterTypes()
		assert(types["dragon"] ~= nil, "Game.getMonsterTypes() missing dragon")

		local container = Container(1988)
		local item = container:addItem(2160, 10)
		assert(item ~= nil, "container:addItem must return item")

		local bc = Game.getBoostedCreature()
		assert(type(bc) == "string" and #bc > 0, "Game.getBoostedCreature must return non-empty string")
		local bb = Game.getBoostedBoss()
		assert(type(bb) == "string" and #bb > 0, "Game.getBoostedBoss must return non-empty string")
	`)
	if err != nil {
		t.Fatalf("TestMonsterTypeByNameAndAddItem failed: %v", err)
	}
}

func TestTileMethods(t *testing.T) {
	w := game.NewWorld()
	e := New(w, nil)

	pos := game.Position{X: 100, Y: 100, Z: 7}
	tile := &game.Tile{
		Creatures: []game.Creature{&game.Player{ID: 1, Name: "Test"}},
	}
	w.Map.SetTile(pos, tile)

	L := e.L
	err := L.DoString(`
		local tile = Tile(100, 100, 7)
		if not tile then
			error("expected Tile(100, 100, 7) to return tile object")
		end
		if tile:getCreatureCount() ~= 1 then
			error("expected creature count 1, got " .. tostring(tile:getCreatureCount()))
		end
		if tile:hasProperty(3) ~= false then
			error("expected hasProperty(3) to be false")
		end
		if tile:hasProperty(CONST_PROP_IMMOVABLEBLOCKSOLID) ~= false then
			error("expected hasProperty(CONST_PROP_IMMOVABLEBLOCKSOLID) to be false")
		end
	`)
	if err != nil {
		t.Fatalf("Lua execution error: %v", err)
	}
}

func TestItemTypeClass(t *testing.T) {
	w := game.NewWorld()
	e := New(w, nil)
	L := e.L
	err := L.DoString(`
		local it = ItemType(2160)
		if not it then error("expected ItemType(2160)") end
		if type(it:getName()) ~= "string" then error("expected string from getName()") end
	`)
	if err != nil {
		t.Fatalf("Lua ItemType error: %v", err)
	}
}
