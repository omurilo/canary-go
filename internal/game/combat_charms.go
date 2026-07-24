package game

import (
	"math"
	"math/rand"

	"github.com/opentibiabr/canary-go/internal/charms"
	"github.com/opentibiabr/canary-go/internal/game/combat"
)

// charmCombatType maps a datapack COMBAT_* value to the engine combat type.
// Mirrors luaengine.luaToCombatType for the elements charms use.
func charmCombatType(v int) combat.CombatType {
	switch v {
	case 0:
		return combat.CombatPhysical
	case 1:
		return combat.CombatFire
	case 2:
		return combat.CombatEarth
	case 3:
		return combat.CombatEnergy
	case 9:
		return combat.CombatIce
	case 10:
		return combat.CombatHoly
	case 11:
		return combat.CombatDeath
	default:
		return combat.CombatPhysical
	}
}

// charmChanceIndex maps an unlock tier (1..3) to a chance-table index (0..2).
// A freshly unlocked charm (tier 1) uses the first chance value and progresses
// up. C++ indexes chance[tier] directly, which reads past the 3-element table
// at max tier; clamping here keeps the sensible per-tier progression.
func charmChanceIndex(tier uint8) int {
	if tier == 0 {
		return 0
	}
	idx := int(tier) - 1
	if idx > 2 {
		idx = 2
	}
	return idx
}

// chanceHit rolls a percentage chance (supporting fractional values), mirroring
// C++ `charm->chance[tier] >= normal_random(1, 10000) / 100.0`.
func chanceHit(pct float32) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	return float64(pct) >= float64(rand.Intn(10000)+1)/100.0
}

// charmsForMonster returns the player's major and minor charms assigned to a
// monster race (nil when unassigned). Mirrors IOBestiary::getCharmFromTarget.
func charmsForMonster(reg *charms.Registry, p *Player, raceID uint16) (major, minor *charms.Charm) {
	if reg == nil || p == nil || raceID == 0 {
		return nil, nil
	}
	for _, id := range charms.UsedRunes(int32(p.UsedRunesBit)) {
		if p.GetCharmRace(id) != raceID {
			continue
		}
		c := reg.Get(id)
		if c == nil {
			continue
		}
		switch c.Category {
		case charms.CategoryMajor:
			major = c
		case charms.CategoryMinor:
			minor = c
		}
	}
	return major, minor
}

// addSpeedCondition applies a haste or paralyze condition using the datapack
// formula variables (mirrors ConditionSpeed::setFormulaVars).
func addSpeedCondition(c Creature, condType combat.ConditionType, ticks int32, mina, minb, maxa, maxb float32) {
	cond := combat.CreateCondition(combat.ConditionId(0), condType, ticks, 0, false)
	if sc, ok := cond.(*combat.ConditionSpeedStruct); ok {
		sc.SetFormulaVars(mina, minb, maxa, maxb)
	}
	if m, ok := c.(*Monster); ok {
		m.AddCondition(cond)
	} else if p, ok := c.(*Player); ok {
		p.AddCondition(cond)
	}
}

// applyCharmRune rolls the player's charm(s) assigned to the target monster and
// applies their on-hit effect. `realDamage` is the damage the triggering hit
// dealt (used by leech and crit-style charms). Mirrors Game::applyCharmRune plus
// the leech/crit passives that C++ folds into the damage pipeline.
func (e *CombatEngine) applyCharmRune(p *Player, target Creature, realDamage int32) {
	if p == nil || e.world == nil || e.world.Charms == nil {
		return
	}
	m, ok := target.(*Monster)
	if !ok || m.Type == nil || m.Type.RaceID == 0 {
		return
	}
	major, minor := charmsForMonster(e.world.Charms, p, m.Type.RaceID)
	for _, c := range [2]*charms.Charm{major, minor} {
		if c != nil {
			e.applyOnHitCharm(p, target, m, c, realDamage)
		}
	}
}

// applyOnHitCharm resolves one charm's player-hits-monster effect.
func (e *CombatEngine) applyOnHitCharm(p *Player, target Creature, m *Monster, c *charms.Charm, realDamage int32) {
	chance := c.Chance[charmChanceIndex(p.GetCharmTier(c.ID))]

	switch c.ID {
	case charms.Wound, charms.Enflame, charms.Poison, charms.Freeze,
		charms.Zap, charms.Curse, charms.Divine:
		// Elemental burst: percent% of max health, capped at 2x level.
		if target.GetHealth() == 0 || !chanceHit(chance) {
			return
		}
		dmg := c.OffensiveDamage(uint32(p.Level), m.GetMaxHealth())
		e.applyCharmDamage(p, target, dmg, charmCombatType(c.DamageType), c.Effect)

	case charms.Overpower:
		// Physical: min(8% target maxHP, 5% player maxHP).
		if target.GetHealth() == 0 || !chanceHit(chance) {
			return
		}
		dmg := minInt32(
			ceilPct(m.GetMaxHealth(), 8),
			ceilPct(p.GetMaxHealth(), uint32(c.Percent)),
		)
		e.applyCharmDamage(p, target, dmg, combat.CombatPhysical, c.Effect)

	case charms.Overflux:
		// Physical: min(8% target maxHP, percent% player maxMana).
		if target.GetHealth() == 0 || !chanceHit(chance) {
			return
		}
		dmg := minInt32(
			ceilPct(m.GetMaxHealth(), 8),
			int32(math.Ceil(float64(p.GetMaxMana())*(c.Percent/100.0))),
		)
		e.applyCharmDamage(p, target, dmg, combat.CombatPhysical, c.Effect)

	case charms.Cripple:
		// Paralyze the monster for 10 seconds.
		if target.GetHealth() == 0 || !chanceHit(chance) {
			return
		}
		addSpeedCondition(target, combat.ConditionParalyze, 10000, -1, 0, -1, 0)

	case charms.Low:
		// Low Blow (crit chance): the port has no crit pipeline, so approximate
		// a critical by repeating the hit's damage as physical on a proc.
		if realDamage <= 0 || target.GetHealth() == 0 || !chanceHit(chance) {
			return
		}
		e.applyCharmDamage(p, target, realDamage, combat.CombatPhysical, c.Effect)

	case charms.Savage:
		// Savage Blow (crit extra damage): approximate with a half-damage burst.
		if realDamage <= 0 || target.GetHealth() == 0 || !chanceHit(chance) {
			return
		}
		e.applyCharmDamage(p, target, realDamage/2, combat.CombatPhysical, c.Effect)

	case charms.Vamp:
		// Life leech: heal the player for chance% of the damage dealt (always on;
		// the port has no leech-gear stat to gate on).
		if realDamage > 0 {
			if heal := int32(float64(realDamage) * float64(chance) / 100.0); heal > 0 {
				p.AddHealth(heal)
			}
		}

	case charms.Void:
		// Mana leech: restore chance% of the damage dealt as mana.
		if realDamage > 0 {
			if mana := int32(float64(realDamage) * float64(chance) / 100.0); mana > 0 {
				p.AddMana(mana)
			}
		}
	}
}

// negativeConditions are the harmful condition types Cleanse can strip.
var negativeConditions = []combat.ConditionType{
	combat.ConditionPoison, combat.ConditionFire, combat.ConditionEnergy,
	combat.ConditionBleeding, combat.ConditionFreezing, combat.ConditionDazzled,
	combat.ConditionCursed, combat.ConditionParalyze,
}

// applyDefensiveCharmRune rolls the player's defensive charm(s) assigned to the
// attacking monster after it lands a hit. Mirrors the target-as-player charm
// block in Game::combatChangeHealth (Parry/Dodge/Adrenaline/Numb/Cleanse).
func (e *CombatEngine) applyDefensiveCharmRune(m *Monster, p *Player, realDamage int32) {
	if p == nil || m == nil || m.Type == nil || m.Type.RaceID == 0 ||
		e.world == nil || e.world.Charms == nil || realDamage <= 0 {
		return
	}
	major, minor := charmsForMonster(e.world.Charms, p, m.Type.RaceID)
	for _, c := range [2]*charms.Charm{major, minor} {
		if c == nil || c.Type != charms.TypeDefensive {
			continue
		}
		if !chanceHit(c.Chance[charmChanceIndex(p.GetCharmTier(c.ID))]) {
			continue
		}
		switch c.ID {
		case charms.Parry:
			// Reflect the damage back to the aggressor.
			e.applyCharmDamage(p, m, realDamage, combat.CombatPhysical, c.Effect)
		case charms.Dodge:
			// Dodge the hit: undo the damage (the port applies charms post-hit).
			p.AddHealth(realDamage)
			if e.world.OnCreatureHealthChange != nil {
				e.world.OnCreatureHealthChange(p)
			}
			if c.Effect != 0 && e.world.OnMagicEffect != nil {
				e.world.OnMagicEffect(p.GetPosition(), c.Effect)
			}
		case charms.Adrenaline:
			addSpeedCondition(p, combat.ConditionHaste, 10000, 2.5, 40, 2.5, 40)
		case charms.Numb:
			addSpeedCondition(m, combat.ConditionParalyze, 10000, -1, 0, -1, 0)
		case charms.Cleanse:
			e.cleansePlayer(p)
		}
		if c.MessageCancel != "" {
			p.SendTextMessage(0x14, c.MessageCancel)
		}
	}
}

// cleansePlayer removes the first active negative condition, mirroring the
// Cleanse charm (temporary immunity is not modelled).
func (e *CombatEngine) cleansePlayer(p *Player) {
	for _, t := range negativeConditions {
		if p.HasCondition(t) {
			p.RemoveCondition(t)
			return
		}
	}
}

// applyCarnageOnDeath deals Carnage's physical splash to monsters adjacent to a
// monster the killing player had the Carnage charm set on. Mirrors the
// CHARM_CARNAGE branch of parseOffensiveCharmCombat + parseCharmCarnage.
func (e *CombatEngine) applyCarnageOnDeath(victim *Monster, killer *Player) {
	if victim == nil || killer == nil || victim.Type == nil || victim.Type.RaceID == 0 ||
		e.world == nil || e.world.Charms == nil || e.world.Map == nil {
		return
	}
	c := e.world.Charms.Get(charms.Carnage)
	if c == nil || killer.GetCharmRace(charms.Carnage) != victim.Type.RaceID {
		return
	}
	if !chanceHit(c.Chance[charmChanceIndex(killer.GetCharmTier(charms.Carnage))]) {
		return
	}
	// Damage per adjacent monster: min(percent% of victim max HP, 6x level).
	dmg := minInt32(
		int32(math.Ceil(float64(victim.GetMaxHealth())*(c.Percent/100.0))),
		int32(killer.Level)*6,
	)
	if dmg <= 0 {
		return
	}
	pos := victim.GetPosition()
	offsets := [4][2]int32{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for _, off := range offsets {
		tile := e.world.Map.GetTile(Position{X: uint16(int32(pos.X) + off[0]), Y: uint16(int32(pos.Y) + off[1]), Z: pos.Z})
		if tile == nil {
			continue
		}
		for _, cr := range tile.Creatures {
			if nb, ok := cr.(*Monster); ok && nb.GetHealth() > 0 {
				e.applyCharmDamage(killer, nb, dmg, combat.CombatPhysical, c.Effect)
			}
		}
	}
}

// applyCharmDamage deals a one-off charm hit to the target and fires the health,
// effect and death hooks, mirroring the tail of the melee/ranged hit handlers.
func (e *CombatEngine) applyCharmDamage(attacker, target Creature, dmg int32, combatType combat.CombatType, effect uint16) {
	if dmg <= 0 {
		return
	}
	c := combat.NewCombat()
	c.SetParam(combat.CombatParamType, uint32(combatType))
	if !c.DoCombatHealth(adaptCreature(attacker), adaptCreature(target), combat.CombatDamage{
		PrimaryType:  combatType,
		PrimaryValue: dmg,
		Origin:       combat.OriginCondition,
	}) {
		return
	}
	if e.world.OnCreatureHealthChange != nil {
		e.world.OnCreatureHealthChange(target)
	}
	if e.world.OnCombatHit != nil {
		e.world.OnCombatHit(attacker, target, dmg, effect)
	}
	if target.GetHealth() == 0 {
		e.handleDeath(target, attacker)
	}
}

// ceilPct returns ceil(value * pct/100) as int32.
func ceilPct(value uint32, pct uint32) int32 {
	return int32(math.Ceil(float64(value) * float64(pct) / 100.0))
}

func minInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
