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

// walkBeat is SERVER_BEAT: the grid every step duration is rounded onto, and so
// the finest cadence the walk loop needs to run at.
const walkBeat = serverBeat * time.Millisecond

// Start begins the AI loop.
//
// Thinking and walking are separate loops because upstream separates them:
// Game::checkCreatures calls onThink once a second, while each creature's own
// walk event is scheduled at exactly its getStepDuration. Running both off the
// one-second think loop meant every monster in the game walked at one tile per
// second — a rat and a dragon at the same pace, ground speed ignored, diagonals
// free.
func (e *AIEngine) Start() {
	GlobalDispatcher.AddEvent(creatureThinkInterval, e.updateAI)
	GlobalDispatcher.AddEvent(walkBeat, e.updateWalk)
}

// updateWalk advances every monster's step clock and moves the ones that are
// due. Upstream schedules one event per creature; a single sweep on the beat
// reaches the same grid without one timer per monster.
func (e *AIEngine) updateWalk() {
	const elapsed = uint32(serverBeat)

	for _, c := range e.world.Creatures() {
		monster, ok := c.(*Monster)
		if !ok || monster.Idle {
			continue
		}
		monster.walkTicks += elapsed
		// The step is chosen first and paid for afterwards, because the duration
		// depends on the direction: a diagonal costs three times a straight one,
		// and that has to be charged for the step actually taken.
		if monster.walkTicks < monster.pendingStepCost {
			continue
		}
		dir, ok := monster.GetNextStep(e.world)
		if !ok {
			// Nothing to do this beat. The clock is not reset, so a monster that
			// was blocked steps the moment its path clears rather than waiting out
			// another full duration.
			continue
		}
		monster.walkTicks = 0
		monster.pendingStepCost = monster.GetStepDuration(e.world, dir)
		e.world.TryMoveCreature(monster, dir)
	}

	GlobalDispatcher.AddEvent(walkBeat, e.updateWalk)
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

		// Idle monsters are off the check list, as in C++ Game::checkCreatures /
		// Npc::manageIdle. They wake through spectator events — a player's step
		// calls OnCreatureEnter via notifyCreatureMove — not by being scanned
		// every tick. Without this gate every one of the ~86k monsters ran a full
		// vision scan per think (SpectatorCreatures used to walk all creatures),
		// which pegged the dispatcher and starved the NPC think loop entirely.
		if monster.Idle {
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
		if !monster.IsHostile() {
			if monster.GetTarget() != nil {
				monster.SetTarget(nil)
			}
			monster.UpdateTargetList(e.world)
			monster.UpdateIdleStatus()
			continue
		}

		// A summon does not pick its own fights. Monster::onThink branches on
		// isSummon() before anything else (monster.cpp:1714) and defers entirely
		// to updateSummonTarget: the summon attacks what its master attacks and
		// otherwise follows it. Nothing called that, so summons ran the full
		// hostile AI below and wandered off after their own targets.
		if monster.Master != nil {
			monster.UpdateSummonTarget(e.world)
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
				monster.SetIdle(true)
				continue
			}
		}

		// Movement is not here: it runs on the walk beat, at each monster's own
		// step duration. See updateWalk.
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
