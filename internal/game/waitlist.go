package game

import (
	"sync"
	"time"
)

// Wait represents a player waiting to log in.
type Wait struct {
	Timeout    time.Time
	PlayerGUID uint32
}

// WaitingList manages the login queue for when the server is full.
type WaitingList struct {
	mu sync.Mutex

	priorityList []Wait
	normalList   []Wait
	// playerGUID -> (index in priorityList or normalList, bool priority)
	playerRefs map[uint32]struct {
		Index    int
		Priority bool
	}
	nextSlot uint64
}

// NewWaitingList creates a new waiting list.
func NewWaitingList() *WaitingList {
	return &WaitingList{
		playerRefs: make(map[uint32]struct {
			Index    int
			Priority bool
		}),
	}
}

// AddPlayer adds a player to the waiting list.
func (wl *WaitingList) AddPlayer(playerGUID uint32, premium bool) {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	// Remove existing entry if any
	if ref, ok := wl.playerRefs[playerGUID]; ok {
		if ref.Priority {
			wl.priorityList = append(wl.priorityList[:ref.Index], wl.priorityList[ref.Index+1:]...)
		} else {
			wl.normalList = append(wl.normalList[:ref.Index], wl.normalList[ref.Index+1:]...)
		}
	}

	wait := Wait{
		Timeout:    time.Now().Add(5 * time.Second),
		PlayerGUID: playerGUID,
	}

	if premium {
		wl.priorityList = append(wl.priorityList, wait)
		wl.playerRefs[playerGUID] = struct {
			Index    int
			Priority bool
		}{len(wl.priorityList) - 1, true}
	} else {
		wl.normalList = append(wl.normalList, wait)
		wl.playerRefs[playerGUID] = struct {
			Index    int
			Priority bool
		}{len(wl.normalList) - 1, false}
	}
}

// RemovePlayer removes a player from the waiting list.
func (wl *WaitingList) RemovePlayer(playerGUID uint32) {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	if ref, ok := wl.playerRefs[playerGUID]; ok {
		if ref.Priority {
			wl.priorityList = append(wl.priorityList[:ref.Index], wl.priorityList[ref.Index+1:]...)
		} else {
			wl.normalList = append(wl.normalList[:ref.Index], wl.normalList[ref.Index+1:]...)
		}
		delete(wl.playerRefs, playerGUID)
	}
}

// GetSlot returns the player's position in the queue.
func (wl *WaitingList) GetSlot(playerGUID uint32) int {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	wl.cleanup()

	if _, ok := wl.playerRefs[playerGUID]; !ok {
		return 0
	}

	slot := 0
	// Count priority list entries that haven't timed out
	for _, w := range wl.priorityList {
		if time.Now().Before(w.Timeout) {
			slot++
		}
	}
	// Find this player in normal list
	for _, w := range wl.normalList {
		if time.Now().Before(w.Timeout) {
			slot++
		}
		if w.PlayerGUID == playerGUID {
			return slot
		}
	}
	return slot
}

// cleanup removes timed-out entries from both lists.
func (wl *WaitingList) cleanup() {
	now := time.Now()
	keep := func(list []Wait) []Wait {
		var result []Wait
		for _, w := range list {
			if now.Before(w.Timeout) {
				result = append(result, w)
			} else {
				delete(wl.playerRefs, w.PlayerGUID)
			}
		}
		return result
	}
	wl.priorityList = keep(wl.priorityList)
	wl.normalList = keep(wl.normalList)
}

// ClientLogin checks if a player can log in (not queued) or adds them to the queue.
// Returns true if the player can log in immediately.
func (wl *WaitingList) ClientLogin(playerGUID uint32, premium bool) bool {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	// If the player is already in the queue and it's their turn, let them in
	if ref, ok := wl.playerRefs[playerGUID]; ok {
		wl.cleanup()
		// Check if they're first in their queue
		if ref.Priority && len(wl.priorityList) > 0 && wl.priorityList[0].PlayerGUID == playerGUID {
			wl.priorityList = wl.priorityList[1:]
			delete(wl.playerRefs, playerGUID)
			return true
		}
		if !ref.Priority && len(wl.normalList) > 0 && wl.normalList[0].PlayerGUID == playerGUID {
			wl.normalList = wl.normalList[1:]
			delete(wl.playerRefs, playerGUID)
			return true
		}
		return false
	}

	// Player not in queue - check if server is full
	// For now, always let them in (queue logic can be enhanced later)
	return true
}
