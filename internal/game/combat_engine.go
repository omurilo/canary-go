package game

import (
	"math/rand"
	"sync"
	"time"

	"github.com/opentibiabr/canary-go/internal/game/combat"
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
	// TODO(monster-data): always resolve Monster.CorpseID from monster data.
	defaultCorpseID = 5964
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
		e.tryAttack(p, e.playerTarget(p), defaultPlayerAttackSpeed)
	}
	// Monsters attack their AI target (set by the AI engine).
	for _, c := range e.world.Creatures() {
		if m, ok := c.(*Monster); ok {
			e.tryAttack(m, m.GetTarget(), defaultMonsterAttackSpeed)
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
	// Melee reach: adjacent on the same floor, matching Position::areInRange<1,1>
	// used by Weapon::useFist (src/items/weapons/weapons.cpp).
	ap, tp := attacker.GetPosition(), target.GetPosition()
	if ap.Z != tp.Z || chebyshevDistance(ap, tp) > 1 {
		return
	}
	if !e.ready(attacker.GetID(), interval) {
		return
	}
	e.doMeleeHit(attacker, target)
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

func (e *CombatEngine) doMeleeHit(attacker, target Creature) {
	dmg := e.meleeDamage(attacker)

	// Apply through the combat engine so the armor/condition/blocking hooks the
	// spells agent will need are exercised. Combat::doCombatHealth negates the
	// primary value for non-healing damage and calls target.ChangeHealth,
	// mirroring Creature::drainHealth -> changeHealth(-damage)
	// (src/creatures/combat/combat.cpp, src/creatures/creature.cpp).
	c := combat.NewCombat()
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
		// TODO(vocations): pass the player's Vocation for the damage multiplier
		// once a vocation registry is available; nil == x1.0 for now.
		return combat.CalculateMeleeDamage(fistAttackValue, skill, 0, nil)
	case *Monster:
		// TODO(monster-data): use the monster's attack/skill data. Flat default
		// so monsters can still fight back.
		return rand.Intn(defaultMonsterAttackValue + 1)
	default:
		return 0
	}
}

// handleDeath mirrors, at a basic level, Creature::onDeath -> dropCorpse ->
// g_game().removeCreature (src/creatures/creature.cpp): drop a corpse on the
// tile and remove the creature from the world. Loot and experience are left to
// the loot/xp agent (see the TODO hooks below).
func (e *CombatEngine) handleDeath(victim, killer Creature) {
	// Player death is a separate milestone (respawn at temple, skill loss);
	// only monsters die-and-corpse here.
	if victim.GetCreatureType() != 1 { // 1 == CREATURETYPE_MONSTER
		return
	}

	pos := victim.GetPosition()

	corpseID := uint16(defaultCorpseID)
	if m, ok := victim.(*Monster); ok && m.CorpseID != 0 {
		corpseID = m.CorpseID
	}

	// Any player targeting the dead creature loses the target.
	for _, p := range e.world.Players() {
		if p.TargetID == victim.GetID() {
			p.SetAttackTarget(0)
			if e.world.OnTargetLost != nil {
				e.world.OnTargetLost(p)
			}
		}
	}

	// TODO(loot/xp): mirror Creature::onDeath's damageMap -> experience/killers
	// and Creature::dropCorpse's dropLoot(corpse->getContainer(), ...) here.
	// `killer` is passed through so that hook has the last-hit creature.
	_ = killer

	// Drop the corpse first (as in dropCorpse, which adds the corpse while the
	// creature is still on the tile), then remove the creature.
	corpse := &Item{ID: corpseID, Count: 1}
	if e.world.Map.AddItem(pos, corpse) && e.world.OnItemAppear != nil {
		e.world.OnItemAppear(pos, corpse)
	}

	e.world.RemoveCreature(victim.GetID())

	// Forget this creature's attack timer.
	e.mu.Lock()
	delete(e.lastAttack, victim.GetID())
	e.mu.Unlock()
}
