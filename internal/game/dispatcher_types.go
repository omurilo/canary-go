package game

import (
	"container/heap"
	"time"
)

// Lane identifies one of the 12 WDRR scheduling lanes. Each lane has its own
// priority heap and deficit counter, ensuring fair resource allocation across
// different classes of work.
type Lane int

const (
	LaneProtocolInput      Lane = iota // Raw protocol packet reads
	LanePlayerWalk                     // Walk/auto-walk decisions
	LanePlayerAction                   // Player actions (use, loot, equip)
	LaneWorldCommit                    // World state commit / tile updates
	LaneWorkerCompletion               // Async worker completion callbacks
	LaneVisibleMonster                 // Visible monster tasks (near players)
	LaneBackgroundMonster              // Background monster tasks (far from players)
	LaneVisibleMonsterAI               // High-priority monster AI decisions
	LaneMonsterAI                      // General monster AI
	LaneDeferred                       // Deferred / best-effort work
	LaneMaintenance                    // Server maintenance tasks (save, cleanup)
	LaneGenericParallel                // General purpose (backward compat for AddEvent)

	laneCount
)

// LaneNames maps each lane to its human-readable name for debugging/telemetry.
var LaneNames = map[Lane]string{
	LaneProtocolInput:      "ProtocolInput",
	LanePlayerWalk:         "PlayerWalk",
	LanePlayerAction:       "PlayerAction",
	LaneWorldCommit:        "WorldCommit",
	LaneWorkerCompletion:   "WorkerCompletion",
	LaneVisibleMonster:     "VisibleMonster",
	LaneBackgroundMonster:  "BackgroundMonster",
	LaneVisibleMonsterAI:   "VisibleMonsterAI",
	LaneMonsterAI:          "MonsterAI",
	LaneDeferred:           "Deferred",
	LaneMaintenance:        "Maintenance",
	LaneGenericParallel:    "GenericParallel",
}

// TaskMeta describes a unit of work queued on a lane's priority heap.
type TaskMeta struct {
	// ID is a monotonic identifier used for cancellation.
	ID uint64
	// Lane identifies which scheduling lane this task belongs to.
	Lane Lane
	// RunAt is the earliest wall-clock time this task may execute.
	RunAt time.Time
	// Priority within the lane; lower values execute first.
	Priority int
	// Action is the closure to invoke when the task is dispatched.
	Action func()
	// index is the position in the heap, managed by container/heap.
	index int
}

// ExecutionMode represents the adaptive budget state of the dispatcher.
type ExecutionMode int

const (
	// ModeNormal is the default state; all lanes run at full quantum.
	ModeNormal ExecutionMode = iota
	// ModeConstrained reduces batch-lane quanta when P99 latency is elevated.
	ModeConstrained
	// ModeEmergency throttles all non-critical lanes when P99 latency is critical.
	ModeEmergency
)

// taskHeap implements heap.Interface for TaskMeta, ordered by (RunAt, Priority).
type taskHeap []*TaskMeta

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	// Primary sort key: execution time (earlier first).
	if !h[i].RunAt.Equal(h[j].RunAt) {
		return h[i].RunAt.Before(h[j].RunAt)
	}
	// Secondary sort key: priority (lower number = higher priority).
	return h[i].Priority < h[j].Priority
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *taskHeap) Push(x any) {
	n := len(*h)
	item := x.(*TaskMeta)
	item.index = n
	*h = append(*h, item)
}

func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // mark as removed
	*h = old[0 : n-1]
	return item
}

// Peek returns the top item without removing it, or nil if empty.
func (h *taskHeap) Peek() *TaskMeta {
	if len(*h) == 0 {
		return nil
	}
	return (*h)[0]
}

// defaultQuanta maps each lane to its WDRR byte-weight quantum. Higher values
// give the lane more work per scheduling cycle.
var defaultQuanta = [laneCount]int32{
	LaneProtocolInput:      80,
	LanePlayerWalk:         60,
	LanePlayerAction:       60,
	LaneWorldCommit:        40,
	LaneWorkerCompletion:   50,
	LaneVisibleMonster:     30,
	LaneBackgroundMonster:  15,
	LaneVisibleMonsterAI:   25,
	LaneMonsterAI:          20,
	LaneDeferred:           10,
	LaneMaintenance:        5,
	LaneGenericParallel:    30,
}

// Ensure taskHeap implements heap.Interface at compile time.
var _ heap.Interface = (*taskHeap)(nil)
