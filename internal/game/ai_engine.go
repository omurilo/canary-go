package game

import (
	"math/rand"
	"time"
)

// creatureThinkInterval is EVENT_CREATURE_THINK_INTERVAL (1000ms), the cadence
// Game::checkCreatures calls Creature::onThink at. The onThink timers below
// accumulate in these units, so changing it changes every monster's yell,
// defense and target-change cadence with it.
const creatureThinkInterval = 1 * time.Second

// AIEngine handles AI logic for creatures.
type AIEngine struct {
	world *World
}

// NewAIEngine creates a new AIEngine.
func NewAIEngine(w *World) *AIEngine {
	return &AIEngine{world: w}
}

// Start begins the AI loop.
func (e *AIEngine) Start() {
	GlobalDispatcher.AddEvent(creatureThinkInterval, e.updateAI)
}

func (e *AIEngine) updateAI() {
	const interval = uint32(creatureThinkInterval / time.Millisecond)

	creatures := e.world.Creatures()
	for _, c := range creatures {
		// Only monsters have AI for now
		monster, isMonster := c.(*Monster)
		if !isMonster {
			continue
		}

		// The Monster::onThink timers (monster.cpp:2140-2310). They run before
		// anything else and regardless of whether the monster has a target — a
		// monster yells at an empty room and a summoner keeps its summons up
		// while walking home.
		monster.OnThinkYell(e.world, interval)
		monster.OnThinkDefense(e.world, interval)
		monster.OnThinkTarget(e.world, interval)

		// Passive AI: If the monster is not hostile (e.g. Cat, Rabbit, Deer),
		// it should never seek targets or keep any existing target.
		if monster.Type != nil && !monster.Type.Flags.Hostile {
			if monster.GetTarget() != nil {
				monster.SetTarget(nil)
			}
			// Just wander
			if rand.Intn(3) == 0 { // 33% chance to move randomly
				dirs := []Direction{DirNorth, DirEast, DirSouth, DirWest}
				dir := dirs[rand.Intn(len(dirs))]
				e.world.TryMoveCreature(monster, dir)
			}
			continue
		}

		// --- target upkeep -------------------------------------------------
		// A target that walked out of range, went ghost or became unattackable is
		// dropped; C++ does the same in onThink before considering a new one.
		if t := monster.GetTarget(); t != nil {
			p, isPlayer := t.(*Player)
			gone := !isPlayer ||
				e.world.PlayerByID(p.GetID()) == nil ||
				p.CannotBeAttacked() || p.Ghost ||
				!monster.GetPosition().InRangeOf(p.GetPosition())
			if gone {
				monster.SetTarget(nil)
			}
		}

		// --- target selection ----------------------------------------------
		// The strategy roll, not "always nearest". A type with
		// strategiesTargetHealth set hunts the weakest thing it can reach.
		if monster.GetTarget() == nil {
			monster.SearchTarget(e.world, TargetSearchDefault)
		}

		target := monster.GetTarget()
		if target == nil {
			// Monster::updateIdleStatus (monster.cpp:1520): with nothing to fight,
			// a monster away from its spawn walks back instead of wandering off.
			//
			// Idle is computed but not used to skip the monster. Upstream drops an
			// idle monster from the creature check list and wakes it from
			// onCreatureEnter; there is no target-list upkeep on creature movement
			// here yet, so skipping would freeze it for good.
			monster.UpdateIdleStatus()
			if monster.IsWalkingBack() {
				e.walkBackToSpawn(monster)
			} else if rand.Intn(3) == 0 {
				dirs := []Direction{DirNorth, DirEast, DirSouth, DirWest}
				e.world.TryMoveCreature(monster, dirs[rand.Intn(len(dirs))])
			}
			continue
		}

		// Face what it is fighting, before moving. Monster::updateLookDirection
		// (monster.cpp:3355) — without it a monster attacks sideways, and on the
		// client it never turns at all.
		if dir := monster.UpdateLookDirection(); dir != monster.GetDirection() {
			monster.SetDirection(dir)
			if e.world.OnCreatureTurn != nil {
				e.world.OnCreatureTurn(monster)
			}
		}

		// --- movement --------------------------------------------------------
		// Monster::doFollowCreature (monster.cpp:2529-2549): a fleeing monster
		// backs away, and one already at its fighting distance dances around the
		// target instead of standing still — but only when staticAttackChance says
		// so, which is what keeps a static caster planted.
		if monster.IsFleeing() {
			if dir, ok := monster.FleeStep(e.world); ok {
				e.world.TryMoveCreature(monster, dir)
			}
			continue
		}

		pos := monster.GetPosition()
		dist := chebyshevDistance(pos, target.GetPosition())
		want := monster.TargetDistanceOf()

		if dist > want {
			// Close the gap. A distance monster stops at its targetDistance rather
			// than walking into melee, which the old engine always did.
			path := FindPath(e.world.Map, e.world.Items, pos, target.GetPosition(), 100)
			for _, step := range path {
				if chebyshevDistance(step, target.GetPosition()) < want {
					break
				}
				e.world.TryMoveCreature(monster, StepDirection(pos, step))
				break
			}
			continue
		}

		static := 0
		if monster.Type != nil {
			static = monster.Type.Flags.StaticAttackChance
		}
		if static < rand.Intn(100)+1 {
			if dir, ok := monster.DanceStep(e.world, true, true); ok {
				e.world.TryMoveCreature(monster, dir)
			}
		}
	}

	GlobalDispatcher.AddEvent(1*time.Second, e.updateAI)
}

// walkBackToSpawn is Monster::doWalkBack (monster.cpp:2500): one step along the
// path home. A monster that cannot find a path is teleported back, which is what
// upstream does once it leaves the despawn radius entirely.
func (e *AIEngine) walkBackToSpawn(m *Monster) {
	pos := m.GetPosition()
	if pos == m.SpawnPosition {
		return
	}
	if !m.IsInSpawnRange(pos) {
		e.world.TeleportCreature(m, m.SpawnPosition)
		m.Idle = true
		return
	}
	path := FindPath(e.world.Map, e.world.Items, pos, m.SpawnPosition, 100)
	if len(path) == 0 {
		return
	}
	e.world.TryMoveCreature(m, StepDirection(pos, path[0]))
}

func chebyshevDistance(p1, p2 Position) int {
	dx := abs(int(p1.X) - int(p2.X))
	dy := abs(int(p1.Y) - int(p2.Y))
	if dx > dy {
		return dx
	}
	return dy
}

func getDirectionTo(from, to Position) Direction {
	if to.X > from.X && to.Y == from.Y {
		return DirEast
	}
	if to.X < from.X && to.Y == from.Y {
		return DirWest
	}
	if to.Y > from.Y && to.X == from.X {
		return DirSouth
	}
	if to.Y < from.Y && to.X == from.X {
		return DirNorth
	}
	if to.X > from.X && to.Y > from.Y {
		return DirSE
	}
	if to.X > from.X && to.Y < from.Y {
		return DirNE
	}
	if to.X < from.X && to.Y > from.Y {
		return DirSW
	}
	if to.X < from.X && to.Y < from.Y {
		return DirNW
	}
	return DirNorth
}
