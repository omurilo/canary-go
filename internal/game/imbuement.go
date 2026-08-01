package game

import "time"

// ImbuementAction mirrors C++ enum class ImbuementAction : uint8_t
type ImbuementAction uint8

const (
	ImbuementActionOpen     ImbuementAction = 0
	ImbuementActionPickItem ImbuementAction = 1
	ImbuementActionScroll   ImbuementAction = 2
)

// Imbuement types
const (
	ImbuementNone          uint8 = 0
	ImbuementLifeLeech     uint8 = 1
	ImbuementManaLeech     uint8 = 2
	ImbuementCriticalHit   uint8 = 3
	ImbuementHPRegen       uint8 = 4
	ImbuementMPRegen       uint8 = 5
	ImbuementSkillAxe      uint8 = 6
	ImbuementSkillSword    uint8 = 7
	ImbuementSkillClub     uint8 = 8
	ImbuementSkillDistance uint8 = 9
	ImbuementSkillShield   uint8 = 10
	ImbuementSkillFist     uint8 = 11
	ImbuementMagicLevel    uint8 = 12
	ImbuementCapacity      uint8 = 13
	ImbuementFireDamage    uint8 = 14
	ImbuementEnergyDamage  uint8 = 15
	ImbuementIceDamage     uint8 = 16
	ImbuementDeathDamage   uint8 = 17
	ImbuementEarthDamage   uint8 = 18
	ImbuementHolyDamage    uint8 = 19
)

// Imbuement tiers
const (
	ImbuementTierBasic     uint8 = 1
	ImbuementTierIntricate uint8 = 2
	ImbuementTierPowerful  uint8 = 3
)

// ImbuementSlot represents a slot on an item where an imbuement can be applied.
type ImbuementSlot struct {
	ID          uint8
	ImbuementID uint8
	Tier        uint8
	Duration    time.Duration // remaining duration
	IsEmpty     bool
}

// PlayerImbuement manages a player's active imbuements across items.
type PlayerImbuement struct {
	Slots    []ImbuementSlot
	maxSlots uint8
}

// NewPlayerImbuement creates a new imbuement manager with given max slots.
func NewPlayerImbuement(maxSlots uint8) *PlayerImbuement {
	return &PlayerImbuement{
		Slots:    make([]ImbuementSlot, 0, maxSlots),
		maxSlots: maxSlots,
	}
}

// GetImbuementCost returns the gold cost for an imbuement type and tier.
func GetImbuementCost(imbuementID, tier uint8) uint64 {
	// Base costs from C++: Basic = 5k, Intricate = 25k, Powerful = 100k
	switch tier {
	case ImbuementTierBasic:
		return 5000
	case ImbuementTierIntricate:
		return 25000
	case ImbuementTierPowerful:
		return 100000
	default:
		return 0
	}
}

// GetImbuementDuration returns the duration in seconds for a given tier.
func GetImbuementDuration(tier uint8) time.Duration {
	switch tier {
	case ImbuementTierBasic:
		return 20 * time.Hour
	case ImbuementTierIntricate:
		return 40 * time.Hour
	case ImbuementTierPowerful:
		return 80 * time.Hour
	default:
		return 0
	}
}

// GetImbuementName returns the display name for an imbuement type.
func GetImbuementName(imbuementID uint8) string {
	switch imbuementID {
	case ImbuementLifeLeech:
		return "Life Leech"
	case ImbuementManaLeech:
		return "Mana Leech"
	case ImbuementCriticalHit:
		return "Critical Hit"
	case ImbuementHPRegen:
		return "HP Regen"
	case ImbuementMPRegen:
		return "MP Regen"
	case ImbuementSkillAxe:
		return "Axe Fighting"
	case ImbuementSkillSword:
		return "Sword Fighting"
	case ImbuementSkillClub:
		return "Club Fighting"
	case ImbuementSkillDistance:
		return "Distance Fighting"
	case ImbuementSkillShield:
		return "Shield Fighting"
	case ImbuementSkillFist:
		return "Fist Fighting"
	case ImbuementMagicLevel:
		return "Magic Level"
	case ImbuementCapacity:
		return "Capacity"
	case ImbuementFireDamage:
		return "Fire Damage"
	case ImbuementEnergyDamage:
		return "Energy Damage"
	case ImbuementIceDamage:
		return "Ice Damage"
	case ImbuementDeathDamage:
		return "Death Damage"
	case ImbuementEarthDamage:
		return "Earth Damage"
	case ImbuementHolyDamage:
		return "Holy Damage"
	default:
		return "Unknown"
	}
}
