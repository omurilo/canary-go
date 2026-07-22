package game

import (
	"strings"
	"testing"
	"time"

	"github.com/opentibiabr/canary-go/internal/creatures"
)

func TestSpawnCreatureLookup(t *testing.T) {
	w := NewWorld()
	reg := creatures.NewTypeRegistry()
	w.TypeRegistry = reg

	// Register a test monster type
	mName := "Azure Frog"
	reg.Monsters[strings.ToLower(mName)] = &creatures.MonsterType{
		Name:      mName,
		MaxHealth: 60,
		Speed:     120,
		Outfit: creatures.Outfit{
			LookType: 226,
		},
	}

	se := NewSpawnEngine(w, reg)

	// Add a spawn for this monster
	se.AddSpawn(mName, Position{X: 100, Y: 100, Z: 7}, 2, 90*time.Second, false)

	if len(se.spawns) != 1 {
		t.Fatalf("expected 1 spawn, got %d", len(se.spawns))
	}

	// Spawn the creature
	se.spawnCreature(se.spawns[0])

	// Check if the spawned creature has the correct properties
	creatureID := se.spawns[0].creatureID
	if creatureID == 0 {
		t.Fatal("creature was not spawned (ID is 0)")
	}

	c := w.CreatureByID(creatureID)
	if c == nil {
		t.Fatal("spawned creature not found in world")
	}

	monster, ok := c.(*Monster)
	if !ok {
		t.Fatalf("spawned creature is not a Monster, got %T", c)
	}

	if monster.GetName() != mName {
		t.Errorf("expected name %q, got %q", mName, monster.GetName())
	}

	if monster.GetMaxHealth() != 60 {
		t.Errorf("expected health 60, got %d (fallback rat value is 100)", monster.GetMaxHealth())
	}

	if monster.GetOutfit().LookType != 226 {
		t.Errorf("expected lookType 226, got %d (fallback rat value is 21)", monster.GetOutfit().LookType)
	}
}
