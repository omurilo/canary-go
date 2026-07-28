package game

// WeaponProficiency stores per-weapon bonus data that feeds into cyclopedia
// OffenceStats (bestiary damage, runes/auto critical, skill percentages, augments).
// Mirrors the C++ WeaponProficiency class at a level sufficient for the
// cyclopedia packet; the full perk/level/exp system is not yet ported.
type WeaponProficiency struct {
	// Per-weapon allocated stats (WeaponProficiencyBonus_t). The cyclopedia reads
	// aggregate values that are the sum of all selected perks across weapons.
	stats map[WeaponProfBonus]float64

	// Skill percentages per skill type (AutoAttack, SpellDamage, SpellHealing).
	skillPcts map[Skill]SkillPercentage

	// Bestiary damage: race name → damage bonus.
	bestiaryDamage map[string]float64

	// Critical bonuses.
	runesCritical    WeaponProfCritical
	autoCrit         WeaponProfCritical
	generalCritical  WeaponProfCritical
	elementCritical  map[int]WeaponProfCritical // combat type → critical

	// Augments: weaponId → list of active augment data.
	augments map[uint16][]WeaponProfAugment

	// Powerful foe damage (influenced/bosses).
	powerfulFoeDamage float64
}

// WeaponProfBonus matches C++ WeaponProficiencyBonus_t.
type WeaponProfBonus uint8

const (
	WpAttackDamage     WeaponProfBonus = 0
	WpDefenseBonus     WeaponProfBonus = 1
	WpWeaponShield     WeaponProfBonus = 2
	WpAttackSkill      WeaponProfBonus = 3
	WpLifeGainOnHit    WeaponProfBonus = 4
	WpManaGainOnHit    WeaponProfBonus = 5
	WpLifeGainOnKill   WeaponProfBonus = 6
	WpManaGainOnKill   WeaponProfBonus = 7
	WpCriticalChance   WeaponProfBonus = 8
	WpCriticalDamage   WeaponProfBonus = 9
	WpPerfectShotWeapon WeaponProfBonus = 10
	WpElementalWeapon  WeaponProfBonus = 11
)

// WeaponProfCritical matches C++ WeaponProficiencyCriticalBonus.
type WeaponProfCritical struct {
	Chance float64
	Damage float64
}

// WeaponProfAugment stores one active augment entry.
type WeaponProfAugment struct {
	Id   uint8
	Data float64
}

// NewWeaponProficiency creates an empty proficiency tracker.
func NewWeaponProficiency() *WeaponProficiency {
	return &WeaponProficiency{
		stats:           make(map[WeaponProfBonus]float64),
		skillPcts:       make(map[Skill]SkillPercentage),
		bestiaryDamage:  make(map[string]float64),
		elementCritical: make(map[int]WeaponProfCritical),
		augments:        make(map[uint16][]WeaponProfAugment),
	}
}

// GetStat returns the aggregate value for a stat type.
func (wp *WeaponProficiency) GetStat(bonus WeaponProfBonus) float64 {
	if wp == nil || wp.stats == nil {
		return 0
	}
	return wp.stats[bonus]
}

// AddStat adds a value to a stat type (used when loading perks).
func (wp *WeaponProficiency) AddStat(bonus WeaponProfBonus, value float64) {
	if wp.stats == nil {
		wp.stats = make(map[WeaponProfBonus]float64)
	}
	wp.stats[bonus] += value
}

// GetRunesCritical returns critical hit bonus for runes.
func (wp *WeaponProficiency) GetRunesCritical() WeaponProfCritical {
	if wp == nil {
		return WeaponProfCritical{}
	}
	return wp.runesCritical
}

// SetRunesCritical sets the runes critical values.
func (wp *WeaponProficiency) SetRunesCritical(c WeaponProfCritical) {
	wp.runesCritical = c
}

// GetAutoAttackCritical returns critical hit bonus for auto attacks.
func (wp *WeaponProficiency) GetAutoAttackCritical() WeaponProfCritical {
	if wp == nil {
		return WeaponProfCritical{}
	}
	return wp.autoCrit
}

// SetAutoAttackCritical sets the auto attack critical values.
func (wp *WeaponProficiency) SetAutoAttackCritical(c WeaponProfCritical) {
	wp.autoCrit = c
}

// GetSkillPercentage returns the skill percentage data for a given skill.
func (wp *WeaponProficiency) GetSkillPercentage(skill Skill) SkillPercentage {
	if wp == nil || wp.skillPcts == nil {
		return SkillPercentage{}
	}
	return wp.skillPcts[skill]
}

// SetSkillPercentage stores the percentage data for a skill.
func (wp *WeaponProficiency) SetSkillPercentage(skill Skill, pct SkillPercentage) {
	if wp.skillPcts == nil {
		wp.skillPcts = make(map[Skill]SkillPercentage)
	}
	wp.skillPcts[skill] = pct
}

// GetActiveBestiariesDamage returns bestiary damage entries.
func (wp *WeaponProficiency) GetActiveBestiariesDamage() []ActiveBestiaryDamage {
	if wp == nil || wp.bestiaryDamage == nil {
		return nil
	}
	var result []ActiveBestiaryDamage
	for name, amount := range wp.bestiaryDamage {
		if amount > 0 {
			result = append(result, ActiveBestiaryDamage{Name: name, Amount: amount})
		}
	}
	return result
}

// AddBestiaryDamage stores a bestiary damage bonus.
func (wp *WeaponProficiency) AddBestiaryDamage(name string, amount float64) {
	if wp.bestiaryDamage == nil {
		wp.bestiaryDamage = make(map[string]float64)
	}
	wp.bestiaryDamage[name] += amount
}

// GetPowerfulFoeDamage returns the damage bonus against bosses/influenced.
func (wp *WeaponProficiency) GetPowerfulFoeDamage() float64 {
	if wp == nil {
		return 0
	}
	return wp.powerfulFoeDamage
}

// SetPowerfulFoeDamage sets the boss/influenced damage bonus.
func (wp *WeaponProficiency) SetPowerfulFoeDamage(v float64) {
	wp.powerfulFoeDamage = v
}

// GetAugments returns the active augments for a weapon.
func (wp *WeaponProficiency) GetAugments(weaponId uint16) []WeaponProfAugment {
	if wp == nil || wp.augments == nil {
		return nil
	}
	return wp.augments[weaponId]
}

// GetAllAugments returns all augments across all weapons.
func (wp *WeaponProficiency) GetAllAugments() []WeaponProfAugment {
	if wp == nil || wp.augments == nil {
		return nil
	}
	var result []WeaponProfAugment
	for _, augs := range wp.augments {
		result = append(result, augs...)
	}
	return result
}

// AddAugment stores an augment for a weapon.
func (wp *WeaponProficiency) AddAugment(weaponId uint16, aug WeaponProfAugment) {
	if wp.augments == nil {
		wp.augments = make(map[uint16][]WeaponProfAugment)
	}
	wp.augments[weaponId] = append(wp.augments[weaponId], aug)
}