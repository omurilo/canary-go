package game

import "testing"

func TestWheelOfDestiny_PointsCalculation(t *testing.T) {
	wheel := NewWheelOfDestiny()
	cases := map[uint16]uint16{50: 0, 100: 50, 500: 450, 699: 649}
	for level, want := range cases {
		if got := wheel.GetTotalPoints(level); got != want {
			t.Errorf("GetTotalPoints(%d) = %d, want %d", level, got, want)
		}
	}
}

// Knight: green200 (slot 1) gives 3 HP + 1 mana per point; green50 (slot 15) is
// a capacity slot giving 5 per point (applied ×100).
func TestWheelBonusKnight(t *testing.T) {
	w := NewWheelOfDestiny()
	w.SetVocation(cipKnight)
	w.SaveSlotPoints(map[uint16]uint16{slotGreen200: 200, slotGreen50: 50})

	if hp := w.GetBonusHealth(); hp != 3*200 {
		t.Errorf("knight HP bonus = %d, want %d", hp, 3*200)
	}
	if mana := w.GetBonusMana(); mana != 1*200 {
		t.Errorf("knight mana bonus = %d, want %d", mana, 1*200)
	}
	if cap := w.GetBonusCapacity(); cap != 5*50*100 {
		t.Errorf("knight cap bonus = %d, want %d", cap, 5*50*100)
	}
	// green200 fully filled (200/200) unlocks Battle Instinct for a knight.
	if !w.bonus_().Instants["Battle Instinct"] {
		t.Errorf("expected Battle Instinct instant unlocked on maxed green200")
	}
}

// Sorcerer: green200 gives 1 HP + 6 mana per point.
func TestWheelBonusSorcerer(t *testing.T) {
	w := NewWheelOfDestiny()
	w.SetVocation(cipSorcerer)
	w.SaveSlotPoints(map[uint16]uint16{slotGreen200: 100})
	if hp := w.GetBonusHealth(); hp != 100 {
		t.Errorf("sorcerer HP = %d, want 100", hp)
	}
	if mana := w.GetBonusMana(); mana != 6*100 {
		t.Errorf("sorcerer mana = %d, want %d", mana, 6*100)
	}
}

// Mitigation and leech: greenTop150 (slot 2) adds 0.03 mitigation/point and,
// when fully filled, +0.25 mana leech.
func TestWheelMitigationAndLeech(t *testing.T) {
	w := NewWheelOfDestiny()
	w.SetVocation(cipKnight)
	w.SaveSlotPoints(map[uint16]uint16{slotGreenTop150: 150})
	if mit := w.GetBonusMitigation(); mit < 4.49 || mit > 4.51 { // 0.03*150 = 4.5
		t.Errorf("mitigation = %v, want ~4.5", mit)
	}
	if ml := w.GetBonusManaLeech(); ml < 0.24 || ml > 0.26 {
		t.Errorf("mana leech = %v, want 0.25 (slot maxed)", ml)
	}
}

func TestPlayerMaxHealthWithWheel(t *testing.T) {
	p := &Player{ID: 1, Level: 100, MaxHealth: 1000, MaxMana: 500}
	if hp := p.GetMaxHealth(); hp != 1000 {
		t.Fatalf("base MaxHealth = %d, want 1000", hp)
	}
	w := p.GetWheel()
	w.SetVocation(cipKnight)
	w.SaveSlotPoints(map[uint16]uint16{slotGreen200: 100}) // knight: +3 HP/pt
	if hp := p.GetMaxHealth(); hp != 1000+300 {
		t.Errorf("MaxHealth with wheel = %d, want 1300", hp)
	}
	if mana := p.GetMaxMana(); mana != 500+100 { // knight green200: +1 mana/pt
		t.Errorf("MaxMana with wheel = %d, want 600", mana)
	}
}

// A slot over its point cap must reject the whole save (nothing applied).
func TestWheelSaveRejectsOverCap(t *testing.T) {
	w := NewWheelOfDestiny()
	w.SetVocation(cipKnight)
	if w.ValidateAndSave(map[uint16]uint16{slotGreen200: 250}, 1000) {
		t.Fatalf("expected over-cap save (250 > 200) to be rejected")
	}
	if w.GetSpentPoints() != 0 {
		t.Fatalf("nothing should be applied on a rejected save, got %d", w.GetSpentPoints())
	}
}

// The total budget binds: with only 50 points, only one rim slot fits.
func TestWheelSaveEnforcesBudget(t *testing.T) {
	w := NewWheelOfDestiny()
	w.SetVocation(cipKnight)
	ok := w.ValidateAndSave(map[uint16]uint16{slotGreen50: 50, slotRed50: 50}, 50)
	if !ok {
		t.Fatalf("packet was structurally valid; ValidateAndSave should return true")
	}
	if spent := w.GetSpentPoints(); spent != 50 {
		t.Errorf("budget 50 should cap spend at 50, got %d", spent)
	}
}

// Adjacency: an inner slot cannot be filled unless a neighbor is full.
func TestWheelSaveEnforcesAdjacency(t *testing.T) {
	w := NewWheelOfDestiny()
	w.SetVocation(cipKnight)

	// greenTop75 alone (no full neighbor) must be dropped.
	w.ValidateAndSave(map[uint16]uint16{slotGreenTop75: 75}, 1000)
	if w.GetSpentPoints() != 0 {
		t.Fatalf("orphan inner slot should be rejected, spent = %d", w.GetSpentPoints())
	}

	// Fill the rim neighbor (green50) first: now greenTop75 is selectable.
	w.ValidateAndSave(map[uint16]uint16{slotGreen50: 50, slotGreenTop75: 75}, 1000)
	sp := w.GetSlotPointsCopy()
	if sp[slotGreen50] != 50 || sp[slotGreenTop75] != 75 {
		t.Fatalf("valid rim→inner chain should apply both, got %v", sp)
	}
}
