package game

import "time"

// DecayManager handles item decaying (e.g. corpses, fields).
type DecayManager struct {
	world *World
}

func NewDecayManager(w *World) *DecayManager {
	return &DecayManager{world: w}
}

// StartDecaying registers an item to decay if it has a DecayTo attribute.
// Since Game doesn't directly import the items catalog, the caller provides the duration and decayTo ID.
func (d *DecayManager) StartDecaying(pos Position, item *Item, duration uint32, decayTo uint16) {
	if duration == 0 || decayTo == 0 {
		return
	}

	GlobalDispatcher.AddEvent(time.Duration(duration)*time.Second, func() {
		// Find the item at the given position
		tile := d.world.Map.GetTile(pos)
		if tile == nil {
			return
		}

		found := false
		var stackPos uint8 = 0

		if tile.Ground == item {
			found = true
		} else {
			stackPos = 1
			for _, it := range tile.Items {
				if it == item {
					found = true
					break
				}
				stackPos++
			}
		}

		if !found {
			return // Item was moved or destroyed
		}

		// Remove the old item
		d.world.Map.RemoveItemPtr(pos, item)

		// Spawn the new item
		newItem := &Item{ID: decayTo, Count: 1}
		d.world.AddItem(pos, newItem)

		// Broadcast removal and addition
		if d.world.OnItemDecay != nil {
			d.world.OnItemDecay(pos, stackPos, item, newItem)
		}
	})
}
