package game

import (
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/omurilo/canary-go/internal/io/propstream"
)

// AnimusMastery tracks which monster types the player has unlocked animus mastery for.
// Provides an experience multiplier based on the number of unlocked masteries.
type AnimusMastery struct {
	mu       sync.RWMutex
	monsters map[uint16]bool   // raceID -> unlocked
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

	base := 1.0 + (monsterXpMultiplier+float64(count)/monstersToMultiply*monstersXpMultiplier)/100.0
	return math.Min(maxMultiplier, base)
}

// Names returns the unlocked monster names in lowercase, sorted so the serialized
// blob is stable across saves.
func (am *AnimusMastery) Names() []string {
	if am == nil {
		return nil
	}
	am.mu.RLock()
	defer am.mu.RUnlock()

	names := make([]string, 0, len(am.names))
	for raceID, unlocked := range am.monsters {
		if !unlocked {
			continue
		}
		if name, ok := am.names[raceID]; ok && name != "" {
			names = append(names, strings.ToLower(name))
		}
	}
	sort.Strings(names)
	return names
}

// Serialize writes the masteries in the C++ PropStream format, porting
// AnimusMastery::serialize (animus_mastery.cpp): a bare sequence of
// length-prefixed lowercase monster names, no count header.
func (am *AnimusMastery) Serialize() []byte {
	w := propstream.NewPropWriteStream()
	for _, name := range am.Names() {
		w.WriteString(name)
	}
	return w.GetStream()
}

// UnserializeAnimusMastery reads the blob written by Serialize, porting
// AnimusMastery::unserialize: read strings until the stream runs out.
//
// The stored form is names only, exactly as C++ writes it. Go keys its map by
// bestiary race id, so each name is resolved through the monster registry; a name
// with no known type is kept with race id 0 so the blob survives a round trip
// instead of silently losing entries.
func UnserializeAnimusMastery(blob []byte, lookup func(name string) (uint16, bool)) *AnimusMastery {
	am := NewAnimusMastery()
	if len(blob) == 0 {
		return am
	}

	s := propstream.NewPropStream(blob)
	for {
		name, err := s.ReadString()
		if err != nil {
			break
		}
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		var raceID uint16
		if lookup != nil {
			if id, ok := lookup(lower); ok {
				raceID = id
			}
		}
		am.Add(raceID, lower)
	}
	return am
}
