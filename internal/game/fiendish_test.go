package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
)

func TestMonsterForgeSystem(t *testing.T) {
	m := NewMonster(100, "Demon", nil)
	m.MaxHealth = 1000
	m.Health = 1000

	if !m.CanBeForgeMonster() {
		// NewMonster set raceID to 0 when mType is nil, so let me set raceID for test
		m.Type = &creatures.MonsterType{RaceID: 35}
	}
	m.Type = &creatures.MonsterType{RaceID: 35}

	if !m.CanBeForgeMonster() {
		t.Fatalf("expected monster to be forgeable")
	}

	// Configure influenced monster with stack = 2
	m.ForgeClassification = ForgeClassifications_Influenced
	m.ConfigureForgeSystem(2)

	if m.ForgeStack != 2 {
		t.Errorf("expected stack 2, got %d", m.ForgeStack)
	}

	// MaxHealth = 1000 * (1 + (15*2 + 35)/100) = 1000 * (1 + 65/100) = 1650
	if m.MaxHealth != 1650 {
		t.Errorf("expected MaxHealth 1650, got %d", m.MaxHealth)
	}

	// Test Fiendish
	m.ForgeClassification = ForgeClassifications_Fiendish
	m.ConfigureForgeSystem(0)
	if m.ForgeStack != 15 {
		t.Errorf("expected stack 15 for Fiendish, got %d", m.ForgeStack)
	}
}

func TestFiendishManager(t *testing.T) {
	w := NewWorld()
	fm := NewFiendishManager(2)

	m1 := NewMonster(1, "Dragon", nil)
	m1.Type = &creatures.MonsterType{RaceID: 34}
	w.creatures[1] = m1

	m2 := NewMonster(2, "Orc", nil)
	m2.Type = &creatures.MonsterType{RaceID: 5}
	w.creatures[2] = m2

	fiend := fm.MakeFiendishMonster(w)
	if fiend == nil {
		t.Fatalf("expected fiendish monster to be selected")
	}

	if fiend.ForgeClassification != ForgeClassifications_Fiendish {
		t.Errorf("expected Fiendish classification, got %d", fiend.ForgeClassification)
	}

	active := fm.GetFiendishMonsters()
	if len(active) != 1 {
		t.Errorf("expected 1 active fiendish monster, got %d", len(active))
	}
}
