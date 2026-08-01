package game

import (
	"math/rand"
)

// Monster AI behaviour ported from src/creatures/monsters/monster.cpp.
//
// The engine before this picked the nearest player, walked the A* path to it and
// otherwise wandered. That is a fraction of what upstream does, and the missing
// parts are the ones a player notices: a ranged monster that closes to melee, a
// wounded monster that never runs, a caster that stands still instead of circling.

// TargetSearchType is TargetSearchType_t. TARGETSEARCH_DEFAULT is expressed as a
// call to SearchTarget with TargetSearchDefault, which rolls against the type's
// strategy percentages.
type TargetSearchType uint8

const (
	TargetSearchDefault TargetSearchType = iota
	TargetSearchNearest
	TargetSearchHP
	TargetSearchDamage
	TargetSearchRandom
)

// resolveSearchType is the roll in Monster::searchTargetImmediate
// (monster.cpp:906-924): one uniform draw walks the four strategy weights in
// order, and whatever the draw overshoots falls through to RANDOM.
func (m *Monster) resolveSearchType(t TargetSearchType) TargetSearchType {
	if t != TargetSearchDefault {
		return t
	}
	if m.Type == nil {
		return TargetSearchNearest
	}
	rnd := rand.Intn(100) + 1
	sum := m.Type.Flags.StrategiesTargetNearest
	if rnd <= sum {
		return TargetSearchNearest
	}
	sum += m.Type.Flags.StrategiesTargetHealth
	if rnd <= sum {
		return TargetSearchHP
	}
	sum += m.Type.Flags.StrategiesTargetDamage
	if rnd <= sum {
		return TargetSearchDamage
	}
	return TargetSearchRandom
}

// TargetDistanceOf is the monster's preferred fighting distance: 1 for melee,
// larger for anything that wants to keep away. Monster::targetDistance starts
// from the type and can be changed at runtime by a script.
func (m *Monster) TargetDistanceOf() int {
	if m.TargetDistance > 0 {
		return int(m.TargetDistance)
	}
	if m.Type != nil && m.Type.Flags.TargetDistance > 0 {
		return m.Type.Flags.TargetDistance
	}
	return 1
}

// IsFleeing is Monster::isFleeing: below runAwayHealth the monster stops
// fighting and puts distance between itself and its target. A runAwayHealth of
// 0 — the default — means it never flees, which is why the check is on > 0 and
// not on the health alone.
func (m *Monster) IsFleeing() bool {
	if m.Type == nil {
		return false
	}
	run := m.Type.Flags.RunHealth
	return run > 0 && int(m.GetHealth()) <= run
}

// CanUseAttack is Monster::canUseAttack: the target has to be inside the
// monster's reach and in line of sight. Without it a distance monster happily
// shoots through a wall, and searchTarget cannot tell a reachable target from an
// unreachable one.
func (m *Monster) CanUseAttack(from Position, target Creature, w *World) bool {
	if target == nil {
		return false
	}
	tp := target.GetPosition()
	if tp.Z != from.Z {
		return false
	}
	dist := chebyshevDistance(from, tp)
	reach := m.TargetDistanceOf()
	if r := m.MaxSpellRange(); r > reach {
		reach = r
	}
	if dist > reach {
		return false
	}
	return w == nil || w.IsSightClear(from, tp)
}

// SearchTarget is Monster::searchTargetImmediate (monster.cpp:905-1010). It
// picks from the creatures this monster can actually attack, by the strategy the
// type asks for, and returns whether a target was set.
//
// The old engine always took the nearest player. That is one of four strategies,
// and monsters.xml gives most bosses a mix — a boss meant to hunt the weakest
// player in the room was instead pinned on whoever stood closest.
func (m *Monster) SearchTarget(w *World, t TargetSearchType) bool {
	if w == nil {
		return false
	}
	searchType := m.resolveSearchType(t)
	myPos := m.GetPosition()
	melee := m.TargetDistanceOf() == 1

	var candidates []Creature
	for _, s := range w.Spectators(myPos, m.GetID()) {
		if s == nil || s.CannotBeAttacked() || s.Ghost {
			continue
		}
		if !melee && !m.CanUseAttack(myPos, s, w) {
			continue
		}
		candidates = append(candidates, s)
	}
	if len(candidates) == 0 {
		return false
	}

	var chosen Creature
	switch searchType {
	case TargetSearchHP:
		best := -1
		for _, c := range candidates {
			if hp := int(c.GetHealth()); best < 0 || hp < best {
				best, chosen = hp, c
			}
		}
	case TargetSearchDamage:
		// Most damage dealt TO this monster, which is what makes a monster turn on
		// whoever is actually hurting it rather than whoever is nearest.
		dmg := m.DamageMap()
		best := int32(-1)
		for _, c := range candidates {
			if d := dmg[c.GetID()].Total; d > best {
				best, chosen = d, c
			}
		}
		if best <= 0 {
			chosen = nil // nobody has hurt it yet: fall through to nearest
		}
	case TargetSearchRandom:
		chosen = candidates[rand.Intn(len(candidates))]
	}
	if chosen == nil { // NEAREST, and the fallback for every other strategy
		best := -1
		for _, c := range candidates {
			if d := chebyshevDistance(myPos, c.GetPosition()); best < 0 || d < best {
				best, chosen = d, c
			}
		}
	}
	if chosen == nil {
		return false
	}
	m.SetTarget(chosen)
	return true
}

// DanceStep is Monster::getDanceStep (monster.cpp:2569-2628): the sidestep a
// monster takes around its target while staying at the same distance from it.
//
// keepAttack keeps the monster where it can still attack; keepDistance stops it
// closing in, which is how a fleeing monster backs away instead of circling.
// A monster nearer than its targetDistance does not dance at all — it needs to
// back off first.
func (m *Monster) DanceStep(w *World, keepAttack, keepDistance bool) (Direction, bool) {
	target := m.GetTarget()
	if target == nil || w == nil {
		return DirNorth, false
	}
	pos := m.GetPosition()
	center := target.GetPosition()

	offsetX := int(pos.X) - int(center.X)
	offsetY := int(pos.Y) - int(center.Y)
	centerToDist := abs(offsetX)
	if abs(offsetY) > centerToDist {
		centerToDist = abs(offsetY)
	}
	if centerToDist < m.TargetDistanceOf() {
		return DirNorth, false
	}

	canAttackNow := m.CanUseAttack(pos, target, w)
	var dirs []Direction
	try := func(dir Direction, nx, ny int) {
		d := abs(nx - int(center.X))
		if dy := abs(ny - int(center.Y)); dy > d {
			d = dy
		}
		if d != centerToDist {
			return
		}
		dest := Position{X: uint16(nx), Y: uint16(ny), Z: pos.Z}
		tile := w.Map.GetTile(dest)
		if tile == nil || !tile.WalkableFor(m, w.Items, w.WorldType) {
			return
		}
		if keepAttack && canAttackNow && !m.CanUseAttack(dest, target, w) {
			return
		}
		dirs = append(dirs, dir)
	}

	if !keepDistance || offsetY >= 0 {
		try(DirNorth, int(pos.X), int(pos.Y)-1)
	}
	if !keepDistance || offsetY <= 0 {
		try(DirSouth, int(pos.X), int(pos.Y)+1)
	}
	if !keepDistance || offsetX <= 0 {
		try(DirEast, int(pos.X)+1, int(pos.Y))
	}
	if !keepDistance || offsetX >= 0 {
		try(DirWest, int(pos.X)-1, int(pos.Y))
	}
	if len(dirs) == 0 {
		return DirNorth, false
	}
	return dirs[rand.Intn(len(dirs))], true
}

// FleeStep walks directly away from the target, the flee arm of
// Monster::getDistanceStep. The full upstream version tries several candidate
// tiles and scores them; this takes the straight retreat and falls back to the
// dance step with keepDistance, which covers the case where the direct line is
// blocked.
func (m *Monster) FleeStep(w *World) (Direction, bool) {
	target := m.GetTarget()
	if target == nil || w == nil {
		return DirNorth, false
	}
	pos, tp := m.GetPosition(), target.GetPosition()
	away := Position{
		X: uint16(int(pos.X) + sign(int(pos.X)-int(tp.X))),
		Y: uint16(int(pos.Y) + sign(int(pos.Y)-int(tp.Y))),
		Z: pos.Z,
	}
	if away != pos {
		if tile := w.Map.GetTile(away); tile != nil && tile.WalkableFor(m, w.Items, w.WorldType) {
			return StepDirection(pos, away), true
		}
	}
	return m.DanceStep(w, false, false)
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// IsSightClear asks whether a projectile can travel from start to destination on
// one floor. It is Map::isSightClear with floorCheck on, which is what combat
// uses; the line walk itself lives in CheckSightLine (sightline.go).
//
// This used to carry its own line walk with a Bresenham diagonal standing in for
// upstream's Xiaolin Wu. The two agree on straight lines and open ground and
// disagree on shallow diagonals, which meant a monster could take a shot the C++
// server would have refused. The real algorithm is ported now.
func (w *World) IsSightClear(start, destination Position) bool {
	return w.IsSightClearFloors(start, destination, true)
}
