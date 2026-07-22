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
	GetIcons() uint64
	SetTicks(ticks int32)
	SetParam(key int32, value int32)
}

func CreateCondition(id ConditionId, condType ConditionType, ticks int32, subId uint32, isPersistent bool) Condition {
	generic := ConditionGeneric{
		Id:           id,
		Type:         condType,
		Ticks:        ticks,
		SubId:        subId,
		IsPersistent: isPersistent,
	}
	switch condType {
	case ConditionHaste, ConditionParalyze:
		return &ConditionSpeedStruct{
			ConditionGeneric: generic,
		}
	case ConditionPoison, ConditionFire, ConditionEnergy, ConditionBleeding, ConditionFreezing, ConditionDazzled, ConditionCursed:
		return &ConditionDamageStruct{
			ConditionGeneric: generic,
		}
	case ConditionRegeneration:
		return &ConditionRegenerationStruct{
			ConditionGeneric: generic,
		}
	case ConditionAttributes:
		return &ConditionAttributesStruct{
			ConditionGeneric: generic,
			Skills:           make(map[int32]int32),
			Stats:            make(map[int32]int32),
			SkillPercent:     make(map[int32]int32),
			StatPercent:      make(map[int32]int32),
		}
	default:
		return &ConditionGeneric{
			Id:           id,
			Type:         condType,
			Ticks:        ticks,
			SubId:        subId,
			IsPersistent: isPersistent,
		}
	}
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
func (c *ConditionGeneric) SetTicks(ticks int32) { c.Ticks = ticks }

func (c *ConditionGeneric) SetParam(key int32, value int32) {
	switch key {
	case 2: // CONDITION_PARAM_TICKS
		c.Ticks = value
	case 3: // CONDITION_PARAM_SUBID
		c.SubId = uint32(value)
	case 4: // CONDITION_PARAM_BUFF
		c.IsBuff = value != 0
	}
}

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

func (c *ConditionGeneric) GetIcons() uint64 {
	var icons uint64
	switch c.Type {
	case ConditionPoison:
		icons |= 1 << uint64(PlayerIconPoison)
	case ConditionFire:
		icons |= 1 << uint64(PlayerIconBurn)
	case ConditionEnergy:
		icons |= 1 << uint64(PlayerIconEnergy)
	case ConditionBleeding:
		icons |= 1 << uint64(PlayerIconBleeding)
	case ConditionHaste:
		icons |= 1 << uint64(PlayerIconHaste)
	case ConditionParalyze:
		icons |= 1 << uint64(PlayerIconParalyze)
	case ConditionManaShield:
		icons |= 1 << uint64(PlayerIconManaShield)
	case ConditionDrunk:
		icons |= 1 << uint64(PlayerIconDrunk)
	case ConditionFreezing:
		icons |= 1 << uint64(PlayerIconFreezing)
	case ConditionDazzled:
		icons |= 1 << uint64(PlayerIconDazzled)
	case ConditionCursed:
		icons |= 1 << uint64(PlayerIconCursed)
	case ConditionInFight:
		icons |= 1 << uint64(PlayerIconSwords)
	}
	return icons
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

// ConditionSpeedStruct handles haste and paralyze conditions
type ConditionSpeedStruct struct {
	ConditionGeneric
	SpeedDelta int32
	Mina       float32
	Minb       float32
	Maxa       float32
	Maxb       float32
}

func (c *ConditionSpeedStruct) SetFormulaVars(mina, minb, maxa, maxb float32) {
	c.Mina = mina
	c.Minb = minb
	c.Maxa = maxa
	c.Maxb = maxb
}

func (c *ConditionSpeedStruct) getFormulaValues(baseSpeed int32) (int32, int32) {
	difference := baseSpeed - 40
	min := int32(c.Mina*float32(difference) + c.Minb)
	max := int32(c.Maxa*float32(difference) + c.Maxb)
	return min, max
}

func (c *ConditionSpeedStruct) uniformRandom(min, max int32) int32 {
	if min >= max {
		return min
	}
	return max
}

func (c *ConditionSpeedStruct) StartCondition(creature Creature) bool {
	if !c.ConditionGeneric.StartCondition(creature) {
		return false
	}

	if c.SpeedDelta == 0 {
		baseSpeed := creature.GetBaseSpeed()
		min, max := c.getFormulaValues(int32(baseSpeed))
		c.SpeedDelta = c.uniformRandom(min, max) - int32(baseSpeed)
		
		if c.Type == ConditionParalyze && c.SpeedDelta < int32(40-baseSpeed) {
			c.SpeedDelta = int32(40 - baseSpeed)
		}
	}

	creature.ChangeSpeed(c.SpeedDelta)
	return true
}

func (c *ConditionSpeedStruct) EndCondition(creature Creature) {
	creature.ChangeSpeed(-c.SpeedDelta)
}

func (c *ConditionSpeedStruct) AddCondition(creature Creature, condition Condition) {
	if c.Type != condition.GetType() {
		return
	}

	if creature != nil {
		creature.ChangeSpeed(-c.SpeedDelta)
	}

	if c.Ticks < condition.GetTicks() {
		c.Ticks = condition.GetTicks()
		c.EndTime = time.Now().UnixMilli() + int64(c.Ticks)
	}

	if speedCond, ok := condition.(*ConditionSpeedStruct); ok {
		c.SpeedDelta = speedCond.SpeedDelta
		c.Mina = speedCond.Mina
		c.Minb = speedCond.Minb
		c.Maxa = speedCond.Maxa
		c.Maxb = speedCond.Maxb

		if c.SpeedDelta == 0 && creature != nil {
			baseSpeed := creature.GetBaseSpeed()
			min, max := c.getFormulaValues(int32(baseSpeed))
			c.SpeedDelta = c.uniformRandom(min, max) - int32(baseSpeed)

			if c.Type == ConditionParalyze && c.SpeedDelta < int32(40-baseSpeed) {
				c.SpeedDelta = int32(40 - baseSpeed)
			}
		}
	}

	if creature != nil {
		creature.ChangeSpeed(c.SpeedDelta)
	}
}

func (c *ConditionSpeedStruct) Clone() Condition {
	return &ConditionSpeedStruct{
		ConditionGeneric: *c.ConditionGeneric.Clone().(*ConditionGeneric),
		SpeedDelta:       c.SpeedDelta,
		Mina:             c.Mina,
		Minb:             c.Minb,
		Maxa:             c.Maxa,
		Maxb:             c.Maxb,
	}
}

// ConditionAttributesStruct handles skill and stat modifications
type ConditionAttributesStruct struct {
	ConditionGeneric
	Skills       map[int32]int32
	Stats        map[int32]int32
	SkillPercent map[int32]int32
	StatPercent  map[int32]int32
}

func (c *ConditionAttributesStruct) SetParam(key int32, value int32) {
	c.ConditionGeneric.SetParam(key, value)
	switch key {
	case 19, 20, 21, 22, 23, 24, 25, 26: // Skill modifiers
		c.Skills[key] = value
	case 27, 28, 30: // Stat modifiers (health, mana, magic level)
		c.Stats[key] = value
	case 31, 32, 34: // Stat percentages (health, mana, magic level)
		c.StatPercent[key] = value
	case 36, 37, 38, 39, 40, 41, 42, 43: // Skill percentages
		c.SkillPercent[key] = value
	}
}

func (c *ConditionAttributesStruct) StartCondition(creature Creature) bool {
	if !c.ConditionGeneric.StartCondition(creature) {
		return false
	}
	if player, ok := creature.(interface{ NotifyStatsChange() }); ok {
		player.NotifyStatsChange()
	}
	return true
}

func (c *ConditionAttributesStruct) EndCondition(creature Creature) {
	if player, ok := creature.(interface{ NotifyStatsChange() }); ok {
		player.NotifyStatsChange()
	}
}

func (c *ConditionAttributesStruct) AddCondition(creature Creature, condition Condition) {
	if c.Type != condition.GetType() {
		return
	}
	if c.Ticks < condition.GetTicks() {
		c.Ticks = condition.GetTicks()
		c.EndTime = time.Now().UnixMilli() + int64(c.Ticks)
	}
	if attrCond, ok := condition.(*ConditionAttributesStruct); ok {
		for k, v := range attrCond.Skills {
			c.Skills[k] = v
		}
		for k, v := range attrCond.Stats {
			c.Stats[k] = v
		}
		for k, v := range attrCond.SkillPercent {
			c.SkillPercent[k] = v
		}
		for k, v := range attrCond.StatPercent {
			c.StatPercent[k] = v
		}
	}
	if player, ok := creature.(interface{ NotifyStatsChange() }); ok {
		player.NotifyStatsChange()
	}
}

func (c *ConditionAttributesStruct) Clone() Condition {
	clone := &ConditionAttributesStruct{
		ConditionGeneric: *c.ConditionGeneric.Clone().(*ConditionGeneric),
		Skills:           make(map[int32]int32, len(c.Skills)),
		Stats:            make(map[int32]int32, len(c.Stats)),
		SkillPercent:     make(map[int32]int32, len(c.SkillPercent)),
		StatPercent:      make(map[int32]int32, len(c.StatPercent)),
	}
	for k, v := range c.Skills {
		clone.Skills[k] = v
	}
	for k, v := range c.Stats {
		clone.Stats[k] = v
	}
	for k, v := range c.SkillPercent {
		clone.SkillPercent[k] = v
	}
	for k, v := range c.StatPercent {
		clone.StatPercent[k] = v
	}
	return clone
}
