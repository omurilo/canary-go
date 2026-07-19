package game

type Creature interface {
	GetID() uint32
	GetName() string
	GetHealth() uint32
	SetHealth(health uint32)
	GetMaxHealth() uint32
	AddHealth(amount int32)
	GetTarget() Creature
	SetTarget(target Creature)
	ChangeTargetDistance(distance int32)
}

type BaseCreature struct {
	ID        uint32
	Name      string
	Health    uint32
	MaxHealth uint32
	Target    Creature
}

func (c *BaseCreature) GetID() uint32 { return c.ID }
func (c *BaseCreature) GetName() string { return c.Name }
func (c *BaseCreature) GetHealth() uint32 { return c.Health }
func (c *BaseCreature) SetHealth(health uint32) {
	c.Health = health
	if c.Health > c.MaxHealth {
		c.Health = c.MaxHealth
	}
}
func (c *BaseCreature) GetMaxHealth() uint32 { return c.MaxHealth }
func (c *BaseCreature) AddHealth(amount int32) {
	if amount > 0 {
		c.Health += uint32(amount)
		if c.Health > c.MaxHealth {
			c.Health = c.MaxHealth
		}
	} else {
		sub := uint32(-amount)
		if sub > c.Health {
			c.Health = 0
		} else {
			c.Health -= sub
		}
	}
}
func (c *BaseCreature) GetTarget() Creature { return c.Target }
func (c *BaseCreature) SetTarget(target Creature) { c.Target = target }
func (c *BaseCreature) ChangeTargetDistance(distance int32) {
	// Logic to change target distance
}
