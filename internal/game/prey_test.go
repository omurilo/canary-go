package game

import (
	"testing"
)

func TestPlayerPreyBonusAndStamina(t *testing.T) {
	p := &Player{}
	prey := p.GetPrey()

	slot := prey.GetSlot(0)
	if slot == nil {
		t.Fatalf("expected slot 0 to exist")
	}

	slot.State = PreyDataState_Active
	slot.SelectedRaceID = 10
	slot.Bonus = PreyBonus_XPBonus
	slot.BonusPercentage = 25
	slot.BonusTimeLeft = 7200

	bonus, active := prey.GetPreyBonus(10, PreyBonus_XPBonus)
	if !active || bonus != 25 {
		t.Errorf("expected active bonus 25, got bonus=%d, active=%v", bonus, active)
	}

	// Test non-matching race
	_, active = prey.GetPreyBonus(99, PreyBonus_XPBonus)
	if active {
		t.Errorf("expected inactive bonus for non-matching race")
	}

	// Test stamina decrease
	prey.TickStamina(100)
	if slot.BonusTimeLeft != 7100 {
		t.Errorf("expected 7100 seconds left, got %d", slot.BonusTimeLeft)
	}
}

func TestTaskHunterKillProgress(t *testing.T) {
	p := &Player{}
	th := p.GetTaskHunter()

	slot := th.GetSlot(0)
	if slot == nil {
		t.Fatalf("expected task slot 0 to exist")
	}

	slot.State = PreyTaskDataState_Active
	slot.SelectedRaceID = 15
	slot.TargetKills = 2
	slot.CurrentKills = 0

	th.OnKillMonster(15)
	if slot.CurrentKills != 1 || slot.State != PreyTaskDataState_Active {
		t.Errorf("expected 1 kill and active state, got kills=%d, state=%d", slot.CurrentKills, slot.State)
	}

	th.OnKillMonster(15)
	if slot.CurrentKills != 2 || slot.State != PreyTaskDataState_Completed {
		t.Errorf("expected 2 kills and completed state, got kills=%d, state=%d", slot.CurrentKills, slot.State)
	}
}
