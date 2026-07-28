package game

import (
	"math/rand"
	"time"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game/combat"
)

// ---------------------------------------------------------------------------
// MonsterSpellTarget indicates who the spell targets.
// ---------------------------------------------------------------------------

// MonsterSpellTarget represents the target type for a monster spell.
type MonsterSpellTarget uint8

const (
	SpellTargetEnemy  MonsterSpellTarget = iota // damages/conditions an enemy
	SpellTargetSelf                              // heals / buffs the caster
	SpellTargetFriend                            // heals / buffs an ally (future use)
)

// ---------------------------------------------------------------------------
// MonsterSpellCondition defines a condition (poison, burn, etc.) attached to a
// monster spell. Mirrors MonsterSpell::condition in TFS.
// ---------------------------------------------------------------------------

// MonsterSpellCondition holds condition parameters for monster spell effects.
type MonsterSpellCondition struct {
	Type       combat.ConditionType
	MinDamage  int32
	MaxDamage  int32
	DurationMs uint32
	Chance     uint8 // 0-100 chance the condition lands
}

// ---------------------------------------------------------------------------
// MonsterSpell defines a spell a monster can cast in the AI engine.
// It is the AI-facing representation; for the combat-engine path the original
// creatures.MonsterAttack is used instead.
// ---------------------------------------------------------------------------

// MonsterSpell defines a spell a monster can cast.
type MonsterSpell struct {
	Name      string
	Chance    uint8 // percentage 0-100
	Range     uint8 // maximum distance to target
	MinDamage int32
	MaxDamage int32

	// Spell classification flags
	IsHealing     bool // if true, Min/MaxDamage are positive heal amounts
	IsMelee       bool // replaces normal melee attack animation
	IsCombatSpell bool // a regular damage-dealing spell
	IsSpeech      bool // purely cosmetic speech (no combat effect)

	// Combat parameters
	CombatType combat.CombatType // physical, fire, energy, etc.
	Length     uint8             // wave length (0 = single-target)
	Spread     uint8             // wave spread (0 = single-target)
	Target     MonsterSpellTarget // enemy / self / friend

	// Optional condition to apply alongside the damage
	Condition *MonsterSpellCondition

	// Lua script override for complex spells
	ScriptName string

	// Cooldown controls how often this spell may fire (ms).
	// 0 means no cooldown (fire every AI tick).
	CooldownMs int64

	// Internal state -- updated at runtime on each Monster instance.
	// lastUsedMs records the UnixMilli timestamp of the last cast.
	lastUsedMs int64
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

// MonsterSpellFromAttack converts a creatures.MonsterAttack into a MonsterSpell
// usable by the AI engine. Fields that have no analogue in MonsterAttack are
// set to sensible defaults.
func MonsterSpellFromAttack(a creatures.MonsterAttack) MonsterSpell {
	s := MonsterSpell{
		Name:          a.Name,
		Chance:        uint8(clampInt(a.Chance, 0, 100)),
		Range:         uint8(clampInt(a.Range, 0, 255)),
		MinDamage:     int32(a.MinDamage),
		MaxDamage:     int32(a.MaxDamage),
		IsCombatSpell: true,
		Target:        SpellTargetEnemy,
		CooldownMs:    int64(a.Interval),
		Length:        uint8(clampInt(a.Length, 0, 255)),
		Spread:        uint8(clampInt(a.Spread, 0, 255)),
		ScriptName:    a.ScriptName,
		CombatType:    stringToCombatType(a.CombatType, a.Name),
		IsMelee:       a.IsMelee(),
	}

	// Detect healing spells (positive damage values)
	if s.MinDamage > 0 && s.MaxDamage > 0 {
		s.IsHealing = true
		s.IsCombatSpell = false
		s.Target = SpellTargetSelf
	}

	// Condition
	if a.ConditionType != "" {
		s.Condition = &MonsterSpellCondition{
			Type:       parseConditionType(a.ConditionType),
			MinDamage:  int32(a.ConditionDamage),
			MaxDamage:  int32(a.ConditionDamage),
			DurationMs: uint32(clampInt(a.Duration, 0, 99999999)),
			Chance:     100,
		}
	}

	return s
}

// ---------------------------------------------------------------------------
// AI spell casting logic
// ---------------------------------------------------------------------------

// CastMonsterSpell attempts to cast a monster spell on the monster's current
// target. Returns true if the spell was fired.
//
// The method:
//  1. Verifies the monster has a target.
//  2. Checks range (Chebyshev distance).
//  3. Rolls the chance.
//  4. Checks the per-spell cooldown.
//  5. Applies healing or damage through the combat engine.
//  6. Broadcasts a magic effect.
func (e *AIEngine) CastMonsterSpell(monster *Monster, spell *MonsterSpell) bool {
	if monster == nil {
		return false
	}
	target := monster.GetTarget()
	if target == nil {
		return false
	}

	// --- range check ---
	if spell.Range > 0 {
		dist := chebyshevDistance(monster.GetPosition(), target.GetPosition())
		if dist > int(spell.Range) {
			return false
		}
	}

	// --- chance roll ---
	if spell.Chance < 100 && rand.Intn(100) >= int(spell.Chance) {
		return false
	}

	// --- cooldown check ---
	if spell.CooldownMs > 0 {
		now := time.Now().UnixMilli()
		if now-spell.lastUsedMs < spell.CooldownMs {
			return false
		}
		spell.lastUsedMs = now
	}

	// --- healing ---
	if spell.IsHealing {
		heal := spell.MinDamage
		if spell.MaxDamage > spell.MinDamage {
			heal += rand.Int31n(spell.MaxDamage - spell.MinDamage + 1)
		}
		monster.AddHealth(heal)

		if e.world.OnMagicEffect != nil {
			e.world.OnMagicEffect(monster.GetPosition(), uint16(3)) // CONST_ME_MAGIC_GREEN
		}
		if e.world.OnCreatureHealthChange != nil {
			e.world.OnCreatureHealthChange(monster)
		}
		return true
	}

	// --- speech only ---
	if spell.IsSpeech {
		return true
	}

	// --- combat damage ---
	dmg := int32(spell.MinDamage)
	if spell.MaxDamage > spell.MinDamage {
		dmg += rand.Int31n(spell.MaxDamage - spell.MinDamage + 1)
	}
	if dmg > 0 {
		dmg = -dmg
	}

	c := combat.NewCombat()
	c.SetParam(combat.CombatParamType, uint32(spell.CombatType))
	c.DoCombatHealth(adaptCreature(monster), adaptCreature(target), combat.CombatDamage{
		PrimaryType:  spell.CombatType,
		PrimaryValue: dmg,
		Origin:       combat.OriginSpell,
	})

	effect := combatTypeToMagicEffect(spell.CombatType)
	if e.world.OnMagicEffect != nil {
		e.world.OnMagicEffect(target.GetPosition(), effect)
	}
	if e.world.OnCreatureHealthChange != nil {
		e.world.OnCreatureHealthChange(target)
	}
	if e.world.OnCombatHit != nil {
		e.world.OnCombatHit(monster, target, dmg, effect)
	}

	return true
}

// ---------------------------------------------------------------------------
// Combat-type helpers
// ---------------------------------------------------------------------------

// stringToCombatType maps the string-based CombatType stored in MonsterAttack
// to the combat.CombatType enum used in the combat engine.
func stringToCombatType(combatType, spellName string) combat.CombatType {
	s := combatType
	if s == "" {
		s = spellName
	}
	switch {
	case containsFold(s, "physical"):
		return combat.CombatPhysical
	case containsFold(s, "fire"):
		return combat.CombatFire
	case containsFold(s, "energy"):
		return combat.CombatEnergy
	case containsFold(s, "earth"), containsFold(s, "poison"):
		return combat.CombatEarth
	case containsFold(s, "ice"):
		return combat.CombatIce
	case containsFold(s, "death"):
		return combat.CombatDeath
	case containsFold(s, "holy"):
		return combat.CombatHoly
	case containsFold(s, "heal"), containsFold(s, "life"):
		return combat.CombatHealing
	case containsFold(s, "lifedrain"), containsFold(s, "life drain"):
		return combat.CombatLifeDrain
	case containsFold(s, "manadrain"), containsFold(s, "mana drain"):
		return combat.CombatManaDrain
	default:
		return combat.CombatPhysical
	}
}

// parseConditionType maps a condition-type string to combat.ConditionType.
func parseConditionType(s string) combat.ConditionType {
	switch {
	case containsFold(s, "poison"):
		return combat.ConditionPoison
	case containsFold(s, "fire"), containsFold(s, "burn"):
		return combat.ConditionFire
	case containsFold(s, "energy"):
		return combat.ConditionEnergy
	case containsFold(s, "haste"):
		return combat.ConditionHaste
	case containsFold(s, "paralyze"), containsFold(s, "slow"):
		return combat.ConditionParalyze
	case containsFold(s, "outfit"):
		return combat.ConditionOutfit
	case containsFold(s, "invisible"):
		return combat.ConditionInvisible
	case containsFold(s, "drunk"):
		return combat.ConditionDrunk
	default:
		return combat.ConditionNone
	}
}

// combatTypeToMagicEffect returns a visual magic effect id for a given combat
// type. These are the CONST_ME_* values used by the client.
func combatTypeToMagicEffect(ct combat.CombatType) uint16 {
	switch ct {
	case combat.CombatPhysical:
		return 1 // CONST_ME_DRAWBLOOD
	case combat.CombatFire:
		return 15 // CONST_ME_FIREATTACK
	case combat.CombatEnergy:
		return 11 // CONST_ME_ENERGYHIT
	case combat.CombatEarth:
		return 20 // CONST_ME_POISON_ATTACK
	case combat.CombatIce:
		return 41 // CONST_ME_ICEATTACK
	case combat.CombatDeath:
		return 17 // CONST_ME_MORTAREA
	case combat.CombatHoly:
		return 16 // CONST_ME_HOLYDAMAGE
	case combat.CombatLifeDrain:
		return 5 // CONST_ME_LIFEDRAIN
	case combat.CombatManaDrain:
		return 7 // CONST_ME_MANADRAIN
	case combat.CombatHealing:
		return 3 // CONST_ME_MAGIC_GREEN
	default:
		return 3 // CONST_ME_POFF
	}
}

// ---------------------------------------------------------------------------
// General utilities
// ---------------------------------------------------------------------------

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func containsFold(s, substr string) bool {
	sLen := len(s)
	subLen := len(substr)
	if subLen == 0 {
		return true
	}
	if subLen > sLen {
		return false
	}
	for i := 0; i <= sLen-subLen; i++ {
		match := true
		for j := 0; j < subLen; j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
