package game

import "math"

type Pathfinder interface {
	FindNextStep(start, goal Position) Position
}

// SimplePathfinder implements a basic target-chasing logic
// It moves 1 step diagonally or straight towards the goal
type SimplePathfinder struct{}

func (p *SimplePathfinder) FindNextStep(start, goal Position) Position {
	next := start
	if next.X < goal.X {
		next.X++
	} else if next.X > goal.X {
		next.X--
	}

	if next.Y < goal.Y {
		next.Y++
	} else if next.Y > goal.Y {
		next.Y--
	}
	// In a real pathfinder, we would check for obstacles here.
	return next
}

// AStarPathfinder is a stub for real A* pathfinding.
// Currently acts similar to SimplePathfinder but can be expanded with map checks.
type AStarPathfinder struct {
	// Map instance would go here
}

func (a *AStarPathfinder) FindNextStep(start, goal Position) Position {
	// Dummy A* implementation, falls back to direct chasing for now.
	// Implementing full A* requires a grid/map implementation to check for unwalkable tiles.
	next := start
	
	dx := goal.X - start.X
	dy := goal.Y - start.Y

	if math.Abs(float64(dx)) > math.Abs(float64(dy)) {
		if dx > 0 {
			next.X++
		} else {
			next.X--
		}
	} else {
		if dy > 0 {
			next.Y++
		} else {
			next.Y--
		}
	}
	
	return next
}
