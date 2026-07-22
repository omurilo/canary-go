package game

import "testing"

func TestWheelOfDestiny_PointsCalculation(t *testing.T) {
	wheel := NewWheelOfDestiny()

	// Level 50 promoted -> 0 points
	if pts := wheel.GetTotalPoints(50, true); pts != 0 {
		t.Errorf("expected 0 points for level 50 promoted, got %d", pts)
	}

	// Level 100 non-promoted -> 0 points
	if pts := wheel.GetTotalPoints(100, false); pts != 0 {
		t.Errorf("expected 0 points for level 100 non-promoted, got %d", pts)
	}

	// Level 100 promoted -> 50 points
	if pts := wheel.GetTotalPoints(100, true); pts != 50 {
		t.Errorf("expected 50 points for level 100 promoted, got %d", pts)
	}

	// Level 500 promoted -> 450 points
	if pts := wheel.GetTotalPoints(500, true); pts != 450 {
		t.Errorf("expected 450 points for level 500 promoted, got %d", pts)
	}
}

func TestWheelOfDestiny_SlotPointsAndBonuses(t *testing.T) {
	wheel := NewWheelOfDestiny()

	// Invest points in slot 1 and slot 2
	wheel.SaveSlotPoints(map[uint16]uint16{
		1: 150,
		2: 50,
	})

	// Total spent points
	if spent := wheel.GetSpentPoints(); spent != 200 {
		t.Errorf("expected 200 spent points, got %d", spent)
	}

	// Check HP bonus (+200 HP)
	if hp := wheel.GetBonusHealth(); hp != 200 {
		t.Errorf("expected 200 HP bonus, got %d", hp)
	}

	// Check Mana bonus (+200 Mana)
	if mana := wheel.GetBonusMana(); mana != 200 {
		t.Errorf("expected 200 Mana bonus, got %d", mana)
	}

	// Check Cap bonus (+20000 oz hundredths)
	if cap := wheel.GetBonusCapacity(); cap != 20000 {
		t.Errorf("expected 20000 Cap bonus, got %d", cap)
	}
}

func TestPlayer_MaxHealthWithWheel(t *testing.T) {
	player := &Player{
		ID:        1,
		Level:     100,
		MaxHealth: 1000,
	}

	// Base HP
	if hp := player.GetMaxHealth(); hp != 1000 {
		t.Fatalf("expected 1000 base MaxHealth, got %d", hp)
	}

	// Allocate 100 points in Wheel
	wheel := player.GetWheel()
	wheel.SaveSlotPoints(map[uint16]uint16{
		1: 100,
	})

	// MaxHealth with Wheel (+100 HP)
	if hp := player.GetMaxHealth(); hp != 1100 {
		t.Errorf("expected 1100 MaxHealth with Wheel, got %d", hp)
	}
}
