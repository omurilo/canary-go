package game

import "github.com/opentibiabr/canary-go/internal/game/combat"

type Creature interface {
	GetID() uint32
	GetName() string
	GetHealth() uint32
	SetHealth(health uint32)
	GetMaxHealth() uint32
	AddHealth(amount int32)
	GetTarget() Creature
	SetTarget(target Creature)
	GetPosition() Position
	SetPosition(pos Position)
	GetDirection() Direction
	SetDirection(dir Direction)
	GetOutfit() Outfit
	GetLightLevel() uint8
	GetLightColor() uint8
	GetSpeed() uint16
	GetBaseSpeed() uint16
	ChangeSpeed(delta int32)
	GetArmor() int32
	GetDefense() int32
	GetCreatureType() uint8 // 0=Player, 1=Monster, 2=NPC
}

type BaseCreature struct {
	conditionStore
	damageTracker

	World      *World
	ID         uint32
	Name       string
	Health     uint32
	MaxHealth  uint32
	Mana       uint32
	MaxMana    uint32
	Target     Creature
	Pos        Position
	Direction  Direction
	Outfit     Outfit
	LightLevel uint8
	LightColor uint8
	Speed      uint16
	// OTCR attached effects
	Shader          string
	AttachedEffects []uint16
}

func (c *BaseCreature) GetShader() string            { return c.Shader }
func (c *BaseCreature) GetAttachedEffects() []uint16 { return c.AttachedEffects }
func (c *BaseCreature) GetID() uint32                { return c.ID }
func (c *BaseCreature) GetName() string              { return c.Name }
func (c *BaseCreature) GetHealth() uint32            { return c.Health }
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
func (c *BaseCreature) GetTarget() Creature        { return c.Target }
func (c *BaseCreature) SetTarget(target Creature)  { c.Target = target }
func (c *BaseCreature) GetPosition() Position      { return c.Pos }
func (c *BaseCreature) SetPosition(pos Position)   { c.Pos = pos }
func (c *BaseCreature) GetDirection() Direction    { return c.Direction }
func (c *BaseCreature) SetDirection(dir Direction) { c.Direction = dir }
func (c *BaseCreature) GetOutfit() Outfit          { return c.Outfit }
func (c *BaseCreature) GetLightLevel() uint8       { return c.LightLevel }
func (c *BaseCreature) GetLightColor() uint8       { return c.LightColor }
func (c *BaseCreature) GetSpeed() uint16           { return c.Speed }
func (c *BaseCreature) GetBaseSpeed() uint16       { return c.Speed }
func (c *BaseCreature) ChangeSpeed(delta int32) {
	// Base creatures don't have SpeedBonus logic yet, just adjust Speed directly
	speed := int32(c.Speed) + delta
	if speed < 0 {
		speed = 0
	}
	if speed > 0xFFFF {
		speed = 0xFFFF
	}
	c.Speed = uint16(speed)
}
func (c *BaseCreature) GetCreatureType() uint8 { return 0 } // Player by default

func (c *BaseCreature) GetArmor() int32   { return 0 }
func (c *BaseCreature) GetDefense() int32 { return 0 }

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

func (c *BaseCreature) GetWorld() *World { return c.World }

func (c *BaseCreature) AddCondition(cond combat.Condition) {
	c.conditionStore.AddCondition(adaptCreature(c), cond)
}

func (c *BaseCreature) TickConditions(interval int32) {
	c.conditionStore.ExecuteConditions(adaptCreature(c), interval)
}
