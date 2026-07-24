package game

import (
	"time"

	"github.com/opentibiabr/canary-go/internal/creatures"
)

type ForgeClassification byte

const (
	ForgeClassifications_None       ForgeClassification = 0
	ForgeClassifications_Influenced ForgeClassification = 1
	ForgeClassifications_Fiendish   ForgeClassification = 2
)

type Monster struct {
	BaseCreature
	TargetDistance int32
	Master         Creature
	// CorpseID is the item id dropped on death. 0 means "unknown" and the
	// combat engine falls back to a default. Populated from MonsterType.Corpse.
	CorpseID uint16

	// Type is the shared, immutable monster definition (attacks, loot,
	// experience, flags). May be nil for synthetic/test monsters.
	Type *creatures.MonsterType

	ForgeClassification ForgeClassification
	ForgeStack          uint16
	TimeToChangeFiendish int64
}

func NewMonster(id uint32, name string, mType *creatures.MonsterType) *Monster {
	maxHealth := uint32(100)
	speed := uint32(200)
	outfit := Outfit{}
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

// CanBeForgeMonster checks if the monster is eligible to become influenced or fiendish.
func (m *Monster) CanBeForgeMonster() bool {
	return m.ForgeStack == 0 && m.Type != nil && m.Type.RaceID > 0
}

// ApplyStacks scales health based on forge stack level.
func (m *Monster) ApplyStacks() {
	if m.ForgeStack == 0 {
		return
	}
	percent := 1.0 + float64(15*m.ForgeStack+35)/100.0
	newMax := uint32(float64(m.MaxHealth) * percent)
	if newMax == 0 {
		newMax = 100
	}
	m.MaxHealth = newMax
	m.Health = newMax
}

// ConfigureForgeSystem sets up stacks and HP scaling for Influenced or Fiendish monsters.
func (m *Monster) ConfigureForgeSystem(stack uint16) {
	if m.ForgeClassification == ForgeClassifications_Fiendish {
		m.ForgeStack = 15
	} else if m.ForgeClassification == ForgeClassifications_Influenced {
		if stack == 0 {
			stack = uint16(1 + (time.Now().UnixNano() % 5))
		}
		m.ForgeStack = stack
	}
	m.ApplyStacks()
}

// ClearFiendishStatus resets fiendish classification and stacks.
func (m *Monster) ClearFiendishStatus() {
	m.ForgeClassification = ForgeClassifications_None
	m.ForgeStack = 0
	m.TimeToChangeFiendish = 0
}
