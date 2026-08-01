package game

import (
	"sync"
	"time"
)

// BanInfo holds ban details returned from the database.
type BanInfo struct {
	BannedBy  string
	Reason    string
	ExpiresAt time.Time
}

// ConnectBlock tracks IP connection rate limiting.
type ConnectBlock struct {
	LastAttempt time.Time
	BlockTime   time.Duration
	Count       uint32
}

// Ban provides IP-based connection rate limiting.
type Ban struct {
	mu    sync.Mutex
	ipMap map[string]*ConnectBlock
}

// NewBan creates a new Ban.
func NewBan() *Ban {
	return &Ban{ipMap: make(map[string]*ConnectBlock)}
}

// AcceptConnection checks and records an IP connection attempt.
// Returns true if the connection is allowed.
func (b *Ban) AcceptConnection(ip string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	block, exists := b.ipMap[ip]

	if !exists {
		b.ipMap[ip] = &ConnectBlock{
			LastAttempt: now,
			BlockTime:   100 * time.Millisecond,
			Count:       1,
		}
		return true
	}

	// Reset if enough time has passed
	if now.Sub(block.LastAttempt) > time.Minute {
		block.Count = 1
		block.BlockTime = 100 * time.Millisecond
		block.LastAttempt = now
		return true
	}

	block.LastAttempt = now
	block.Count++

	// Progressive blocking
	switch {
	case block.Count > 20:
		block.BlockTime = 10 * time.Second
		return false
	case block.Count > 10:
		block.BlockTime = 1 * time.Second
		return false
	}

	return true
}

// IOBan provides database ban checking functions.
type IOBan struct{}

// IsAccountBanned checks if an account is banned.
func (IOBan) IsAccountBanned(accountID uint32) (bool, BanInfo) {
	return false, BanInfo{}
}

// IsIPBanned checks if an IP is banned.
func (IOBan) IsIPBanned(ip string) (bool, BanInfo) {
	return false, BanInfo{}
}

// IsPlayerNamelocked checks if a player name is locked.
func (IOBan) IsPlayerNamelocked(playerID uint32) bool {
	return false
}
