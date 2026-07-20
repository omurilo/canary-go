package combat

// CombatParams represents the configuration for a combat
type CombatParams struct {
	ConditionList []Condition
	CombatType    CombatType
	Origin        CombatOrigin
	DispelType    ConditionType

	ImpactEffect   uint16
	DistanceEffect uint16
	ChainEffect    uint16

	BlockedByArmor        bool
	BlockedByShield       bool
	TargetCasterOrTopMost bool
	Aggressive            bool
	UseCharges            bool
}

// Combat engine struct
type Combat struct {
	Params CombatParams

	FormulaType FormulaType
	MinA, MinB  float64
	MaxA, MaxB  float64

	// Area holds the resolved combat area (nil for single-target spells),
	// mirroring Combat::area (src/creatures/combat/combat.hpp).
	Area *AreaCombat

	// InstantSpellName mirrors Combat::setInstantSpellName; captured for parity
	// with the C++ combat:execute path but not otherwise consumed yet.
	InstantSpellName string
}

// SetArea assigns the combat area. Mirrors Combat::setArea (combat.cpp).
func (c *Combat) SetArea(a *AreaCombat) { c.Area = a }

// HasArea reports whether this combat targets an area, mirroring
// Combat::hasArea (src/creatures/combat/combat.hpp).
func (c *Combat) HasArea() bool { return c.Area != nil }

// CombatType returns the configured primary combat type.
func (c *Combat) CombatType() CombatType { return c.Params.CombatType }

// IsManaDrain reports whether the combat drains mana rather than health,
// matching the COMBAT_MANADRAIN branch of Combat::doCombat (combat.cpp:1350).
func (c *Combat) IsManaDrain() bool { return c.Params.CombatType == CombatManaDrain }

// NewCombat creates a new combat instance
func NewCombat() *Combat {
	return &Combat{
		Params: CombatParams{
			Aggressive: true,
		},
	}
}

// DoCombatHealth applies combat damage or healing to health
func (c *Combat) DoCombatHealth(caster Creature, target Creature, damage CombatDamage) bool {
	if c.Params.Aggressive && !CanDoCombat(caster, target) {
		return false
	}

	// Apply dispel if configured
	if c.Params.DispelType != ConditionNone {
		target.RemoveCondition(c.Params.DispelType)
	}

	// Calculate and apply armor/shield blocking here if needed

	finalDamage := damage.PrimaryValue

	// If it's damage
	if damage.PrimaryType != CombatHealing {
		finalDamage = -finalDamage
	}

	if finalDamage != 0 {
		target.ChangeHealth(finalDamage)
	}

	// Apply conditions
	for _, cond := range c.Params.ConditionList {
		t := cond.GetType()
		if t == ConditionPoison || t == ConditionFire || t == ConditionEnergy || t == ConditionBleeding || t == ConditionFreezing || t == ConditionDazzled || t == ConditionCursed {
			// Add the damage condition
			target.AddCondition(cond.Clone())
		} else {
			target.AddCondition(cond.Clone())
		}
	}

	return true
}

// DoCombatMana applies combat damage to mana
func (c *Combat) DoCombatMana(caster Creature, target Creature, damage CombatDamage) bool {
	if c.Params.Aggressive && !CanDoCombat(caster, target) {
		return false
	}

	finalDamage := damage.PrimaryValue
	if damage.PrimaryType != CombatHealing {
		finalDamage = -finalDamage
	}

	if finalDamage != 0 {
		target.ChangeMana(finalDamage)
	}

	return true
}

// DoCombatCondition applies only conditions to the target
func (c *Combat) DoCombatCondition(caster Creature, target Creature) bool {
	if c.Params.Aggressive && !CanDoCombat(caster, target) {
		return false
	}

	for _, cond := range c.Params.ConditionList {
		target.AddCondition(cond.Clone())
	}

	return true
}

// CanDoCombat checks if combat is allowed between caster and target (e.g. PVP zone checks)
func CanDoCombat(caster Creature, target Creature) bool {
	if caster == nil || target == nil {
		return false
	}
	// Add protection zone or other checks here
	return true
}

// RollValue computes the (signed) primary combat value for a player-cast spell,
// mirroring Combat::getCombatDamage (src/creatures/combat/combat.cpp:52) and
// Combat::getLevelFormula (combat.cpp:33) for the two formulas instant spells
// use. Damage spells have negative coefficients (value < 0); healing spells have
// positive coefficients (value > 0). normal_random is approximated by a uniform
// draw over the [min,max] range, matching the existing melee/distance formulas.
//
// LEVELMAGIC: levelFormula = level*2 + magicLevel*3;
//
//	value = rand(levelFormula*minA + minB, levelFormula*maxA + maxB)
//
// DAMAGE:     value = rand(minA, maxA)
func (c *Combat) RollValue(level, magicLevel int) int32 {
	var lo, hi int
	switch c.FormulaType {
	case CombatFormulaLevelMagic:
		levelFormula := level*2 + magicLevel*3
		lo = int(float64(levelFormula)*c.MinA + c.MinB)
		hi = int(float64(levelFormula)*c.MaxA + c.MaxB)
	case CombatFormulaDamage:
		lo = int(c.MinA)
		hi = int(c.MaxA)
	default:
		return 0
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	if hi == lo {
		return int32(lo)
	}
	return int32(lo + randInt(hi-lo+1))
}

// SetPlayerCombatValues sets the minimum and maximum damage formula values
func (c *Combat) SetPlayerCombatValues(formulaType FormulaType, minA, minB, maxA, maxB float64) {
	c.FormulaType = formulaType
	c.MinA = minA
	c.MinB = minB
	c.MaxA = maxA
	c.MaxB = maxB
}

// AddCondition adds a condition to be applied on combat hit
func (c *Combat) AddCondition(cond Condition) {
	c.Params.ConditionList = append(c.Params.ConditionList, cond)
}

// SetParam sets a param for the combat
func (c *Combat) SetParam(param CombatParam, value uint32) {
	switch param {
	case CombatParamType:
		c.Params.CombatType = CombatType(value)
	case CombatParamEffect:
		c.Params.ImpactEffect = uint16(value)
	case CombatParamDistanceEffect:
		c.Params.DistanceEffect = uint16(value)
	case CombatParamBlockArmor:
		c.Params.BlockedByArmor = value != 0
	case CombatParamBlockShield:
		c.Params.BlockedByShield = value != 0
	case CombatParamAggressive:
		c.Params.Aggressive = value != 0
	case CombatParamDispel:
		c.Params.DispelType = ConditionType(value)
	}
}
