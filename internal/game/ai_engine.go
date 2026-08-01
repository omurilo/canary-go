package game

import (
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

		// Monster::onThink runs the challenge timer first, so a monster dragged
		// into melee is back at its own fighting distance on the tick the
		// challenge lapses rather than one tick later (monster.cpp:1608-1615).
		monster.tickChallenge(e.world, interval)

		// The Monster::onThink timers (monster.cpp:2140-2310). They run before
		// anything else and regardless of whether the monster has a target — a
		// monster yells at an empty room and a summoner keeps its summons up
		// while walking home.
		monster.OnThinkYell(e.world, interval)
		monster.OnThinkDefense(e.world, interval)
		monster.OnThinkTarget(e.world, interval)

		// Passive AI: a non-hostile monster (Cat, Rabbit, Deer) never seeks a
		// target, but it still keeps a target list — that list is what tells it
		// whether anyone is around, and upstream only lets a monster wander while
		// it is not idle. A rabbit alone in a field stands on its spawn; one with
		// a player nearby hops about.
		if monster.Type != nil && !monster.Type.Flags.Hostile {
			if monster.GetTarget() != nil {
				monster.SetTarget(nil)
			}
			monster.UpdateTargetList(e.world)
			monster.UpdateIdleStatus()
			if dir, ok := monster.GetNextStep(e.world); ok {
				e.world.TryMoveCreature(monster, dir)
			}
			continue
		}

		// --- target upkeep -------------------------------------------------
		// A target that died, walked out of range, went ghost or became
		// unattackable is dropped; C++ does the same in onThink before considering
		// a new one.
		//
		// This used to require the target to be a *Player and dropped anything
		// else outright. Now that factions let a monster fight another monster,
		// that would have released every non-player target on the tick after it
		// was chosen.
		if t := monster.GetTarget(); t != nil {
			gone := t.GetHealth() == 0 ||
				!monster.GetPosition().InRangeOf(t.GetPosition()) ||
				!monster.IsOpponent(t)
			if p, isPlayer := t.(*Player); isPlayer {
				gone = gone || e.world.PlayerByID(p.GetID()) == nil || p.CannotBeAttacked() || p.Ghost
			} else if e.world.CreatureByID(t.GetID()) == nil {
				gone = true
			}
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

		// Face what it is fighting, before moving. Monster::updateLookDirection
		// (monster.cpp:3355) — without it a monster attacks sideways, and on the
		// client it never turns at all.
		if monster.GetTarget() != nil {
			if dir := monster.UpdateLookDirection(); dir != monster.GetDirection() {
				monster.SetDirection(dir)
				if e.world.OnCreatureTurn != nil {
					e.world.OnCreatureTurn(monster)
				}
			}
		} else {
			// Monster::updateIdleStatus (monster.cpp:1520): with nothing to fight,
			// a monster away from its spawn heads back rather than wandering off.
			monster.UpdateIdleStatus()
			if !monster.IsInSpawnRange(monster.GetPosition()) {
				// Past the despawn radius, upstream teleports rather than walking.
				e.world.TeleportCreature(monster, monster.SpawnPosition)
				monster.Idle = true
				continue
			}
		}

		// --- movement --------------------------------------------------------
		// One entry point, Monster::getNextStep (monster.cpp:2442): follow, walk
		// back, or wander, then push whatever is in the way of the chosen tile.
		if dir, ok := monster.GetNextStep(e.world); ok {
			e.world.TryMoveCreature(monster, dir)
		}
	}

	GlobalDispatcher.AddEvent(creatureThinkInterval, e.updateAI)
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
