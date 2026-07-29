package game

import (
	"sort"
	"sync"
	"time"
)

// budgetManager monitors P99 task latency and transitions between execution
// modes (Normal → Constrained → Emergency) to apply back-pressure when the
// dispatcher is under load.
type budgetManager struct {
	mu          sync.Mutex
	currentMode ExecutionMode
	samples     []time.Duration
	head        int
	count       int
	normalSince int // consecutive windows below recovery threshold
}

const (
	windowSize           = 1000
	constrainedThreshold = 50 * time.Millisecond
	emergencyThreshold   = 200 * time.Millisecond
	recoveryThreshold    = 30 * time.Millisecond
	recoveryWindows      = 5
)

func newBudgetManager() *budgetManager {
	return &budgetManager{
		currentMode: ModeNormal,
		samples:     make([]time.Duration, windowSize),
	}
}

// reportLatency records a task execution duration into the sliding window and
// transitions the execution mode if the P99 crosses the configured thresholds.
func (bm *budgetManager) reportLatency(d time.Duration) {
	bm.mu.Lock()
	bm.samples[bm.head] = d
	bm.head = (bm.head + 1) % windowSize
	if bm.count < windowSize {
		bm.count++
	}
	bm.mu.Unlock()

	p99 := bm.computeP99()
	mode := bm.getMode()

	switch {
	case p99 >= emergencyThreshold:
		if mode != ModeEmergency {
			bm.setMode(ModeEmergency)
		}
		bm.resetNormalSince()
	case p99 >= constrainedThreshold:
		if mode == ModeNormal {
			bm.setMode(ModeConstrained)
		}
		bm.resetNormalSince()
	case p99 < recoveryThreshold:
		bm.incNormalSince()
		if bm.normalSince >= recoveryWindows {
			bm.setMode(ModeNormal)
		}
	default:
		bm.resetNormalSince()
	}
}

// computeP99 calculates the P99 latency from the sliding window.
func (bm *budgetManager) computeP99() time.Duration {
	bm.mu.Lock()
	count := bm.count
	sorted := make([]time.Duration, count)
	for i := 0; i < count; i++ {
		sorted[i] = bm.samples[i]
	}
	bm.mu.Unlock()

	if count == 0 {
		return 0
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(count) * 0.99)
	if idx >= count {
		idx = count - 1
	}
	return sorted[idx]
}

// getMode returns the current execution mode safely.
func (bm *budgetManager) getMode() ExecutionMode {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.currentMode
}

// setMode sets the execution mode.
func (bm *budgetManager) setMode(m ExecutionMode) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.currentMode = m
}

// resetNormalSince resets the recovery counter.
func (bm *budgetManager) resetNormalSince() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.normalSince = 0
}

// incNormalSince increments the recovery counter.
func (bm *budgetManager) incNormalSince() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.normalSince++
}

// scale returns the divisor applied to a lane's base quantum under the current
// execution mode. The effective quantum is base / scale.
//
// ModeNormal:     all lanes 1.0x (scale=1)
// ModeConstrained: batch lanes 0.5x (scale=2), interactive lanes 1.0x (scale=1)
// ModeEmergency:   all lanes 0.25x (scale=4) except ProtocolInput/PlayerWalk 1.0x
func (bm *budgetManager) scale(lane Lane) int32 {
	mode := bm.getMode()

	switch mode {
	case ModeNormal:
		return 1
	case ModeConstrained:
		switch lane {
		case LaneBackgroundMonster, LaneMaintenance, LaneDeferred:
			return 2 // 0.5x quantum
		default:
			return 1 // 1.0x quantum
		}
	case ModeEmergency:
		switch lane {
		case LaneProtocolInput, LanePlayerWalk:
			return 1 // 1.0x — keep the server responsive
		default:
			return 4 // 0.25x quantum
		}
	default:
		return 1
	}
}

// effectiveQuantum returns the quantum for a lane after applying the budget
// mode's scale factor.
func (bm *budgetManager) effectiveQuantum(quanta [laneCount]int32, lane Lane) int32 {
	return quanta[lane] / bm.scale(lane)
}
