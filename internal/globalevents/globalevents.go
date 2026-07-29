// Package globalevents provides a scheduling engine for server-wide events
// (startup, think, time, record, shutdown, period-change, save). The engine
// manages event registration, a background tick scheduler for timed events,
// and lifecycle hooks triggered from the server's main loop.
package globalevents

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// EventType categorises a global event's trigger.
type EventType int

const (
	TypeStartup      EventType = iota // fires once when the server starts
	TypeThink                         // fires on a recurring interval (ms)
	TypeTime                          // fires at a specific wall-clock HH:MM
	TypeRecord                        // fires when a new online-player record is set
	TypeShutdown                      // fires when the server shuts down
	TypePeriodChange                  // fires on day/night period transitions
	TypeSave                          // fires when the server saves
)

var typeNames = map[EventType]string{
	TypeStartup:      "startup",
	TypeThink:        "think",
	TypeTime:         "time",
	TypeRecord:       "record",
	TypeShutdown:     "shutdown",
	TypePeriodChange: "periodchange",
	TypeSave:         "save",
}

func (t EventType) String() string {
	if s, ok := typeNames[t]; ok {
		return s
	}
	return "unknown"
}

// Event represents a single registered globalevent script.
type Event struct {
	Name     string
	Type     EventType
	Interval int64  // milliseconds between think executions
	TimeStr  string // "HH:MM" for time-triggered events

	// Callback is the function invoked when the event fires. It is set by the
	// Lua bindings and must return true on success, false (or panic) on failure.
	Callback func() bool

	// lastExecution tracks when this event last ran (unix nanos), used by
	// the scheduler to respect interval timing.
	lastExecution int64
}

// Engine owns the global event lifecycle and background scheduler.
type Engine struct {
	mu     sync.Mutex
	events []*Event
	byType map[EventType][]*Event // fast lookup by type
	log    *slog.Logger

	// scheduler state
	ticker  *time.Ticker
	stopCh  chan struct{}
	running bool

	// record tracking
	maxPlayers int // highest concurrent player count seen
}

// NewEngine creates an empty globalevents engine.
func NewEngine(log *slog.Logger) *Engine {
	if log == nil {
		log = slog.Default()
	}
	return &Engine{
		log:    log,
		byType: make(map[EventType][]*Event),
		stopCh: make(chan struct{}),
	}
}

// Register adds an event to the engine. It must be called before Start;
// events registered after the scheduler is running are appended to the
// internal lists on the next tick boundary.
func (e *Engine) Register(ev *Event) {
	if ev == nil {
		return
	}
	e.mu.Lock()
	e.events = append(e.events, ev)
	e.byType[ev.Type] = append(e.byType[ev.Type], ev)
	e.mu.Unlock()
}

// EventsByType returns a snapshot of events for the given type.
func (e *Engine) EventsByType(t EventType) []*Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]*Event, len(e.byType[t]))
	copy(out, e.byType[t])
	return out
}

// AllEvents returns a snapshot of all registered events.
func (e *Engine) AllEvents() []*Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]*Event, len(e.events))
	copy(out, e.events)
	return out
}

// ExecuteStartup runs all startup events. It returns the count of successful
// executions. Errors are logged via e.log.
func (e *Engine) ExecuteStartup() int {
	events := e.EventsByType(TypeStartup)
	count := 0
	for _, ev := range events {
		if ev.Callback == nil {
			continue
		}
		e.log.Debug("executing startup event", "name", ev.Name)
		if ev.Callback() {
			count++
		} else {
			e.log.Warn("startup event returned false", "name", ev.Name)
		}
	}
	e.log.Info("executed startup global events", "count", count)
	return count
}

// ExecuteShutdown runs all shutdown events.
func (e *Engine) ExecuteShutdown() {
	events := e.EventsByType(TypeShutdown)
	for _, ev := range events {
		if ev.Callback == nil {
			continue
		}
		e.log.Debug("executing shutdown event", "name", ev.Name)
		if !ev.Callback() {
			e.log.Warn("shutdown event returned false", "name", ev.Name)
		}
	}
}

// ExecuteSave runs all save events.
func (e *Engine) ExecuteSave() {
	events := e.EventsByType(TypeSave)
	for _, ev := range events {
		if ev.Callback == nil {
			continue
		}
		e.log.Debug("executing save event", "name", ev.Name)
		if !ev.Callback() {
			e.log.Warn("save event returned false", "name", ev.Name)
		}
	}
}

// ExecutePeriodChange runs all period-change events.
func (e *Engine) ExecutePeriodChange() {
	events := e.EventsByType(TypePeriodChange)
	for _, ev := range events {
		if ev.Callback == nil {
			continue
		}
		e.log.Debug("executing periodchange event", "name", ev.Name)
		if !ev.Callback() {
			e.log.Warn("periodchange event returned false", "name", ev.Name)
		}
	}
}

// CheckRecord compares current with the stored max and fires record events
// if current exceeds the previous high.
func (e *Engine) CheckRecord(current int) {
	if current <= e.maxPlayers {
		return
	}
	old := e.maxPlayers
	e.maxPlayers = current

	events := e.EventsByType(TypeRecord)
	for _, ev := range events {
		if ev.Callback == nil {
			continue
		}
		e.log.Debug("executing record event", "name", ev.Name, "current", current, "old", old)
		if !ev.Callback() {
			e.log.Warn("record event returned false", "name", ev.Name)
		}
	}
}

// Start launches the background scheduler on a 1-second tick. It dispatches
// think and time events. Call from the server's main goroutine after all Lua
// scripts have been loaded.
func (e *Engine) Start(ctx context.Context) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.ticker = time.NewTicker(1 * time.Second)
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			e.running = false
			if e.ticker != nil {
				e.ticker.Stop()
				e.ticker = nil
			}
			e.mu.Unlock()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.stopCh:
				return
			case <-e.ticker.C:
				e.tick()
			}
		}
	}()
}

// Stop terminates the background scheduler.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		close(e.stopCh)
		// reset the channel so Start can be called again
		e.stopCh = make(chan struct{})
	}
}

// tick is called every second by the scheduler goroutine.
func (e *Engine) tick() {
	now := time.Now()
	nowUnix := now.Unix()
	nowNano := now.UnixNano()

	// Snapshot the think and time event lists under the lock.
	var thinkEvents, timeEvents []*Event
	e.mu.Lock()
	if think, ok := e.byType[TypeThink]; ok {
		thinkEvents = make([]*Event, len(think))
		copy(thinkEvents, think)
	}
	if tm, ok := e.byType[TypeTime]; ok {
		timeEvents = make([]*Event, len(tm))
		copy(timeEvents, tm)
	}
	e.mu.Unlock()

	// Process think events (recurring interval).
	for _, ev := range thinkEvents {
		if ev.Callback == nil || ev.Interval <= 0 {
			continue
		}
		elapsed := nowNano - ev.lastExecution
		if elapsed >= ev.Interval*1_000_000 { // Interval is in ms, convert to nanos
			ev.lastExecution = nowNano
			// e.log.Debug("firing think event", "name", ev.Name)
			if !ev.Callback() {
				e.log.Warn("think event returned false", "name", ev.Name)
			}
		}
	}

	// Process time events (wall-clock HH:MM).
	currentHHMM := now.Format("15:04")
	for _, ev := range timeEvents {
		if ev.Callback == nil || ev.TimeStr == "" {
			continue
		}
		// Only fire once per day - compare date+time
		if currentHHMM == ev.TimeStr {
			// Check we haven't already fired this minute
			if ev.lastExecution < nowUnix*1_000_000_000 { // last was before this second
				ev.lastExecution = nowNano
				e.log.Debug("firing time event", "name", ev.Name, "time", ev.TimeStr)
				if !ev.Callback() {
					e.log.Warn("time event returned false", "name", ev.Name)
				}
			}
		}
	}
}
