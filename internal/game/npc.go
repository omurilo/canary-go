package game

type Npc struct {
	BaseCreature
}

func NewNpc(id uint32, name string) *Npc {
	return &Npc{
		BaseCreature: BaseCreature{
			ID:   id,
			Name: name,
		},
	}
}

func (n *Npc) Say(text string) {
	// Logic for NPC to say something
}

func (n *Npc) TurnToCreature(c Creature) {
	// Logic for NPC to turn to a creature
}
