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

		target := c.GetTarget()
		if target != nil {
			// Check if target is still valid
			if targetPlayer, ok := target.(*Player); ok {
				if e.world.PlayerByID(targetPlayer.GetID()) == nil ||
					targetPlayer.CannotBeAttacked() ||
					!c.GetPosition().InRangeOf(target.GetPosition()) {
					c.SetTarget(nil)
				}
			} else {
				c.SetTarget(nil)
			}
		}

		// Aggro: Find a target if we don't have one. Staff (god/gm/community
		// manager) and ghosts cannot be attacked, so monsters never aggro them —
		// mirroring PlayerFlags_t::CannotBeAttacked.
		if c.GetTarget() == nil {
			players := e.world.Spectators(c.GetPosition(), c.GetID())
			var closest *Player
			var minDist int = 1000
			for _, p := range players {
				if p.CannotBeAttacked() {
					continue
				}
				dist := chebyshevDistance(c.GetPosition(), p.GetPosition())
				if dist < minDist {
					minDist = dist
					closest = p
				}
			}
			if closest != nil {
				c.SetTarget(closest)
			}
		}

		// Move towards target or wander
		if c.GetTarget() != nil {
			// Pathfinding towards target
			path := FindPath(e.world.Map, e.world.Items, c.GetPosition(), c.GetTarget().GetPosition(), 100)
			if len(path) > 0 {
				nextPos := path[0]
				e.world.TryMoveCreature(c, getDirectionTo(c.GetPosition(), nextPos))
			}
		} else {
			// Wander
			if rand.Intn(3) == 0 { // 33% chance to move randomly
				dirs := []Direction{DirNorth, DirEast, DirSouth, DirWest}
				dir := dirs[rand.Intn(len(dirs))]
				e.world.TryMoveCreature(c, dir)
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

