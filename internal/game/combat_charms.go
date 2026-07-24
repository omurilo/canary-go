package game

import (
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

// isElementalCharm reports whether a charm is an on-hit elemental damage charm
// (the flagship offensive charms whose damage formula is 5% of the target's max
// health). Other offensive charms (Carnage on-death AoE, Overpower/Overflux,
// Cripple) and defensive/passive charms are not modelled yet.
func isElementalCharm(id uint8) bool {
	switch id {
	case charms.Wound, charms.Enflame, charms.Poison, charms.Freeze,
		charms.Zap, charms.Curse, charms.Divine:
		return true
	}
	return false
}

// charmChanceIndex maps an unlock tier (1..3) to a chance-table index (0..2).
// A freshly unlocked charm (tier 1) uses the first chance value and progresses
// up. C++ indexes chance[tier] directly, which reads past the 3-element table
// at max tier; clamping here keeps the sensible 5%/10%/11% progression.
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

// applyCharmRune rolls the player's charm(s) assigned to the target monster and,
// on a successful offensive elemental proc, deals a burst of the charm's
// element. Mirrors Game::applyCharmRune restricted to the elemental on-hit
// charms (IOBestiary::parseOffensiveCharmCombat default case).
func (e *CombatEngine) applyCharmRune(p *Player, target Creature) {
	if p == nil || e.world == nil || e.world.Charms == nil {
		return
	}
	m, ok := target.(*Monster)
	if !ok || m.Type == nil || m.Type.RaceID == 0 {
		return
	}
	major, minor := charmsForMonster(e.world.Charms, p, m.Type.RaceID)
	for _, c := range [2]*charms.Charm{major, minor} {
		if c == nil || c.Type != charms.TypeOffensive || !isElementalCharm(c.ID) {
			continue
		}
		if target.GetHealth() == 0 {
			return
		}
		tier := p.GetCharmTier(c.ID)
		chance := c.Chance[charmChanceIndex(tier)]
		if int(chance) < rand.Intn(100)+1 {
			continue
		}
		dmg := c.OffensiveDamage(uint32(p.Level), m.GetMaxHealth())
		if dmg <= 0 {
			continue
		}
		e.applyCharmDamage(p, target, dmg, charmCombatType(c.DamageType), c.Effect)
	}
}

// applyCharmDamage deals a one-off charm hit to the target and fires the health,
// effect and death hooks, mirroring the tail of the melee/ranged hit handlers.
func (e *CombatEngine) applyCharmDamage(attacker, target Creature, dmg int32, combatType combat.CombatType, effect uint16) {
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
