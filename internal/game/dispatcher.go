package game

import (
	"container/heap"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// DispatcherStats is a snapshot of the dispatcher's telemetry for monitoring.
type DispatcherStats struct {
	Pending       [laneCount]int32
	Drops         uint64
	P50           time.Duration
	P95           time.Duration
	P99           time.Duration
	ExecutionMode ExecutionMode
}

// WDRRDispatcher implements a Weighted Deficit Round Robin scheduler with 12
// independent priority lanes, adaptive budgets based on P99 latency, and
// concurrent telemetry.
//
// Use GlobalDispatcher for the process-wide instance. Engines that need lane-
// aware scheduling should use AddTask; the legacy AddEvent method maps to
// LaneGenericParallel for backward compatibility.
type WDRRDispatcher struct {
	mu       sync.Mutex
	lanes    [laneCount]*taskHeap
	deficits [laneCount]int32
	quanta   [laneCount]int32
	paused   [laneCount]bool
	seq      uint64

	budgets   *budgetManager
	telemetry *telemetryCollector

	stopCh chan struct{}
	ticker *time.Ticker
	once   sync.Once
}

// NewWDRRDispatcher creates a WDRR dispatcher with default quanta for each lane.
// The dispatcher is not started until Start(ctx) is called.
func NewWDRRDispatcher() *WDRRDispatcher {
	d := &WDRRDispatcher{
		stopCh:    make(chan struct{}),
		ticker:    time.NewTicker(10 * time.Millisecond),
		budgets:   newBudgetManager(),
		telemetry: newTelemetryCollector(),
	}
	for i := 0; i < int(laneCount); i++ {
		h := make(taskHeap, 0)
		d.lanes[i] = &h
		d.quanta[i] = defaultQuanta[i]
	}
	return d
}

// AddEvent runs action once after delay. It is the backward-compatible entry
// point, mapping to LaneGenericParallel with default priority. All existing
// callers of GlobalDispatcher.AddEvent continue to work unchanged.
func (d *WDRRDispatcher) AddEvent(delay time.Duration, action func()) {
	if action == nil {
		return
	}
	d.AddTask(LaneGenericParallel, delay, 0, action)
}

// AddTask enqueues action on the specified lane. It executes at or after delay,
// ordered by priority within its lane (lower priority values run first). Returns
// a monotonic task ID that can be passed to Cancel.
func (d *WDRRDispatcher) AddTask(lane Lane, delay time.Duration, priority int, action func()) uint64 {
	if action == nil || lane < 0 || int(lane) >= int(laneCount) {
		return 0
	}
	id := atomic.AddUint64(&d.seq, 1)
	task := &TaskMeta{
		ID:       id,
		Lane:     lane,
		RunAt:    time.Now().Add(delay),
		Priority: priority,
		Action:   action,
	}
	d.mu.Lock()
	heap.Push(d.lanes[lane], task)
	d.mu.Unlock()
	return id
}

// Cancel removes a pending task by its ID. Returns true if the task was found
// and removed, false if it already ran or the ID is unknown.
func (d *WDRRDispatcher) Cancel(id uint64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := 0; i < int(laneCount); i++ {
		for _, task := range *d.lanes[i] {
			if task.ID == id {
				heap.Remove(d.lanes[i], task.index)
				return true
			}
		}
	}
	return false
}

// Pause prevents a lane from dispatching tasks until Resume is called. Tasks
// already queued remain in the heap and will run once the lane is resumed.
func (d *WDRRDispatcher) Pause(lane Lane) {
	d.mu.Lock()
	d.paused[lane] = true
	d.mu.Unlock()
}

// Resume re-enables a paused lane.
func (d *WDRRDispatcher) Resume(lane Lane) {
	d.mu.Lock()
	d.paused[lane] = false
	d.mu.Unlock()
}

// Start begins the WDRR scheduling loop in a background goroutine. It runs
// until ctx is cancelled or Stop is called.
func (d *WDRRDispatcher) Start(ctx context.Context) {
	go d.scheduleLoop(ctx)
}

// Stop terminates the scheduling loop.
func (d *WDRRDispatcher) Stop() {
	d.once.Do(func() {
		close(d.stopCh)
	})
}

// Stats returns a snapshot of the dispatcher's telemetry, including pending
// counts per lane and latency percentiles.
func (d *WDRRDispatcher) Stats() DispatcherStats {
	s := DispatcherStats{
		ExecutionMode: d.budgets.getMode(),
		Drops:         d.telemetry.drops.Load(),
	}
	for i := 0; i < int(laneCount); i++ {
		d.mu.Lock()
		s.Pending[i] = int32(d.lanes[i].Len())
		d.mu.Unlock()
	}
	p50, p95, p99 := d.telemetry.percentiles()
	s.P50 = p50
	s.P95 = p95
	s.P99 = p99
	return s
}

// GlobalDispatcher is the process-wide WDRR dispatcher singleton. It is NOT
// started automatically — the caller (main.go) must call GlobalDispatcher.Start(ctx).
var GlobalDispatcher = NewWDRRDispatcher()
