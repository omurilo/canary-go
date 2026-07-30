package game

import "sync"

// Hireling represents a player-owned hireling NPC.
type Hireling struct {
	ID       uint32
	PlayerID uint32
	Name     string
	Active   bool
	Sex      uint8
	Pos      Position
	LookBody uint8
	LookFeet uint8
	LookHead uint8
	LookLegs uint8
	LookType uint16
}

// HirelingManager holds global hireling data (skills and outfits).
type HirelingManager struct {
	mu      sync.Mutex
	skills  map[uint16]string  // skillID -> skill name
	outfits map[uint16]string  // outfitID -> outfit name
}

// NewHirelingManager creates a new hireling manager with default data.
func NewHirelingManager() *HirelingManager {
	hm := &HirelingManager{
		skills: map[uint16]string{
			1001: "banker",
			1002: "cooking",
			1003: "steward",
			1004: "trader",
		},
		outfits: map[uint16]string{
			2001: "Banker",
			2002: "Cooking",
			2003: "Steward",
			2004: "Trader",
			2005: "Servant",
			2006: "Hydra",
			2007: "Ferumbras",
			2008: "Bonelord",
			2009: "Dragon",
		},
	}
	return hm
}

// Skills returns a copy of the skills map.
func (hm *HirelingManager) Skills() map[uint16]string {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	out := make(map[uint16]string, len(hm.skills))
	for k, v := range hm.skills {
		out[k] = v
	}
	return out
}

// Outfits returns a copy of the outfits map.
func (hm *HirelingManager) Outfits() map[uint16]string {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	out := make(map[uint16]string, len(hm.outfits))
	for k, v := range hm.outfits {
		out[k] = v
	}
	return out
}
