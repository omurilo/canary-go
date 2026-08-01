package game

import (
	"math/rand"

	"github.com/opentibiabr/canary-go/internal/game/combat"
	"github.com/opentibiabr/canary-go/internal/items"
)

// Monster movement, ported from src/creatures/monsters/monster.cpp.
//
// The engine before this had one movement rule: walk the A* path towards the
// target, and if already close enough, sidestep. Upstream has three, picked by
// getNextStep — follow, walk back to spawn, or wander — and a 500-line
// getDistanceStep that decides how a monster backs away from something it wants
// to keep at range. A distance monster with a player walking into it used to
// stand still; upstream steps away along the axis the player is NOT on.

// CanWalkTo is Monster::canWalkTo (monster.cpp:3215): can this monster step one
// tile in moveDirection from pos.
//
// The three conditions are all load-bearing. Leaving the spawn range out lets a
// monster wander off the map; leaving the creature check out makes it try to
// walk through whoever is standing there, which reads as the monster freezing.
func (m *Monster) CanWalkTo(w *World, pos Position, moveDirection Direction) bool {
	if w == nil {
		return false
	}
	dest := pos.Offset(moveDirection)
	if !m.IsInSpawnRange(dest) {
		return false
	}
	tile := w.Map.GetTile(dest)
	if tile == nil {
		return false
	}
	for _, c := range tile.Creatures {
		if c != nil && c.GetID() != m.GetID() {
			return false
		}
	}
	return tile.WalkableFor(m, w.Items, w.WorldType)
}

// CanWalkOnFieldType is Monster::canWalkOnFieldType (monster.cpp:195): whether
// the monster may step into a magic field of this damage type. Anything that is
// not energy, fire or earth is always walkable.
func (m *Monster) CanWalkOnFieldType(combatType combat.CombatType) bool {
	if m.Type == nil {
		return true
	}
	switch combatType {
	case combat.CombatEnergy:
		return m.Type.Flags.CanWalkOnEnergy
	case combat.CombatFire:
		return m.Type.Flags.CanWalkOnFire
	case combat.CombatEarth:
		return m.Type.Flags.CanWalkOnPoison
	default:
		return true
	}
}

// GetRandomStep is Monster::getRandomStep (monster.cpp:2551): the four cardinal
// directions in a random order, first walkable one wins. Diagonals are excluded
// — a wandering monster never moves diagonally.
func (m *Monster) GetRandomStep(w *World, from Position) (Direction, bool) {
	dirs := []Direction{DirNorth, DirWest, DirEast, DirSouth}
	rand.Shuffle(len(dirs), func(i, j int) { dirs[i], dirs[j] = dirs[j], dirs[i] })
	for _, dir := range dirs {
		if m.CanWalkTo(w, from, dir) {
			return dir, true
		}
	}
	return DirNorth, false
}

// GetDistanceStep is Monster::getDistanceStep (monster.cpp:2631-3125): the step
// a monster takes to keep — or regain — its fighting distance from a target.
//
// Two things about the shape of this function are deliberate and easy to
// "clean up" into a different behaviour:
//
//   - It returns true in cases where it never assigned a direction. The caller
//     keeps whatever it passed in. That is why the signature carries the
//     incoming direction through rather than returning a fresh one.
//   - Several branches assign a direction and then fall through to the shared
//     `return true` instead of returning at the assignment. Reordering them
//     changes which direction wins.
//
// flee inverts the priorities: a fleeing monster will take a step towards its
// target if that is the only tile left, which is how a cornered monster squeezes
// past instead of standing still.
func (m *Monster) GetDistanceStep(w *World, targetPos Position, current Direction, flee bool) (Direction, bool) {
	if w == nil {
		return current, false
	}
	pos := m.GetPosition()
	moveDirection := current

	can := func(dir Direction) bool { return m.CanWalkTo(w, pos, dir) }
	// boolean_random: an even coin between two equally good directions.
	pick := func(a, b Direction) Direction {
		if rand.Intn(2) == 0 {
			return a
		}
		return b
	}

	dx := abs(int(pos.X) - int(targetPos.X))
	dy := abs(int(pos.Y) - int(targetPos.Y))
	distance := dx
	if dy > distance {
		distance = dy
	}

	if !flee && (distance > m.TargetDistanceOf() || !w.IsSightClear(pos, targetPos)) {
		return moveDirection, false // let A* handle it
	}
	if !flee && distance == m.TargetDistanceOf() {
		// Already where we want to be; the dance step handles moving around.
		return moveDirection, true
	}

	// getOffsetX(creaturePos, targetPos) is creature minus target, so a positive
	// offset means the monster is east/south of its target.
	offsetX := int(pos.X) - int(targetPos.X)
	offsetY := int(pos.Y) - int(targetPos.Y)

	// stepDuration slows a monster down when its target is adjacent; isTargetNearby
	// reads it.
	if dx <= 1 && dy <= 1 {
		if m.stepDuration < 2 {
			m.stepDuration++
		}
	} else if m.stepDuration > 0 {
		m.stepDuration--
	}

	if offsetX == 0 && offsetY == 0 {
		// The target is standing on the monster.
		return m.GetRandomStep(w, pos)
	}

	if dx == dy {
		switch {
		case offsetX >= 1 && offsetY >= 1: // target is NW; escape SE, S or E
			s, e := can(DirSouth), can(DirEast)
			switch {
			case s && e:
				return pick(DirSouth, DirEast), true
			case s:
				return DirSouth, true
			case e:
				return DirEast, true
			case can(DirSE):
				return DirSE, true
			}
			n, wst := can(DirNorth), can(DirWest)
			if flee {
				switch {
				case n && wst:
					return pick(DirNorth, DirWest), true
				case n:
					return DirNorth, true
				case wst:
					return DirWest, true
				}
			}
			if wst && can(DirSW) {
				moveDirection = DirWest
			} else if n && can(DirNE) {
				moveDirection = DirNorth
			}
			return moveDirection, true

		case offsetX <= -1 && offsetY <= -1: // target is SE; escape NW, W or N
			wst, n := can(DirWest), can(DirNorth)
			switch {
			case wst && n:
				return pick(DirWest, DirNorth), true
			case wst:
				return DirWest, true
			case n:
				return DirNorth, true
			}
			if can(DirNW) {
				return DirNW, true
			}
			s, e := can(DirSouth), can(DirEast)
			if flee {
				switch {
				case s && e:
					return pick(DirSouth, DirEast), true
				case s:
					return DirSouth, true
				case e:
					return DirEast, true
				}
			}
			if s && can(DirSW) {
				moveDirection = DirSouth
			} else if e && can(DirNE) {
				moveDirection = DirEast
			}
			return moveDirection, true

		case offsetX >= 1 && offsetY <= -1: // target is SW; escape NE, N or E
			n, e := can(DirNorth), can(DirEast)
			switch {
			case n && e:
				return pick(DirNorth, DirEast), true
			case n:
				return DirNorth, true
			case e:
				return DirEast, true
			}
			if can(DirNE) {
				return DirNE, true
			}
			s, wst := can(DirSouth), can(DirWest)
			if flee {
				switch {
				case s && wst:
					return pick(DirSouth, DirWest), true
				case s:
					return DirSouth, true
				case wst:
					return DirWest, true
				}
			}
			if wst && can(DirNW) {
				moveDirection = DirWest
			} else if s && can(DirSE) {
				moveDirection = DirSouth
			}
			return moveDirection, true

		case offsetX <= -1 && offsetY >= 1: // target is NE; escape SW, S or W
			wst, s := can(DirWest), can(DirSouth)
			switch {
			case wst && s:
				return pick(DirWest, DirSouth), true
			case wst:
				return DirWest, true
			case s:
				return DirSouth, true
			case can(DirSW):
				return DirSW, true
			}
			n, e := can(DirNorth), can(DirEast)
			if flee {
				switch {
				case n && e:
					return pick(DirNorth, DirEast), true
				case n:
					return DirNorth, true
				case e:
					return DirEast, true
				}
			}
			if e && can(DirSE) {
				moveDirection = DirEast
			} else if n && can(DirNW) {
				moveDirection = DirNorth
			}
			return moveDirection, true
		}
	}

	// Not diagonal: escape along the dominant axis, and only sidestep along the
	// other one when the offset does not push us back towards the target.
	if dy > dx {
		away, towards := DirSouth, DirNorth // target is north
		diagA, diagB := DirSW, DirSE
		if offsetY < 0 {
			away, towards = DirNorth, DirSouth // target is south
			diagA, diagB = DirNW, DirNE
		}
		if can(away) {
			return away, true
		}
		wst, e := can(DirWest), can(DirEast)
		switch {
		case wst && e && offsetX == 0:
			return pick(DirWest, DirEast), true
		case wst && offsetX <= 0:
			return DirWest, true
		case e && offsetX >= 0:
			return DirEast, true
		}
		if flee {
			switch {
			case wst && e:
				return pick(DirWest, DirEast), true
			case wst:
				return DirWest, true
			case e:
				return DirEast, true
			}
		}
		dA, dB := can(diagA), can(diagB)
		if dA || dB {
			switch {
			case dA && dB:
				moveDirection = pick(diagA, diagB)
			case wst:
				moveDirection = DirWest
			case dA:
				moveDirection = diagA
			case e:
				moveDirection = DirEast
			case dB:
				moveDirection = diagB
			}
			return moveDirection, true
		}
		if flee && can(towards) {
			return towards, true
		}
		return moveDirection, true
	}

	away, towards := DirEast, DirWest // target is west
	diagA, diagB := DirSE, DirNE
	if offsetX < 0 {
		away, towards = DirWest, DirEast // target is east
		diagA, diagB = DirNW, DirSW
	}
	if can(away) {
		return away, true
	}
	n, s := can(DirNorth), can(DirSouth)
	switch {
	case n && s && offsetY == 0:
		return pick(DirNorth, DirSouth), true
	case n && offsetY <= 0:
		return DirNorth, true
	case s && offsetY >= 0:
		return DirSouth, true
	}
	if flee {
		switch {
		case n && s:
			return pick(DirNorth, DirSouth), true
		case n:
			return DirNorth, true
		case s:
			return DirSouth, true
		}
	}
	dA, dB := can(diagA), can(diagB)
	if dA || dB {
		// The pairing of the plain directions differs between the two axes:
		// east-facing checks south first, west-facing checks north first
		// (monster.cpp:3029-3042 against :3089-3102).
		first, second := DirSouth, DirNorth
		firstOK, secondOK := s, n
		if offsetX < 0 {
			first, second = DirNorth, DirSouth
			firstOK, secondOK = n, s
		}
		switch {
		case dA && dB:
			moveDirection = pick(diagA, diagB)
		case firstOK:
			moveDirection = first
		case dA:
			moveDirection = diagA
		case secondOK:
			moveDirection = second
		case dB:
			moveDirection = diagB
		}
		return moveDirection, true
	}
	if flee && can(towards) {
		return towards, true
	}
	return moveDirection, true
}

// IsTargetNearby is Monster::isTargetNearby (monster.cpp:3127): set while the
// target has been adjacent for long enough that the monster slowed down.
func (m *Monster) IsTargetNearby() bool { return m.stepDuration >= 1 }

// ---------------------------------------------------------------------------
// Pushing
// ---------------------------------------------------------------------------

// pushItemLocationOptions is Monster::getPushItemLocationOptions
// (monster.cpp:3948): where a blocking item may be shoved, given the direction
// the monster is walking. Always sideways relative to the movement, never into
// the monster's own path.
func getPushItemLocationOptions(dir Direction) [][2]int {
	switch dir {
	case DirWest, DirEast:
		return [][2]int{{0, -1}, {0, 1}}
	case DirNorth, DirSouth:
		return [][2]int{{-1, 0}, {1, 0}}
	case DirNW:
		return [][2]int{{0, -1}, {-1, 0}}
	case DirNE:
		return [][2]int{{0, -1}, {1, 0}}
	case DirSW:
		return [][2]int{{0, 1}, {-1, 0}}
	case DirSE:
		return [][2]int{{0, 1}, {1, 0}}
	}
	return nil
}

// pushItem is Monster::pushItem (monster.cpp:2311). House tiles are exempt —
// a monster must not rearrange a player's furniture.
func (m *Monster) pushItem(w *World, from Position, item *Item, dir Direction) bool {
	if item == nil {
		return false
	}
	fromTile := w.Map.GetTile(from)
	if fromTile == nil || fromTile.HouseID != 0 {
		return false
	}
	for _, off := range getPushItemLocationOptions(dir) {
		to := Position{X: uint16(int(from.X) + off[0]), Y: uint16(int(from.Y) + off[1]), Z: from.Z}
		toTile := w.Map.GetTile(to)
		if toTile == nil || !w.CanThrowObjectTo(from, to, true, true, MaxClientViewportX, MaxClientViewportY) {
			continue
		}
		if !w.Map.RemoveItemPtr(from, item) {
			continue
		}
		if w.AddItem(to, item) {
			return true
		}
		// Put it back rather than deleting it if the destination refused it.
		w.AddItem(from, item)
	}
	return false
}

// PushItems is Monster::pushItems (monster.cpp:2339): a monster with
// canPushItems shoves the blocking items off the tile it is about to enter, and
// destroys what it cannot shove.
//
// The two counters are upstream's and they are separate budgets: up to 20 items
// moved and up to 10 destroyed per step. The iteration runs backwards over the
// down-items because moving one shifts the ones after it.
func (m *Monster) PushItems(w *World, tile *Tile, pos Position, nextDirection Direction) {
	if w == nil || tile == nil || len(tile.Items) == 0 || tile.HouseID != 0 {
		return
	}
	moveCount, removeCount := 0, 0

	downItems := make([]*Item, len(tile.Items))
	copy(downItems, tile.Items)

	for i := len(downItems) - 1; i >= 0; i-- {
		item := downItems[i]
		if item == nil {
			continue
		}
		it := w.Items.Get(item.ID)
		if it == nil || !it.HasProperty(items.PropMovable) {
			continue
		}
		if !it.HasProperty(items.PropBlockPath) && !it.HasProperty(items.PropBlockSolid) {
			continue
		}
		if moveCount < 20 && m.pushItem(w, pos, item, nextDirection) {
			moveCount++
			continue
		}
		if removeCount < 10 && !it.IsCorpse && w.Map.RemoveItemPtr(pos, item) {
			removeCount++
		}
	}

	if removeCount > 0 && w.OnMagicEffect != nil {
		w.OnMagicEffect(pos, constMePoff)
	}
}

// pushCreature is Monster::pushCreature (monster.cpp:2386): shove one monster
// out of the way, trying the four cardinal directions in a random order.
func (m *Monster) pushCreature(w *World, other *Monster) bool {
	dirs := []Direction{DirNorth, DirWest, DirEast, DirSouth}
	rand.Shuffle(len(dirs), func(i, j int) { dirs[i], dirs[j] = dirs[j], dirs[i] })
	for _, dir := range dirs {
		to := other.GetPosition().Offset(dir)
		toTile := w.Map.GetTile(to)
		if toTile == nil || !toTile.WalkableFor(other, w.Items, w.WorldType) {
			continue
		}
		if _, ok := w.TryMoveCreature(other, dir); ok {
			return true
		}
	}
	return false
}

// PushCreatures is Monster::pushCreatures (monster.cpp:2404). A pushable monster
// that cannot be shoved anywhere is killed outright — that is upstream's
// behaviour, not a shortcut, and it is how a stampede clears a corridor.
//
// lastPushedMonster stops the same monster being pushed twice in one pass, which
// would otherwise let it be pushed and then killed in the same step.
func (m *Monster) PushCreatures(w *World, tile *Tile, pos Position) {
	if w == nil || tile == nil || len(tile.Creatures) == 0 {
		return
	}
	creaturesCopy := make([]Creature, len(tile.Creatures))
	copy(creaturesCopy, tile.Creatures)

	killedCount := 0
	var lastPushed *Monster

	for i := len(creaturesCopy) - 1; i >= 0; i-- {
		other, ok := creaturesCopy[i].(*Monster)
		if !ok || other == nil || other == m {
			continue
		}
		if other.Type == nil || !other.Type.Flags.Pushable {
			continue
		}
		if other != lastPushed && m.pushCreature(w, other) {
			lastPushed = other
			continue
		}
		other.SetHealth(0)
		killedCount++
		if w.OnCreatureDied != nil {
			w.OnCreatureDied(other)
		}
	}

	if killedCount > 0 && w.OnMagicEffect != nil {
		w.OnMagicEffect(pos, constMeBlockHit)
	}
}

// GetNextStep is Monster::getNextStep (monster.cpp:2442): the one entry point
// that decides how a monster moves this tick, then applies the pushing that
// stepping onto the chosen tile implies.
//
// The order is fixed: follow a target if there is one, else walk back to the
// spawn if out of place, else wander. Idle or dead, it does not move at all.
func (m *Monster) GetNextStep(w *World) (Direction, bool) {
	if m.Idle || m.GetHealth() == 0 {
		return DirNorth, false
	}

	var dir Direction
	var ok bool
	switch {
	case m.GetTarget() != nil:
		dir, ok = m.doFollowCreature(w)
	case m.walkingBack:
		dir, ok = m.doWalkBack(w)
	default:
		dir, ok = m.GetRandomStep(w, m.GetPosition())
	}
	if !ok {
		return dir, false
	}

	canPushItems := m.Type != nil && m.Type.Flags.CanPushItems
	canPushCreatures := m.Type != nil && m.Type.Flags.CanPushCreatures
	if !canPushItems && !canPushCreatures {
		return dir, true
	}

	next := m.GetPosition().Offset(dir)
	tile := w.Map.GetTile(next)
	if tile == nil {
		return dir, true
	}
	if canPushItems {
		m.PushItems(w, tile, next, dir)
	}
	if canPushCreatures {
		m.PushCreatures(w, tile, next)
	}
	return dir, true
}

// followStep is Monster::doFollowCreature (monster.cpp:2529): head towards the
// target, and when there is no path left, dance around it instead.
//
// staticAttackChance is what keeps a caster planted: rolled against 1..100, a
// chance of 100 never dances.
func (m *Monster) doFollowCreature(w *World) (Direction, bool) {
	target := m.GetTarget()
	if target == nil {
		return DirNorth, false
	}
	m.randomStepping = false
	pos, targetPos := m.GetPosition(), target.GetPosition()

	// Keep-distance monsters get their own step logic before the pathfinder,
	// because A* would happily walk them into melee.
	if m.TargetDistanceOf() > 1 || m.IsFleeing() {
		if dir, ok := m.GetDistanceStep(w, targetPos, m.GetDirection(), m.IsFleeing()); ok {
			return dir, true
		}
	}

	if path := FindPath(w.Map, w.Items, pos, targetPos, 100); len(path) > 0 {
		want := m.TargetDistanceOf()
		if chebyshevDistance(path[0], targetPos) >= want || want <= 1 {
			return StepDirection(pos, path[0]), true
		}
	}

	if m.IsFleeing() {
		return m.GetDanceStep(w, false, false)
	}
	static := 0
	if m.Type != nil {
		static = m.Type.Flags.StaticAttackChance
	}
	if static < rand.Intn(100)+1 {
		return m.GetDanceStep(w, true, true)
	}
	return DirNorth, false
}

// walkBackStep is Monster::doWalkBack (monster.cpp:2500). Reaching the spawn
// clears the flag; failing to find a path clears it too, so the monster stops
// trying rather than standing still forever.
func (m *Monster) doWalkBack(w *World) (Direction, bool) {
	pos := m.GetPosition()
	if pos == m.SpawnPosition {
		m.walkingBack = false
		return DirNorth, false
	}
	path := FindPath(w.Map, w.Items, pos, m.SpawnPosition, 100)
	if len(path) == 0 {
		m.walkingBack = false
		return DirNorth, false
	}
	return StepDirection(pos, path[0]), true
}

const (
	constMePoff     uint16 = 2
	constMeBlockHit uint16 = 4
)
