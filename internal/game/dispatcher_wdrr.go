package game

import (
	"container/heap"
	"context"
	"log/slog"
	"time"
)

const wdrrTickInterval = 10 * time.Millisecond

// scheduleLoop is the main WDRR scheduling goroutine. It ticks every 10ms and
// dispatches work from each non-paused lane using deficit round-robin.
func (d *WDRRDispatcher) scheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(wdrrTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.dispatchCycle()
		}
	}
}

// dispatchCycle runs one WDRR round across all lanes. For each non-paused lane,
// its deficit is topped up by the effective quantum, then tasks are popped and
// executed while the deficit allows.
func (d *WDRRDispatcher) dispatchCycle() {
	for lane := Lane(0); lane < laneCount; lane++ {
		if d.isPaused(lane) {
			continue
		}

		effective := d.budgets.effectiveQuantum(d.quanta, lane)
		d.deficits[lane] += effective

		d.dispatchLane(lane)
	}
}

// dispatchLane pops and executes tasks from a lane's priority heap while the
// deficit counter is positive and tasks are due. Unused deficit carries forward.
func (d *WDRRDispatcher) dispatchLane(lane Lane) {
	now := time.Now()

	for d.deficits[lane] > 0 {
		d.mu.Lock()
		h := d.lanes[lane]
		task := h.Peek()
		if task == nil {
			d.mu.Unlock()
			return
		}
		if task.RunAt.After(now) {
			// Next task is not yet due; keep it queued.
			d.mu.Unlock()
			return
		}
		heap.Remove(h, task.index)
		d.deficits[lane]--
		d.mu.Unlock()

		// Execute outside the lock.
		d.executeTask(lane, task)
	}
}

// executeTask runs a single task and records its duration in telemetry.
func (d *WDRRDispatcher) executeTask(lane Lane, task *TaskMeta) {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dispatcher task panicked", "lane", LaneNames[lane], "taskID", task.ID, "panic", r)
		}
		d.telemetry.record(start)
		d.budgets.reportLatency(time.Since(start))
	}()

	task.Action()
}

// isPaused returns whether a lane is currently paused.
func (d *WDRRDispatcher) isPaused(lane Lane) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.paused[lane]
}
