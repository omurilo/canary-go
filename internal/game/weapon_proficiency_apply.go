package game

import "math"

// ApplyPerks derives the aggregated bonus cache from the persisted perks of one
// weapon, porting WeaponProficiency::applyPerks (weapon_proficiency.cpp:441).
//
// This is the step that was missing: the perks were being persisted in KV but
// nothing turned them into the stats the cyclopedia and combat paths read, so
// every derived bonus stayed at zero after a relog.
//
// Bonuses ACCUMULATE across weapons, matching the add* semantics upstream. Call
// ResetDerived first if recomputing from scratch.
func (wp *WeaponProficiency) ApplyPerks(perks []ProficiencyPerk) {
	if wp == nil {
		return
	}
	for _, perk := range perks {
		switch perk.Type {
		case uint8(WpSpellAugment):
			wp.applySpellAugment(perk)

		case uint8(WpSpecializedMagicLevel):
			wp.AddSpecializedMagic(perk.Element, perk.Value)

		case uint8(WpAutoAttackCriticalExtraDmg),
			uint8(WpAutoAttackCriticalHitChance),
			uint8(WpElementalHitChance),
			uint8(WpElementalCriticalExtraDamage),
			uint8(WpRuneCriticalHitChance),
			uint8(WpRuneCriticalExtraDamage),
			uint8(WpCriticalHitChance),
			uint8(WpCriticalExtraDamage):
			wp.applyCriticalBonus(perk)

		case uint8(WpBestiary):
			// C++ keys this by bestiaryId; the Go cache is keyed by name, and the
			// perk carries both.
			wp.AddBestiaryDamage(perk.BestiaryName, perk.Value)

		case uint8(WpPowerfulFoeBonus):
			wp.AddPowerfulFoeDamage(perk.Value)

		case uint8(WpSkillBonus):
			wp.AddSkillBonus(Skill(perk.SkillID), perk.Value)

		case uint8(WpLifeLeech), uint8(WpManaLeech):
			// Upstream stores leech as basis points: value * 10000.
			skill := SkillManaLeechAmount
			if perk.Type == uint8(WpLifeLeech) {
				skill = SkillLifeLeechAmount
			}
			wp.AddSkillBonus(skill, perk.Value*10000)

		case uint8(WpPerfectShotDamage):
			wp.SetPerfectShotBonus(perk.Range, perk.Value)

		case uint8(WpSkillPercentageAutoAttack),
			uint8(WpSkillPercentageSpellDamage),
			uint8(WpSkillPercentageSpellHealing):
			wp.applySkillPercentageBonus(perk)

		default:
			wp.AddStat(WeaponProfBonus(perk.Type), perk.Value)
		}
	}
}

// applySpellAugment ports the SPELL_AUGMENT arm of applyPerks.
func (wp *WeaponProficiency) applySpellAugment(perk ProficiencyPerk) {
	value := perk.Value
	if perk.AugmentType == AugmentCooldown {
		// Cooldown is stored as a whole number of milliseconds, rounded from the
		// absolute value (weapon_proficiency.cpp:457).
		value = math.Round(math.Abs(perk.Value) * 1000.0)
	}

	switch perk.AugmentType {
	case AugmentDamage, AugmentHeal, AugmentCooldown,
		AugmentLifeLeech, AugmentManaLeech,
		AugmentCriticalDamage, AugmentCriticalChance:
		wp.AddAugment(perk.SpellID, WeaponProfAugment{
			SpellID: perk.SpellID,
			Id:      perk.AugmentType,
			Data:    value,
		})
	default:
		// Unknown augment type: upstream logs and skips.
	}
}

// applyCriticalBonus ports WeaponProficiency::applyCriticalBonus
// (weapon_proficiency.cpp:522). A perk contributes to either chance or damage,
// never both, decided by its type.
func (wp *WeaponProficiency) applyCriticalBonus(perk ProficiencyPerk) {
	var bonus WeaponProfCritical
	switch perk.Type {
	case uint8(WpAutoAttackCriticalHitChance), uint8(WpElementalHitChance),
		uint8(WpRuneCriticalHitChance), uint8(WpCriticalHitChance):
		bonus.Chance = perk.Value
	case uint8(WpAutoAttackCriticalExtraDmg), uint8(WpElementalCriticalExtraDamage),
		uint8(WpRuneCriticalExtraDamage), uint8(WpCriticalExtraDamage):
		bonus.Damage = perk.Value
	}

	switch perk.Type {
	case uint8(WpAutoAttackCriticalExtraDmg), uint8(WpAutoAttackCriticalHitChance):
		wp.AddAutoAttackCritical(bonus)
	case uint8(WpElementalHitChance), uint8(WpElementalCriticalExtraDamage):
		wp.AddElementCritical(int(perk.Element), bonus)
	case uint8(WpRuneCriticalHitChance), uint8(WpRuneCriticalExtraDamage):
		wp.AddRunesCritical(bonus)
	case uint8(WpCriticalHitChance), uint8(WpCriticalExtraDamage):
		wp.AddGeneralCritical(bonus)
	}
}

// applySkillPercentageBonus ports WeaponProficiency::applySkillPercentageBonus
// (weapon_proficiency.cpp:551).
func (wp *WeaponProficiency) applySkillPercentageBonus(perk ProficiencyPerk) {
	var kind uint8
	switch perk.Type {
	case uint8(WpSkillPercentageAutoAttack):
		kind = SkillPctAutoAttack
	case uint8(WpSkillPercentageSpellDamage):
		kind = SkillPctSpellDmg
	case uint8(WpSkillPercentageSpellHealing):
		kind = SkillPctSpellHeal
	default:
		return
	}
	wp.AddSkillPercentage(Skill(perk.SkillID), kind, perk.Value)
}

// --- accumulating sinks, mirroring the C++ add* helpers ---

// AddSpecializedMagic adds specialized magic level for one element.
func (wp *WeaponProficiency) AddSpecializedMagic(element uint8, value float64) {
	if wp.specializedMagic == nil {
		wp.specializedMagic = make(map[uint8]float64)
	}
	wp.specializedMagic[element] += value
}

// GetSpecializedMagic returns the specialized magic level for one element.
func (wp *WeaponProficiency) GetSpecializedMagic(element uint8) float64 {
	if wp == nil || wp.specializedMagic == nil {
		return 0
	}
	return wp.specializedMagic[element]
}

// AddSkillBonus adds a flat skill bonus.
func (wp *WeaponProficiency) AddSkillBonus(skill Skill, value float64) {
	if wp.skillBonus == nil {
		wp.skillBonus = make(map[Skill]float64)
	}
	wp.skillBonus[skill] += value
}

// GetSkillBonus returns the accumulated flat bonus for a skill.
func (wp *WeaponProficiency) GetSkillBonus(skill Skill) float64 {
	if wp == nil || wp.skillBonus == nil {
		return 0
	}
	return wp.skillBonus[skill]
}

// SetPerfectShotBonus records perfect-shot damage at a given range. Upstream uses
// a setter here rather than accumulating, so the last perk for a range wins.
func (wp *WeaponProficiency) SetPerfectShotBonus(rng uint8, value float64) {
	if wp.perfectShot == nil {
		wp.perfectShot = make(map[uint8]float64)
	}
	wp.perfectShot[rng] = value
}

// GetPerfectShotBonus returns the perfect-shot damage for a range.
func (wp *WeaponProficiency) GetPerfectShotBonus(rng uint8) float64 {
	if wp == nil || wp.perfectShot == nil {
		return 0
	}
	return wp.perfectShot[rng]
}

// AddPowerfulFoeDamage accumulates, unlike the previous SetPowerfulFoeDamage.
func (wp *WeaponProficiency) AddPowerfulFoeDamage(value float64) {
	wp.powerfulFoeDamage += value
}

// AddAutoAttackCritical accumulates chance and damage.
func (wp *WeaponProficiency) AddAutoAttackCritical(b WeaponProfCritical) {
	wp.autoCrit.Chance += b.Chance
	wp.autoCrit.Damage += b.Damage
}

// AddRunesCritical accumulates chance and damage.
func (wp *WeaponProficiency) AddRunesCritical(b WeaponProfCritical) {
	wp.runesCritical.Chance += b.Chance
	wp.runesCritical.Damage += b.Damage
}

// AddGeneralCritical accumulates chance and damage.
func (wp *WeaponProficiency) AddGeneralCritical(b WeaponProfCritical) {
	wp.generalCritical.Chance += b.Chance
	wp.generalCritical.Damage += b.Damage
}

// AddElementCritical accumulates chance and damage for one element.
func (wp *WeaponProficiency) AddElementCritical(element int, b WeaponProfCritical) {
	if wp.elementCritical == nil {
		wp.elementCritical = make(map[int]WeaponProfCritical)
	}
	cur := wp.elementCritical[element]
	cur.Chance += b.Chance
	cur.Damage += b.Damage
	wp.elementCritical[element] = cur
}

// GetElementCritical returns the critical bonus for one element.
func (wp *WeaponProficiency) GetElementCritical(element int) WeaponProfCritical {
	if wp == nil || wp.elementCritical == nil {
		return WeaponProfCritical{}
	}
	return wp.elementCritical[element]
}

// AddSkillPercentage accumulates into the right arm of the SkillPercentage entry.
func (wp *WeaponProficiency) AddSkillPercentage(skill Skill, kind uint8, value float64) {
	if wp.skillPcts == nil {
		wp.skillPcts = make(map[Skill]SkillPercentage)
	}
	entry := wp.skillPcts[skill]
	entry.Skill = skill
	switch kind {
	case SkillPctAutoAttack:
		entry.AutoAttack += value
	case SkillPctSpellDmg:
		entry.SpellDamage += value
	case SkillPctSpellHeal:
		entry.SpellHealing += value
	}
	wp.skillPcts[skill] = entry
}

// ResetDerived clears every derived bonus so ApplyPerks can rebuild from scratch.
// The persisted perks live on Player.Proficiency and are untouched.
func (wp *WeaponProficiency) ResetDerived() {
	if wp == nil {
		return
	}
	wp.stats = make(map[WeaponProfBonus]float64)
	wp.skillPcts = make(map[Skill]SkillPercentage)
	wp.bestiaryDamage = make(map[string]float64)
	wp.elementCritical = make(map[int]WeaponProfCritical)
	wp.augments = make(map[uint16][]WeaponProfAugment)
	wp.specializedMagic = make(map[uint8]float64)
	wp.skillBonus = make(map[Skill]float64)
	wp.perfectShot = make(map[uint8]float64)
	wp.runesCritical = WeaponProfCritical{}
	wp.autoCrit = WeaponProfCritical{}
	wp.generalCritical = WeaponProfCritical{}
	wp.powerfulFoeDamage = 0
}

// RebuildWeaponProficiency recomputes the derived bonus cache from every weapon's
// persisted perks. This is what C++ achieves by calling applyPerks as perks are
// selected; here it runs once after the KV load.
func (p *Player) RebuildWeaponProficiency() {
	if p == nil || p.WeaponProficiency == nil {
		return
	}
	p.WeaponProficiency.ResetDerived()
	for _, data := range p.Proficiency {
		p.WeaponProficiency.ApplyPerks(data.Perks)
	}
}
