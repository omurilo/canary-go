package game

import (
	"container/heap"
	"math"

	"github.com/omurilo/canary-go/internal/items"
)

type pathNode struct {
	pos    Position
	g, h   int
	parent *pathNode
	index  int
}

type priorityQueue []*pathNode

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return (pq[i].g + pq[i].h) < (pq[j].g + pq[j].h)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*pathNode)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

type Pathfinder interface {
	FindNextStep(start, goal Position) Position
}

type AStarPathfinder struct {
	M       *Map
	Catalog *items.Catalog
}

func (a *AStarPathfinder) FindNextStep(start, goal Position) Position {
	path := FindPath(a.M, a.Catalog, start, goal, 200)
	if len(path) > 0 {
		return path[0]
	}
	// Fallback to simple pathfinder logic if path not found
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

func chebyshevDist(p1, p2 Position) int {
	dx := int(p1.X) - int(p2.X)
	if dx < 0 {
		dx = -dx
	}
	dy := int(p1.Y) - int(p2.Y)
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

func FindPath(m *Map, catalog *items.Catalog, start, end Position, maxNodes int) []Position {
	if start == end {
		return nil
	}

	pq := make(priorityQueue, 0)
	heap.Init(&pq)

	startNode := &pathNode{pos: start, g: 0, h: chebyshevDist(start, end)}
	heap.Push(&pq, startNode)

	closedList := make(map[Position]bool)
	openMap := make(map[Position]*pathNode)
	openMap[start] = startNode

	nodesEvaluated := 0

	for pq.Len() > 0 && nodesEvaluated < maxNodes {
		curr := heap.Pop(&pq).(*pathNode)
		delete(openMap, curr.pos)
		closedList[curr.pos] = true
		nodesEvaluated++

		if curr.pos == end || chebyshevDist(curr.pos, end) == 1 {
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

			snap := m.GetSectorSnapshot(int(nextPos.X), int(nextPos.Y), int(nextPos.Z))
			cell := snap.Cell(int(nextPos.X)%8, int(nextPos.Y)%8)
			if !cell.HasGround || cell.BlockSolid {
				if nextPos != end {
					continue
				}
			}

			g := curr.g + 1
			h := chebyshevDist(nextPos, end)

			if inOpen, ok := openMap[nextPos]; ok {
				if g < inOpen.g {
					inOpen.g = g
					inOpen.parent = curr
					heap.Fix(&pq, inOpen.index)
				}
			} else {
				newNode := &pathNode{pos: nextPos, g: g, h: h, parent: curr}
				heap.Push(&pq, newNode)
				openMap[nextPos] = newNode
			}
		}
	}
	return nil
}

// StepDirection returns the single step that moves from `from` towards the
// adjacent tile `to`. Exported form of the helper the AI engine uses, so the
// protocol layer can turn a path from FindPath into the direction list the walk
// machinery consumes.
func StepDirection(from, to Position) Direction { return getDirectionTo(from, to) }
