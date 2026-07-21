package game

import (
	"math/rand"
	"sync"
	"time"

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
func (e *CombatEngine) Start() {
	GlobalDispatcher.AddEvent(combatTickInterval, e.tick)
}

func (e *CombatEngine) tick() {
	e.resolveAttacks()
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
	// Monsters never harm players who cannot be attacked (staff/ghost),
	// mirroring PlayerFlags_t::CannotBeAttacked.
	if _, atkIsMonster := attacker.(*Monster); atkIsMonster {
		if tp, ok := target.(*Player); ok && tp.CannotBeAttacked() {
			return
		}
	}
	// Melee reach: adjacent on the same floor, matching Position::areInRange<1,1>
	// used by Weapon::useFist (src/items/weapons/weapons.cpp).
	ap, tp := attacker.GetPosition(), target.GetPosition()
	if ap.Z != tp.Z || chebyshevDistance(ap, tp) > 1 {
		return
	}
	if !e.ready(attacker.GetID(), interval) {
		return
	}
	e.doMeleeHit(combat.NewCombat(), attacker, target)
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
	dmg := e.meleeDamage(attacker)

	// Apply through the combat engine so the armor/condition/blocking hooks the
	// spells agent will need are exercised. Combat::doCombatHealth negates the
	// primary value for non-healing damage and calls target.ChangeHealth,
	// mirroring Creature::drainHealth -> changeHealth(-damage)
	// (src/creatures/combat/combat.cpp, src/creatures/creature.cpp).
	c.SetParam(combat.CombatParamType, uint32(combat.CombatPhysical))
	c.DoCombatHealth(adaptCreature(attacker), adaptCreature(target), combat.CombatDamage{
		PrimaryType:  combat.CombatPhysical,
		PrimaryValue: int32(dmg),
		Origin:       combat.OriginMelee,
	})

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
	}
}

// meleeDamage computes one melee hit's damage using the existing Canary-style
// formula in combat.CalculateMeleeDamage.
//
// NOTE: combat.CalculateMeleeDamage uses the classic (skill+4)*attack*0.0605
// formula. The modern Canary formula is
// round(0.085 * attackFactor * attackValue * attackSkill + level/5)
// (Weapons::getMaxWeaponDamage, src/items/weapons/weapons.cpp:94). The task
// directs using CalculateMeleeDamage; revisit the formula for full fidelity.
func (e *CombatEngine) meleeDamage(attacker Creature) int {
	switch a := attacker.(type) {
	case *Player:
		skill := int(a.Skills[SkillFist])
		voc := vocations.GetVocation(uint32(a.Vocation))
		return CalculateMeleeDamage(fistAttackValue, skill, 0, voc, int(a.Level))
	case *Monster:
		// Use the monster's real melee attack block. The Lua attack stores raw
		// combat values (minDamage..maxDamage, typically <= 0, e.g. rat 0..-8);
		// the damage dealt is the magnitude. Mirrors Monster::doAttacking picking
		// the melee spellBlock and rolling minCombatValue..maxCombatValue
		// (src/creatures/monsters/monster.cpp:1753, monsters.cpp:57-70).
		atk := a.MeleeAttack()
		if atk == nil {
			// Fall back to a small default so monsters without parsed attack
			// data can still fight back.
			return rand.Intn(defaultMonsterAttackValue + 1)
		}
		// Per-swing chance gate; a miss deals no damage (poff effect).
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
func rollLoot(loot []creatures.LootBlock) []*Item {
	var out []*Item
	for _, lb := range loot {
		if lb.ID == 0 {
			continue // unresolved name; skip
		}
		if rand.Intn(maxLootChance) >= int(lb.Chance) {
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
		}
		item := &Item{ID: lb.ID, Count: count}
		if len(lb.ChildLoot) > 0 {
			item.Contents = rollLoot(lb.ChildLoot)
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
		p.ApplyDeathPenalty()
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

	pos := victim.GetPosition()

	corpseID := uint16(defaultCorpseID)
	if m, ok := victim.(*Monster); ok && m.CorpseID != 0 {
		corpseID = m.CorpseID
	}

	// Award experience to the killer. Basic version of Creature::onDeath's
	// experienceMap -> onGainExperience: the whole reward goes to the last hitter
	// (no damage-share split, party, stamina or bonus multipliers yet).
	// (src/creatures/creature.cpp:609-656, player.cpp:3560).
	if m, ok := victim.(*Monster); ok {
		if p, ok := killer.(*Player); ok {
			if exp := m.Experience(); exp > 0 {
				// TODO(loot/xp): split experience across all damagers, apply
				// party sharing and rate/stamina/VIP multipliers.
				p.AddExperience(exp)
				if e.world.OnPlayerStatsChange != nil {
					e.world.OnPlayerStatsChange(p)
				}
			}
		}
	}

	// Drop the corpse first (as in dropCorpse, which adds the corpse while the
	// creature is still on the tile), then remove the creature. The corpse is a
	// container whose Contents are the rolled loot table.
	corpse := &Item{ID: corpseID, Count: 1}
	if m, ok := victim.(*Monster); ok && m.Type != nil && m.Type.Flags.LootDrop {
		corpse.Contents = rollLoot(m.Type.Loot)
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
