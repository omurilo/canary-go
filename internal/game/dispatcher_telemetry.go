package game

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// telemetryCollector records task execution durations in a sliding window for
// latency percentiles and maintains atomic counters for drops and pending tasks.
type telemetryCollector struct {
	mu      sync.Mutex
	samples []time.Duration
	head    int
	count   int

	drops   atomic.Uint64
	pending [laneCount]atomic.Int32
}

func newTelemetryCollector() *telemetryCollector {
	return &telemetryCollector{
		samples: make([]time.Duration, 1000),
	}
}

// record measures one task execution and feeds it into the latency window.
func (tc *telemetryCollector) record(start time.Time) {
	d := time.Since(start)
	tc.mu.Lock()
	tc.samples[tc.head] = d
	tc.head = (tc.head + 1) % len(tc.samples)
	if tc.count < len(tc.samples) {
		tc.count++
	}
	tc.mu.Unlock()
}

// percentiles computes P50, P95, P99 from the sliding window.
func (tc *telemetryCollector) percentiles() (p50, p95, p99 time.Duration) {
	tc.mu.Lock()
	count := tc.count
	sorted := make([]time.Duration, count)
	for i := 0; i < count; i++ {
		sorted[i] = tc.samples[i]
	}
	tc.mu.Unlock()

	if count == 0 {
		return 0, 0, 0
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	p50 = sorted[int(float64(count)*0.50)]
	p95 = sorted[int(float64(count)*0.95)]
	p99 = sorted[int(float64(count)*0.99)]
	return
}

// incDrop increments the drop counter.
func (tc *telemetryCollector) incDrop() {
	tc.drops.Add(1)
}

// incPending increments the pending counter for a lane.
func (tc *telemetryCollector) incPending(lane Lane) {
	if int(lane) < len(tc.pending) {
		tc.pending[lane].Add(1)
	}
}

// decPending decrements the pending counter for a lane.
func (tc *telemetryCollector) decPending(lane Lane) {
	if int(lane) < len(tc.pending) {
		tc.pending[lane].Add(-1)
	}
}
