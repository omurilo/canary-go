package game

import (
	"strings"
	"time"

	"github.com/omurilo/canary-go/internal/creatures"
)

// SpawnMonster, ported from src/creatures/monsters/spawns/spawn_monster.cpp.
//
// One SpawnBlock here is one C++ SpawnMonster: a centre, a radius, and a set of
// numbered slots each holding a position and the monster types that may occupy
// it. SpawnEngine is SpawnsMonster, the collection.
//
// The port had the per-slot occupancy right but the spawn decision wrong in a
// way that touches nearly every monster in the game. See CheckSpawnMonster.

const (
	// MONSTER_MINSPAWN_INTERVAL / MONSTER_MAXSPAWN_INTERVAL: the bounds a
	// spawntime is clamped into, in case a spawn file asks for something absurd.
	monsterMinSpawnInterval = 10 * time.Second
	monsterMaxSpawnInterval = 24 * time.Hour
)

// IsInZone is SpawnsMonster::isInZone (spawn_monster.cpp:157).
//
// It is a SQUARE, not a circle: |dx| <= radius AND |dy| <= radius. A circular
// test rejects the corners of the spawn area, so monsters never appear there
// and the spawn is quietly smaller than the map says.
//
// A radius of -1 means unbounded.
func IsInZone(centerPos Position, radius int, pos Position) bool {
	if radius == -1 {
		return true
	}
	return abs(int(pos.X)-int(centerPos.X)) <= radius &&
		abs(int(pos.Y)-int(centerPos.Y)) <= radius
}

// IsInSpawnMonsterZone is SpawnMonster::isInSpawnMonsterZone (spawn_monster.cpp:207).
func (b *SpawnBlock) IsInSpawnMonsterZone(pos Position) bool {
	return IsInZone(b.centerPos, b.radius, pos)
}

// GetCenterPos is SpawnMonster::getCenterPos (spawn_monster.cpp:343).
func (b *SpawnBlock) GetCenterPos() Position { return b.centerPos }

// FindPlayer is SpawnMonster::findPlayer (spawn_monster.cpp:200): is there a
// player who can see this tile.
//
// Upstream asks the spectator list, which is the client viewport. The port used
// a ten-tile circle, which is both a different shape and a different size, so
// spawns behaved differently at the edges than the same spawn does in C++.
func (b *SpawnBlock) FindPlayer(w *World, pos Position) bool {
	if w == nil {
		return false
	}
	for _, p := range w.Spectators(pos, 0) {
		if p == nil || p.CannotBeAttacked() {
			continue
		}
		return true
	}
	return false
}

// CheckSpawnMonster is SpawnMonster::checkSpawnMonster (spawn_monster.cpp:278).
//
// The branch the port had backwards:
//
//	if (!canSpawn || (isBlockable && findPlayer(pos))) { lastSpawn = now; skip }
//	if (isBlockable) spawnMonster(...) else scheduleSpawn(..., 3 * 1400ms)
//
// A nearby player only holds back a BLOCKABLE monster. Everything else spawns
// regardless, after a short delay that shows a teleport effect first so the
// player sees it coming. The port applied the player check to every monster and
// spawned all of them instantly.
//
// 1647 of the 1655 datapack monsters declare isBlockable = false. So the old
// behaviour was: standing in a spawn stopped it respawning — for essentially
// every monster in the game — and when it did fire, the monster appeared out of
// nowhere with no effect.
func (b *SpawnBlock) CheckSpawnMonster(w *World, now time.Time) {
	b.Cleanup(now)

	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	for id, sb := range b.blocks {
		if _, alive := b.spawned[id]; alive {
			continue
		}
		mType := sb.getMonsterType()
		if mType == nil {
			continue
		}

		if mType.Flags.IsBlockable && b.FindPlayer(w, sb.pos) {
			// Push the clock forward: the respawn timer restarts while a player
			// stands there rather than firing the instant they leave.
			sb.lastSpawn = now
			continue
		}
		if !sb.lastSpawn.IsZero() && now.Before(sb.lastSpawn.Add(sb.interval)) {
			continue
		}

		if mType.Flags.IsBlockable {
			b.spawnMonsterLocked(w, id, sb, mType, now)
			continue
		}
		b.ScheduleSpawn(w, id, sb, mType, 3*nonBlockableInterval, false)
	}
}

// ScheduleSpawn is SpawnMonster::scheduleSpawn (spawn_monster.cpp:317): count
// down in 1400ms steps, showing a teleport effect at each one, then spawn.
//
// The effect on every step is upstream's, not just on the last: it is what
// makes a non-blockable monster visibly materialise instead of blinking in.
func (b *SpawnBlock) ScheduleSpawn(w *World, id uint32, sb *spawnBlock, mType *creatures.MonsterType, interval time.Duration, startup bool) {
	if interval <= 0 {
		b.stateMu.Lock()
		b.spawnMonsterLocked(w, id, sb, mType, time.Now())
		b.stateMu.Unlock()
		return
	}
	if w != nil && w.OnMagicEffect != nil {
		w.OnMagicEffect(sb.pos, constMeTeleport)
	}
	GlobalDispatcher.AddEvent(nonBlockableInterval, func() {
		b.ScheduleSpawn(w, id, sb, mType, interval-nonBlockableInterval, startup)
	})
}

// SpawnMonster is SpawnMonster::spawnMonster (spawn_monster.cpp:211).
func (b *SpawnBlock) SpawnMonster(w *World, id uint32, sb *spawnBlock, mType *creatures.MonsterType, startup bool) bool {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	return b.spawnMonsterLocked(w, id, sb, mType, time.Now())
}

// spawnMonsterLocked is the body of spawnMonster, with stateMu already held.
//
// The slot check is repeated here and not only in the caller: ScheduleSpawn
// takes 4.2 seconds to run down, and the slot can be filled in the meantime by
// a script or a reload.
func (b *SpawnBlock) spawnMonsterLocked(w *World, id uint32, sb *spawnBlock, mType *creatures.MonsterType, now time.Time) bool {
	if _, alive := b.spawned[id]; alive {
		return false
	}
	creature := b.engine.spawnCreatureInBlock(b, id, sb, mType, now)
	if creature == nil {
		return false
	}
	b.spawned[id] = creature
	if m, ok := creature.(*Monster); ok {
		m.SpawnPosition = sb.pos
		m.OnSpawn(w, sb.pos)
	}
	return true
}

// Cleanup is SpawnMonster::cleanup (spawn_monster.cpp:328): forget the monsters
// that are gone and restart their slot's respawn clock.
//
// The clock restart belongs here, at the moment the corpse is noticed, not at
// the moment of death — that is what makes the respawn interval count from the
// kill rather than from whenever the sweep next runs.
func (b *SpawnBlock) Cleanup(now time.Time) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	// isRemoved(), not health. A monster is freed from its slot when it leaves
	// the world, which death does a moment later via RemoveCreature. Keying on
	// health instead frees the slot the instant the killing blow lands — and
	// frees it forever for any monster whose type has no maxHealth set.
	for id, c := range b.spawned {
		if c != nil && b.stillInWorld(c) {
			continue
		}
		if sb, ok := b.blocks[id]; ok {
			sb.lastSpawn = now
		}
		delete(b.spawned, id)
	}
}

// Startup is SpawnMonster::startup (spawn_monster.cpp:240): fill every slot at
// boot, bypassing the respawn timers.
//
// startup = true means the placement skips the appear broadcast — there is
// nobody online to receive it — which is why it does not go through the normal
// spawn path.
func (b *SpawnBlock) Startup(w *World, delayed bool) {
	b.stateMu.Lock()
	slots := make([]uint32, 0, len(b.blocks))
	for id := range b.blocks {
		slots = append(slots, id)
	}
	b.stateMu.Unlock()

	for _, id := range slots {
		sb := b.blocks[id]
		mType := sb.getMonsterType()
		if mType == nil {
			continue
		}
		if delayed {
			blockID, block := id, sb
			GlobalDispatcher.AddEvent(0, func() {
				b.ScheduleSpawn(w, blockID, block, mType, 0, true)
			})
			continue
		}
		b.ScheduleSpawn(w, id, sb, mType, 0, true)
	}
}

// AddMonster is SpawnMonster::addMonster (spawn_monster.cpp:347).
//
// Two rules upstream enforces and the port did not: a slot may not hold the
// same monster twice, and a boss may not share a slot with anything. A boss
// sharing a slot would be picked at random against the other entries, so the
// boss would spawn only some of the time.
//
// The interval is clamped into [MIN, MAX] rather than trusted, because a spawn
// file with a zero spawntime otherwise respawns the monster on every sweep.
func (b *SpawnBlock) AddMonster(name string, pos Position, dir Direction, scheduleInterval time.Duration, weight uint32) bool {
	if b.engine == nil || b.engine.Types == nil {
		return false
	}
	mType := b.engine.Types.Monsters[lowerName(name)]
	if mType == nil {
		return false
	}
	if scheduleInterval < monsterMinSpawnInterval {
		scheduleInterval = monsterMinSpawnInterval
	} else if scheduleInterval > monsterMaxSpawnInterval {
		scheduleInterval = monsterMaxSpawnInterval
	}

	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	var slot *spawnBlock
	for _, existing := range b.blocks {
		if existing.pos == pos {
			slot = existing
			break
		}
	}
	if slot != nil {
		if _, dup := slot.monsterTypes[mType]; dup {
			return false
		}
		if mType.IsBoss() && len(slot.monsterTypes) > 0 {
			return false
		}
		if slotHasBoss(slot) {
			return false
		}
		slot.monsterTypes[mType] = weight
		slot.direction = dir
		slot.interval = scheduleInterval
		return true
	}

	id := b.engine.nextID()
	b.blocks[id] = &spawnBlock{
		pos:          pos,
		monsterTypes: map[*creatures.MonsterType]uint32{mType: weight},
		interval:     scheduleInterval,
		direction:    dir,
	}
	return true
}

// RemoveMonster is SpawnMonster::removeMonster (spawn_monster.cpp:413): free
// the slot the monster occupied, without touching the slot's definition.
func (b *SpawnBlock) RemoveMonster(m *Monster) {
	if m == nil {
		return
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	for id, c := range b.spawned {
		if c != nil && c.GetID() == m.GetID() {
			delete(b.spawned, id)
			return
		}
	}
}

// RemoveMonsters is SpawnMonster::removeMonsters (spawn_monster.cpp:424): drop
// the slot definitions as well as the occupants, which is what a reload needs.
func (b *SpawnBlock) RemoveMonsters() {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.blocks = make(map[uint32]*spawnBlock)
	b.spawned = make(map[uint32]Creature)
}

// SetMonsterVariant is SpawnMonster::setMonsterVariant (spawn_monster.cpp:429):
// swap every slot's monster types for their "variant|name" counterparts.
//
// A type with no variant is DROPPED, not kept: upstream builds a fresh map and
// only inserts what resolved. A zone declaring a variant that a monster has no
// version of makes that monster stop spawning there, which is the intent.
func (b *SpawnBlock) SetMonsterVariant(variant string) {
	if b.engine == nil || b.engine.Types == nil {
		return
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	for _, slot := range b.blocks {
		replaced := make(map[*creatures.MonsterType]uint32)
		for mType, weight := range slot.monsterTypes {
			if mType == nil || mType.Name == "" {
				continue
			}
			if v := b.engine.Types.Monsters[lowerName(variant+"|"+mType.Name)]; v != nil {
				replaced[v] = weight
			}
		}
		slot.monsterTypes = replaced
	}
}

// StartSpawnMonsterCheck is SpawnMonster::startSpawnMonsterCheck (spawn_monster.cpp:165).
func (b *SpawnBlock) StartSpawnMonsterCheck() { b.checkActive = true }

// StopEvent is SpawnMonster::stopEvent (spawn_monster.cpp:450).
func (b *SpawnBlock) StopEvent() { b.checkActive = false }

func slotHasBoss(slot *spawnBlock) bool {
	for mType := range slot.monsterTypes {
		if mType != nil && mType.IsBoss() {
			return true
		}
	}
	return false
}

// lowerName is the case folding the monster registry is keyed by.
func lowerName(name string) string { return strings.ToLower(name) }

// stillInWorld is Creature::isRemoved, inverted: the world is the registry a
// creature is removed from.
func (b *SpawnBlock) stillInWorld(c Creature) bool {
	if b.engine == nil || b.engine.world == nil {
		return true
	}
	return b.engine.world.CreatureByID(c.GetID()) != nil
}
