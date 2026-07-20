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
	GetPosition() Position
	SetPosition(pos Position)
	GetDirection() Direction
	SetDirection(dir Direction)
	GetOutfit() Outfit
	GetLightLevel() uint8
	GetLightColor() uint8
	GetSpeed() uint16
	GetCreatureType() uint8 // 0=Player, 1=Monster, 2=NPC
}

type BaseCreature struct {
	conditionStore

	World     *World
	ID        uint32
	Name      string
	Health    uint32
	MaxHealth uint32
	Mana      uint32
	MaxMana   uint32
	Target     Creature
	Pos        Position
	Direction  Direction
	Outfit     Outfit
	LightLevel uint8
	LightColor uint8
	Speed      uint16
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
func (c *BaseCreature) GetPosition() Position { return c.Pos }
func (c *BaseCreature) SetPosition(pos Position) { c.Pos = pos }
func (c *BaseCreature) GetDirection() Direction { return c.Direction }
func (c *BaseCreature) SetDirection(dir Direction) { c.Direction = dir }
func (c *BaseCreature) GetOutfit() Outfit { return c.Outfit }
func (c *BaseCreature) GetLightLevel() uint8 { return c.LightLevel }
func (c *BaseCreature) GetLightColor() uint8 { return c.LightColor }
func (c *BaseCreature) GetSpeed() uint16 { return c.Speed }
func (c *BaseCreature) GetCreatureType() uint8 { return 0 } // Player by default

// GetMana/GetMaxMana/AddMana provide the mana accessors the combat adapter
// needs. Monsters/NPCs default to zero mana; drainMana clamps at 0 like
// Creature::changeMana in src/creatures/creature.cpp.
func (c *BaseCreature) GetMana() uint32    { return c.Mana }
func (c *BaseCreature) GetMaxMana() uint32 { return c.MaxMana }
func (c *BaseCreature) AddMana(amount int32) {
	if amount > 0 {
		c.Mana += uint32(amount)
		if c.Mana > c.MaxMana {
			c.Mana = c.MaxMana
		}
	} else {
		sub := uint32(-amount)
		if sub > c.Mana {
			c.Mana = 0
		} else {
			c.Mana -= sub
		}
	}
}

