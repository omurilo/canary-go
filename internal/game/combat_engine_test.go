package game

import (
	"testing"
	"time"

	"github.com/opentibiabr/canary-go/internal/game/combat"
)

// newTestCreature-ish helpers: build a small world with two adjacent tiles.
func newCombatWorld() *World {
	w := NewWorld()
	w.Map.SetTile(Position{X: 100, Y: 100, Z: 7}, &Tile{Ground: &Item{ID: 1}})
	w.Map.SetTile(Position{X: 101, Y: 100, Z: 7}, &Tile{Ground: &Item{ID: 1}})
	return w
}

// TestCombatEngine_MeleeHitDamagesTarget verifies a ready attacker adjacent to a
// live target lands a hit that reduces the target's health and fires the
// health-change + combat-hit hooks.
func TestCombatEngine_MeleeHitDamagesTarget(t *testing.T) {
	w := newCombatWorld()

	var healthChanges, hits int
	w.OnCreatureHealthChange = func(Creature) { healthChanges++ }
	w.OnCombatHit = func(_, _ Creature, _ int32, _ uint16) { hits++ }

	monster := NewMonster(1, "Rat", nil)
	monster.MaxHealth, monster.Health = 1000, 1000
	monster.SetPosition(Position{X: 101, Y: 100, Z: 7})
	w.AddCreature(monster)

	e := NewCombatEngine(w)

	// A high-skill fist guarantees non-zero damage.
	attacker := &Player{Skills: [SkillCount]uint16{SkillFist: 120}}
	attacker.SetPosition(Position{X: 100, Y: 100, Z: 7})

	before := monster.GetHealth()
	e.doMeleeHit(combat.NewCombat(), attacker, monster)

	if monster.GetHealth() >= before {
		t.Fatalf("expected monster health to drop from %d, got %d", before, monster.GetHealth())
	}
	if healthChanges != 1 {
		t.Errorf("expected 1 health-change hook call, got %d", healthChanges)
	}
	if hits != 1 {
		t.Errorf("expected 1 combat-hit hook call, got %d", hits)
	}
}

// TestCombatEngine_Death verifies that killing a monster drops a corpse item on
// its tile, removes it from the world, and clears any player's target.
func TestCombatEngine_Death(t *testing.T) {
	w := newCombatWorld()

	var itemsAppeared, removed, targetsLost int
	w.OnItemAppear = func(Position, *Item) { itemsAppeared++ }
	w.OnCreatureRemove = func(Creature, map[uint32]int) { removed++ }
	w.OnTargetLost = func(*Player) { targetsLost++ }

	pos := Position{X: 101, Y: 100, Z: 7}
	monster := NewMonster(42, "Rat", nil)
	monster.MaxHealth, monster.Health = 5, 5
	monster.CorpseID = 5964
	monster.SetPosition(pos)
	w.AddCreature(monster)

	attacker := &Player{Skills: [SkillCount]uint16{SkillFist: 200}}
	attacker.ID = 0x10000001
	attacker.SetPosition(Position{X: 100, Y: 100, Z: 7})
	attacker.SetAttackTarget(42)
	w.players[attacker.ID] = attacker

	e := NewCombatEngine(w)

	// Force the kill.
	monster.SetHealth(0)
	e.handleDeath(monster, attacker)

	if w.CreatureByID(42) != nil {
		t.Errorf("expected monster to be removed from the world")
	}
	if removed != 1 {
		t.Errorf("expected creature-remove hook once, got %d", removed)
	}
	if itemsAppeared != 2 {
		t.Errorf("expected corpse item-appear hook twice (AddItem + explicit), got %d", itemsAppeared)
	}
	if targetsLost != 1 {
		t.Errorf("expected target-lost hook once, got %d", targetsLost)
	}
	if attacker.TargetID != 0 {
		t.Errorf("expected attacker target cleared, got %d", attacker.TargetID)
	}
	tile := w.Map.GetTile(pos)
	if tile == nil || len(tile.Items) != 1 || tile.Items[0].ID != 5964 {
		t.Errorf("expected corpse 5964 on tile, got %+v", tile)
	}
}

// TestCombatEngine_Cooldown verifies the per-creature attack interval gate.
func TestCombatEngine_Cooldown(t *testing.T) {
	e := NewCombatEngine(newCombatWorld())
	if !e.ready(7, 50*time.Millisecond) {
		t.Fatal("first attack should be ready")
	}
	if e.ready(7, 50*time.Millisecond) {
		t.Fatal("immediate second attack should be gated")
	}
	time.Sleep(60 * time.Millisecond)
	if !e.ready(7, 50*time.Millisecond) {
		t.Fatal("attack should be ready after the interval")
	}
}
