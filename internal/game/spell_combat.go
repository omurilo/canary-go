package game

import (
	"fmt"

	"github.com/opentibiabr/canary-go/internal/game/combat"
)

// This file drives Lua-defined spell combats through the same hit/death path as
// melee, mirroring the Combat::doCombat family (src/creatures/combat/combat.cpp).
// The Lua onCastSpell closure calls combat:execute, which lands here via
// world.Combat.

// toCombatPos / fromCombatPos bridge the game.Position (z:uint8) and the combat
// package's Position (z:uint16).
func toCombatPos(p Position) combat.Position {
	return combat.Position{X: p.X, Y: p.Y, Z: uint16(p.Z)}
}

func fromCombatPos(p combat.Position) Position {
	return Position{X: p.X, Y: p.Y, Z: uint8(p.Z)}
}

// DoCombatTarget applies a single-target combat, mirroring
// Combat::doCombat(caster, target) -> doCombatHealth/doCombatMana
// (src/creatures/combat/combat.cpp:1337,1345).
func (e *CombatEngine) DoCombatTarget(c *combat.Combat, caster, target Creature) {
	if target == nil {
		return
	}
	if c.Params.DistanceEffect != 0 && caster != nil && e.world.OnDistanceEffect != nil {
		e.world.OnDistanceEffect(caster.GetPosition(), target.GetPosition(), c.Params.DistanceEffect)
	}
	dmg := e.rollSpellDamage(c, caster)
	e.applySpellHit(c, caster, target, dmg)
}

// DoCombatArea resolves the combat area around pos and applies the (single)
// rolled damage to every creature on the affected tiles, mirroring
// Combat::doCombat(caster, position) -> CombatFunc (combat.cpp:1362,1383). The
// area matrix is oriented by the caster->target direction.
func (e *CombatEngine) DoCombatArea(c *combat.Combat, caster Creature, pos Position) {
	var positions []Position
	if c.HasArea() {
		center := pos
		if caster != nil {
			center = caster.GetPosition()
		}
		for _, p := range c.Area.GetList(toCombatPos(center), toCombatPos(pos)) {
			positions = append(positions, fromCombatPos(p))
		}
	} else {
		positions = []Position{pos}
	}

	if c.Params.DistanceEffect != 0 && caster != nil && e.world.OnDistanceEffect != nil {
		e.world.OnDistanceEffect(caster.GetPosition(), pos, c.Params.DistanceEffect)
	}

	// getCombatDamage is called once per doCombat; the same damage is applied to
	// every creature in the area (combat.cpp:1369).
	dmg := e.rollSpellDamage(c, caster)

	seen := make(map[uint32]bool)
	for _, p := range positions {
		targets := e.world.CreaturesAt(p)
		if len(targets) == 0 {
			// Show the area effect on empty tiles too (combatTileEffects).
			if c.Params.ImpactEffect != 0 && e.world.OnMagicEffect != nil {
				e.world.OnMagicEffect(p, c.Params.ImpactEffect)
			}
			continue
		}
		for _, tgt := range targets {
			if seen[tgt.GetID()] {
				continue
			}
			seen[tgt.GetID()] = true
			e.applySpellHit(c, caster, tgt, dmg)
		}
	}
}

// rollSpellDamage computes the combat value once, mirroring
// Combat::getCombatDamage (combat.cpp:52). The value is stored as a positive
// magnitude with the combat type; combat.DoCombatHealth negates it for damage.
func (e *CombatEngine) rollSpellDamage(c *combat.Combat, caster Creature) combat.CombatDamage {
	level, magic := 0, 0
	if p, ok := caster.(*Player); ok {
		level = int(p.Level)
		magic = int(p.GetEffectiveMagLevel())
	}
	raw := c.RollValue(level, magic)
	if raw < 0 {
		raw = -raw
	}
	return combat.CombatDamage{
		PrimaryType:  c.Params.CombatType,
		PrimaryValue: raw,
		Origin:       combat.OriginSpell,
	}
}

// applySpellHit applies one resolved hit/heal to target and fires the client
// updates + death handling, reusing the melee path. Mirrors the tail of
// Combat::doCombatHealth / doCombatMana + Creature::onDeath.
func (e *CombatEngine) applySpellHit(c *combat.Combat, caster, target Creature, dmg combat.CombatDamage) {
	isHeal := c.Params.CombatType == combat.CombatHealing
	amount := dmg.PrimaryValue

	if c.IsManaDrain() {
		if !c.DoCombatMana(adaptCreature(caster), adaptCreature(target), dmg) {
			return
		}
	} else {
		if !c.DoCombatHealth(adaptCreature(caster), adaptCreature(target), dmg) {
			return
		}
	}

	if e.world.OnCreatureHealthChange != nil {
		e.world.OnCreatureHealthChange(target)
	}

	if isHeal || c.IsManaDrain() {
		// Healing / mana: just show the spell effect (no red damage text).
		if c.Params.ImpactEffect != 0 && e.world.OnMagicEffect != nil {
			e.world.OnMagicEffect(target.GetPosition(), c.Params.ImpactEffect)
		}
	} else if e.world.OnCombatHit != nil {
		// Damage: effect + animated damage text at the victim.
		e.world.OnCombatHit(caster, target, amount, c.Params.ImpactEffect)
	}

	casterName := "nil"
	if caster != nil {
		casterName = caster.GetName()
	}
	fmt.Printf("applySpellHit: caster=%s target=%s amount=%d isHeal=%t health_after=%d\n", casterName, target.GetName(), amount, isHeal, target.GetHealth())

	// Refresh the target's own stat bars (HP/mana) if it is a player.
	if p, ok := target.(*Player); ok && e.world.OnPlayerStatsChange != nil {
		e.world.OnPlayerStatsChange(p)
	}

	if target.GetHealth() == 0 {
		e.handleDeath(target, caster)
	}
}

// DoTargetCombatHealth applies a health combat effect (healing or damage) directly to target.
func (e *CombatEngine) DoTargetCombatHealth(caster, target Creature, combatType combat.CombatType, min, max int32, effect uint16) {
	if target == nil {
		return
	}
	c := combat.NewCombat()
	c.Params.CombatType = combatType
	c.Params.ImpactEffect = effect

	val := combat.RandomRange(int(min), int(max))

	dmg := combat.CombatDamage{
		PrimaryType:  combatType,
		PrimaryValue: val,
		Origin:       combat.OriginSpell,
	}
	e.applySpellHit(c, caster, target, dmg)
}

// DoTargetCombatMana applies a mana combat effect directly to target.
func (e *CombatEngine) DoTargetCombatMana(caster, target Creature, min, max int32, effect uint16) {
	if target == nil {
		return
	}
	c := combat.NewCombat()
	c.Params.CombatType = combat.CombatManaDrain
	c.Params.ImpactEffect = effect

	val := combat.RandomRange(int(min), int(max))

	primaryType := combat.CombatManaDrain
	if min > 0 || max > 0 {
		primaryType = combat.CombatHealing
	}

	dmg := combat.CombatDamage{
		PrimaryType:  primaryType,
		PrimaryValue: val,
		Origin:       combat.OriginSpell,
	}
	e.applySpellHit(c, caster, target, dmg)
}

// DoAreaCombatHealth applies a health combat effect over an area around pos.
func (e *CombatEngine) DoAreaCombatHealth(caster Creature, combatType combat.CombatType, pos Position, area *combat.AreaCombat, min, max int32, effect uint16) {
	c := combat.NewCombat()
	c.Params.CombatType = combatType
	c.Params.ImpactEffect = effect
	c.Area = area

	e.DoCombatArea(c, caster, pos)
}

// DoAreaCombatMana applies a mana combat effect over an area around pos.
func (e *CombatEngine) DoAreaCombatMana(caster Creature, pos Position, area *combat.AreaCombat, min, max int32, effect uint16) {
	c := combat.NewCombat()
	c.Params.CombatType = combat.CombatManaDrain
	c.Params.ImpactEffect = effect
	c.Area = area

	e.DoCombatArea(c, caster, pos)
}

