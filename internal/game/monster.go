package game

import (
	"time"

	"github.com/opentibiabr/canary-go/internal/creatures"
)

type Monster struct {
	BaseCreature
	TargetDistance int32
	// CorpseID is the item id dropped on death. 0 means "unknown" and the
	// combat engine falls back to a default. Populated from MonsterType.Corpse.
	CorpseID uint16

	// Type is the shared, immutable monster definition (attacks, loot,
	// experience, flags). May be nil for synthetic/test monsters.
	Type *creatures.MonsterType
}

func NewMonster(id uint32, name string, mType *creatures.MonsterType) *Monster {
	maxHealth := uint32(100)
	speed := uint32(200)
	outfit := Outfit{LookType: 21}
	corpse := uint16(0)

	if mType != nil {
		maxHealth = mType.MaxHealth
		speed = mType.Speed
		corpse = mType.Corpse
		outfit = Outfit{
			LookType:  mType.Outfit.LookType,
			Head:      mType.Outfit.Head,
			Body:      mType.Outfit.Body,
			Legs:      mType.Outfit.Legs,
			Feet:      mType.Outfit.Feet,
			Addons:    mType.Outfit.Addons,
			LookMount: mType.Outfit.LookMount,
		}
	}

	return &Monster{
		BaseCreature: BaseCreature{
			ID:        id,
			Name:      name,
			Health:    maxHealth,
			MaxHealth: maxHealth,
			Speed:     uint16(speed),
			Outfit:    outfit,
		},
		CorpseID: corpse,
		Type:     mType,
	}
}

func (m *Monster) ChangeTargetDistance(distance int32) {
	m.TargetDistance = distance
}

func (m *Monster) GetCreatureType() uint8 { return 1 } // CREATURETYPE_MONSTER

// MeleeAttack returns the monster's basic melee attack block, or nil if it has
// none. Mirrors selecting the name=="melee" spellBlock in Monster::doAttacking
// (src/creatures/monsters/monster.cpp:1753).
func (m *Monster) MeleeAttack() *creatures.MonsterAttack {
	if m.Type == nil {
		return nil
	}
	for i := range m.Type.Attacks {
		if m.Type.Attacks[i].IsMelee() {
			return &m.Type.Attacks[i]
		}
	}
	return nil
}

// AttackInterval is the cadence of the monster's melee attack (ms). Falls back
// to the MonsterType default of 2000ms (src/creatures/monsters/monsters.hpp).
func (m *Monster) AttackInterval() time.Duration {
	if atk := m.MeleeAttack(); atk != nil && atk.Interval > 0 {
		return time.Duration(atk.Interval) * time.Millisecond
	}
	return defaultMonsterAttackSpeed
}

// Experience is the exp awarded to the killer.
func (m *Monster) Experience() uint64 {
	if m.Type == nil {
		return 0
	}
	return m.Type.Experience
}
