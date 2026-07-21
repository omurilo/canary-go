package game

import "testing"

func TestApplyDeathPenaltyLosesExpAndLevel(t *testing.T) {
	p := &Player{Level: 20, Vocation: 1, SkillLoss: true, MaxHealth: 500, MaxMana: 300}
	p.Experience = ExpForLevel(20) + 10 // just into level 20
	p.Health, p.Mana = 0, 0

	before := p.Experience
	p.ApplyDeathPenalty()

	if p.Experience >= before {
		t.Errorf("experience not reduced: %d >= %d", p.Experience, before)
	}
	// 10% loss from just-into-20 must drop at least one level.
	if p.Level >= 20 {
		t.Errorf("level did not drop: %d", p.Level)
	}
	// Level must stay consistent with the remaining experience.
	if p.Experience < ExpForLevel(uint64(p.Level)) {
		t.Errorf("level %d inconsistent with exp %d", p.Level, p.Experience)
	}
	// Vitals refilled.
	if p.Health != p.MaxHealth || p.Mana != p.MaxMana {
		t.Errorf("vitals not refilled: hp %d/%d mana %d/%d", p.Health, p.MaxHealth, p.Mana, p.MaxMana)
	}
}

func TestDeathNoPenaltyBelowLevel8(t *testing.T) {
	p := &Player{Level: 5, Vocation: 1, SkillLoss: true, MaxHealth: 100, MaxMana: 50}
	p.Experience = ExpForLevel(5) + 5
	before := p.Experience
	p.ApplyDeathPenalty()
	if p.Experience != before {
		t.Errorf("low-level player lost exp: %d != %d", p.Experience, before)
	}
}
