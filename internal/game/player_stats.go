package game


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

func (p *Player) GetSkillsEquipment() [SkillCount]SkillsEquipment {
	return [SkillCount]SkillsEquipment{}
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

// ============================================================================
// Combat absorbs
// ============================================================================

type CombatAbsorb struct {
	Element uint8
	Absorb  uint16
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
	{7, "absorbpercentlifedrain"}, // healing
	{8, "absorbpercentdrown"},
	{9, "absorbpercentlifedrain"},
	{10, "absorbpercentmanadrain"},
}

func (p *Player) GetCombatAbsorbs() []CombatAbsorb {
	var result []CombatAbsorb
	for _, ek := range elementKeyMap {
		v := sumItemStat(p, ek.key)
		if v > 0 {
			result = append(result, CombatAbsorb{
				Element: uint8(ek.idx),
				Absorb:  uint16(v),
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
// Bestiary, critical, proficiency — all need weapon proficiency system
// ============================================================================

type ActiveBestiaryDamage struct {
	Name   string
	Amount float64
}

func (p *Player) GetActiveBestiariesDamage() []ActiveBestiaryDamage {
	return nil
}

type CriticalHit struct {
	Chance float64
	Damage float64
}

func (p *Player) GetRunesCritical() CriticalHit {
	return CriticalHit{}
}

func (p *Player) GetAutoAttackCritical() CriticalHit {
	return CriticalHit{}
}

type SkillPercentage struct {
	Skill        Skill
	AutoAttack   float64
	SpellDamage  float64
	SpellHealing float64
}

func (p *Player) GetSkillPercentage(skill Skill) SkillPercentage {
	return SkillPercentage{}
}

func (p *Player) HasCharmExpansion() bool {
	return false
}

// ============================================================================
// Consumables / augments — need system support
// ============================================================================

type ActiveConcoction struct {
	ItemID   uint16
	TimeLeft uint32
}

func (p *Player) GetActiveConcoctions() []ActiveConcoction {
	return nil
}

type ActiveFood struct {
	ItemID   uint16
	TimeLeft uint32
}

func (p *Player) GetActiveFoods() []ActiveFood {
	return nil
}

type WeaponProficiencyAugment struct {
	Id   uint8
	Data float64
}

func (p *Player) GetWeaponProficiencyAugments() []WeaponProficiencyAugment {
	return nil
}

func (p *Player) GetWheelAugments() []uint8 {
	return nil
}

func (p *Player) GetEquippedAugments() []uint8 {
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