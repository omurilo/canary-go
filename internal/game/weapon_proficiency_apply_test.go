package game

import "testing"

// The enum values are the wire/storage contract with C++: a persisted perk stores
// its type as a WeaponProficiencyBonus_t, so a mismatch means each server reads a
// different bonus out of the same perk.
func TestWeaponProfBonusMatchesUpstream(t *testing.T) {
	// src/enums/weapon_proficiency.hpp:18
	want := map[WeaponProfBonus]uint8{
		WpAttackDamage: 0, WpDefenseBonus: 1, WpWeaponShieldModifier: 2,
		WpSkillBonus: 3, WpSpecializedMagicLevel: 4, WpSpellAugment: 5,
		WpBestiary: 6, WpPowerfulFoeBonus: 7, WpCriticalHitChance: 8,
		WpElementalHitChance: 9, WpRuneCriticalHitChance: 10,
		WpAutoAttackCriticalHitChance: 11, WpCriticalExtraDamage: 12,
		WpElementalCriticalExtraDamage: 13, WpRuneCriticalExtraDamage: 14,
		WpAutoAttackCriticalExtraDmg: 15, WpManaLeech: 16, WpLifeLeech: 17,
		WpManaGainOnHit: 18, WpLifeGainOnHit: 19, WpManaGainOnKill: 20,
		WpLifeGainOnKill: 21, WpPerfectShotDamage: 22, WpRangedHitChance: 23,
		WpAttackRange: 24, WpSkillPercentageAutoAttack: 25,
		WpSkillPercentageSpellDamage: 26, WpSkillPercentageSpellHealing: 27,
		WpAlphaStrikeExtraDamage: 28, WpOmegaStrikeExtraDamage: 29,
		WpArmorPenetration: 30, WpElementalPierce: 31,
	}
	for got, expected := range want {
		if uint8(got) != expected {
			t.Errorf("bonus %d should be %d", got, expected)
		}
	}
}

func TestApplyPerksDefaultGoesToStat(t *testing.T) {
	wp := NewWeaponProficiency()
	wp.ApplyPerks([]ProficiencyPerk{
		{Type: uint8(WpAttackDamage), Value: 12},
		{Type: uint8(WpAttackDamage), Value: 8}, // accumulates
		{Type: uint8(WpLifeGainOnHit), Value: 5},
	})

	if got := wp.GetStat(WpAttackDamage); got != 20 {
		t.Errorf("attack damage: got %v want 20", got)
	}
	if got := wp.GetStat(WpLifeGainOnHit); got != 5 {
		t.Errorf("life gain on hit: got %v want 5", got)
	}
}

func TestApplyPerksCriticalRouting(t *testing.T) {
	wp := NewWeaponProficiency()
	wp.ApplyPerks([]ProficiencyPerk{
		{Type: uint8(WpAutoAttackCriticalHitChance), Value: 3},
		{Type: uint8(WpAutoAttackCriticalExtraDmg), Value: 40},
		{Type: uint8(WpRuneCriticalHitChance), Value: 2},
		{Type: uint8(WpCriticalExtraDamage), Value: 15},
		{Type: uint8(WpElementalHitChance), Value: 7, Element: 4},
	})

	if c := wp.GetAutoAttackCritical(); c.Chance != 3 || c.Damage != 40 {
		t.Errorf("auto crit: %+v", c)
	}
	if c := wp.GetRunesCritical(); c.Chance != 2 || c.Damage != 0 {
		t.Errorf("rune crit: %+v", c)
	}
	if c := wp.GetGeneralCritical(); c.Chance != 0 || c.Damage != 15 {
		t.Errorf("general crit: %+v", c)
	}
	if c := wp.GetElementCritical(4); c.Chance != 7 {
		t.Errorf("element crit: %+v", c)
	}
}

// Leech is stored in basis points upstream: value * 10000.
func TestApplyPerksLeechScaling(t *testing.T) {
	wp := NewWeaponProficiency()
	wp.ApplyPerks([]ProficiencyPerk{
		{Type: uint8(WpLifeLeech), Value: 1.5},
		{Type: uint8(WpManaLeech), Value: 2},
	})
	if got := wp.GetSkillBonus(SkillLifeLeechAmount); got != 15000 {
		t.Errorf("life leech: got %v want 15000", got)
	}
	if got := wp.GetSkillBonus(SkillManaLeechAmount); got != 20000 {
		t.Errorf("mana leech: got %v want 20000", got)
	}
}

// A cooldown augment is rounded to whole milliseconds from the absolute value.
func TestApplyPerksCooldownAugment(t *testing.T) {
	wp := NewWeaponProficiency()
	wp.ApplyPerks([]ProficiencyPerk{
		{Type: uint8(WpSpellAugment), AugmentType: AugmentCooldown, SpellID: 42, Value: -2.4},
		{Type: uint8(WpSpellAugment), AugmentType: AugmentDamage, SpellID: 42, Value: 10},
	})

	augs := wp.GetAugments(42)
	if len(augs) != 2 {
		t.Fatalf("expected 2 augments, got %d", len(augs))
	}
	var cooldown, damage float64
	for _, a := range augs {
		switch a.Id {
		case AugmentCooldown:
			cooldown = a.Data
		case AugmentDamage:
			damage = a.Data
		}
	}
	if cooldown != 2400 {
		t.Errorf("cooldown: got %v want 2400", cooldown)
	}
	if damage != 10 {
		t.Errorf("damage: got %v want 10", damage)
	}
}

func TestApplyPerksSkillPercentageRouting(t *testing.T) {
	wp := NewWeaponProficiency()
	wp.ApplyPerks([]ProficiencyPerk{
		{Type: uint8(WpSkillPercentageAutoAttack), SkillID: uint8(SkillSword), Value: 5},
		{Type: uint8(WpSkillPercentageSpellDamage), SkillID: uint8(SkillSword), Value: 3},
		{Type: uint8(WpSkillPercentageSpellHealing), SkillID: uint8(SkillSword), Value: 1},
	})
	pct := wp.GetSkillPercentage(SkillSword)
	if pct.AutoAttack != 5 || pct.SpellDamage != 3 || pct.SpellHealing != 1 {
		t.Errorf("skill percentages: %+v", pct)
	}
}

func TestApplyPerksMiscRouting(t *testing.T) {
	wp := NewWeaponProficiency()
	wp.ApplyPerks([]ProficiencyPerk{
		{Type: uint8(WpSpecializedMagicLevel), Element: 2, Value: 4},
		{Type: uint8(WpBestiary), BestiaryName: "dragon", Value: 12},
		{Type: uint8(WpPowerfulFoeBonus), Value: 6},
		{Type: uint8(WpPowerfulFoeBonus), Value: 4}, // accumulates
		{Type: uint8(WpSkillBonus), SkillID: uint8(SkillAxe), Value: 3},
		{Type: uint8(WpPerfectShotDamage), Range: 3, Value: 25},
	})

	if got := wp.GetSpecializedMagic(2); got != 4 {
		t.Errorf("specialized magic: got %v want 4", got)
	}
	if got := wp.GetPowerfulFoeDamage(); got != 10 {
		t.Errorf("powerful foe: got %v want 10", got)
	}
	if got := wp.GetSkillBonus(SkillAxe); got != 3 {
		t.Errorf("skill bonus: got %v want 3", got)
	}
	if got := wp.GetPerfectShotBonus(3); got != 25 {
		t.Errorf("perfect shot: got %v want 25", got)
	}
	bestiary := wp.GetActiveBestiariesDamage()
	if len(bestiary) != 1 || bestiary[0].Name != "dragon" || bestiary[0].Amount != 12 {
		t.Errorf("bestiary damage: %+v", bestiary)
	}
}

// RebuildWeaponProficiency must be idempotent: running it twice cannot double the
// bonuses, or a second save/load cycle would inflate every stat.
func TestRebuildWeaponProficiencyIdempotent(t *testing.T) {
	p := &Player{WeaponProficiency: NewWeaponProficiency()}
	p.Proficiency = map[uint16]WeaponProficiencyData{
		3264: {Experience: 100, Perks: []ProficiencyPerk{
			{Type: uint8(WpAttackDamage), Value: 10},
			{Type: uint8(WpCriticalHitChance), Value: 2},
		}},
	}

	p.RebuildWeaponProficiency()
	first := p.WeaponProficiency.GetStat(WpAttackDamage)
	firstCrit := p.WeaponProficiency.GetGeneralCritical()

	p.RebuildWeaponProficiency()
	if got := p.WeaponProficiency.GetStat(WpAttackDamage); got != first {
		t.Errorf("stat drifted on rebuild: %v then %v", first, got)
	}
	if got := p.WeaponProficiency.GetGeneralCritical(); got != firstCrit {
		t.Errorf("crit drifted on rebuild: %+v then %+v", firstCrit, got)
	}
	if first != 10 {
		t.Errorf("expected 10, got %v", first)
	}
}

// Bonuses from several weapons accumulate into the shared cache.
func TestRebuildWeaponProficiencyAccumulatesAcrossWeapons(t *testing.T) {
	p := &Player{WeaponProficiency: NewWeaponProficiency()}
	p.Proficiency = map[uint16]WeaponProficiencyData{
		1: {Perks: []ProficiencyPerk{{Type: uint8(WpAttackDamage), Value: 5}}},
		2: {Perks: []ProficiencyPerk{{Type: uint8(WpAttackDamage), Value: 7}}},
	}
	p.RebuildWeaponProficiency()
	if got := p.WeaponProficiency.GetStat(WpAttackDamage); got != 12 {
		t.Errorf("got %v want 12", got)
	}
}
