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
			// Nothing queued — don't let the deficit bank up for a lane that has
			// no work. The spawn engine floods LaneGenericParallel with ~84k
			// placement tasks at boot; if the lane had been idle before that (the
			// whole map/script load runs on the main goroutine while the
			// dispatcher waits), the accumulated deficit was huge and dispatchLane
			// spent it in ONE call, processing the entire flood back-to-back and
			// starving every other lane — including the NPC think tick — for
			// minutes. Resetting on an empty heap keeps a lane's budget tied to
			// work it actually has.
			d.deficits[lane] = 0
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
		// A task that takes longer than half a second stalls the whole loop (the
		// dispatcher is single-threaded). The hireling onAppear/RemoveCreature were
		// silently dropped this way, so surface slow tasks rather than masking them.
		if dur := time.Since(start); dur > 500*time.Millisecond {
			slog.Warn("dispatcher task slow", "lane", LaneNames[lane], "taskID", task.ID, "duration", dur.String())
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
