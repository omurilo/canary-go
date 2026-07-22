package game

import "sync"

// WheelOfDestiny models the character progression tree introduced in Tibia 13.10+.
type WheelOfDestiny struct {
	mu           sync.RWMutex
	BonusPoints  uint16            // Additional bonus points granted by quests/GM
	ActivePreset uint8             // Current active preset (0-2)
	SlotPoints   map[uint16]uint16 // Slot ID -> Allocated Points
}

// NewWheelOfDestiny initializes a new WheelOfDestiny instance.
func NewWheelOfDestiny() *WheelOfDestiny {
	return &WheelOfDestiny{
		SlotPoints: make(map[uint16]uint16),
	}
}

// GetTotalPoints returns total available Wheel Points for a given level and vocation state.
// Promoted vocations earn 1 point per level above level 50.
func (w *WheelOfDestiny) GetTotalPoints(level uint16, isPromoted bool) uint16 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var levelPoints uint16
	if isPromoted && level > 50 {
		levelPoints = level - 50
	}
	return levelPoints + w.BonusPoints
}

// GetSpentPoints calculates total points currently invested across all slots.
func (w *WheelOfDestiny) GetSpentPoints() uint16 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var spent uint16
	for _, pts := range w.SlotPoints {
		spent += pts
	}
	return spent
}

// SaveSlotPoints updates the slot point allocation map.
func (w *WheelOfDestiny) SaveSlotPoints(points map[uint16]uint16) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.SlotPoints = make(map[uint16]uint16)
	for slot, pts := range points {
		if pts > 0 {
			w.SlotPoints[slot] = pts
		}
	}
}

// GetSlotPointsCopy returns a thread-safe copy of the slot points map.
func (w *WheelOfDestiny) GetSlotPointsCopy() map[uint16]uint16 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	copyMap := make(map[uint16]uint16, len(w.SlotPoints))
	for k, v := range w.SlotPoints {
		copyMap[k] = v
	}
	return copyMap
}

// GetBonusHealth returns additional max health granted by invested points.
func (w *WheelOfDestiny) GetBonusHealth() uint32 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Every invested point grants 1 HP bonus
	return uint32(w.GetSpentPoints())
}

// GetBonusMana returns additional max mana granted by invested points.
func (w *WheelOfDestiny) GetBonusMana() uint32 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Every invested point grants 1 Mana bonus
	return uint32(w.GetSpentPoints())
}

// GetBonusCapacity returns additional capacity granted by invested points.
func (w *WheelOfDestiny) GetBonusCapacity() uint32 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Every invested point grants 1 oz (100 hundredths of an oz)
	return uint32(w.GetSpentPoints()) * 100
}
