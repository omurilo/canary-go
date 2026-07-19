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
		if _, isMonster := c.(*Monster); !isMonster {
			continue
		}

		target := c.GetTarget()
		if target != nil {
			// Check if target is still valid
			if targetPlayer, ok := target.(*Player); ok {
				if e.world.PlayerByID(targetPlayer.GetID()) == nil || !c.GetPosition().InRangeOf(target.GetPosition()) {
					c.SetTarget(nil)
				}
			} else {
				c.SetTarget(nil)
			}
		}

		// Aggro: Find a target if we don't have one
		if c.GetTarget() == nil {
			players := e.world.Spectators(c.GetPosition(), c.GetID())
			var closest *Player
			var minDist int = 1000
			for _, p := range players {
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
			path := findPath(e.world.Map, c.GetPosition(), c.GetTarget().GetPosition())
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

// Basic A* Pathfinding (very simple implementation)
type node struct {
	pos Position
	g, h, f int
	parent *node
}

func findPath(m *Map, start, end Position) []Position {
	if start == end {
		return nil
	}

	openList := []*node{{pos: start, g: 0, h: chebyshevDistance(start, end)}}
	closedList := make(map[Position]bool)
	openList[0].f = openList[0].g + openList[0].h

	for len(openList) > 0 && len(closedList) < 100 { // Max 100 nodes to prevent lag
		var curr *node
		var currIdx int
		for i, n := range openList {
			if curr == nil || n.f < curr.f {
				curr = n
				currIdx = i
			}
		}

		openList = append(openList[:currIdx], openList[currIdx+1:]...)
		closedList[curr.pos] = true

		if curr.pos == end || chebyshevDistance(curr.pos, end) == 1 {
			// Found path
			var path []Position
			for curr != nil && curr.pos != start {
				path = append([]Position{curr.pos}, path...)
				curr = curr.parent
			}
			return path
		}

		dirs := []Direction{DirNorth, DirEast, DirSouth, DirWest, DirNE, DirNW, DirSE, DirSW}
		for _, d := range dirs {
			nextPos := curr.pos.Offset(d)
			if closedList[nextPos] {
				continue
			}

			// Check if walkable (basic check)
			tile := m.GetTile(nextPos)
			if tile == nil || !tile.Walkable() {
				// We can't walk there
				if nextPos != end { // If it's the target, we don't need to walk ON it
					continue
				}
			}

			g := curr.g + 1
			h := chebyshevDistance(nextPos, end)
			f := g + h

			var inOpen *node
			for _, n := range openList {
				if n.pos == nextPos {
					inOpen = n
					break
				}
			}

			if inOpen == nil {
				openList = append(openList, &node{pos: nextPos, g: g, h: h, f: f, parent: curr})
			} else if g < inOpen.g {
				inOpen.g = g
				inOpen.f = f
				inOpen.parent = curr
			}
		}
	}
	return nil
}
