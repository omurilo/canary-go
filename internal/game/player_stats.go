package game

import (
	"strconv"
	"time"
)


// ============================================================================
// Base types
// ============================================================================

type BaseCritical struct {
	Chance uint16
	Damage uint16
}

func (p *Player) GetBaseCritical() BaseCritical {
	return BaseCritical{Chance: p.CriticalChance, Damage: p.CriticalDamage}
}

type SkillsEquipment struct {
	Equipment float64
	Imbuement float64
}


func (p *Player) GetForgeSkillStat(slot uint8) float64 {
	return 0.0
}

// ============================================================================
// Weapon attack helpers (catalog-independent)
// ============================================================================

func (p *Player) GetWeaponSkill(item *Item) int32 {
	if item == nil {
		return int32(p.GetEffectiveSkill(SkillFist))
	}
	switch item.WeaponType(nil) {
	case "sword":
		return int32(p.GetEffectiveSkill(SkillSword))
	case "club":
		return int32(p.GetEffectiveSkill(SkillClub))
	case "axe":
		return int32(p.GetEffectiveSkill(SkillAxe))
	case "distance", "ammunition", "missile":
		return int32(p.GetEffectiveSkill(SkillDistance))
	default:
		return int32(p.GetEffectiveSkill(SkillFist))
	}
}

func GetWeaponSkillId(weaponType string) uint8 {
	switch weaponType {
	case "sword":
		return 8
	case "club":
		return 9
	case "axe":
		return 10
	case "distance", "ammunition", "missile":
		return 7
	case "wand":
		return 0
	default:
		return CipbiaSkillFist
	}
}

func (p *Player) GetAttackSkill(item *Item) uint16 {
	if item == nil {
		return p.GetEffectiveSkill(SkillFist)
	}
	skill := float64(p.GetWeaponSkill(item))
	skillFactor := (skill + 4) / 28.0
	attack := float64(item.Attack(nil))*skillFactor - float64(item.Attack(nil))
	if attack < 0 {
		return 0
	}
	return uint16(attack)
}

func (p *Player) GetDistanceAttackSkill(skill, weaponAttack int32) uint16 {
	skillF := float64(skill)
	skillFactor := (skillF + 4) / 28.0
	result := float64(weaponAttack)*skillFactor - float64(weaponAttack)
	if result < 0 {
		return 0
	}
	return uint16(result)
}

func (p *Player) AttackRawTotal(flatBonus, equipment, skill uint16) uint16 {
	r := uint32(flatBonus) + uint32(equipment) + uint32(skill)*2
	if r > 65535 {
		return 65535
	}
	return uint16(r)
}

func (p *Player) AttackTotal(flatBonus, equipment, skill uint16) uint16 {
	fightFactor := 1.0
	switch p.FightMode {
	case 1:
		fightFactor = 1.2
	case 2:
		fightFactor = 1.0
	case 3:
		fightFactor = 0.6
	}
	attack := float64(p.AttackRawTotal(flatBonus, equipment, skill))
	total := uint16(attack * fightFactor)
	if total < 1 {
		total = 1
	}
	return total
}

func (p *Player) CalculateFlatDamageHealing() uint16 {
	level := uint32(p.Level)
	if level == 0 {
		return 0
	}
	var prevAgg float64
	var curBase uint32
	var factor float64 = 1.0 / 5.0
	threshold := uint32(500)
	step := uint32(600)
	tier := uint32(1)
	for level >= threshold {
		curBase = threshold
		prevFactor := 1.0 / (5.0 + float64(tier-1))
		prevAgg += float64(threshold) * prevFactor
		factor = 1.0 / (5.0 + float64(tier))
		tier++
		threshold += step
		step += 100
	}
	computed := prevAgg + float64(level-curBase)*factor
	intPart := int64(computed)
	if computed > float64(intPart) {
		intPart++
	}
	if intPart > 65535 {
		return 65535
	}
	return uint16(intPart)
}

func (p *Player) GetDamageAccuracy(item *Item) []float64 {
	return nil
}

// ============================================================================
// Stat sum helpers — read from equipped item Stats map
// ============================================================================

// sumItemStat returns the sum of a numeric attribute across all equipment slots.
func sumItemStat(p *Player, key string) int32 {
	var total int32
	for s := ConstSlotFirst; s <= ConstSlotLast; s++ {
		if int(s) >= len(p.Inventory) || p.Inventory[s] == nil {
			continue
		}
		t := p.World.Items.Get(p.Inventory[s].ID)
		if t != nil && t.Stats != nil {
			if v, ok := t.Stats[key]; ok {
				total += v
			}
		}
	}
	return total
}

func (p *Player) GetCleavePercent() int32 {
	return sumItemStat(p, "cleavepercent")
}

func (p *Player) GetPerfectShotDamage(range_ uint8) int32 {
	return sumItemStat(p, "perfectshotdamage")
}

func (p *Player) GetMagicShieldCapacityFlat() int32 {
	return sumItemStat(p, "magicshieldcapacityflat")
}

func (p *Player) GetMagicShieldCapacityPercent() int32 {
	return sumItemStat(p, "magicshieldcapacitypercent")
}

func (p *Player) GetReflectFlat(combatType int) int32 {
	return sumItemStat(p, "reflectdamage")
}

func (p *Player) GetMantra() int32 {
	return sumItemStat(p, "mantra")
}

func (p *Player) GetDefenseEquipment() uint16 {
	var total uint16
	for s := ConstSlotFirst; s <= ConstSlotLast; s++ {
		if int(s) >= len(p.Inventory) || p.Inventory[s] == nil {
			continue
		}
		if d := p.Inventory[s].Defense(nil); d > 0 {
			total += uint16(d)
		}
	}
	return total
}

func (p *Player) GetMitigation() float64 {
	return 0.0
}

func (p *Player) GetSkillsEquipment() [SkillCount]SkillsEquipment {
	var skills [SkillCount]SkillsEquipment
	for s := ConstSlotFirst; s <= ConstSlotLast; s++ {
		if int(s) >= len(p.Inventory) || p.Inventory[s] == nil || p.World == nil {
			continue
		}
		t := p.World.Items.Get(p.Inventory[s].ID)
		if t == nil || t.Stats == nil {
			continue
		}
		skillKeys := []struct {
			idx Skill
			key string
		}{
			{SkillFist, "skillfist"},
			{SkillClub, "skillclub"},
			{SkillSword, "skillsword"},
			{SkillAxe, "skillaxe"},
			{SkillDistance, "skilldist"},
			{SkillShielding, "skillshield"},
			{SkillFishing, "skillfishing"},
		}
		for _, sk := range skillKeys {
			if v, ok := t.Stats[sk.key]; ok && v > 0 {
				skills[sk.idx].Equipment += float64(v)
			}
		}
	}
	return skills
}

// ============================================================================
// Combat absorbs — multiplicative stacking matching C++ calculateAbsorbValues
// ============================================================================

type CombatAbsorb struct {
	Element uint8
	Absorb  float64 // client modifier: (10000 - damageModifier) / 10000.
}

// elementKeyMap maps combat type index to the absorb stat key.
var elementKeyMap = []struct {
	idx int
	key string
}{
	{0, "absorbpercentphysical"},
	{1, "absorbpercentfire"},
	{2, "absorbpercentearth"},
	{3, "absorbpercentenergy"},
	{4, "absorbpercentice"},
	{5, "absorbpercentholy"},
	{6, "absorbpercentdeath"},
	{7, "absorbpercentlifedrain"}, // healing (C++ COMBAT_HEALING)
	{8, "absorbpercentdrown"},
	{9, "absorbpercentlifedrain"}, // lifedrain (same key in OTB, different combat type)
	{10, "absorbpercentmanadrain"},
}

// GetCombatAbsorbs computes multiplicative absorb per combat type from all
// equipped items (matching C++ calculateAbsorbValues). It also applies the
// player's base AbsorbPercent. Imbuement and wheel contributions are stubbed
// (0) until those systems expose per-element absorb data.
func (p *Player) GetCombatAbsorbs() []CombatAbsorb {
	const combatCount = 11 // match elementKeyMap length
	// Start at 10000 (no reduction)
	var mods [combatCount]int32
	for i := range mods {
		mods[i] = 10000
	}

	for slot := ConstSlotFirst; slot <= ConstSlotLast; slot++ {
		if int(slot) >= len(p.Inventory) || p.Inventory[slot] == nil {
			continue
		}
		t := p.World.Items.Get(p.Inventory[slot].ID)
		if t == nil || t.Stats == nil {
			continue
		}
		for _, ek := range elementKeyMap {
			if v, ok := t.Stats[ek.key]; ok && v != 0 {
				// Multiplicative stacking: mod *= (100 - absorb%) / 100
				mods[ek.idx] = int32(float64(mods[ek.idx]) * (100.0 - float64(v)) / 100.0)
			}
		}
		// Imbuement absorb contribution — stubbed until ImbuementType exposes per-element absorbs
	}

	// Player base absorb percent (applied as flat subtraction, matching C++)
	baseAbsorb := int32(p.GetAbsorbPercent()) * 100
	_ = baseAbsorb // reserved for when GetAbsorbPercent becomes per-type

	// Wheel resistance — stubbed (0) until Wheel exposes GetResistance(combatType)

	var result []CombatAbsorb
	for i := 0; i < combatCount; i++ {
		if mods[i] != 10000 {
			clientModifier := float64(10000-mods[i]) / 10000.0
			result = append(result, CombatAbsorb{
				Element: uint8(i),
				Absorb:  clientModifier,
			})
		}
	}
	return result
}

func (p *Player) GetSpecializedMagicLevel(combatTypeIndex int) int32 {
	// Element-specific magic level from equipment (e.g. elementfire, elementice)
	elemKeys := []string{
		"elementphysical", "elementfire", "elementearth", "elementenergy",
		"elementice", "elementholy", "elementdeath",
	}
	if combatTypeIndex >= 0 && combatTypeIndex < len(elemKeys) {
		return sumItemStat(p, elemKeys[combatTypeIndex])
	}
	return 0
}

// ============================================================================
// Bestiary, critical, proficiency — from WeaponProficiency
// ============================================================================

type ActiveBestiaryDamage struct {
	Name   string
	Amount float64
}

func (p *Player) GetActiveBestiariesDamage() []ActiveBestiaryDamage {
	if p.WeaponProficiency == nil {
		return nil
	}
	return p.WeaponProficiency.GetActiveBestiariesDamage()
}

type CriticalHit struct {
	Chance float64
	Damage float64
}

func (p *Player) GetRunesCritical() CriticalHit {
	if p.WeaponProficiency == nil {
		return CriticalHit{}
	}
	c := p.WeaponProficiency.GetRunesCritical()
	return CriticalHit{Chance: c.Chance, Damage: c.Damage}
}

func (p *Player) GetAutoAttackCritical() CriticalHit {
	if p.WeaponProficiency == nil {
		return CriticalHit{}
	}
	c := p.WeaponProficiency.GetAutoAttackCritical()
	return CriticalHit{Chance: c.Chance, Damage: c.Damage}
}

type SkillPercentage struct {
	Skill        Skill
	AutoAttack   float64
	SpellDamage  float64
	SpellHealing float64
}

func (p *Player) GetSkillPercentage(skill Skill) SkillPercentage {
	if p.WeaponProficiency == nil {
		return SkillPercentage{}
	}
	return p.WeaponProficiency.GetSkillPercentage(skill)
}

func (p *Player) HasCharmExpansion() bool {
	return false
}

func (p *Player) GetWeaponProficiencyAugments() []WeaponProfAugment {
	if p.WeaponProficiency == nil {
		return nil
	}
	augs := p.WeaponProficiency.GetAllAugments()
	var result []WeaponProfAugment
	for _, a := range augs {
		result = append(result, a)
	}
	return result
}

// ============================================================================
// Consumables / augments — need system support
// ============================================================================

type ActiveConcoction struct {
	ItemID   uint16
	TimeLeft uint32
}

func (p *Player) GetActiveConcoctions() []ActiveConcoction {
	if p.Concoctions == nil {
		return nil
	}
	now := time.Now().Unix()
	var result []ActiveConcoction
	for name, expiry := range p.Concoctions {
		if expiry > now {
			remaining := uint32(expiry - now)
			// Map concoction name to item ID
			id := concoctionNameToID(name)
			if id > 0 {
				result = append(result, ActiveConcoction{
					ItemID:   id,
					TimeLeft: remaining,
				})
			}
		}
	}
	return result
}

func concoctionNameToID(name string) uint16 {
	switch name {
	case "bullseye":
		return 26031
	case "berserk":
		return 26032
	case "mastermind":
		return 26033
	case "fatal":
		return 26034
	case "relic":
		return 26035
	default:
		// Try numeric ID from C++ naming like "24325"
		if id, err := strconv.ParseUint(name, 10, 16); err == nil {
			return uint16(id)
		}
		return 0
	}
}

type ActiveFood struct {
	ItemID   uint16
	TimeLeft uint32
}

func (p *Player) GetActiveFoods() []ActiveFood {
	return nil
}

type WeaponProficiencyAugment struct {
	SpellID uint16
	Id      uint8
	Data    float64
}


func (p *Player) GetWheelAugments() []WeaponProfAugment {
	return nil
}

func (p *Player) GetEquippedAugments() []WeaponProfAugment {
	return nil
}

// ============================================================================
// CIPBIA constants
// ============================================================================

const (
	CipbiaSkillFist     = 11
	CipbiaSkillClub     = 9
	CipbiaSkillSword    = 8
	CipbiaSkillAxe      = 10
	CipbiaSkillDistance = 7
	CipbiaSkillShield   = 6
	CipbiaSkillFishing  = 13
)

const (
	CipbiaElementPhysical  = 0
	CipbiaElementFire      = 1
	CipbiaElementEarth     = 2
	CipbiaElementEnergy    = 3
	CipbiaElementIce       = 4
	CipbiaElementHoly      = 5
	CipbiaElementDeath     = 6
	CipbiaElementHealing   = 7
	CipbiaElementDrown     = 8
	CipbiaElementLifedrain = 9
	CipbiaElementManadrain = 10
	CipbiaElementAgony     = 11
	CipbiaElementUndefined = 12
)

func GetCipbiaElement(idx int) uint8 {
	switch idx {
	case 0:
		return CipbiaElementPhysical
	case 1:
		return CipbiaElementFire
	case 2:
		return CipbiaElementEarth
	case 3:
		return CipbiaElementEnergy
	case 4:
		return CipbiaElementIce
	case 5:
		return CipbiaElementHoly
	case 6:
		return CipbiaElementDeath
	case 7:
		return CipbiaElementHealing
	case 8:
		return CipbiaElementDrown
	case 9:
		return CipbiaElementLifedrain
	case 10:
		return CipbiaElementManadrain
	case 11:
		return CipbiaElementAgony
	default:
		return CipbiaElementUndefined
	}
}

// GetWeaponCipbiaSkill maps a Skill type to the CIPBIA skill ID used in
// cyclopedia packets. Returns CipbiaSkillFist for unrecognized skills.
func GetWeaponCipbiaSkill(s Skill) uint8 {
	switch s {
	case SkillFist:
		return CipbiaSkillFist
	case SkillClub:
		return CipbiaSkillClub
	case SkillSword:
		return CipbiaSkillSword
	case SkillAxe:
		return CipbiaSkillAxe
	case SkillDistance:
		return CipbiaSkillDistance
	case SkillShielding:
		return CipbiaSkillShield
	case SkillFishing:
		return CipbiaSkillFishing
	default:
		return CipbiaSkillFist
	}
}