package game

import "context"

// MonsterComputeTask represents a unit of AI computation work submitted to the
// worker pool.
type MonsterComputeTask struct {
	MonsterID uint32
	Result    chan MonsterAIResult
}

// MonsterAIResult is the outcome of a monster's AI computation cycle.
type MonsterAIResult struct {
	TargetID uint32
	NextPos  Position
	Decision string // "attack", "wander", "flee", "idle"
}

// MonsterComputeService is a bounded goroutine pool for offloading monster AI
// computation from the main scheduling loop. In Phase 1, the service is wired
// but not yet used for actual monster AI (the AI engine still runs inline).
// Phase 2+ will migrate AI to submit work here.
type MonsterComputeService struct {
	workers chan struct{} // bounded semaphore
	tasks   chan MonsterComputeTask
	world   *World
}

// NewMonsterComputeService creates a worker pool with the given number of
// workers. Each worker runs a background goroutine that processes tasks.
func NewMonsterComputeService(world *World, numWorkers int) *MonsterComputeService {
	if numWorkers < 1 {
		numWorkers = 4
	}
	mcs := &MonsterComputeService{
		workers: make(chan struct{}, numWorkers),
		tasks:   make(chan MonsterComputeTask, 128),
		world:   world,
	}
	for i := 0; i < numWorkers; i++ {
		go mcs.worker(i)
	}
	return mcs
}

// worker is the background goroutine that processes AI computation tasks.
func (mcs *MonsterComputeService) worker(id int) {
	for task := range mcs.tasks {
		result := mcs.compute(task)
		task.Result <- result
	}
}

// compute runs the AI computation for a single monster. In Phase 1 this is a
// stub; Phase 2 will implement real pathfinding/targeting here.
func (mcs *MonsterComputeService) compute(task MonsterComputeTask) MonsterAIResult {
	// Phase 1: return a default idle result.
	// Phase 2: replace with the full AI evaluation (target selection, pathfinding).
	return MonsterAIResult{
		TargetID: 0,
		Decision: "idle",
	}
}

// Submit enqueues a monster AI computation task. The caller receives the result
// on the returned channel. The channel is closed when the task is complete.
func (mcs *MonsterComputeService) Submit(monsterID uint32) <-chan MonsterAIResult {
	result := make(chan MonsterAIResult, 1)
	mcs.tasks <- MonsterComputeTask{
		MonsterID: monsterID,
		Result:    result,
	}
	return result
}

// Start begins the worker pool. Workers are already running from New; this is
// provided for lifecycle consistency with other services.
func (mcs *MonsterComputeService) Start(ctx context.Context) {
	// Workers are created in NewMonsterComputeService.
	// This method exists for lifecycle symmetry.
}

// Stop shuts down the worker pool and waits for all workers to finish.
func (mcs *MonsterComputeService) Stop() {
	close(mcs.tasks)
}
