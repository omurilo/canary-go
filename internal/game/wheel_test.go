package game

import "testing"

func TestWheelOfDestiny_PointsCalculation(t *testing.T) {
	wheel := NewWheelOfDestiny()

	// Level 50 -> 0 points
	if pts := wheel.GetTotalPoints(50); pts != 0 {
		t.Errorf("expected 0 points for level 50, got %d", pts)
	}

	// Level 100 -> 50 points
	if pts := wheel.GetTotalPoints(100); pts != 50 {
		t.Errorf("expected 50 points for level 100, got %d", pts)
	}

	// Level 500 -> 450 points
	if pts := wheel.GetTotalPoints(500); pts != 450 {
		t.Errorf("expected 450 points for level 500, got %d", pts)
	}

	// Level 699 -> 649 points
	if pts := wheel.GetTotalPoints(699); pts != 649 {
		t.Errorf("expected 649 points for level 699, got %d", pts)
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

	// MaxMana with Wheel (+100 Mana)
	player.MaxMana = 500
	if mana := player.GetMaxMana(); mana != 600 {
		t.Errorf("expected 600 MaxMana with Wheel, got %d", mana)
	}

	// Capacity with Wheel (+100 oz = 10000 hundredths)
	player.Capacity = 1000
	if cap := player.GetCapacity(); cap != 11000 {
		t.Errorf("expected 11000 Capacity with Wheel, got %d", cap)
	}
}
