package game

import (
	"strings"
	"time"

	"github.com/omurilo/canary-go/internal/game/combat"
	"github.com/omurilo/canary-go/internal/items"
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
// If the combat has chain configuration, additional targets are hit via BFS.
func (e *CombatEngine) DoCombatTarget(c *combat.Combat, caster, target Creature) {
	if target == nil {
		return
	}
	if c.Params.DistanceEffect != 0 && caster != nil && e.world.OnDistanceEffect != nil {
		e.world.OnDistanceEffect(caster.GetPosition(), target.GetPosition(), c.Params.DistanceEffect)
	}
	if c.CallbackTargetCreature != "" && e.world.OnTargetCreature != nil {
		e.world.OnTargetCreature(c.CallbackTargetCreature, caster, target)
	}
	dmg := e.rollSpellDamage(c, caster)
	e.applySpellHit(c, caster, target, dmg)

	// Chain combat: resolve additional targets via BFS and apply delayed hits.
	e.doCombatChain(c, caster, target, dmg)
}

// combatChainDelay is the delay in ms between each chain jump.
const combatChainDelay = 150

// doCombatChain resolves chain targets from the initial target and applies
// delayed spell hits to each, with visual effects between caster and target.
func (e *CombatEngine) doCombatChain(c *combat.Combat, caster, initialTarget Creature, dmg combat.CombatDamage) {
	if c.ChainCallback == "" {
		return
	}
	if caster == nil || e.world == nil {
		return
	}

	// Resolve chain targets via BFS. For now, use default chain params.
	// The Lua callback (ChainCallback) can override these in a future pass.
	maxTargets := 3
	chainDistance := 5
	backtracking := false

	targets := combat.ResolveChainTargets(
		initialTarget.GetID(),
		toCombatPos(initialTarget.GetPosition()),
		func(pos combat.Position, dist int) []combat.ChainTarget {
			return e.getNearbyCreaturesForChain(pos, dist, initialTarget.GetID())
		},
		maxTargets, chainDistance,
		backtracking,
		nil, // pickerCallback — will be wired from Lua in future
	)
	if len(targets) == 0 {
		return
	}

	// Apply delayed hits to each chain target.
	for i, ct := range targets {
		idx := i
		targetID := ct.ID
		delay := time.Duration(idx+1) * combatChainDelay * time.Millisecond

		if c.Params.ChainEffect != 0 && e.world.OnDistanceEffect != nil {
			GlobalDispatcher.AddEvent(delay, func() {
				if t := e.world.CreatureByID(targetID); t != nil {
					e.world.OnDistanceEffect(caster.GetPosition(), t.GetPosition(), c.Params.ChainEffect)
				}
			})
		}

		GlobalDispatcher.AddEvent(delay+50*time.Millisecond, func() {
			if t := e.world.CreatureByID(targetID); t != nil {
				e.applySpellHit(c, caster, t, dmg)
				if c.Params.ImpactEffect != 0 && e.world.OnMagicEffect != nil {
					e.world.OnMagicEffect(t.GetPosition(), c.Params.ImpactEffect)
				}
			}
		})
	}
}

// getNearbyCreaturesForChain returns creatures IDs and positions around pos
// within Chebyshev distance, excluding excludeID.
func (e *CombatEngine) getNearbyCreaturesForChain(pos combat.Position, distance int, excludeID uint32) []combat.ChainTarget {
	gamePos := fromCombatPos(pos)
	minX := int(gamePos.X) - distance
	maxX := int(gamePos.X) + distance
	minY := int(gamePos.Y) - distance
	maxY := int(gamePos.Y) + distance
	seen := make(map[uint32]bool)

	var targets []combat.ChainTarget
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			checkPos := Position{X: uint16(x), Y: uint16(y), Z: gamePos.Z}
			for _, c := range e.world.CreaturesAt(checkPos) {
				id := c.GetID()
				if id == excludeID || seen[id] {
					continue
				}
				seen[id] = true
				targets = append(targets, combat.ChainTarget{
					ID:       id,
					Position: toCombatPos(c.GetPosition()),
				})
			}
		}
	}
	return targets
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
			if c.CallbackTargetCreature != "" && e.world.OnTargetCreature != nil {
				e.world.OnTargetCreature(c.CallbackTargetCreature, caster, tgt)
			}
			e.applySpellHit(c, caster, tgt, dmg)
		}
		if c.CallbackTargetTile != "" && e.world.OnTargetTile != nil {
			e.world.OnTargetTile(c.CallbackTargetTile, caster, p)
		}
	}
}

// rollSpellDamage computes the combat value once, mirroring
// Combat::getCombatDamage (combat.cpp:52). The value is stored as a positive
// magnitude with the combat type; combat.DoCombatHealth negates it for damage.
// Damage-boosting augments from the caster's equipment are applied here.
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

	// Apply spell damage augments from equipped items.
	if caster != nil && e.world != nil && c.InstantSpellName != "" {
		if p, ok := caster.(*Player); ok {
			augVal := getCombatDataAugment(p, e.world.Items, c.InstantSpellName, items.AugmentSpellDamage)
			if augVal > 0 {
				raw = int32(float64(raw) * (1.0 + float64(augVal)/100.0))
			}
		}
	}

	return combat.CombatDamage{
		PrimaryType:  c.Params.CombatType,
		PrimaryValue: raw,
		Origin:       combat.OriginSpell,
	}
}

// getCombatDataAugment searches the caster's equipped items for an augment matching
// the given spell name and augment type. Returns the augment value, or 0 if not found.
func getCombatDataAugment(p *Player, catalog *items.Catalog, spellName string, augType items.AugmentType) int32 {
	if p == nil || catalog == nil || spellName == "" {
		return 0
	}
	for _, item := range p.GetEquippedAugmentItemsByType(catalog, augType, spellName) {
		itemType := catalog.Get(item.ID)
		if itemType == nil {
			continue
		}
		for _, aug := range itemType.Augments {
			if aug.Type == augType && strings.EqualFold(aug.SpellName, spellName) {
				return aug.Value
			}
		}
	}
	return 0
}

// calculateAugmentSpellCooldownReduction computes the total cooldown reduction (in ms)
// from all equipped augments of type AugmentSpellCooldown for the given spell name.
func calculateAugmentSpellCooldownReduction(p *Player, catalog *items.Catalog, spellName string, baseCooldownMs int64) int64 {
	if p == nil || catalog == nil || spellName == "" || baseCooldownMs <= 0 {
		return baseCooldownMs
	}
	var reduction int64
	for _, item := range p.GetEquippedAugmentItemsByType(catalog, items.AugmentSpellCooldown, spellName) {
		itemType := catalog.Get(item.ID)
		if itemType == nil {
			continue
		}
		for _, aug := range itemType.Augments {
			if aug.Type == items.AugmentSpellCooldown && strings.EqualFold(aug.SpellName, spellName) {
				reduction += int64(aug.Value)
			}
		}
	}
	reduced := baseCooldownMs - reduction
	if reduced < 0 {
		reduced = 0
	}
	return reduced
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
