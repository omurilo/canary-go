package game

import "testing"

func TestPlayerProperties(t *testing.T) {
	p := &Player{}

	// Test Blessings default
	for i, b := range p.Blessings {
		if b != 0 {
			t.Errorf("expected blessing %d to be 0, got %d", i, b)
		}
	}

	// Test Offline Training default properties
	if p.OfflineTrainingTime != 0 {
		t.Errorf("expected OfflineTrainingTime to be 0, got %d", p.OfflineTrainingTime)
	}
	if p.OfflineTrainingSkill != 0 {
		t.Errorf("expected OfflineTrainingSkill to be 0, got %d", p.OfflineTrainingSkill)
	}

	// Test Setting properties
	p.OfflineTrainingTime = 43200000
	p.OfflineTrainingSkill = 5
	p.Blessings[0] = 1

	if p.OfflineTrainingTime != 43200000 {
		t.Errorf("expected OfflineTrainingTime to be 43200000, got %d", p.OfflineTrainingTime)
	}
	if p.OfflineTrainingSkill != 5 {
		t.Errorf("expected OfflineTrainingSkill to be 5, got %d", p.OfflineTrainingSkill)
	}
	if p.Blessings[0] != 1 {
		t.Errorf("expected blessing 0 to be 1, got %d", p.Blessings[0])
	}
}
