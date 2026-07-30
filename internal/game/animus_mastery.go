package game

import (
	"math"
	"sync"
)

// AnimusMastery tracks which monster types the player has unlocked animus mastery for.
// Provides an experience multiplier based on the number of unlocked masteries.
type AnimusMastery struct {
	mu       sync.RWMutex
	monsters map[uint16]bool  // raceID -> unlocked
	names    map[uint16]string // raceID -> monster name
}

// NewAnimusMastery creates a new AnimusMastery instance.
func NewAnimusMastery() *AnimusMastery {
	return &AnimusMastery{
		monsters: make(map[uint16]bool),
		names:    make(map[uint16]string),
	}
}

// Add unlocks animus mastery for the given monster race ID.
func (am *AnimusMastery) Add(raceID uint16, name string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.monsters[raceID] = true
	am.names[raceID] = name
}

// Remove removes animus mastery for the given monster race ID.
func (am *AnimusMastery) Remove(raceID uint16) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.monsters, raceID)
	delete(am.names, raceID)
}

// Has returns true if the player has animus mastery for the given race ID.
func (am *AnimusMastery) Has(raceID uint16) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.monsters[raceID]
}

// Count returns the number of unlocked animus masteries.
func (am *AnimusMastery) Count() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.monsters)
}

// GetExperienceMultiplier calculates the XP multiplier based on unlocked animus
// masteries. Formula (from C++):
//
//	base = 1 + (monsterXpMultiplier + (count / monstersToMultiply * monstersXpMultiplier)) / 100
//	result = min(maxMultiplier, base)
//
// Default config values:
//   - maxMultiplier: 4.0
//   - monsterXpMultiplier: 2.0  (% per unique monster)
//   - monstersXpMultiplier: 0.1 (% additional per batch)
//   - monstersToMultiply: 10    (batch size for additional multiplier)
func (am *AnimusMastery) GetExperienceMultiplier() float64 {
	am.mu.RLock()
	count := len(am.monsters)
	am.mu.RUnlock()

	if count == 0 {
		return 1.0
	}

	maxMultiplier := 4.0
	monsterXpMultiplier := 2.0
	monstersXpMultiplier := 0.1
	monstersToMultiply := 10.0

	base := 1.0 + (monsterXpMultiplier + float64(count)/monstersToMultiply*monstersXpMultiplier)/100.0
	return math.Min(maxMultiplier, base)
}
