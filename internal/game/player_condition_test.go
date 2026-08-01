package game_test

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/game/combat"
)

func TestPlayerConditionAttributes(t *testing.T) {
	p := &game.Player{
		Name:      "BuffTester",
		Level:     100,
		MaxHealth: 1000,
		MaxMana:   1000,
		MagLevel:  50,
	}
	p.Skills[game.SkillSword] = 80
	p.Skills[game.SkillShielding] = 70

	// Check base stats
	if p.GetMaxHealth() != 1000 {
		t.Errorf("expected max health 1000, got %d", p.GetMaxHealth())
	}
	if p.GetMaxMana() != 1000 {
		t.Errorf("expected max mana 1000, got %d", p.GetMaxMana())
	}
	if p.GetEffectiveMagLevel() != 50 {
		t.Errorf("expected effective magic level 50, got %d", p.GetEffectiveMagLevel())
	}
	if p.GetEffectiveSkill(game.SkillSword) != 80 {
		t.Errorf("expected effective sword skill 80, got %d", p.GetEffectiveSkill(game.SkillSword))
	}

	// Create and apply an Attribute condition
	cond := combat.CreateCondition(1, combat.ConditionAttributes, 10000, 0, false)
	// Modify sword skill (+10)
	cond.SetParam(22, 10) // CONDITION_PARAM_SKILL_SWORD (22)
	// Modify magic level (+5)
	cond.SetParam(30, 5) // CONDITION_PARAM_STAT_MAGICPOINTS (30)
	// Modify max health (+100) and max health percent (110%)
	cond.SetParam(27, 100) // CONDITION_PARAM_STAT_MAXHITPOINTS (27)
	cond.SetParam(31, 110) // CONDITION_PARAM_STAT_MAXHITPOINTSPERCENT (31)

	p.AddCondition(cond)

	// Check effective stats after condition
	// MaxHealth = (1000 + 100) * 110% = 1100 * 1.1 = 1210
	if p.GetMaxHealth() != 1210 {
		t.Errorf("expected max health 1210, got %d", p.GetMaxHealth())
	}
	// MagLevel = 50 + 5 = 55
	if p.GetEffectiveMagLevel() != 55 {
		t.Errorf("expected effective magic level 55, got %d", p.GetEffectiveMagLevel())
	}
	// Sword skill = 80 + 10 = 90
	if p.GetEffectiveSkill(game.SkillSword) != 90 {
		t.Errorf("expected effective sword skill 90, got %d", p.GetEffectiveSkill(game.SkillSword))
	}

	// Remove condition and check stats reverted
	p.RemoveCondition(combat.ConditionAttributes)
	if p.GetMaxHealth() != 1000 {
		t.Errorf("expected reverted max health 1000, got %d", p.GetMaxHealth())
	}
	if p.GetEffectiveMagLevel() != 50 {
		t.Errorf("expected reverted effective magic level 50, got %d", p.GetEffectiveMagLevel())
	}
	if p.GetEffectiveSkill(game.SkillSword) != 80 {
		t.Errorf("expected reverted effective sword skill 80, got %d", p.GetEffectiveSkill(game.SkillSword))
	}
}

func TestPlayerConditionSpeed(t *testing.T) {
	p := &game.Player{
		Name:      "SpeedTester",
		Level:     100,
		MaxHealth: 1000,
		MaxMana:   1000,
	}

	// Base speed of a level 100 character in Canary-Go:
	// BaseSpeed = 110 + (Level - 1) = 110 + 99 = 209
	if p.GetBaseSpeed() != 209 {
		t.Errorf("expected base speed 209, got %d", p.GetBaseSpeed())
	}
	if p.GetSpeed() != 209 {
		t.Errorf("expected initial current speed 209, got %d", p.GetSpeed())
	}

	// Create and apply a Haste Speed condition using standard haste formula: (1.3, 40, 1.3, 40)
	cond1 := combat.CreateCondition(1, combat.ConditionHaste, 10000, 0, false)
	if speedCond, ok := cond1.(*combat.ConditionSpeedStruct); ok {
		speedCond.SetFormulaVars(1.3, 40, 1.3, 40)
	}

	// Apply condition first time
	p.AddCondition(cond1)

	speedAfterFirst := p.GetSpeed()
	if speedAfterFirst <= 209 {
		t.Errorf("expected speed to increase after applying haste, got %d", speedAfterFirst)
	}

	// Create and apply a second Haste Speed condition (refreshing)
	cond2 := combat.CreateCondition(2, combat.ConditionHaste, 10000, 0, false)
	if speedCond, ok := cond2.(*combat.ConditionSpeedStruct); ok {
		speedCond.SetFormulaVars(1.3, 40, 1.3, 40)
	}

	// Apply condition second time (should merge, refresh, and NOT stack)
	p.AddCondition(cond2)

	speedAfterSecond := p.GetSpeed()
	if speedAfterSecond != speedAfterFirst {
		t.Errorf("expected speed to remain %d on refresh, but it changed/stacked to %d", speedAfterFirst, speedAfterSecond)
	}

	// Remove condition and ensure speed reverts to base speed
	p.RemoveCondition(combat.ConditionHaste)
	if p.GetSpeed() != 209 {
		t.Errorf("expected reverted current speed 209, got %d", p.GetSpeed())
	}
}
