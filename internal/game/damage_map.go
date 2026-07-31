package game

import (
	"sync"
	"time"
)

// Per-attacker damage tracking, the port of Creature::damageMap
// (src/creatures/creature.hpp:689). Nothing tracked who hurt a creature, so on
// death the server had no way to name the attacker who dealt the most damage: the
// player_deaths row got the last hitter for both fields, experience could not be
// split by contribution, and loot ownership had nothing to key on.

// DamageBlock is CountBlock_t: how much an attacker has dealt in total, and when
// they last landed a hit.
type DamageBlock struct {
	Total int32
	// LastHit is a unix millisecond timestamp, matching OTSYS_TIME().
	LastHit int64
}

// damageTracker is embedded in BaseCreature and Player. Its zero value is usable.
type damageTracker struct {
	mu                sync.RWMutex
	damage            map[uint32]DamageBlock
	lastHitCreatureID uint32
}

// AddDamagePoints records damage from an attacker, mirroring
// Creature::addDamagePoints: non-positive damage is ignored, the running total
// accumulates, the timestamp is refreshed on every hit, and the attacker becomes
// the last hitter.
func (d *damageTracker) AddDamagePoints(attackerID uint32, points int32) {
	if points <= 0 || attackerID == 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.damage == nil {
		d.damage = map[uint32]DamageBlock{}
	}
	block := d.damage[attackerID]
	block.Total += points
	block.LastHit = time.Now().UnixMilli()
	d.damage[attackerID] = block
	d.lastHitCreatureID = attackerID
}

// DamageMap returns a copy of the per-attacker totals.
func (d *damageTracker) DamageMap() map[uint32]DamageBlock {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make(map[uint32]DamageBlock, len(d.damage))
	for id, block := range d.damage {
		out[id] = block
	}
	return out
}

// LastHitCreatureID is the attacker who landed the final blow, or 0.
func (d *damageTracker) LastHitCreatureID() uint32 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastHitCreatureID
}

// ClearDamage drops the tracking, for a creature being reused or revived.
func (d *damageTracker) ClearDamage() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.damage = nil
	d.lastHitCreatureID = 0
}

// MostDamageAttackerID picks the attacker with the highest total whose last hit is
// still within inFightMillis, the selection Creature::onDeath makes
// (src/creatures/creature.cpp): a bigger total that has gone stale loses to a
// smaller recent one, which is what stops someone who left the fight long ago from
// being blamed for the kill.
//
// Ties go to the lower creature id so the result is deterministic; C++ iterates a
// std::map keyed by id and takes the first strict maximum, which is the same rule.
func (d *damageTracker) MostDamageAttackerID(inFightMillis int64) uint32 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now().UnixMilli()
	var best uint32
	var bestTotal int32
	for id, block := range d.damage {
		if id == 0 || block.Total == 0 || block.LastHit == 0 {
			continue
		}
		if now-block.LastHit > inFightMillis {
			continue
		}
		if block.Total > bestTotal || (block.Total == bestTotal && best != 0 && id < best) {
			bestTotal = block.Total
			best = id
		}
	}
	return best
}

// DefaultInFightMillis mirrors PZ_LOCKED, the window Creature::onDeath uses to
// decide whether an attacker still counts (config.lua `pzLocked`, 60s).
const DefaultInFightMillis int64 = 60 * 1000

// damageTracked is what a creature must expose for its death to be attributed.
// BaseCreature and Player both embed damageTracker and so satisfy it.
type damageTracked interface {
	LastHitCreatureID() uint32
	MostDamageAttackerID(inFightMillis int64) uint32
}

// Killers resolves the two creatures a death is attributed to: the one who landed
// the last hit, and the one who dealt the most damage inside the in-fight window.
// Either may be nil — a creature that drowns has no killer at all.
func (w *World) Killers(victim Creature) (lastHit Creature, mostDamage Creature) {
	tracker, ok := victim.(damageTracked)
	if !ok {
		return nil, nil
	}
	if id := tracker.LastHitCreatureID(); id != 0 {
		lastHit = w.CreatureByID(id)
	}
	if id := tracker.MostDamageAttackerID(DefaultInFightMillis); id != 0 {
		mostDamage = w.CreatureByID(id)
	}
	// C++ falls back to the last hitter when nobody qualifies on damage, which is
	// what happens when every contributor's last hit has gone stale.
	if mostDamage == nil {
		mostDamage = lastHit
	}
	return lastHit, mostDamage
}
