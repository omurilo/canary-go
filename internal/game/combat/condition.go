package combat

import (
	"time"
)

// Condition interface represents a condition applied to a creature
type Condition interface {
	GetId() ConditionId
	GetType() ConditionType
	GetTicks() int32
	GetEndTime() int64
	StartCondition(creature Creature) bool
	ExecuteCondition(creature Creature, interval int32) bool
	EndCondition(creature Creature)
	AddCondition(creature Creature, condition Condition)
	Clone() Condition
}

// ConditionGeneric is a basic condition implementation
type ConditionGeneric struct {
	Id           ConditionId
	Type         ConditionType
	Ticks        int32
	EndTime      int64
	SubId        uint32
	IsBuff       bool
	IsPersistent bool
}

func (c *ConditionGeneric) GetId() ConditionId { return c.Id }
func (c *ConditionGeneric) GetType() ConditionType { return c.Type }
func (c *ConditionGeneric) GetTicks() int32 { return c.Ticks }
func (c *ConditionGeneric) GetEndTime() int64 { return c.EndTime }

func (c *ConditionGeneric) StartCondition(creature Creature) bool {
	c.EndTime = time.Now().UnixMilli() + int64(c.Ticks)
	return true
}

func (c *ConditionGeneric) ExecuteCondition(creature Creature, interval int32) bool {
	c.Ticks -= interval
	if c.Ticks <= 0 {
		return false
	}
	return true
}

func (c *ConditionGeneric) EndCondition(creature Creature) {
	// Implementation to revert stats or buffs if needed
}

func (c *ConditionGeneric) AddCondition(creature Creature, condition Condition) {
	if c.Ticks < condition.GetTicks() {
		c.Ticks = condition.GetTicks()
		c.EndTime = time.Now().UnixMilli() + int64(c.Ticks)
	}
}

func (c *ConditionGeneric) Clone() Condition {
	return &ConditionGeneric{
		Id:           c.Id,
		Type:         c.Type,
		Ticks:        c.Ticks,
		EndTime:      c.EndTime,
		SubId:        c.SubId,
		IsBuff:       c.IsBuff,
		IsPersistent: c.IsPersistent,
	}
}

// ConditionDamageStruct applies damage over time (e.g. poison, fire, energy)
type ConditionDamageStruct struct {
	ConditionGeneric
	MaxDamage     int32
	MinDamage     int32
	StartDamage   int32
	PeriodDamage  int32
	TickInterval  int32
	DamageList    []IntervalInfo
	DamageTicks   int32
}

func (c *ConditionDamageStruct) StartCondition(creature Creature) bool {
	return c.ConditionGeneric.StartCondition(creature)
}

func (c *ConditionDamageStruct) ExecuteCondition(creature Creature, interval int32) bool {
	c.DamageTicks += interval
	if c.DamageTicks >= c.TickInterval {
		if len(c.DamageList) > 0 {
			info := c.DamageList[0]
			c.DamageList = c.DamageList[1:]
			creature.ChangeHealth(-info.Damage)
			c.DamageTicks = 0
		} else if c.PeriodDamage > 0 {
			creature.ChangeHealth(-c.PeriodDamage)
			c.DamageTicks = 0
		}
	}
	return c.ConditionGeneric.ExecuteCondition(creature, interval)
}

func (c *ConditionDamageStruct) AddDamage(rounds int32, ticks int32, value int32) {
	for i := int32(0); i < rounds; i++ {
		c.DamageList = append(c.DamageList, IntervalInfo{Damage: value, Ticks: ticks})
		c.Ticks += ticks
	}
	c.TickInterval = ticks
}

func (c *ConditionDamageStruct) Clone() Condition {
	clone := &ConditionDamageStruct{
		ConditionGeneric: *c.ConditionGeneric.Clone().(*ConditionGeneric),
		MaxDamage:     c.MaxDamage,
		MinDamage:     c.MinDamage,
		StartDamage:   c.StartDamage,
		PeriodDamage:  c.PeriodDamage,
		TickInterval:  c.TickInterval,
		DamageList:    make([]IntervalInfo, len(c.DamageList)),
		DamageTicks:   c.DamageTicks,
	}
	copy(clone.DamageList, c.DamageList)
	return clone
}

// ConditionRegenerationStruct handles health/mana regen (e.g. food)
type ConditionRegenerationStruct struct {
	ConditionGeneric
	InternalHealthTicks uint32
	InternalManaTicks   uint32
	HealthTicks         uint32
	ManaTicks           uint32
	HealthGain          uint32
	ManaGain            uint32
}

func (c *ConditionRegenerationStruct) ExecuteCondition(creature Creature, interval int32) bool {
	c.InternalHealthTicks += uint32(interval)
	if c.InternalHealthTicks >= c.HealthTicks {
		creature.ChangeHealth(int32(c.HealthGain))
		c.InternalHealthTicks = 0
	}

	c.InternalManaTicks += uint32(interval)
	if c.InternalManaTicks >= c.ManaTicks {
		creature.ChangeMana(int32(c.ManaGain))
		c.InternalManaTicks = 0
	}

	return c.ConditionGeneric.ExecuteCondition(creature, interval)
}

func (c *ConditionRegenerationStruct) Clone() Condition {
	return &ConditionRegenerationStruct{
		ConditionGeneric: *c.ConditionGeneric.Clone().(*ConditionGeneric),
		InternalHealthTicks: c.InternalHealthTicks,
		InternalManaTicks:   c.InternalManaTicks,
		HealthTicks:         c.HealthTicks,
		ManaTicks:           c.ManaTicks,
		HealthGain:          c.HealthGain,
		ManaGain:            c.ManaGain,
	}
}
