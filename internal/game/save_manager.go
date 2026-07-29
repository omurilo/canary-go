package game

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// PlayerSaver is the interface the SaveManager uses to persist a player.
// The db.DB type satisfies this interface.
type PlayerSaver interface {
	SavePlayer(ctx context.Context, p *Player) error
}

// SaveManager provides an asynchronous, deduplicated player save queue. Saves
// are coalesced by player DBID so that only the latest state is persisted when
// the flush interval fires.
//
// In Phase 1, this is wired as infrastructure alongside the existing inline
// saves (death, logout). Engines may optionally use EnqueuePlayer for periodic
// saves without blocking the caller.
type SaveManager struct {
	queue   chan saveRequest
	pending map[uint32]struct{} // dedup by player DBID
	mu      sync.Mutex
	world   *World
	db      PlayerSaver
}

type saveRequest struct {
	PlayerID uint32
	Cause    string // "death", "interval", "logout"
}

// NewSaveManager creates a save manager with a 128-entry buffered queue and
// starts the background flush goroutine.
func NewSaveManager(world *World, db PlayerSaver) *SaveManager {
	sm := &SaveManager{
		queue:   make(chan saveRequest, 128),
		pending: make(map[uint32]struct{}),
		world:   world,
		db:      db,
	}
	return sm
}

// Start launches the background flush loop. Must be called before EnqueuePlayer.
func (sm *SaveManager) Start(ctx context.Context) {
	go sm.run(ctx)
}

// EnqueuePlayer adds a player to the save queue. If a save for the same player
// DBID is already pending, this new request is coalesced (dedup).
func (sm *SaveManager) EnqueuePlayer(p *Player, cause string) {
	if p == nil {
		return
	}
	sm.mu.Lock()
	if _, exists := sm.pending[p.DBID]; exists {
		sm.mu.Unlock()
		return // already queued
	}
	sm.pending[p.DBID] = struct{}{}
	sm.mu.Unlock()

	sm.queue <- saveRequest{PlayerID: p.DBID, Cause: cause}
}

// run is the background goroutine that drains the save queue in batches.
func (sm *SaveManager) run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var batch []saveRequest
	for {
		select {
		case <-ctx.Done():
			sm.flush(batch)
			return
		case req := <-sm.queue:
			batch = append(batch, req)
			if len(batch) >= 10 {
				sm.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				sm.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

// flush persists all players in the batch, then clears their pending flags.
func (sm *SaveManager) flush(batch []saveRequest) {
	for _, req := range batch {
		p := sm.world.PlayerByDBID(req.PlayerID)
		if p == nil {
			sm.removePending(req.PlayerID)
			continue
		}
		if err := sm.db.SavePlayer(context.Background(), p); err != nil {
			slog.Error("save manager: failed to save player",
				"playerID", req.PlayerID, "cause", req.Cause, "err", err)
		}
		sm.removePending(req.PlayerID)
	}
}

// removePending removes a player from the dedup set.
func (sm *SaveManager) removePending(playerID uint32) {
	sm.mu.Lock()
	delete(sm.pending, playerID)
	sm.mu.Unlock()
}
