package game

import "github.com/opentibiabr/canary-go/internal/creatures"

type Monster struct {
	BaseCreature
	TargetDistance int32
}

func NewMonster(id uint32, name string, mType *creatures.MonsterType) *Monster {
	maxHealth := uint32(100)
	speed := uint32(200)
	outfit := Outfit{LookType: 21}

	if mType != nil {
		maxHealth = mType.MaxHealth
		speed = mType.Speed
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
	}
}

func (m *Monster) ChangeTargetDistance(distance int32) {
	m.TargetDistance = distance
}

func (m *Monster) GetCreatureType() uint8 { return 1 } // CREATURETYPE_MONSTER
