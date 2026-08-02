package game

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestNpcTickNotStarvedBySpawnFlood simulates boot: a flood of spawn tasks on
// LaneGenericParallel must not delay the NpcEngine tick on its own lane.
func TestNpcTickNotStarvedBySpawnFlood(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := NewWDRRDispatcher()
	d.Start(ctx)

	// Flood LaneGenericParallel with 2000 tasks that each do a little work.
	for i := 0; i < 2000; i++ {
		d.AddTask(LaneGenericParallel, 0, 0, func() {
			time.Sleep(time.Millisecond)
		})
	}

	var ticks atomic.Int32
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			d.AddTask(LaneNpcThink, 50*time.Millisecond, 0, func() {
				ticks.Add(1)
			})
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Give the spawn flood time to be mostly processed; the tick must still run.
	time.Sleep(500 * time.Millisecond)
	firstTick := ticks.Load()
	time.Sleep(500 * time.Millisecond)
	close(stop)

	if firstTick == 0 {
		t.Fatalf("no tick ran during the spawn flood — the dedicated lane is starved")
	}
	if ticks.Load() <= firstTick {
		t.Errorf("ticks stalled: first=%d later=%d", firstTick, ticks.Load())
	}
	t.Logf("ticks during flood=%d total=%d", firstTick, ticks.Load())
}

// TestIdleLaneDeficitDoesNotStarveOthers is the boot scenario the spawn flood
// used to trip: a lane sits idle (the dispatcher runs for a whole map/script
// load with nothing on the lane), banking deficit every cycle, then a flood of
// tasks lands. Before the empty-heap reset the flood ran in a single
// dispatchLane call, starving the NPC tick lane for the whole flood.
func TestIdleLaneDeficitDoesNotStarveOthers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := NewWDRRDispatcher()
	d.Start(ctx)

	// Let LaneGenericParallel sit empty long enough to bank a large deficit.
	time.Sleep(200 * time.Millisecond)

	start := time.Now()
	// Flood it with tasks that each take ~1ms.
	var done atomic.Int32
	for i := 0; i < 3000; i++ {
		d.AddTask(LaneGenericParallel, 0, 0, func() {
			time.Sleep(time.Millisecond)
			done.Add(1)
		})
	}

	// A periodic task on another lane must fire while the flood is underway.
	var ticks atomic.Int32
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			d.AddTask(LaneNpcThink, 50*time.Millisecond, 0, func() { ticks.Add(1) })
			time.Sleep(50 * time.Millisecond)
		}
	}()
	defer close(stop)

	time.Sleep(400 * time.Millisecond)
	if ticks.Load() == 0 {
		t.Fatalf("tick lane starved while the flood lane processes its backlog")
	}
	if done.Load() >= 3000 {
		t.Errorf("flood finished too fast for the tick to interleave: done=%d", done.Load())
	}
	t.Logf("flood done=%d ticks=%d elapsed=%s", done.Load(), ticks.Load(), time.Since(start).Round(time.Millisecond))
}
