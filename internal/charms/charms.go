// Package charms models the bestiary charm-rune system: the charm definitions
// (loaded from the datapack), the per-charm tier/cost tables, the unlocked /
// assigned bitmasks, and the offensive on-hit damage formula. It mirrors the
// C++ Charm class (src/io/iobestiary.hpp) and IOBestiary charm logic.
package charms

import "math"

// Category mirrors charmCategory_t (creatures_definitions.hpp).
const (
	CategoryAll   uint8 = 0
	CategoryMajor uint8 = 1
	CategoryMinor uint8 = 2
)

// Type mirrors charm_t.
const (
	TypeUndefined uint8 = 0
	TypeOffensive uint8 = 1
	TypeDefensive uint8 = 2
	TypePassive   uint8 = 3
)

// charmRune_t ids (creatures_definitions.hpp). CharmNone is -1 in C++; here the
// registry only ever holds ids >= 0, so a plain uint8 suffices for entries.
const (
	Wound        uint8 = 0
	Enflame      uint8 = 1
	Poison       uint8 = 2
	Freeze       uint8 = 3
	Zap          uint8 = 4
	Curse        uint8 = 5
	Cripple      uint8 = 6
	Parry        uint8 = 7
	Dodge        uint8 = 8
	Adrenaline   uint8 = 9
	Numb         uint8 = 10
	Cleanse      uint8 = 11
	Bless        uint8 = 12
	Scavenge     uint8 = 13
	Gut          uint8 = 14
	Low          uint8 = 15
	Divine       uint8 = 16
	Vamp         uint8 = 17
	Void         uint8 = 18
	Savage       uint8 = 19
	Fatal        uint8 = 20
	VoidInversion uint8 = 21
	Carnage      uint8 = 22
	Overpower    uint8 = 23
	Overflux     uint8 = 24
)

// Charm is a single charm-rune definition. Mirrors the C++ Charm class.
type Charm struct {
	ID          uint8
	Name        string
	Description string
	Category    uint8
	Type        uint8
	DamageType  int     // COMBAT_* value from the datapack (creatures_definitions.hpp)
	Percent     float64 // percent of the reference health/mana used as damage
	Chance      [3]uint16
	Points      [3]uint16 // charm-point (major) or minor-echo (minor) cost per tier
	Effect      uint16
	CastSound   uint16
	ImpactSound uint16
	MessageCancel    string
	MessageServerLog bool
	// Binary is the bitmask for this charm in the unlocked/used rune bitsets.
	// C++ sets binary = 1 << id.
	Binary int32
}

// Registry holds all charm definitions in id order.
type Registry struct {
	List []*Charm
	ByID map[uint8]*Charm
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{ByID: make(map[uint8]*Charm)}
}

// Add inserts or replaces a charm, keeping List sorted by id (as the datapack
// registers ids 0..24 in ascending order via ipairs).
func (r *Registry) Add(c *Charm) {
	c.Binary = int32(1) << c.ID
	if existing, ok := r.ByID[c.ID]; ok {
		*existing = *c
		return
	}
	r.ByID[c.ID] = c
	// insert keeping List ordered by id
	i := 0
	for i < len(r.List) && r.List[i].ID < c.ID {
		i++
	}
	r.List = append(r.List, nil)
	copy(r.List[i+1:], r.List[i:])
	r.List[i] = c
}

// Get returns the charm by id, or nil.
func (r *Registry) Get(id uint8) *Charm {
	if r == nil {
		return nil
	}
	return r.ByID[id]
}

// Len returns the number of charms.
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.List)
}

// HasBit reports whether charm id's bit is set in the rune bitset.
// Mirrors IOBestiary::hasCharmUnlockedRuneBit with binary = 1 << id.
func HasBit(bits int32, id uint8) bool {
	return bits&(int32(1)<<id) != 0
}

// SetBit returns bits with charm id's bit turned on (bitToggle on).
func SetBit(bits int32, id uint8) int32 {
	return bits | (int32(1) << id)
}

// ClearBit returns bits with charm id's bit turned off (bitToggle off).
func ClearBit(bits int32, id uint8) int32 {
	return bits &^ (int32(1) << id)
}

// UsedRunes returns the ids of all charms whose bit is set, ascending.
// Mirrors IOBestiary::getCharmUsedRuneBitAll.
func UsedRunes(bits int32) []uint8 {
	var out []uint8
	for i := range uint8(32) {
		if bits&(int32(1)<<i) != 0 {
			out = append(out, i)
		}
	}
	return out
}

// OffensiveDamage returns the (positive) damage for an elemental on-hit charm
// (Wound/Enflame/Poison/Freeze/Zap/Curse/Divine): the lesser of 2x the player
// level and percent% of the target's max health. Mirrors
// IOBestiary::parseOffensiveCharmCombat's default case.
func (c *Charm) OffensiveDamage(playerLevel uint32, targetMaxHealth uint32) int32 {
	const maxLevelsLimit = 2.0
	byLevel := math.Ceil(float64(playerLevel) * maxLevelsLimit)
	byHealth := math.Ceil(float64(targetMaxHealth) * (c.Percent / 100.0))
	v := byLevel
	if byHealth < v {
		v = byHealth
	}
	if v < 0 {
		v = 0
	}
	return int32(v)
}

// UnlockCostReached reports whether the charm is at max tier (tier > 2).
func (c *Charm) IsMaxTier(tier uint8) bool { return tier > 2 }

// TierCost returns the point/echo cost to unlock the given tier (0-based).
func (c *Charm) TierCost(tier uint8) uint16 {
	if int(tier) >= len(c.Points) {
		return 0
	}
	return c.Points[tier]
}

// MinorEchoesGain returns the minor-charm-echo reward for unlocking a MAJOR
// charm at the given (pre-increment) tier: 25*t^2 + 25*t + 50.
func MinorEchoesGain(tier uint8) uint32 {
	t := uint32(tier)
	return 25*t*t + 25*t + 50
}
