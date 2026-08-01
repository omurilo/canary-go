package game

import (
	"math/rand"
	"time"
)

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
	GlobalDispatcher.AddEvent(1*time.Second, e.updateAI)
}

func (e *AIEngine) updateAI() {
	creatures := e.world.Creatures()
	for _, c := range creatures {
		// Only monsters have AI for now
		monster, isMonster := c.(*Monster)
		if !isMonster {
			continue
		}

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
			if rand.Intn(3) == 0 {
				dirs := []Direction{DirNorth, DirEast, DirSouth, DirWest}
				e.world.TryMoveCreature(monster, dirs[rand.Intn(len(dirs))])
			}
			continue
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

func chebyshevDistance(p1, p2 Position) int {
	dx := abs(int(p1.X) - int(p2.X))
	dy := abs(int(p1.Y) - int(p2.Y))
	if dx > dy {
		return dx
	}
	return dy
}

func getDirectionTo(from, to Position) Direction {
	if to.X > from.X && to.Y == from.Y { return DirEast }
	if to.X < from.X && to.Y == from.Y { return DirWest }
	if to.Y > from.Y && to.X == from.X { return DirSouth }
	if to.Y < from.Y && to.X == from.X { return DirNorth }
	if to.X > from.X && to.Y > from.Y { return DirSE }
	if to.X > from.X && to.Y < from.Y { return DirNE }
	if to.X < from.X && to.Y > from.Y { return DirSW }
	if to.X < from.X && to.Y < from.Y { return DirNW }
	return DirNorth
}

