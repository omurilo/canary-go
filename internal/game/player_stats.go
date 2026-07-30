package game

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/game/vocations"
)


// ============================================================================
// Base types
// ============================================================================

type BaseCritical struct {
	Chance uint16
	Damage uint16
}

func (p *Player) GetBaseCritical() BaseCritical {
	return BaseCritical{Chance: p.GetCriticalChance(), Damage: p.GetCriticalDamage()}
}

type SkillsEquipment struct {
	Equipment float64
	Imbuement float64
}


func (p *Player) GetForgeSkillStat(slot uint8) float64 {
	// Map cyclopedia forge slot to inventory slot + forge type.
	// Slot mapping from C++ ExaltationForge:
	//   1 = head (Momentum)
	//   3 = weapon (Onslaught)
	//   5 = armor (Dodge)
	//   7 = legs
	//   8 = feet
	inventorySlot := mapSlotToInventory(slot)
	if inventorySlot < 0 || int(inventorySlot) >= len(p.Inventory) || p.Inventory[inventorySlot] == nil {
		return 0.0
	}
	item := p.Inventory[inventorySlot]
	tier := item.GetTier()
	if tier == 0 {
		return 0.0
	}
	// Forge bonus = tier * dustLevelFactor
	// Each forge tier provides a bonus that scales with the player's dust level cap.
	dustLevel := float64(p.ForgeDustLevel)
	if dustLevel <= 0 {
		dustLevel = 100 // default cap
	}
	// Simplified forge formula: each tier gives (dustLevel/100) bonus
	return float64(tier) * (dustLevel / 100.0)
}

// mapSlotForgeToInventory maps the cyclopedia forge skill slot number to its
// corresponding inventory slot constant.
func mapSlotToInventory(slot uint8) int {
	switch slot {
	case 1: // Head (Momentum)
		return int(ConstSlotHead)
	case 3: // Weapon (Onslaught)
		return int(ConstSlotLeft)
	case 5: // Armor (Dodge)
		return int(ConstSlotArmor)
	case 7: // Legs
		return int(ConstSlotLegs)
	case 8: // Feet
		return int(ConstSlotFeet)
	default:
		return -1
	}
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
	if item == nil {
		return nil
	}

	distanceSkill := float64(p.GetEffectiveSkill(SkillDistance))
	ammoType := item.AmmoType(p.World.Items)

	if ammoType == "bolt" || ammoType == "arrow" {
		return []float64{
			math.Min(90, 1.20*(distanceSkill+1)),
			math.Min(90, 3.20*distanceSkill),
			math.Min(90, 2.00*distanceSkill),
			math.Min(90, 1.55*distanceSkill),
			math.Min(90, 1.20*(distanceSkill+1)),
			math.Min(90, distanceSkill),
		}
	}

	// Other distance weapons (spear, throwing star, throwing knife, stone, etc.)
	return []float64{
		math.Min(75, distanceSkill+1),
		math.Min(75, 2.40*(distanceSkill+8)),
		math.Min(75, 1.55*(distanceSkill+6)),
		math.Min(75, 1.25*(distanceSkill+3)),
		math.Min(75, distanceSkill+1),
		math.Min(75, 0.80*(distanceSkill+3)),
		math.Min(75, 0.70*(distanceSkill+2)),
	}
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
	voc := vocations.GetVocation(uint32(p.Vocation))
	if voc == nil {
		return 0.0
	}

	skill := int32(p.GetEffectiveSkill(SkillShielding))
	defenseValue := int32(0)
	fightFactor := 1.0
	shieldFactor := voc.Mitigation.PrimaryShield
	distanceFactor := 1.0

	switch p.FightMode {
	case 1: // attack
		fightFactor = 0.8
	case 2: // balanced
		fightFactor = 1.0
	case 3: // defense
		fightFactor = 1.2
	}

	// Right slot: shield/spellbook/quiver
	if int(ConstSlotRight) < len(p.Inventory) && p.Inventory[ConstSlotRight] != nil {
		shield := p.Inventory[ConstSlotRight]
		wt := shield.WeaponType(p.World.Items)
		isShield := wt == "shield" && !shield.IsQuiver(p.World.Items)
		if isShield {
			shieldFactor = voc.Mitigation.PrimaryShield
			defenseValue = shield.Defense(p.World.Items)
		} else {
			// Spellbook or quiver in right slot
			distanceFactor = voc.Mitigation.SecondaryShield
		}
	}

	// Left slot: weapon
	if int(ConstSlotLeft) < len(p.Inventory) && p.Inventory[ConstSlotLeft] != nil {
		weapon := p.Inventory[ConstSlotLeft]
		ammoType := weapon.AmmoType(p.World.Items)
		if ammoType == "bolt" || ammoType == "arrow" {
			distanceFactor = voc.Mitigation.SecondaryShield
		} else if p.World.Items != nil {
			it := p.World.Items.Get(weapon.ID)
			if it != nil && it.SlotPosition == "two-handed" {
				defenseValue = weapon.Defense(p.World.Items) + weapon.ExtraDefense(p.World.Items)
				shieldFactor = voc.Mitigation.SecondaryShield
			} else {
				defenseValue += weapon.ExtraDefense(p.World.Items)
				shieldFactor = voc.Mitigation.PrimaryShield
			}
		}
	}

	base := (float64(skill)*voc.Mitigation.Multiplier + shieldFactor*float64(defenseValue)) / 100.0
	mitigation := math.Ceil(base*fightFactor*distanceFactor*100.0) / 100.0

	return mitigation
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
	{4, "absorbpercentundefined"}, // COMBAT_UNDEFINEDDAMAGE (4)
	{5, "absorbpercentlifedrain"}, // COMBAT_LIFEDRAIN (5)
	{6, "absorbpercentmanadrain"}, // COMBAT_MANADRAIN (6)
	{7, "absorbpercentlifedrain"}, // COMBAT_HEALING (7) — same OTB key
	{8, "absorbpercentdrown"},     // COMBAT_DROWNDAMAGE (8)
	{9, "absorbpercentice"},       // COMBAT_ICEDAMAGE (9)
	{10, "absorbpercentholy"},     // COMBAT_HOLYDAMAGE (10)
	{11, "absorbpercentdeath"},    // COMBAT_DEATHDAMAGE (11)
	{12, "absorbpercentagony"},    // COMBAT_AGONYDAMAGE (12)
	{13, "absorbpercentneutral"},  // COMBAT_NEUTRALDAMAGE (13)
}

// combatTypeToCipbia maps C++ combat type index (0-13) to CIPBIA element byte.
// C++ calculateAbsorbValues writes getCipbiaElement(indexToCombatType(i)).
var combatTypeToCipbia = [14]uint8{
	0,  // 0: COMBAT_PHYSICAL → CIPBIA_PHYSICAL
	1,  // 1: COMBAT_FIRE     → CIPBIA_FIRE
	2,  // 2: COMBAT_EARTH    → CIPBIA_EARTH
	3,  // 3: COMBAT_ENERGY   → CIPBIA_ENERGY
	12, // 4: COMBAT_UNDEFINED→ CIPBIA_UNDEFINED
	9,  // 5: COMBAT_LIFEDRAIN→ CIPBIA_LIFEDRAIN
	10, // 6: COMBAT_MANADRAIN→ CIPBIA_MANADRAIN
	7,  // 7: COMBAT_HEALING  → CIPBIA_HEALING
	8,  // 8: COMBAT_DROWN    → CIPBIA_DROWN
	4,  // 9: COMBAT_ICE      → CIPBIA_ICE
	5,  // 10: COMBAT_HOLY    → CIPBIA_HOLY
	6,  // 11: COMBAT_DEATH   → CIPBIA_DEATH
	11, // 12: COMBAT_AGONY   → CIPBIA_AGONY
	12, // 13: COMBAT_NEUTRAL → CIPBIA_UNDEFINED
}
// equipped items (matching C++ calculateAbsorbValues). It also applies the
// player's base AbsorbPercent. Imbuement and wheel contributions are stubbed
// (0) until those systems expose per-element absorb data.
func (p *Player) GetCombatAbsorbs() []CombatAbsorb {
	const combatCount = 14 // match C++ COMBAT_COUNT
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
				Element: combatTypeToCipbia[i],
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
	case "kooldown-aid":
		return 36723
	case "stamina-extension":
		return 36724
	case "strike-enhancement":
		return 36725
	case "charm-upgrade":
		return 36726
	case "wealth-duplex":
		return 36727
	case "bestiary-betterment":
		return 36728
	case "fire-resilience":
		return 36729
	case "ice-resilience":
		return 36730
	case "earth-resilience":
		return 36731
	case "energy-resilience":
		return 36732
	case "holy-resilience":
		return 36733
	case "death-resilience":
		return 36734
	case "physical-resilience":
		return 36735
	case "fire-amplification":
		return 36736
	case "ice-amplification":
		return 36737
	case "earth-amplification":
		return 36738
	case "energy-amplification":
		return 36739
	case "holy-amplification":
		return 36740
	case "death-amplification":
		return 36741
	case "physical-amplification":
		return 36742
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
	if p.ActiveFoodItems == nil {
		return nil
	}
	var result []ActiveFood
	for itemID, timeLeft := range p.ActiveFoodItems {
		if timeLeft > 0 {
			result = append(result, ActiveFood{
				ItemID:   itemID,
				TimeLeft: timeLeft,
			})
		}
	}
	return result
}

// UpdateFoodItem adds or updates a food item's remaining time. If timeLeft is 0
// the entry is removed. Matches C++ Player::updateFood().
func (p *Player) UpdateFoodItem(itemID uint16, timeLeft uint32) {
	if timeLeft == 0 {
		delete(p.ActiveFoodItems, itemID)
	} else {
		if p.ActiveFoodItems == nil {
			p.ActiveFoodItems = make(map[uint16]uint32)
		}
		p.ActiveFoodItems[itemID] = timeLeft
	}
}

// IsFoodActive returns true if the given food item ID has remaining time > 0.
func (p *Player) IsFoodActive(itemID uint16) bool {
	if p.ActiveFoodItems == nil {
		return false
	}
	remaining, ok := p.ActiveFoodItems[itemID]
	return ok && remaining > 0
}

type WeaponProficiencyAugment struct {
	SpellID uint16
	Id      uint8
	Data    float64
}


func (p *Player) GetWheelAugments() []WeaponProfAugment {
	if p.Wheel == nil {
		return nil
	}

	var result []WeaponProfAugment

	// Unlocked instants — spells granted by maxing wheel slots
	instants := p.Wheel.GetUnlockedInstants()
	for name := range instants {
		spellID := wheelInstantSpellID(name)
		if spellID > 0 {
			result = append(result, WeaponProfAugment{
				SpellID: spellID,
				Id:      0, // augment type: instant unlocked
				Data:    1.0,
			})
		}
	}

	// Revelation stages — per-spell upgrades
	for spellName, stage := range p.Wheel.RevelationStages {
		if stage > 0 {
			spellID := wheelRevelationSpellID(spellName)
			if spellID > 0 {
				result = append(result, WeaponProfAugment{
					SpellID: spellID,
					Id:      1, // augment type: revelation upgrade
					Data:    float64(stage),
				})
			}
		}
	}

	// Revelation points progress
	for spellName, pts := range p.Wheel.RevelationPoints {
		if pts > 0 {
			spellID := wheelRevelationSpellID(spellName)
			if spellID > 0 {
				stage := p.Wheel.RevelationStages[spellName]
				result = append(result, WeaponProfAugment{
					SpellID: spellID,
					Id:      2, // augment type: revelation points
					Data:    float64(pts) - float64(stage)*250.0,
				})
			}
		}
	}

	return result
}

// wheelInstantSpellID maps wheel instant names to spell IDs for cyclopedia use.
func wheelInstantSpellID(name string) uint16 {
	switch name {
	case "green":
		return 1
	case "purple":
		return 2
	default:
		return 0
	}
}

// wheelRevelationSpellID maps revelation spell names to spell IDs.
func wheelRevelationSpellID(name string) uint16 {
	switch name {
	case "inflict_wound", "Inflict Wound":
		return 230
	case "curse", "Curse":
		return 231
	case "blood_rage", "Blood Rage":
		return 232
	case "divine_caldera", "Divine Caldera", "divinecaldera":
		return 235
	case "groundshaker", "Groundshaker":
		return 240
	case "berserk", "Berserk":
		return 241
	case "fierce_berserk", "Fierce Berserk":
		return 242
	case "whirlwind_throw", "Whirlwind Throw":
		return 245
	case "ethereal_spear", "Ethereal Spear":
		return 246
	case "annihilation", "Annihilation":
		return 247
	case "expose_weakness", "Expose Weakness":
		return 248
	case "executioner_throw", "Executioner Throw":
		return 249
	case "revelation_magic":
		return 250
	case "divine_missile", "Divine Missile":
		return 251
	default:
		return 0
	}
}

func (p *Player) GetEquippedAugments() []WeaponProfAugment {
	var result []WeaponProfAugment
	seen := make(map[string]bool) // dedup by "spellID:id"

	for s := ConstSlotFirst; s <= ConstSlotLast; s++ {
		if int(s) >= len(p.Inventory) || p.Inventory[s] == nil {
			continue
		}
		item := p.Inventory[s]

		// Check weapon proficiency augments for this item's weapon type
		if p.WeaponProficiency != nil {
			augs := p.WeaponProficiency.GetAugments(item.ID)
			for _, a := range augs {
				key := fmt.Sprintf("%d:%d", a.SpellID, a.Id)
				if !seen[key] {
					seen[key] = true
					result = append(result, a)
				}
			}
		}

		// Check item type Stats for augment-like attributes
		t := p.World.Items.Get(item.ID)
		if t != nil && t.Stats != nil {
			for key, val := range t.Stats {
				if !strings.HasPrefix(key, "augment") {
					continue
				}
				// Parse augment data from stat key: "augment_<spellID>_<id>"
				aug := parseAugmentStat(key, val)
				if aug != nil {
					dup := fmt.Sprintf("%d:%d", aug.SpellID, aug.Id)
					if !seen[dup] {
						seen[dup] = true
						result = append(result, *aug)
					}
				}
			}
		}
	}

	return result
}

// parseAugmentStat attempts to extract a WeaponProfAugment from a stat key like
// "augment_<spellID>_<id>". Returns nil if the key doesn't match the pattern.
func parseAugmentStat(key string, val int32) *WeaponProfAugment {
	parts := strings.Split(key, "_")
	if len(parts) < 3 {
		return nil
	}
	if parts[0] != "augment" {
		return nil
	}
	spellID, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return nil
	}
	augID, err := strconv.ParseUint(parts[2], 10, 8)
	if err != nil {
		return nil
	}
	return &WeaponProfAugment{
		SpellID: uint16(spellID),
		Id:      uint8(augID),
		Data:    float64(val),
	}
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

// ShieldSkillMitigationFactor returns the base mitigation factor contribution from
// shield skill: shieldSkill * vocation.mitigationFactor / 10000.
// Used in cyclopedia DefenceStats to match C++ protocol output.
func (p *Player) ShieldSkillMitigationFactor() float64 {
	voc := vocations.GetVocation(uint32(p.Vocation))
	if voc == nil {
		return 0.0
	}
	return float64(p.GetEffectiveSkill(SkillShielding)) * voc.Mitigation.Multiplier / 10000.0
}

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