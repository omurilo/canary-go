package game

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
)

// WeaponProficiency stores per-weapon bonus data that feeds into cyclopedia
// OffenceStats (bestiary damage, runes/auto critical, skill percentages, augments).
type WeaponProficiency struct {
	// mu guards the experience map. The stats maps are written once at load and
	// read on every hit, but experience is written from the kill path.
	mu                sync.RWMutex
	stats             map[WeaponProfBonus]float64
	skillPcts         map[Skill]SkillPercentage
	bestiaryDamage    map[string]float64
	runesCritical     WeaponProfCritical
	autoCrit          WeaponProfCritical
	generalCritical   WeaponProfCritical
	elementCritical   map[int]WeaponProfCritical
	augments          map[uint16][]WeaponProfAugment
	powerfulFoeDamage float64
	// experience is the per-weapon proficiency progress. Nothing wrote it before:
	// the type modelled what a proficiency gives you and not how it is earned.
	experience map[uint16]*weaponProfState

	// Derived sinks that applyPerks feeds and that had no Go counterpart before.
	specializedMagic map[uint8]float64 // element → bonus magic level
	skillBonus       map[Skill]float64 // skill → flat bonus
	perfectShot      map[uint8]float64 // range → damage
}

type WeaponProfBonus uint8

// WeaponProfBonus mirrors WeaponProficiencyBonus_t
// (src/enums/weapon_proficiency.hpp:18). These values are NOT free to choose:
// they are what a persisted ProficiencyPerk carries in its `type` field, so they
// have to match upstream or a perk written by either server is read as a
// different bonus by the other.
//
// The previous Go enum was an invented 12-value numbering that collided
// semantically from value 4 onwards — 4 meant LIFE_GAIN_ON_HIT here but
// SPECIALIZED_MAGIC_LEVEL upstream.
const (
	WpAttackDamage                 WeaponProfBonus = 0
	WpDefenseBonus                 WeaponProfBonus = 1
	WpWeaponShieldModifier         WeaponProfBonus = 2
	WpSkillBonus                   WeaponProfBonus = 3
	WpSpecializedMagicLevel        WeaponProfBonus = 4
	WpSpellAugment                 WeaponProfBonus = 5
	WpBestiary                     WeaponProfBonus = 6
	WpPowerfulFoeBonus             WeaponProfBonus = 7
	WpCriticalHitChance            WeaponProfBonus = 8
	WpElementalHitChance           WeaponProfBonus = 9
	WpRuneCriticalHitChance        WeaponProfBonus = 10
	WpAutoAttackCriticalHitChance  WeaponProfBonus = 11
	WpCriticalExtraDamage          WeaponProfBonus = 12
	WpElementalCriticalExtraDamage WeaponProfBonus = 13
	WpRuneCriticalExtraDamage      WeaponProfBonus = 14
	WpAutoAttackCriticalExtraDmg   WeaponProfBonus = 15
	WpManaLeech                    WeaponProfBonus = 16
	WpLifeLeech                    WeaponProfBonus = 17
	WpManaGainOnHit                WeaponProfBonus = 18
	WpLifeGainOnHit                WeaponProfBonus = 19
	WpManaGainOnKill               WeaponProfBonus = 20
	WpLifeGainOnKill               WeaponProfBonus = 21
	WpPerfectShotDamage            WeaponProfBonus = 22
	WpRangedHitChance              WeaponProfBonus = 23
	WpAttackRange                  WeaponProfBonus = 24
	WpSkillPercentageAutoAttack    WeaponProfBonus = 25
	WpSkillPercentageSpellDamage   WeaponProfBonus = 26
	WpSkillPercentageSpellHealing  WeaponProfBonus = 27
	WpAlphaStrikeExtraDamage       WeaponProfBonus = 28
	WpOmegaStrikeExtraDamage       WeaponProfBonus = 29
	WpArmorPenetration             WeaponProfBonus = 30
	WpElementalPierce              WeaponProfBonus = 31
)

// Augment types a SPELL_AUGMENT perk can carry
// (weapon_proficiency.cpp:31 — note the values are sparse).
const (
	AugmentDamage         uint8 = 2
	AugmentHeal           uint8 = 3
	AugmentCooldown       uint8 = 6
	AugmentLifeLeech      uint8 = 14
	AugmentManaLeech      uint8 = 15
	AugmentCriticalDamage uint8 = 16
	AugmentCriticalChance uint8 = 17
)

// SkillPercentage_t (src/enums/weapon_proficiency.hpp:53).
const (
	SkillPctAutoAttack uint8 = 0
	SkillPctSpellDmg   uint8 = 1
	SkillPctSpellHeal  uint8 = 2
)

type WeaponProfCritical struct {
	Chance float64 `json:"chance"`
	Damage float64 `json:"damage"`
}

type WeaponProfAugment struct {
	SpellID uint16  `json:"spellId"`
	Id      uint8   `json:"id"`
	Data    float64 `json:"data"`
}

func NewWeaponProficiency() *WeaponProficiency {
	return &WeaponProficiency{
		stats:           make(map[WeaponProfBonus]float64),
		skillPcts:       make(map[Skill]SkillPercentage),
		bestiaryDamage:  make(map[string]float64),
		elementCritical: make(map[int]WeaponProfCritical),
		augments:        make(map[uint16][]WeaponProfAugment),
	}
}

// --- JSON serialization for DB persistence ---

type wpJSON struct {
	Stats             map[string]float64             `json:"stats"`
	SkillPcts         map[string]*SkillPercentage    `json:"skillPcts,omitempty"`
	BestiaryDamage    map[string]float64             `json:"bestiaryDamage,omitempty"`
	RunesCritical     *WeaponProfCritical            `json:"runesCritical,omitempty"`
	AutoCrit          *WeaponProfCritical            `json:"autoCrit,omitempty"`
	GeneralCritical   *WeaponProfCritical            `json:"generalCritical,omitempty"`
	PowerfulFoeDamage float64                        `json:"powerfulFoeDamage"`
	Augments          map[string][]WeaponProfAugment `json:"augments,omitempty"`
}

func bonusKey(b WeaponProfBonus) string { return fmt.Sprintf("%d", b) }
func skillKey(s Skill) string           { return fmt.Sprintf("%d", s) }

// MarshalJSON serializes the proficiency data.
func (wp *WeaponProficiency) MarshalJSON() ([]byte, error) {
	j := &wpJSON{
		Stats:             make(map[string]float64),
		BestiaryDamage:    wp.bestiaryDamage,
		PowerfulFoeDamage: wp.powerfulFoeDamage,
	}
	for k, v := range wp.stats {
		j.Stats[bonusKey(k)] = v
	}
	if len(wp.skillPcts) > 0 {
		j.SkillPcts = make(map[string]*SkillPercentage)
		for k, v := range wp.skillPcts {
			p := v
			j.SkillPcts[skillKey(k)] = &p
		}
	}
	if wp.runesCritical != (WeaponProfCritical{}) {
		j.RunesCritical = &wp.runesCritical
	}
	if wp.autoCrit != (WeaponProfCritical{}) {
		j.AutoCrit = &wp.autoCrit
	}
	if wp.generalCritical != (WeaponProfCritical{}) {
		j.GeneralCritical = &wp.generalCritical
	}
	if len(wp.augments) > 0 {
		j.Augments = make(map[string][]WeaponProfAugment)
		for k, v := range wp.augments {
			j.Augments[fmt.Sprintf("%d", k)] = v
		}
	}
	return json.Marshal(j)
}

// UnmarshalJSON deserializes proficiency data.
func (wp *WeaponProficiency) UnmarshalJSON(data []byte) error {
	j := &wpJSON{}
	if err := json.Unmarshal(data, j); err != nil {
		return err
	}
	wp.stats = make(map[WeaponProfBonus]float64)
	for k, v := range j.Stats {
		if idx, err := strconv.ParseUint(k, 10, 64); err == nil {
			wp.stats[WeaponProfBonus(idx)] = v
		}
	}
	wp.bestiaryDamage = j.BestiaryDamage
	wp.powerfulFoeDamage = j.PowerfulFoeDamage
	if len(j.SkillPcts) > 0 {
		wp.skillPcts = make(map[Skill]SkillPercentage)
		for k, v := range j.SkillPcts {
			if v != nil {
				if idx, err := strconv.ParseUint(k, 10, 64); err == nil {
					wp.skillPcts[Skill(idx)] = *v
				}
			}
		}
	}
	if j.RunesCritical != nil {
		wp.runesCritical = *j.RunesCritical
	}
	if j.AutoCrit != nil {
		wp.autoCrit = *j.AutoCrit
	}
	if j.GeneralCritical != nil {
		wp.generalCritical = *j.GeneralCritical
	}
	if len(j.Augments) > 0 {
		wp.augments = make(map[uint16][]WeaponProfAugment)
		for k, v := range j.Augments {
			if idx, err := strconv.ParseUint(k, 10, 64); err == nil {
				wp.augments[uint16(idx)] = v
			}
		}
	}
	return nil
}

// --- Rest of the methods unchanged ---

func (wp *WeaponProficiency) GetStat(bonus WeaponProfBonus) float64 {
	if wp == nil || wp.stats == nil {
		return 0
	}
	return wp.stats[bonus]
}

func (wp *WeaponProficiency) AddStat(bonus WeaponProfBonus, value float64) {
	if wp.stats == nil {
		wp.stats = make(map[WeaponProfBonus]float64)
	}
	wp.stats[bonus] += value
}

func (wp *WeaponProficiency) GetRunesCritical() WeaponProfCritical {
	if wp == nil {
		return WeaponProfCritical{}
	}
	return wp.runesCritical
}

func (wp *WeaponProficiency) SetRunesCritical(c WeaponProfCritical) { wp.runesCritical = c }

func (wp *WeaponProficiency) GetAutoAttackCritical() WeaponProfCritical {
	if wp == nil {
		return WeaponProfCritical{}
	}
	return wp.autoCrit
}

func (wp *WeaponProficiency) SetAutoAttackCritical(c WeaponProfCritical) { wp.autoCrit = c }

func (wp *WeaponProficiency) GetGeneralCritical() WeaponProfCritical {
	if wp == nil {
		return WeaponProfCritical{}
	}
	return wp.generalCritical
}

func (wp *WeaponProficiency) SetGeneralCritical(c WeaponProfCritical) { wp.generalCritical = c }

func (wp *WeaponProficiency) GetSkillPercentage(skill Skill) SkillPercentage {
	if wp == nil || wp.skillPcts == nil {
		return SkillPercentage{}
	}
	return wp.skillPcts[skill]
}

func (wp *WeaponProficiency) SetSkillPercentage(skill Skill, pct SkillPercentage) {
	if wp.skillPcts == nil {
		wp.skillPcts = make(map[Skill]SkillPercentage)
	}
	wp.skillPcts[skill] = pct
}

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

func (wp *WeaponProficiency) AddBestiaryDamage(name string, amount float64) {
	if wp.bestiaryDamage == nil {
		wp.bestiaryDamage = make(map[string]float64)
	}
	wp.bestiaryDamage[name] += amount
}

func (wp *WeaponProficiency) GetPowerfulFoeDamage() float64 {
	if wp == nil {
		return 0
	}
	return wp.powerfulFoeDamage
}

func (wp *WeaponProficiency) SetPowerfulFoeDamage(v float64) { wp.powerfulFoeDamage = v }

func (wp *WeaponProficiency) GetAugments(weaponId uint16) []WeaponProfAugment {
	if wp == nil || wp.augments == nil {
		return nil
	}
	return wp.augments[weaponId]
}

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

func (wp *WeaponProficiency) AddAugment(weaponId uint16, aug WeaponProfAugment) {
	if wp.augments == nil {
		wp.augments = make(map[uint16][]WeaponProfAugment)
	}
	wp.augments[weaponId] = append(wp.augments[weaponId], aug)
}
