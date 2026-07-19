package game

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatcher_Tasks(t *testing.T) {
	d := NewDispatcher(10 * time.Millisecond)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go d.Start(ctx)

	var counter int32

	d.AddTask(0, func() {
		atomic.AddInt32(&counter, 1)
	})

	d.AddTask(20*time.Millisecond, func() {
		atomic.AddInt32(&counter, 2)
	})

	// Wait enough time for the tasks to finish
	time.Sleep(50 * time.Millisecond)

	val := atomic.LoadInt32(&counter)
	if val != 3 {
		t.Errorf("Expected counter to be 3, got %d", val)
	}
	d.Stop()
}
