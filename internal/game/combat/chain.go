package combat

// ChainTarget is a resolved chain jump target.
type ChainTarget struct {
	ID       uint32
	Position Position
}

// ResolveChainTargets performs a BFS from initialTarget to find up to
// maxTargets creatures within chainDistance, respecting backtracking.
// getNearby returns the IDs and positions of all creatures around a position.
func ResolveChainTargets(
	initialID uint32,
	initialPos Position,
	getNearby func(Position, int) []ChainTarget,
	maxTargets, chainDistance int,
	backtracking bool,
	pickerCallback func(id uint32) bool,
) []ChainTarget {
	if maxTargets <= 1 || chainDistance <= 0 {
		return nil
	}

	visited := make(map[uint32]bool)
	result := make([]ChainTarget, 0, maxTargets-1)
	queue := []ChainTarget{{ID: initialID, Position: initialPos}}

	visited[initialID] = true

	for len(queue) > 0 && len(result) < maxTargets-1 {
		current := queue[0]
		queue = queue[1:]

		candidates := getNearby(current.Position, chainDistance)
		for _, candidate := range candidates {
			if visited[candidate.ID] {
				continue
			}
			if !backtracking && isBehind(initialPos, current.Position, candidate.Position) {
				continue
			}
			if pickerCallback != nil && !pickerCallback(candidate.ID) {
				continue
			}
			visited[candidate.ID] = true
			result = append(result, candidate)
			queue = append(queue, candidate)
			if len(result) >= maxTargets-1 {
				break
			}
		}
	}
	return result
}

// isBehind returns true if candidate is on the same side of current as the
// reference point (caster). Used when backtracking=false to prevent the chain
// from jumping back toward the caster.
func isBehind(reference, current, candidate Position) bool {
	dx := int(current.X) - int(reference.X)
	dy := int(current.Y) - int(reference.Y)
	if dx == 0 && dy == 0 {
		return false
	}
	cx := int(candidate.X) - int(current.X)
	cy := int(candidate.Y) - int(current.Y)
	dot := dx*cx + dy*cy
	return dot < 0
}
