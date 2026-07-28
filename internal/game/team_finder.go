package game

import "sync"

// TeamFinder represents a team assemble listing.
type TeamFinder struct {
	MinLevel   uint16
	MaxLevel   uint16
	VocationID uint8
	TeamSlots  uint16
	FreeSlots  uint16
	PartyBool  bool
	Timestamp  uint32
	TeamType   uint8   // 1=boss, 2=hunt, 3=quest
	BossID     uint16
	HuntType   uint16
	HuntArea   uint16
	QuestID    uint16
	LeaderGUID uint32
	// MembersMap: player GUID -> status (0=invited, 1=accepted, 2=declined, 3=member)
	MembersMap map[uint32]uint8
}

// TeamFinderManager manages team finder listings.
type TeamFinderManager struct {
	mu       sync.RWMutex
	entries  map[uint32]*TeamFinder // leader GUID -> entry
}

// NewTeamFinderManager creates a new TeamFinderManager.
func NewTeamFinderManager() *TeamFinderManager {
	return &TeamFinderManager{
		entries: make(map[uint32]*TeamFinder),
	}
}

// GetOrCreateFinder gets or creates a team finder entry for a player.
func (tm *TeamFinderManager) GetOrCreateFinder(leaderGUID uint32) *TeamFinder {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if e, ok := tm.entries[leaderGUID]; ok {
		return e
	}
	e := &TeamFinder{
		LeaderGUID: leaderGUID,
		MembersMap: make(map[uint32]uint8),
	}
	tm.entries[leaderGUID] = e
	return e
}

// GetFinder returns the team finder entry for a player.
func (tm *TeamFinderManager) GetFinder(leaderGUID uint32) *TeamFinder {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.entries[leaderGUID]
}

// RemoveFinder removes a team finder entry.
func (tm *TeamFinderManager) RemoveFinder(leaderGUID uint32) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.entries, leaderGUID)
}

// GetAllEntries returns all team finder entries.
func (tm *TeamFinderManager) GetAllEntries() map[uint32]*TeamFinder {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	result := make(map[uint32]*TeamFinder, len(tm.entries))
	for k, v := range tm.entries {
		result[k] = v
	}
	return result
}
