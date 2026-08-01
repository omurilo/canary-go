package game

import (
	"math/rand"
	"strings"

	"github.com/opentibiabr/canary-go/internal/creatures"
)

// The Monster::onThink pipeline, ported from src/creatures/monsters/monster.cpp.
//
// Upstream splits a monster's per-tick work into four independent timers —
// target, defense, yell, sound — each accumulating its own tick counter and
// firing on its own interval. None of them existed here: the AI loop picked a
// target, walked, and did nothing else. A monster in the port never yelled,
// never healed itself, never summoned, never re-picked a target once it had
// one, and never turned to face what it was fighting.
//
// The four counters live on Monster rather than in the engine because upstream
// keeps them on the creature and they have to survive the monster being skipped
// on a tick (out of sight, idle) without resetting.

// TalkType values used by onThinkYell, matching the NPC side.
const (
	talkTypeMonsterSay  byte = 36 // TALKTYPE_MONSTER_SAY
	talkTypeMonsterYell byte = 37 // TALKTYPE_MONSTER_YELL
)

// OnThinkYell is Monster::onThinkYell (monster.cpp:2273). yellSpeedTicks of 0
// disables the whole thing; the chance is rolled only once the interval is up,
// so a missed roll costs a full interval rather than being retried next tick.
func (m *Monster) OnThinkYell(w *World, interval uint32) {
	if m.Type == nil || m.Type.YellInterval == 0 {
		return
	}
	m.yellTicks += int(interval)
	if m.yellTicks < m.Type.YellInterval {
		return
	}
	m.yellTicks = 0

	if len(m.Type.Voices) == 0 || m.Type.YellChance < rand.Intn(100)+1 {
		return
	}
	voice := m.Type.Voices[rand.Intn(len(m.Type.Voices))]
	talkType := talkTypeMonsterSay
	if voice.Yell {
		talkType = talkTypeMonsterYell
	}
	if w != nil && w.OnCreatureSay != nil {
		w.OnCreatureSay(m, talkType, voice.Text)
	}
}

// OnThinkTarget is Monster::onThinkTarget (monster.cpp:2140): the timer that
// makes a monster reconsider who it is fighting. changeTargetSpeed of 0 — or a
// summon, which follows its master's target — means it never does.
//
// The cooldown is a second timer that runs after a change and blocks further
// ones, which is what stops a monster with a short interval from thrashing
// between two players standing side by side.
func (m *Monster) OnThinkTarget(w *World, interval uint32) {
	if m.Type == nil || m.Master != nil {
		return
	}
	speed := m.Type.ChangeTargetInterval
	if speed == 0 {
		return
	}

	canChange := true
	if m.challengeFocusDuration > 0 {
		m.challengeFocusDuration -= int(interval)
		canChange = false
		if m.challengeFocusDuration < 0 {
			m.challengeFocusDuration = 0
		}
	}
	if m.targetChangeCooldown > 0 {
		m.targetChangeCooldown -= int(interval)
		if m.targetChangeCooldown <= 0 {
			m.targetChangeCooldown = 0
			m.targetChangeTicks = speed
		} else {
			canChange = false
		}
	}
	if !canChange {
		return
	}

	m.targetChangeTicks += int(interval)
	if m.targetChangeTicks < speed {
		return
	}
	m.targetChangeTicks = 0
	m.targetChangeCooldown = speed
	m.challengeFocusDuration = 0

	if m.Type.ChangeTargetChance < rand.Intn(100)+1 {
		return
	}
	// A melee monster re-rolls at random; anything that fights at range looks
	// for the nearest instead (monster.cpp:2185-2187).
	searchType := TargetSearchNearest
	if m.Type.Flags.TargetDistance <= 1 {
		searchType = TargetSearchRandom
	}
	m.SearchTarget(w, searchType)
}

// OnThinkDefense is Monster::onThinkDefense (monster.cpp:2201): the defensive
// spells a monster casts on itself, then its summons.
//
// The tick arithmetic is upstream's and is easy to get wrong. defenseTicks is a
// single counter shared by every block. A block whose speed has not been
// reached yet holds resetTicks down so the counter keeps growing for the slower
// blocks; the `defenseTicks % speed >= interval` guard is what makes a block
// fire once per period instead of on every tick after the first.
func (m *Monster) OnThinkDefense(w *World, interval uint32) {
	if m.Type == nil {
		return
	}
	resetTicks := true
	m.defenseTicks += int(interval)

	for i := range m.Type.Defenses {
		block := &m.Type.Defenses[i]
		speed := block.Interval
		if speed <= 0 {
			continue
		}
		if speed > m.defenseTicks {
			resetTicks = false
			continue
		}
		if m.defenseTicks%speed >= int(interval) {
			continue // already fired this round
		}
		if block.Chance < rand.Intn(100)+1 {
			continue
		}
		m.castDefenseSpell(w, block)
	}

	if m.Master == nil {
		resetTicks = m.thinkSummons(w, interval) && resetTicks
	}

	if resetTicks {
		m.defenseTicks = 0
	}
}

// castDefenseSpell applies a defensive block to the monster itself. Only the
// healing case has an effect today; the rest are routed through the same Lua
// hook the attack spells use so a scripted defense is not silently dropped.
func (m *Monster) castDefenseSpell(w *World, block *creatures.MonsterAttack) {
	if w == nil {
		return
	}
	if strings.EqualFold(block.CombatType, "healing") || block.MinDamage > 0 || block.MaxDamage > 0 {
		lo, hi := block.MinDamage, block.MaxDamage
		if lo > hi {
			lo, hi = hi, lo
		}
		heal := lo
		if hi > lo {
			heal = lo + rand.Intn(hi-lo+1)
		}
		if heal > 0 {
			m.AddHealth(int32(heal))
			if w.OnCreatureHealthChange != nil {
				w.OnCreatureHealthChange(m)
			}
		}
	}
	if block.Effect != 0 && w.OnMagicEffect != nil {
		w.OnMagicEffect(m.GetPosition(), block.Effect)
	}
	if block.Name != "" && w.OnCastSpell != nil {
		w.OnCastSpell(block.Name, m, m)
	}
}

// thinkSummons is the summon arm of onThinkDefense (monster.cpp:2223-2270). It
// returns whether the defense tick counter may be reset — a summon block whose
// interval has not elapsed keeps it running, exactly like a spell block.
func (m *Monster) thinkSummons(w *World, interval uint32) bool {
	resetTicks := true
	if w == nil || len(m.Type.Summons) == 0 || m.Type.MaxSummons <= 0 {
		return resetTicks
	}
	if len(m.Summons) >= m.Type.MaxSummons {
		return resetTicks
	}

	for i := range m.Type.Summons {
		s := &m.Type.Summons[i]
		if s.Interval <= 0 {
			continue
		}
		if s.Interval > m.defenseTicks {
			resetTicks = false
			continue
		}
		if len(m.Summons) >= m.Type.MaxSummons {
			continue
		}
		if m.defenseTicks%s.Interval >= int(interval) {
			continue
		}
		alive := 0
		for _, existing := range m.Summons {
			if existing != nil && strings.EqualFold(existing.GetName(), s.Name) {
				alive++
			}
		}
		if alive >= s.Count {
			continue
		}
		if s.Chance < rand.Intn(100)+1 {
			continue
		}
		m.placeSummon(w, s)
	}
	return resetTicks
}

// placeSummon creates one summon next to its master and links the two. Upstream
// plays CONST_ME_MAGIC_BLUE on the master and CONST_ME_TELEPORT on the summon.
func (m *Monster) placeSummon(w *World, s *creatures.MonsterSummon) {
	if w.TypeRegistry == nil {
		return
	}
	mType := w.TypeRegistry.Monsters[strings.ToLower(s.Name)]
	if mType == nil {
		mType = w.TypeRegistry.Monsters[s.Name]
	}
	if mType == nil {
		return
	}
	pos, ok := w.freeTileAround(m.GetPosition(), s.Force)
	if !ok {
		return
	}

	summon := NewMonster(w.nextCreatureID.Add(1), mType.Name, mType)
	summon.SetPosition(pos)
	summon.SpawnPosition = pos
	summon.Master = m
	m.Summons = append(m.Summons, summon)
	w.AddCreature(summon)

	if w.OnMagicEffect != nil {
		w.OnMagicEffect(m.GetPosition(), constMeMagicBlue)
		w.OnMagicEffect(pos, constMeTeleport)
	}
}

// freeTileAround finds a walkable tile adjacent to center. With force set the
// occupancy check is skipped, which is how upstream's placeCreature(force=true)
// lets a boss summon into a crowded room.
func (w *World) freeTileAround(center Position, force bool) (Position, bool) {
	offsets := [8][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}, {1, -1}, {1, 1}, {-1, 1}, {-1, -1}}
	for _, off := range offsets {
		pos := Position{X: uint16(int(center.X) + off[0]), Y: uint16(int(center.Y) + off[1]), Z: center.Z}
		tile := w.Map.GetTile(pos)
		if tile == nil {
			continue
		}
		if !force && !tile.WalkableFor(nil, w.Items, w.WorldType) {
			continue
		}
		return pos, true
	}
	if force {
		return center, true
	}
	return center, false
}

// UpdateLookDirection is Monster::updateLookDirection (monster.cpp:3355). On an
// exact diagonal the monster keeps the axis it is already facing rather than
// snapping, which is why the tie case reads the current direction.
func (m *Monster) UpdateLookDirection() Direction {
	dir := m.GetDirection()
	target := m.GetTarget()
	if target == nil {
		return dir
	}
	pos, tp := m.GetPosition(), target.GetPosition()
	offsetX := int(tp.X) - int(pos.X)
	offsetY := int(tp.Y) - int(pos.Y)
	dx, dy := abs(offsetX), abs(offsetY)

	switch {
	case dx > dy:
		if offsetX < 0 {
			return DirWest
		}
		return DirEast
	case dx < dy:
		if offsetY < 0 {
			return DirNorth
		}
		return DirSouth
	}

	switch {
	case offsetX < 0 && offsetY < 0:
		if dir == DirSouth {
			return DirWest
		}
		if dir == DirEast {
			return DirNorth
		}
	case offsetX < 0 && offsetY > 0:
		if dir == DirNorth {
			return DirWest
		}
		if dir == DirEast {
			return DirSouth
		}
	case offsetX > 0 && offsetY < 0:
		if dir == DirSouth {
			return DirEast
		}
		if dir == DirWest {
			return DirNorth
		}
	default:
		if dir == DirNorth {
			return DirEast
		}
		if dir == DirWest {
			return DirSouth
		}
	}
	return dir
}

// IsInSpawnRange is Monster::isInSpawnRange (monster.cpp:3321). A monster with
// no spawn — summoned, scripted, or placed by a GM — is always in range, which
// is what stops it being teleported to position zero.
func (m *Monster) IsInSpawnRange(pos Position) bool {
	if m.SpawnPosition == (Position{}) {
		return true
	}
	if monsterDespawnRadius == 0 {
		return true
	}
	if chebyshevDistance(m.SpawnPosition, pos) > monsterDespawnRadius {
		return false
	}
	if monsterDespawnRange == 0 {
		return true
	}
	return distanceZ(m.SpawnPosition, pos) <= monsterDespawnRange
}

// UpdateIdleStatus is Monster::updateIdleStatus (monster.cpp:1520), reduced to
// the parts that mean anything here: a monster with no target and nothing to do
// goes idle at its spawn, and one away from its spawn walks back instead.
//
// Idle matters beyond behaviour — an idle monster is skipped by the AI loop, so
// without this every monster on the map is processed every tick.
func (m *Monster) UpdateIdleStatus() {
	idle := false
	if m.Master == nil && m.GetTarget() == nil && len(m.Targets) == 0 {
		if m.GetPosition() == m.SpawnPosition || m.SpawnPosition == (Position{}) {
			idle = true
		} else {
			m.walkingBack = true
		}
	}
	m.SetIdle(idle)
	if !idle {
		m.walkingBack = m.walkingBack && m.GetPosition() != m.SpawnPosition
	}
}

// IsWalkingBack reports whether the monster is returning to its spawn.
func (m *Monster) IsWalkingBack() bool { return m.walkingBack }

// despawn thresholds, Monster::despawnRadius / despawnRange. Upstream reads them
// from config.lua; these are the shipped defaults.
const (
	monsterDespawnRadius = 50
	monsterDespawnRange  = 2

	constMeMagicBlue uint16 = 12
	constMeTeleport  uint16 = 11
)
