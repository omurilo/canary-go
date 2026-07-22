package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game/combat"
)

func TestProtectionZone_TileFlags(t *testing.T) {
	tile := &Tile{Flags: 0}
	if tile.IsProtectionZone() {
		t.Error("expected fresh tile NOT to be a protection zone")
	}

	tile.Flags = 1 // 1 is TILESTATE_PROTECTIONZONE
	if !tile.IsProtectionZone() {
		t.Error("expected tile with flag 1 to be a protection zone")
	}
}

func TestProtectionZone_CanDoCombat_BlocksAggressiveActions(t *testing.T) {
	w := NewWorld()
	p1 := &Player{}
	p1.World = w
	p1.SetPosition(Position{X: 100, Y: 100, Z: 7})
	w.Map.SetTile(p1.GetPosition(), &Tile{Ground: &Item{ID: 1}, Flags: 1}) // Inside PZ

	p2 := &Player{}
	p2.World = w
	p2.SetPosition(Position{X: 101, Y: 100, Z: 7})
	w.Map.SetTile(p2.GetPosition(), &Tile{Ground: &Item{ID: 1}, Flags: 0}) // Outside PZ

	c1 := combatAdapter{c: p1}
	c2 := combatAdapter{c: p2}

	// Case 1: Attacker is inside PZ
	if combat.CanDoCombat(c1, c2) {
		t.Error("expected combat to be BLOCKED when attacker is in protection zone")
	}

	// Case 2: Target is inside PZ
	if combat.CanDoCombat(c2, c1) {
		t.Error("expected combat to be BLOCKED when target is in protection zone")
	}

	// Case 3: Neither inside PZ
	w.Map.GetTile(p1.GetPosition()).Flags = 0 // Remove PZ flag from p1 tile
	if !combat.CanDoCombat(c1, c2) {
		t.Error("expected combat to be ALLOWED when neither party is in protection zone")
	}
}

func TestProtectionZone_MovementRestrictions(t *testing.T) {
	w := NewWorld()
	w.Map.SetTile(Position{X: 100, Y: 100, Z: 7}, &Tile{Ground: &Item{ID: 1}, Flags: 0}) // Normal tile
	w.Map.SetTile(Position{X: 101, Y: 100, Z: 7}, &Tile{Ground: &Item{ID: 1}, Flags: 1}) // PZ tile

	// Player can enter Protection Zone
	player := &Player{}
	player.World = w
	player.SetPosition(Position{X: 100, Y: 100, Z: 7})
	
	_, ok := w.TryMoveCreature(player, DirEast)
	if !ok {
		t.Error("expected player to be allowed to move into protection zone")
	}
	if player.GetPosition().X != 101 {
		t.Errorf("expected player to move to X=101, got %d", player.GetPosition().X)
	}

	// Monster CANNOT enter Protection Zone
	monster := NewMonster(100, "Rat", nil)
	monster.World = w
	monster.SetPosition(Position{X: 100, Y: 100, Z: 7})

	_, ok = w.TryMoveCreature(monster, DirEast)
	if ok {
		t.Error("expected monster movement into protection zone to be BLOCKED")
	}
	if monster.GetPosition().X != 100 {
		t.Errorf("expected monster to remain at X=100, got %d", monster.GetPosition().X)
	}
}

func TestPassiveMonster_AIIgnoresTargets(t *testing.T) {
	w := NewWorld()
	ai := NewAIEngine(w)

	player := &Player{}
	player.ID = 1
	player.World = w
	player.SetPosition(Position{X: 101, Y: 100, Z: 7})
	w.players[player.ID] = player

	// Create passive monster type
	passiveType := &creatures.MonsterType{
		Name:  "Cat",
		Flags: creatures.MonsterFlags{Hostile: false}, // Passive
	}

	monster := NewMonster(10, "Cat", passiveType)
	monster.World = w
	monster.SetPosition(Position{X: 100, Y: 100, Z: 7})
	w.AddCreature(monster)

	// Set target manually (e.g. if set by a bug/script)
	monster.SetTarget(player)

	// Run AI update cycle once
	ai.updateAI()

	if monster.GetTarget() != nil {
		t.Error("expected passive monster target to be cleared during AI update")
	}
}

func TestCombat_SelfDamageAndSecureMode(t *testing.T) {
	w := NewWorld()
	p1 := &Player{}
	p1.ID = 100
	p1.World = w
	p1.SecureMode = true // Secure mode is ON (Dove)

	p2 := &Player{}
	p2.ID = 101
	p2.World = w
	p2.SecureMode = false // Secure mode is OFF (Aggressive)

	c1 := combatAdapter{c: p1}
	c2 := combatAdapter{c: p2}

	// 1. Caster should never aggressively hit themselves
	if combat.CanDoCombat(c1, c1) {
		t.Error("expected aggressive combat to be BLOCKED on self")
	}

	// 2. p1 (secure mode ON) cannot aggressively hit p2
	if combat.CanDoCombat(c1, c2) {
		t.Error("expected aggressive combat from player in secure mode to be BLOCKED")
	}

	// 3. p2 (secure mode OFF) CAN aggressively hit p1
	if !combat.CanDoCombat(c2, c1) {
		t.Error("expected aggressive combat from player in aggressive mode to be ALLOWED")
	}
}

