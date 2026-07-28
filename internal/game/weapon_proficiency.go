package game

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// WeaponProficiency stores per-weapon bonus data that feeds into cyclopedia
// OffenceStats (bestiary damage, runes/auto critical, skill percentages, augments).
type WeaponProficiency struct {
	stats           map[WeaponProfBonus]float64
	skillPcts       map[Skill]SkillPercentage
	bestiaryDamage  map[string]float64
	runesCritical   WeaponProfCritical
	autoCrit        WeaponProfCritical
	generalCritical WeaponProfCritical
	elementCritical map[int]WeaponProfCritical
	augments        map[uint16][]WeaponProfAugment
	powerfulFoeDamage float64
}

type WeaponProfBonus uint8

const (
	WpAttackDamage      WeaponProfBonus = 0
	WpDefenseBonus      WeaponProfBonus = 1
	WpWeaponShield      WeaponProfBonus = 2
	WpAttackSkill       WeaponProfBonus = 3
	WpLifeGainOnHit     WeaponProfBonus = 4
	WpManaGainOnHit     WeaponProfBonus = 5
	WpLifeGainOnKill    WeaponProfBonus = 6
	WpManaGainOnKill    WeaponProfBonus = 7
	WpCriticalChance    WeaponProfBonus = 8
	WpCriticalDamage    WeaponProfBonus = 9
	WpPerfectShotWeapon WeaponProfBonus = 10
	WpElementalWeapon   WeaponProfBonus = 11
)

type WeaponProfCritical struct {
	Chance float64 `json:"chance"`
	Damage float64 `json:"damage"`
}

type WeaponProfAugment struct {
	Id   uint8   `json:"id"`
	Data float64 `json:"data"`
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
	Stats             map[string]float64              `json:"stats"`
	SkillPcts         map[string]*SkillPercentage     `json:"skillPcts,omitempty"`
	BestiaryDamage    map[string]float64              `json:"bestiaryDamage,omitempty"`
	RunesCritical     *WeaponProfCritical             `json:"runesCritical,omitempty"`
	AutoCrit          *WeaponProfCritical             `json:"autoCrit,omitempty"`
	GeneralCritical   *WeaponProfCritical             `json:"generalCritical,omitempty"`
	PowerfulFoeDamage float64                         `json:"powerfulFoeDamage"`
	Augments          map[string][]WeaponProfAugment  `json:"augments,omitempty"`
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