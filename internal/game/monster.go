package game

import "github.com/opentibiabr/canary-go/internal/creatures"

type Monster struct {
	BaseCreature
	TargetDistance int32
	// CorpseID is the item id dropped on death. 0 means "unknown" and the
	// combat engine falls back to a default.
	// TODO(monster-data): populate from MonsterType.Corpse (monster.corpse in
	// the Lua/XML definition).
	CorpseID uint16
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
	}
}

func (m *Monster) ChangeTargetDistance(distance int32) {
	m.TargetDistance = distance
}

func (m *Monster) GetCreatureType() uint8 { return 1 } // CREATURETYPE_MONSTER
