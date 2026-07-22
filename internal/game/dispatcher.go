package game

import (
	"context"
	"sync"
	"time"
)

// scheduledTask is a unit of delayed work queued on a Dispatcher's task loop.
type scheduledTask struct {
	runAt time.Time
	fn    func()
}

// Dispatcher schedules delayed work. It supports two independent styles:
//
//   - AddEvent(delay, fn): fire-and-forget one-shot timers (time.AfterFunc).
//     Used by the AI/spawn/combat engines via GlobalDispatcher.
//   - NewDispatcher(interval) + Start/AddTask/Stop: a drained task queue that
//     wakes on a fixed interval and runs any tasks whose delay has elapsed.
type Dispatcher struct {
	interval time.Duration

	mu    sync.Mutex
	tasks []scheduledTask
	stop  chan struct{}
	once  sync.Once
}

// NewDispatcher creates a dispatcher whose Start loop wakes every interval.
func NewDispatcher(interval time.Duration) *Dispatcher {
	if interval <= 0 {
		interval = time.Millisecond
	}
	return &Dispatcher{
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// AddEvent runs action once after delay on its own timer.
func (d *Dispatcher) AddEvent(delay time.Duration, action func()) {
	time.AfterFunc(delay, action)
}

// AddTask enqueues fn to run at or after delay, drained by the Start loop.
func (d *Dispatcher) AddTask(delay time.Duration, fn func()) {
	d.mu.Lock()
	d.tasks = append(d.tasks, scheduledTask{runAt: time.Now().Add(delay), fn: fn})
	d.mu.Unlock()
}

// Start drains due tasks every interval until ctx is cancelled or Stop is called.
func (d *Dispatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stop:
			return
		case <-ticker.C:
			d.runDue()
		}
	}
}

func (d *Dispatcher) runDue() {
	now := time.Now()
	d.mu.Lock()
	var due []func()
	remaining := d.tasks[:0]
	for _, t := range d.tasks {
		if !t.runAt.After(now) {
			due = append(due, t.fn)
		} else {
			remaining = append(remaining, t)
		}
	}
	d.tasks = remaining
	d.mu.Unlock()
	for _, fn := range due {
		fn()
	}
}

// Stop terminates a running Start loop.
func (d *Dispatcher) Stop() {
	if d.stop == nil {
		return
	}
	d.once.Do(func() { close(d.stop) })
}

// GlobalDispatcher is the process-wide one-shot scheduler used by the engines.
var GlobalDispatcher = &Dispatcher{}
