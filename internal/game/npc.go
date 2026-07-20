package game

import "github.com/opentibiabr/canary-go/internal/creatures"

type Npc struct {
	BaseCreature
}

func NewNpc(id uint32, name string, nType *creatures.NpcType) *Npc {
	maxHealth := uint32(100)
	speed := uint32(100)
	outfit := Outfit{}

	if nType != nil {
		maxHealth = nType.MaxHealth
		speed = nType.Speed
		outfit = Outfit{
			LookType:  nType.Outfit.LookType,
			Head:      nType.Outfit.Head,
			Body:      nType.Outfit.Body,
			Legs:      nType.Outfit.Legs,
			Feet:      nType.Outfit.Feet,
			Addons:    nType.Outfit.Addons,
			LookMount: nType.Outfit.LookMount,
		}
	}

	return &Npc{
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

func (n *Npc) Say(text string) {
	// Logic for NPC to say something
}

func (n *Npc) TurnToCreature(c Creature) {
	// Logic for NPC to turn to a creature
}

func (n *Npc) GetCreatureType() uint8 { return 2 } // CREATURETYPE_NPC
