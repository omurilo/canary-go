package game

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/opentibiabr/canary-go/internal/bestiary"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game/combat"
	"github.com/opentibiabr/canary-go/internal/game/vocations"
)

const (
	// combatTickInterval is how often the engine re-evaluates attacks. Each
	// creature still only lands a hit once its own attack interval elapses.
	combatTickInterval = 100 * time.Millisecond

	// defaultPlayerAttackSpeed mirrors Vocation::attackSpeed's default of 1500ms
	// (src/creatures/players/vocations/vocation.hpp) used by Player::getAttackSpeed.
	// TODO(vocations): read the player's real vocation attackSpeed once a
	// vocation registry is wired into the world.
	defaultPlayerAttackSpeed = 1500 * time.Millisecond

	// defaultMonsterAttackSpeed mirrors MonsterType interval default of 2000ms
	// (src/creatures/monsters/monsters.hpp).
	defaultMonsterAttackSpeed = 2000 * time.Millisecond

	// fistAttackValue is the fist weapon's attack value from Weapon::useFist
	// (src/items/weapons/weapons.cpp): constexpr int32_t attackValue = 7.
	fistAttackValue = 7

	// defaultMonsterAttackValue is a placeholder melee damage ceiling for
	// monsters until their attack blocks are parsed.
	// TODO(monster-data): replace with the monster's attack/skill/spell block.
	defaultMonsterAttackValue = 15

	// Magic effects from src/utils/utils_definitions.hpp.
	effectDrawBlood = 1 // CONST_ME_DRAWBLOOD (physical hit on a blood creature)
	effectPoff      = 3 // CONST_ME_POFF (blocked / no-damage hit)

	// defaultCorpseID is used when a monster has no parsed corpse id. 5964 is
	// the rat corpse, a real item id used only as a safe placeholder.
	defaultCorpseID = 5964

	// maxLootChance is MAX_LOOTCHANCE from src/utils/const.hpp: loot chances in
	// monster data are out of 100000.
	maxLootChance = 100000
)

// CombatEngine resolves melee attacks on a fixed tick. It runs on its own
// goroutine (via the dispatcher) and only mutates world/creature state through
// the World's guarded helpers plus its own lock over the attack-timer map.
type CombatEngine struct {
	world *World

	mu         sync.Mutex
	lastAttack map[uint32]time.Time
}

// NewCombatEngine creates a combat engine for the world.
func NewCombatEngine(w *World) *CombatEngine {
	return &CombatEngine{
		world:      w,
		lastAttack: make(map[uint32]time.Time),
	}
}

// Start begins the self-rescheduling combat loop.
// regenInterval is how often food regeneration drains and heals.
const regenInterval = 1 * time.Second

func (e *CombatEngine) Start() {
	GlobalDispatcher.AddEvent(combatTickInterval, e.tick)
	GlobalDispatcher.AddEvent(regenInterval, e.regenTick)
	GlobalDispatcher.AddEvent(1*time.Second, e.imbuementDecayTick)
}

// regenTick drains active food (CONDITION_REGENERATION) and regenerates a small
// amount of HP/mana while it lasts, mirroring the regeneration condition. It is
// a simplified fixed-rate gain (per-vocation gain amounts are a later milestone).
// Also decays equipped imbuements while the player is in combat.
func (e *CombatEngine) regenTick() {
	for _, p := range e.world.Players() {
		if p.RegenTicks <= 0 {
			continue
		}
		p.RegenTicks -= int32(regenInterval / time.Millisecond)
		if p.RegenTicks < 0 {
			p.RegenTicks = 0
		}
		// Heal once every 3 seconds to preserve standard regeneration rate
		if (p.RegenTicks / 1000) % 3 == 0 {
			if p.Health < p.GetMaxHealth() {
				p.AddHealth(1)
				if e.world.OnCreatureHealthChange != nil {
					e.world.OnCreatureHealthChange(p)
				}
			}
			if p.Mana < p.GetMaxMana() {
				p.AddMana(1)
			}
		}
		// Always refresh stats while food is active so the client's food timer
		// counts down and HP/mana/soul stay in sync.
		if e.world.OnPlayerStatsChange != nil {
			e.world.OnPlayerStatsChange(p)
		}
	}
	GlobalDispatcher.AddEvent(regenInterval, e.regenTick)
}

// imbuementDecayTick runs every second and decays equipped imbuements for
// players who are in combat (CONDITION_INFIGHT) and outside protection zones.
// Mirrors C++ ImbuementDecay::checkImbuementDecay.
func (e *CombatEngine) imbuementDecayTick() {
	for _, p := range e.world.Players() {
		if p.IsInProtectionZone() {
			continue
		}
		if !p.HasCondition(combat.ConditionInFight) {
			continue
		}
		for s := ConstSlotFirst; s <= ConstSlotLast; s++ {
			if int(s) >= len(p.Inventory) || p.Inventory[s] == nil {
				continue
			}
			item := p.Inventory[s]
			item.imbueMu.Lock()
			if item.Imbuements == nil {
				item.imbueMu.Unlock()
				continue
			}
			for slotID, info := range item.Imbuements {
				if info.Duration == 0 {
					continue
				}
				newDuration := info.Duration - 1
				if newDuration > info.Duration {
					newDuration = 0
				}
				if newDuration == 0 {
					delete(item.Imbuements, slotID)
				} else {
					item.Imbuements[slotID] = ImbuementInfo{ID: info.ID, Duration: newDuration}
				}
			}
			item.imbueMu.Unlock()
		}
	}
	GlobalDispatcher.AddEvent(1*time.Second, e.imbuementDecayTick)
}

func (e *CombatEngine) tick() {
	e.resolveAttacks()

	for _, p := range e.world.Players() {
		p.TickConditions(100)
	}

	for _, c := range e.world.Creatures() {
		if c.GetCreatureType() != 0 {
			if bc, ok := c.(*BaseCreature); ok {
				bc.TickConditions(100)
			}
		}
	}

	GlobalDispatcher.AddEvent(combatTickInterval, e.tick)
}

func (e *CombatEngine) resolveAttacks() {
	// Players attack the creature they selected (player.TargetID).
	for _, p := range e.world.Players() {
		e.tryAttack(p, e.playerTarget(p), p.AttackSpeed())
	}
	// Monsters attack their AI target (set by the AI engine), each at its own
	// melee interval (MonsterType attack block; default 2000ms).
	for _, c := range e.world.Creatures() {
		if m, ok := c.(*Monster); ok {
			e.tryAttack(m, m.GetTarget(), m.AttackInterval())
		}
	}
}

// playerTarget resolves a player's live attack target, clearing it if the
// creature is gone.
func (e *CombatEngine) playerTarget(p *Player) Creature {
	if p.TargetID == 0 {
		return nil
	}
	target := e.world.CreatureByID(p.TargetID)
	if target == nil {
		p.SetAttackTarget(0)
		return nil
	}
	return target
}

func (e *CombatEngine) tryAttack(attacker, target Creature, interval time.Duration) {
	if attacker == nil || target == nil {
		return
	}
	if target.GetHealth() == 0 {
		return
	}
	if e.world != nil && e.world.Map != nil {
		if tile := e.world.Map.GetTile(attacker.GetPosition()); tile != nil && tile.IsProtectionZone() {
			return
		}
		if tile := e.world.Map.GetTile(target.GetPosition()); tile != nil && tile.IsProtectionZone() {
			return
		}
	}
	// Monsters never harm players who cannot be attacked (staff/ghost),
	// mirroring PlayerFlags_t::CannotBeAttacked.
	if _, atkIsMonster := attacker.(*Monster); atkIsMonster {
		if tp, ok := target.(*Player); ok && tp.CannotBeAttacked() {
			return
		}
	}

	maxRange := int32(1)
	var weapon *Item
	var launcher *Item
	var weaponType string

	if m, ok := attacker.(*Monster); ok {
		// Use the monster's maximum attack range so that ranged spells can
		// reach targets beyond melee distance. Falls back to 1 (adjacent).
		if m.Type != nil {
			for _, atk := range m.Type.Attacks {
				r := int32(atk.Range)
				if r > maxRange {
					maxRange = r
				}
			}
		}
	} else if p, ok := attacker.(*Player); ok {
		weapon = p.GetWeapon(e.world.Items, false)
		launcher = p.GetWeapon(e.world.Items, true)

		if launcher != nil && weapon == nil {
			// Equipped a distance launcher (bow/crossbow) but no matching ammunition found!
			// Player cannot attack.
			return
		}

		if weapon != nil {
			weaponType = weapon.WeaponType(e.world.Items)
			if weaponType == "distance" || weaponType == "missile" || weaponType == "ammunition" || weaponType == "ammo" {
				if launcher != nil {
					maxRange = launcher.Range(e.world.Items)
				} else {
					maxRange = weapon.Range(e.world.Items)
				}
				if maxRange <= 0 {
					maxRange = 1
				}
			} else if weaponType == "wand" {
				maxRange = weapon.Range(e.world.Items)
				if maxRange <= 0 {
					maxRange = 5 // Standard default wand range
				}
			}
		}
	}

	ap, tp := attacker.GetPosition(), target.GetPosition()
	if ap.Z != tp.Z || chebyshevDistance(ap, tp) > int(maxRange) {
		return
	}
	if !e.ready(attacker.GetID(), interval) {
		return
	}

	if p, ok := attacker.(*Player); ok {
		if weaponType == "wand" {
			e.doWandHit(p, target, weapon)
		} else if weaponType == "distance" || weaponType == "missile" || weaponType == "ammunition" || weaponType == "ammo" {
			e.doDistanceHit(p, target, weapon, launcher)
		} else {
			e.doMeleeHit(combat.NewCombat(), p, target)
		}
	} else if m, ok := attacker.(*Monster); ok {
		e.doMonsterAttack(m, target)
	} else {
		e.doMeleeHit(combat.NewCombat(), attacker, target)
	}
}

// ready reports whether the creature's attack cooldown has elapsed, and if so
// stamps the current time. Mirrors Player::doAttacking's
// (OTSYS_TIME() - lastAttack) >= getAttackSpeed() gate.
func (e *CombatEngine) ready(id uint32, interval time.Duration) bool {
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	if last, ok := e.lastAttack[id]; ok && now.Sub(last) < interval {
		return false
	}
	e.lastAttack[id] = now
	return true
}

func (e *CombatEngine) doMeleeHit(c *combat.Combat, attacker, target Creature) {
	dmg := int32(e.meleeDamage(attacker))
	dmg = applyDamageModifiers(attacker, target, dmg)
	// Apply through the combat engine so the armor/condition/blocking hooks are exercised.
	c.SetParam(combat.CombatParamType, uint32(combat.CombatPhysical))
	c.SetParam(combat.CombatParamBlockArmor, 1)
	c.SetParam(combat.CombatParamBlockShield, 1)

	if p, ok := attacker.(*Player); ok {
		weapon := p.GetWeapon(e.world.Items, false)
		skill := SkillFist
		if weapon != nil {
			switch weapon.WeaponType(e.world.Items) {
			case "sword":
				skill = SkillSword
			case "axe":
				skill = SkillAxe
			case "club":
				skill = SkillClub
			}
		}
		p.AddSkillTries(skill, 1)
	}
	if tp, ok := target.(*Player); ok {
		tp.AddSkillTries(SkillShielding, 1)
	}

	if !c.DoCombatHealth(adaptCreature(attacker), adaptCreature(target), combat.CombatDamage{
		PrimaryType:  combat.CombatPhysical,
		PrimaryValue: int32(dmg),
		Origin:       combat.OriginMelee,
	}) {
		return
	}

	if p, ok := attacker.(*Player); ok {
		p.AddInFightTicks()
	}
	if p, ok := target.(*Player); ok {
		p.AddInFightTicks()
	}

	effect := uint16(effectDrawBlood)
	if dmg <= 0 {
		effect = effectPoff
	}

	// Health bar update for all spectators of the victim (item #3).
	if e.world.OnCreatureHealthChange != nil {
		e.world.OnCreatureHealthChange(target)
	}
	// Impact effect + animated damage text (item #4).
	if e.world.OnCombatHit != nil {
		e.world.OnCombatHit(attacker, target, int32(dmg), effect)
	}

	if target.GetHealth() == 0 {
		e.handleDeath(target, attacker)
	} else if pl, ok := attacker.(*Player); ok {
		e.applyCharmRune(pl, target, dmg)
	} else if mon, ok := attacker.(*Monster); ok {
		if tp, ok := target.(*Player); ok {
			e.applyDefensiveCharmRune(mon, tp, dmg)
		}
	}
}

func (e *CombatEngine) doDistanceHit(p *Player, target Creature, ammo *Item, launcher *Item) {
	skill := int(p.GetEffectiveSkill(SkillDistance))
	voc := vocations.GetVocation(uint32(p.Vocation))
	attackValue := int(ammo.Attack(e.world.Items))
	if launcher != nil {
		attackValue += int(launcher.Attack(e.world.Items))
	}
	if attackValue <= 0 {
		attackValue = 10 // fallback
	}

	// Calculate maximum distance damage using the modern formula:
	// Weapons::getMaxWeaponDamage(level, playerSkill, totalAttack, attackFactor, false)
	maxDmg := float64(0.09*p.GetAttackFactor()*float64(attackValue)*float64(skill) + float64(p.Level/5))
	if voc != nil && voc.Formula.DistDamage > 0 {
		maxDmg *= voc.Formula.DistDamage
	}

	minDmg := float64(p.Level / 5)
	if minDmg > maxDmg {
		minDmg = maxDmg
	}

	dmg := int32(0)
	if maxDmg > 0 {
		dmg = randomRange(int(minDmg), int(maxDmg))
	}
	dmg = applyDamageModifiers(p, target, dmg)

	// Consume ammunition
	ammoType := ammo.AmmoType(e.world.Items)
	if ammoType != "" && ammoType != "none" {
		p.ConsumeAmmo(e.world.Items, ammoType)
	} else {
		// It's a throwable weapon (e.g. spear) equipped in Left or Right slot.
		// Consume the weapon itself!
		if p.Inventory[ConstSlotLeft] == ammo {
			p.ConsumeWeaponInHand(ConstSlotLeft)
		} else if p.Inventory[ConstSlotRight] == ammo {
			p.ConsumeWeaponInHand(ConstSlotRight)
		}
	}

	p.AddSkillTries(SkillDistance, 1)

	// Apply combat
	c := combat.NewCombat()
	c.SetParam(combat.CombatParamType, uint32(combat.CombatPhysical))
	c.SetParam(combat.CombatParamBlockArmor, 1)
	c.SetParam(combat.CombatParamBlockShield, 1)

	if !c.DoCombatHealth(adaptCreature(p), adaptCreature(target), combat.CombatDamage{
		PrimaryType:  combat.CombatPhysical,
		PrimaryValue: dmg,
		Origin:       combat.OriginRanged,
	}) {
		return
	}

	p.AddInFightTicks()
	if tp, ok := target.(*Player); ok {
		tp.AddInFightTicks()
	}

	// Dispatch distance shoot effect
	shootStr := ammo.ShootType(e.world.Items)
	if shootStr == "" {
		shootStr = "arrow"
	}
	if e.world.OnDistanceEffect != nil {
		e.world.OnDistanceEffect(p.GetPosition(), target.GetPosition(), mapShootType(shootStr))
	}

	effect := uint16(effectDrawBlood)
	if dmg <= 0 {
		effect = effectPoff
	}

	if e.world.OnCreatureHealthChange != nil {
		e.world.OnCreatureHealthChange(target)
	}
	if e.world.OnCombatHit != nil {
		e.world.OnCombatHit(p, target, dmg, effect)
	}

	if target.GetHealth() == 0 {
		e.handleDeath(target, p)
	} else {
		e.applyCharmRune(p, target, dmg)
	}
}

func (e *CombatEngine) doWandHit(p *Player, target Creature, wand *Item) {
	// Consume wand/rod shoot mana (defaults to 5 mana if config/attributes unavailable)
	manaCost := int32(5)
	if p.Mana < uint32(manaCost) {
		return
	}
	p.AddMana(-manaCost)

	attack := wand.Attack(e.world.Items)
	if attack <= 0 {
		attack = 10 // safe default
	}
	lo := int(float64(attack) * 0.8)
	hi := int(float64(attack) * 1.2)
	dmg := applyDamageModifiers(p, target, randomRange(lo, hi))

	combatType := combat.CombatEnergy
	shootStr := wand.ShootType(e.world.Items)
	switch strings.ToLower(shootStr) {
	case "fire":
		combatType = combat.CombatFire
	case "earth", "poison":
		combatType = combat.CombatEarth
	case "ice":
		combatType = combat.CombatIce
	case "death":
		combatType = combat.CombatDeath
	case "holy":
		combatType = combat.CombatHoly
	}

	// Apply combat: wand damage is magic, bypassing armor and shield block
	c := combat.NewCombat()
	c.SetParam(combat.CombatParamType, uint32(combatType))

	if !c.DoCombatHealth(adaptCreature(p), adaptCreature(target), combat.CombatDamage{
		PrimaryType:  combatType,
		PrimaryValue: dmg,
		Origin:       combat.OriginRanged,
	}) {
		return
	}

	p.AddInFightTicks()
	if tp, ok := target.(*Player); ok {
		tp.AddInFightTicks()
	}

	// Dispatch distance projectile effect
	if e.world.OnDistanceEffect != nil {
		e.world.OnDistanceEffect(p.GetPosition(), target.GetPosition(), mapShootType(shootStr))
	}

	impactEffect := uint16(effectDrawBlood)
	switch combatType {
	case combat.CombatFire:
		impactEffect = 15 // CONST_ME_FIREATTACK
	case combat.CombatEnergy:
		impactEffect = 11 // CONST_ME_ENERGYHIT
	case combat.CombatEarth:
		impactEffect = 20 // CONST_ME_POISON
	case combat.CombatIce:
		impactEffect = 41 // CONST_ME_ICEATTACK
	case combat.CombatDeath:
		impactEffect = 17 // CONST_ME_MORTAREA
	}

	if e.world.OnCreatureHealthChange != nil {
		e.world.OnCreatureHealthChange(target)
	}
	if e.world.OnCombatHit != nil {
		e.world.OnCombatHit(p, target, dmg, impactEffect)
	}

	if target.GetHealth() == 0 {
		e.handleDeath(target, p)
	} else {
		e.applyCharmRune(p, target, dmg)
	}
}

// applyDamageModifiers applies Prey and Fiendish multipliers to raw damage.
func applyDamageModifiers(attacker, target Creature, dmg int32) int32 {
	if dmg <= 0 {
		return 0
	}
	multiplier := 1.0

	// 1. Fiendish / Influenced multipliers
	if m, ok := attacker.(*Monster); ok {
		// getAttackMultiplier (1.35 + (stacks-1)*0.1)
		if m.ForgeStack > 0 {
			multiplier *= (1.35 + float64(m.ForgeStack-1)*0.1)
		}
	}
	if m, ok := target.(*Monster); ok {
		// getDefenseMultiplier (1 + 0.1*stacks)
		// More defense means LESS damage taken.
		if m.ForgeStack > 0 {
			multiplier /= (1.0 + 0.1*float64(m.ForgeStack))
		}
	}

	// 3. Hazard System modifiers
	if m, ok := attacker.(*Monster); ok {
		if m.HazardDamageBoost {
			pct := float64(m.HazardPoints) * 2
			if pct > 30 {
				pct = 30
			}
			multiplier *= (100.0 + pct) / 100.0
		}
		if m.HazardDodge && hazardDodgeRoll(m) {
			return 0
		}
	}
	if m, ok := target.(*Monster); ok {
		if m.HazardDefenseBoost {
			pct := float64(m.HazardPoints) * 2
			if pct > 30 {
				pct = 30
			}
			multiplier *= (100.0 - pct) / 100.0
		}
	}

	// 4. Prey Bonuses
	if p, ok := attacker.(*Player); ok {
		if m, ok := target.(*Monster); ok && m.Type != nil {
			if bonus, ok := p.GetPrey().GetPreyBonus(m.Type.RaceID, PreyBonus_DamageBoost); ok {
				multiplier *= float64(100+bonus) / 100.0
			}
		}
	}
	if p, ok := target.(*Player); ok {
		if m, ok := attacker.(*Monster); ok && m.Type != nil {
			if bonus, ok := p.GetPrey().GetPreyBonus(m.Type.RaceID, PreyBonus_DamageReduction); ok {
				multiplier *= float64(100-bonus) / 100.0
			}
		}
	}

	return int32(float64(dmg) * multiplier)
}

// hazardDodgeRoll rolls the dodge chance for a hazard monster.
// Returns true if the attack should be completely dodged.
func hazardDodgeRoll(m *Monster) bool {
	if m == nil || !m.HazardDodge {
		return false
	}
	// Base dodge chance: 5% + 2% per hazard point, max 20%
	chance := 5 + int(m.HazardPoints)*2
	if chance > 20 {
		chance = 20
	}
	return rand.Intn(100) < chance
}

// hazardExpBonus calculates the experience bonus from hazard system.
// Formula: 1.75 * points * multiplier / 100.0
func hazardExpBonus(hazardPoints int32) float64 {
	if hazardPoints <= 0 {
		return 0
	}
	const hazardExpBonusMultiplier = 1.0
	return 1.75 * float64(hazardPoints) * hazardExpBonusMultiplier / 100.0
}

func (e *CombatEngine) meleeDamage(attacker Creature) int {
	switch a := attacker.(type) {
	case *Player:
		var weapon *Item
		var weaponType string
		weapon = a.GetWeapon(e.world.Items, false)
		if weapon != nil {
			weaponType = weapon.WeaponType(e.world.Items)
		}

		skill := int(a.GetWeaponSkillForItem(e.world.Items, weapon))
		attackValue := fistAttackValue
		if weapon != nil && weaponType != "distance" && weaponType != "missile" && weaponType != "wand" && weaponType != "ammunition" && weaponType != "ammo" {
			attackValue = int(weapon.Attack(e.world.Items))
		}

		maxDmg := float64(0.085*a.GetAttackFactor()*float64(attackValue)*float64(skill) + float64(a.Level/5))
		if voc := vocations.GetVocation(uint32(a.Vocation)); voc != nil && voc.Formula.MeleeDamage > 0 {
			maxDmg *= voc.Formula.MeleeDamage
		}

		minDmg := float64(a.Level / 5)
		if minDmg > maxDmg {
			minDmg = maxDmg
		}

		if maxDmg <= 0 {
			return 0
		}
		return int(randomRange(int(minDmg), int(maxDmg)))
	case *Monster:
		atk := a.MeleeAttack()
		if atk == nil {
			return rand.Intn(defaultMonsterAttackValue + 1)
		}
		if atk.Chance < 100 && rand.Intn(100) >= atk.Chance {
			return 0
		}
		lo, hi := absDamageRange(atk.MinDamage, atk.MaxDamage)
		if hi <= 0 {
			return 0
		}
		return lo + rand.Intn(hi-lo+1)
	default:
		return 0
	}
}

func mapShootType(s string) uint16 {
	switch strings.ToLower(s) {
	case "spear":
		return 1
	case "bolt":
		return 2
	case "arrow":
		return 3
	case "fire":
		return 4
	case "energy":
		return 5
	case "poisonarrow":
		return 6
	case "burstarrow":
		return 7
	case "throwingstar":
		return 8
	case "throwingknife":
		return 9
	case "smallstone":
		return 10
	case "death":
		return 11
	case "holy":
		return 31
	case "ice":
		return 29
	case "earth":
		return 30
	case "suddendeath":
		return 32
	case "diamondarrow":
		return 57
	default:
		return 3 // default arrow
	}
}

// absDamageRange converts a monster attack's raw min/max combat values (which
// are negative for damage) into a positive [lo, hi] damage span. Mirrors
// Monsters::deserializeSpell taking min/max of the two values
// (src/creatures/monsters/monsters.cpp:69).
func absDamageRange(minDamage, maxDamage int) (lo, hi int) {
	a, b := abs(minDamage), abs(maxDamage)
	if a > b {
		a, b = b, a
	}
	return a, b
}

// rollLoot rolls a monster's loot table into a slice of items for the corpse
// container. Mirrors MonsterType:generateLootRoll (data/libs/functions/monstertype.lua)
// / getLootRandom (data/libs/functions/functions.lua): for each entry roll a
// value in [0, MAX_LOOTCHANCE) and drop the item when it is below the entry's
// chance. Stackable stacks (CountMax > 1) get a random count in
// [CountMin, CountMax]. Container entries recurse into their child loot.
//
// Simplifications vs C++ (TODOs for later): loot rate/factor multipliers, the
// dynamic 95..105% jitter, gut/charm bonuses, and unique-item de-duplication are
// omitted (rate assumed 1x). Stackability is inferred from CountMax rather than
// item metadata because the combat engine has no item catalog.
func rollLoot(loot []creatures.LootBlock, lootMultiplier float64) []*Item {
	var out []*Item
	for _, lb := range loot {
		if lb.ID == 0 {
			continue // unresolved name; skip
		}
		chance := int(float64(lb.Chance) * lootMultiplier)
		if rand.Intn(maxLootChance) >= chance {
			continue
		}
		count := uint16(1)
		if lb.CountMax > 1 {
			lo := lb.CountMin
			if lo < 1 {
				lo = 1
			}
			hi := lb.CountMax
			if hi < lo {
				hi = lo
			}
			count = uint16(lo + uint32(rand.Intn(int(hi-lo+1))))
			if count > 100 {
				count = 100
			}
		}
		item := &Item{ID: lb.ID, Count: count}
		if len(lb.ChildLoot) > 0 {
			item.Contents = rollLoot(lb.ChildLoot, lootMultiplier)
		}
		out = append(out, item)
	}
	return out
}

// handleDeath mirrors, at a basic level, Creature::onDeath -> dropCorpse ->
// g_game().removeCreature (src/creatures/creature.cpp): drop a corpse on the
// tile and remove the creature from the world. Loot and experience are left to
// the loot/xp agent (see the TODO hooks below).
func (e *CombatEngine) handleDeath(victim, killer Creature) {
	// Any player targeting the dead creature loses the target (applies to both
	// player and monster deaths).
	for _, p := range e.world.Players() {
		if p.TargetID == victim.GetID() {
			p.SetAttackTarget(0)
			if e.world.OnTargetLost != nil {
				e.world.OnTargetLost(p)
			}
		}
	}

	// Player death: apply the penalty and hand off to the protocol layer for
	// the temple respawn + client refresh.
	if p, ok := victim.(*Player); ok {
		p.ApplyDeathPenaltyWith(e.blessDeathReduction(p, killer))
		
		// Drop loot
		var hasAoL bool
		necklace := p.Inventory[ConstSlotNecklace]
		if necklace != nil && necklace.ID == 2173 {
			hasAoL = true
			p.Inventory[ConstSlotNecklace] = nil // consume Amulet of Loss
		}
		
		blessCount := 0
		for _, b := range p.Blessings {
			if b > 0 { blessCount++ }
		}

		if !hasAoL && blessCount < 5 {
			corpse := &Item{ID: 3058} // Dead human male
			if p.Sex == 0 { // Female
				corpse.ID = 3065
			}
			
			// Backpack always drops in Tibia (if no AoL/Bless)
			if bp := p.Inventory[ConstSlotBackpack]; bp != nil {
				corpse.Contents = append(corpse.Contents, bp)
				p.Inventory[ConstSlotBackpack] = nil
			}
			// Other items have 10% chance
			for i := ConstSlotHead; i <= ConstSlotAmmo; i++ {
				if i == ConstSlotBackpack {
					continue
				}
				if it := p.Inventory[i]; it != nil {
					if rand.Float32() < 0.10 {
						corpse.Contents = append(corpse.Contents, it)
						p.Inventory[i] = nil
					}
				}
			}

			if len(corpse.Contents) > 0 {
				if e.world.AddItem(p.Pos, corpse) && e.world.OnItemAppear != nil {
					e.world.OnItemAppear(p.Pos, corpse)
				}
			}
		}

		if e.world.OnPlayerDeath != nil {
			e.world.OnPlayerDeath(p, killer)
		}
		e.mu.Lock()
		delete(e.lastAttack, victim.GetID())
		e.mu.Unlock()
		return
	}

	// Non-player, non-monster creatures (e.g. NPCs) don't die-and-corpse here.
	if victim.GetCreatureType() != 1 { // 1 == CREATURETYPE_MONSTER
		return
	}

	// Carnage charm: splash damage to monsters adjacent to the dying monster if
	// the killing player had Carnage set on it (before the corpse is placed).
	if vm, ok := victim.(*Monster); ok {
		ck := killer
		if mk, ok := killer.(*Monster); ok && mk.Master != nil {
			ck = mk.Master
		}
		if p, ok := ck.(*Player); ok {
			e.applyCarnageOnDeath(vm, p)
		}
	}

	pos := victim.GetPosition()

	corpseID := uint16(defaultCorpseID)
	if m, ok := victim.(*Monster); ok && m.CorpseID != 0 {
		corpseID = m.CorpseID
	}

	lootMultiplier := 1.0

	// Award experience to the killer. Basic version of Creature::onDeath's
	// experienceMap -> onGainExperience: the whole reward goes to the last hitter
	// (no damage-share split, party, stamina or bonus multipliers yet).
	// (src/creatures/creature.cpp:609-656, player.cpp:3560).
	if m, ok := victim.(*Monster); ok {
		actualKiller := killer
		if mKiller, ok := killer.(*Monster); ok && mKiller.Master != nil {
			actualKiller = mKiller.Master
		}

		if p, ok := actualKiller.(*Player); ok {
			var raceID uint16
			if m.Type != nil {
				raceID = m.Type.RaceID
			}
			if exp := m.Experience(); exp > 0 {
				finalExp := exp
				if e.world.OnGainExperience != nil {
					finalExp = e.world.OnGainExperience(p, victim, exp, exp)
				}
				if bonus, ok := p.GetPrey().GetPreyBonus(raceID, PreyBonus_XPBonus); ok {
					finalExp = uint64(float64(exp) * float64(100+bonus) / 100.0)
				}
				// Hazard system experience bonus
				if m, ok := victim.(*Monster); ok && m.HazardPoints > 0 {
					bonus := hazardExpBonus(m.HazardPoints)
					if bonus > 0 {
						finalExp = uint64(float64(finalExp) * (1.0 + bonus))
					}
				}
				p.AddExperience(finalExp)
				if e.world.OnPlayerStatsChange != nil {
					e.world.OnPlayerStatsChange(p)
				}
				if e.world.OnTextMessage != nil {
					e.world.OnTextMessage(p, 26, finalExp, fmt.Sprintf("You gained %d experience points.", finalExp))
				}
			}
			p.GetTaskHunter().OnKillMonster(raceID)

			// Bosstiary (bosses) / Bestiary (regular monsters): credit the kill
			// and refresh the cyclopedia entry on a stage change.
			if m.Type != nil && m.Type.IsBoss() {
				if p.AddBosstiaryKill(m.Type.BosstiaryRaceID, m.Type.BosstiaryRace, 1) {
					if e.world.OnBosstiaryEntryChanged != nil {
						e.world.OnBosstiaryEntryChanged(p, m.Type.BosstiaryRaceID)
					}
				}
			} else if m.Type != nil && m.Type.RaceID > 0 && m.Type.BestiaryToKill > 0 {
				th := bestiary.Thresholds{
					FirstUnlock:  m.Type.BestiaryFirstUnlock,
					SecondUnlock: m.Type.BestiarySecondUnlock,
					ToKill:       m.Type.BestiaryToKill,
				}
				if p.AddBestiaryKill(m.Type.RaceID, th, m.Type.BestiaryCharmsPoints, 1) {
					if e.world.OnBestiaryEntryChanged != nil {
						e.world.OnBestiaryEntryChanged(p, m.Type.RaceID)
					}
				}
			}

			if bonus, ok := p.GetPrey().GetPreyBonus(raceID, PreyBonus_ImprovedLoot); ok {
				lootMultiplier *= float64(100+bonus) / 100.0
			}

			if m.ForgeClassification == ForgeClassifications_Fiendish {
				// Grant dust to killer player
				p.ForgeDusts += uint64(3 + rand.Intn(3)) // 3-5 dust
			}
		}
	}

	if e.world.OnCreatureDied != nil {
		e.world.OnCreatureDied(victim)
	}

	// Drop the corpse first (as in dropCorpse, which adds the corpse while the
	// creature is still on the tile), then remove the creature. The corpse is a
	// container whose Contents are the rolled loot table.
	corpse := &Item{ID: corpseID, Count: 1}
	if m, ok := victim.(*Monster); ok && m.Type != nil && m.Type.Flags.LootDrop {
		corpse.Contents = rollLoot(m.Type.Loot, lootMultiplier)
	}
	if e.world.AddItem(pos, corpse) && e.world.OnItemAppear != nil {
		e.world.OnItemAppear(pos, corpse)
	}

	e.world.RemoveCreature(victim.GetID())

	// Forget this creature's attack timer.
	e.mu.Lock()
	delete(e.lastAttack, victim.GetID())
	e.mu.Unlock()
}

func randomRange(min, max int) int32 {
	if min >= max {
		return int32(min)
	}
	return int32(min + rand.Intn(max-min+1))
}

func (e *CombatEngine) doMonsterAttack(m *Monster, target Creature) {
	if m.Type == nil || len(m.Type.Attacks) == 0 {
		e.doMeleeHit(combat.NewCombat(), m, target)
		return
	}

	var spells []creatures.MonsterAttack

	for i := range m.Type.Attacks {
		atk := &m.Type.Attacks[i]
		if !atk.IsMelee() {
			spells = append(spells, *atk)
		}
	}

	ap, tp := m.GetPosition(), target.GetPosition()
	dist := chebyshevDistance(ap, tp)

	for _, s := range spells {
		chanceRoll := rand.Intn(100)
		if chanceRoll < s.Chance {
			maxRange := s.Range
			if maxRange <= 0 {
				maxRange = 1
			}
			if dist <= maxRange {
				e.executeMonsterSpell(m, target, s)
				return
			}
		}
	}

	if dist <= 1 {
		e.doMeleeHit(combat.NewCombat(), m, target)
	}
}

func (e *CombatEngine) executeMonsterSpell(m *Monster, target Creature, s creatures.MonsterAttack) {
	if e.world.OnCastSpell != nil {
		if e.world.OnCastSpell(s.Name, m, target) {
			// Lua spell executed successfully. All damage, conditions and effects are handled there.
			return
		}
	}

	if s.ShootEffect != 0 && e.world.OnDistanceEffect != nil {
		e.world.OnDistanceEffect(m.GetPosition(), target.GetPosition(), s.ShootEffect)
	}

	minDmg := s.MinDamage
	maxDmg := s.MaxDamage
	if minDmg < 0 {
		minDmg = -minDmg
	}
	if maxDmg < 0 {
		maxDmg = -maxDmg
	}
	if minDmg > maxDmg {
		minDmg, maxDmg = maxDmg, minDmg
	}

	dmg := minDmg
	if maxDmg > minDmg {
		dmg = minDmg + rand.Intn(maxDmg-minDmg+1)
	}

	effect := s.Effect
	if effect == 0 {
		effect = uint16(effectDrawBlood)
	}

	c := combat.NewCombat()
	cType := combat.CombatPhysical
	sNameLower := strings.ToLower(s.Name)
	sTypeLower := strings.ToLower(s.CombatType)
	if strings.Contains(sTypeLower, "fire") || strings.Contains(sNameLower, "fire") {
		cType = combat.CombatFire
	} else if strings.Contains(sTypeLower, "ice") || strings.Contains(sNameLower, "ice") {
		cType = combat.CombatIce
	} else if strings.Contains(sTypeLower, "energy") || strings.Contains(sNameLower, "energy") {
		cType = combat.CombatEnergy
	} else if strings.Contains(sTypeLower, "poison") || strings.Contains(sNameLower, "poison") || strings.Contains(sNameLower, "earth") {
		cType = combat.CombatEarth
	} else if strings.Contains(sTypeLower, "death") || strings.Contains(sNameLower, "death") {
		cType = combat.CombatDeath
	} else if strings.Contains(sTypeLower, "holy") || strings.Contains(sNameLower, "holy") {
		cType = combat.CombatHoly
	}

	c.SetParam(combat.CombatParamType, uint32(cType))

	if !c.DoCombatHealth(adaptCreature(m), adaptCreature(target), combat.CombatDamage{
		PrimaryType:  cType,
		PrimaryValue: int32(dmg),
		Origin:       combat.OriginSpell,
	}) {
		return
	}

	if e.world.OnCreatureHealthChange != nil {
		e.world.OnCreatureHealthChange(target)
	}

	if e.world.OnMagicEffect != nil && effect != 0 {
		e.world.OnMagicEffect(target.GetPosition(), effect)
	}

	if e.world.OnCombatHit != nil {
		e.world.OnCombatHit(m, target, int32(dmg), effect)
	}

	if target.GetHealth() == 0 {
		e.handleDeath(target, m)
	} else if tp, ok := target.(*Player); ok {
		e.applyDefensiveCharmRune(m, tp, int32(dmg))
	}
}

