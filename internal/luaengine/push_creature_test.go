package luaengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/talkactions"
	lua "github.com/yuin/gopher-lua"
)

func TestPushCreatureTalkActionAndClosestFreePosition(t *testing.T) {
	e := newTestEngine()

	// Register monster in TypeRegistry
	ratType := &creatures.MonsterType{
		Name:      "Rat",
		MaxHealth: 20,
		Speed:     150,
		Outfit: creatures.Outfit{
			LookType: 21,
		},
	}
	e.world.TypeRegistry.Monsters["rat"] = ratType

	coreDir, _ := filepath.Abs("../../data")
	dataDir, _ := filepath.Abs("../../data-otservbr-global")
	e.L.SetGlobal("CORE_DIRECTORY", lua.LString(coreDir))
	e.L.SetGlobal("DATA_DIRECTORY", lua.LString(dataDir))

	_ = e.DoFile(filepath.Join(coreDir, "global.lua"))
	_ = filepath.Walk(filepath.Join(coreDir, "libs"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && filepath.Ext(path) == ".lua" {
			_ = e.DoFile(path)
		}
		return nil
	})

	// Load push_creature.lua
	pushScript := filepath.Join(coreDir, "scripts", "talkactions", "gm", "push_creature.lua")
	if err := e.DoFile(pushScript); err != nil {
		t.Fatalf("load push_creature.lua: %v", err)
	}

	// Create God player
	p := &game.Player{Name: "GodPlayer", GroupID: 3, AccountType: 6}
	p.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	e.world.AddPlayer(p, nil)

	// Create a 5x5 grid of ground tiles around (100, 100, 7)
	for x := 98; x <= 102; x++ {
		for y := 98; y <= 102; y++ {
			tilePos := game.Position{X: uint16(x), Y: uint16(y), Z: 7}
			if e.world.Map.GetTile(tilePos) == nil {
				e.world.Map.SetTile(tilePos, &game.Tile{
					Ground: &game.Item{ID: 400},
				})
			}
		}
	}

	// Create a Rat monster on tile (101, 100, 7)
	pos := game.Position{X: 101, Y: 100, Z: 7}
	rat := game.NewMonster(e.world.GenerateCreatureID(), "Rat", ratType)
	rat.SetPosition(pos)
	e.world.AddCreature(rat)

	// Execute /c Rat
	ta, param := talkactions.FindByWords("/c Rat")
	if ta == nil {
		t.Fatal("talkaction /c not found")
	}

	success := e.CallTalkAction(ta, p, 1, "/c Rat", param)
	if !success {
		t.Fatal("CallTalkAction /c Rat failed")
	}

	// Verify Rat was teleported to a free position near player
	newPos := rat.GetPosition()
	if newPos.X == 101 && newPos.Y == 100 {
		t.Errorf("Rat position did not change: %v", newPos)
	}
}

func TestGameCreateMonsterAndNpcWithOutfit(t *testing.T) {
	e := newTestEngine()

	// Register monster & npc
	e.world.TypeRegistry.Monsters["demon"] = &creatures.MonsterType{
		Name:      "Demon",
		MaxHealth: 8200,
		Outfit: creatures.Outfit{
			LookType: 35,
		},
	}
	e.world.TypeRegistry.Npcs["rashid"] = &creatures.NpcType{
		Name:      "Rashid",
		MaxHealth: 100,
		Outfit: creatures.Outfit{
			LookType: 130,
		},
	}

	// Create map tile
	pos := game.Position{X: 100, Y: 100, Z: 7}
	e.world.Map.SetTile(pos, &game.Tile{
		Ground: &game.Item{ID: 400},
	})

	// Run Lua script calling Game.createMonster and Game.createNpc
	script := `
		local m = Game.createMonster("Demon", Position(100, 100, 7))
		assert(m ~= nil, "Game.createMonster('Demon') returned nil")
		
		local n = Game.createNpc("Rashid", Position(100, 100, 7))
		assert(n ~= nil, "Game.createNpc('Rashid') returned nil")
	`
	if err := e.DoString(script); err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	// Verify created monster has correct LookType
	var demon *game.Monster
	for _, cr := range e.world.Map.GetTile(pos).Creatures {
		if m, ok := cr.(*game.Monster); ok {
			demon = m
			break
		}
	}
	if demon == nil {
		t.Fatal("Demon monster not found on tile")
	}
	if demon.Outfit.LookType != 35 {
		t.Errorf("Demon LookType = %d, want 35", demon.Outfit.LookType)
	}

	// Verify created npc has correct LookType
	var rashid *game.Npc
	for _, cr := range e.world.Map.GetTile(pos).Creatures {
		if n, ok := cr.(*game.Npc); ok {
			rashid = n
			break
		}
	}
	if rashid == nil {
		t.Fatal("Rashid NPC not found on tile")
	}
	if rashid.Outfit.LookType != 130 {
		t.Errorf("Rashid LookType = %d, want 130", rashid.Outfit.LookType)
	}
}
