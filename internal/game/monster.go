package game

type Monster struct {
	BaseCreature
	TargetDistance int32
}

func NewMonster(id uint32, name string, maxHealth uint32) *Monster {
	return &Monster{
		BaseCreature: BaseCreature{
			ID:        id,
			Name:      name,
			Health:    maxHealth,
			MaxHealth: maxHealth,
		},
	}
}

func (m *Monster) ChangeTargetDistance(distance int32) {
	m.TargetDistance = distance
}
