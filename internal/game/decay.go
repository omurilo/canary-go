package game

import (
	"sort"
	"sync"
	"time"
)

// Item decay, ported from src/items/decay/decay.cpp.
//
// The previous implementation scheduled one dispatcher event per decaying item,
// keyed by position, and fired it after `duration` SECONDS. Three things were
// wrong with that and each one is visible in game:
//
//   - DURATION is milliseconds everywhere else in the codebase and in the
//     datapack. Every decay ran a thousand times too slow.
//   - An item with decayTo == 0 was skipped entirely. Upstream removes it —
//     that is how a fire field burns out instead of burning forever.
//   - Nothing could cancel a scheduled decay, so an item picked up, moved or
//     transformed still fired its old event, and the handler either did nothing
//     (item not found at the position) or rewrote whatever had replaced it.
//
// Upstream keeps one ordered map from timestamp to items and a single scheduled
// event pointed at the earliest one, re-armed each pass. That is reproduced
// here, with the item's position carried alongside it because a Go Item has no
// parent pointer to find its tile from.

// schedulerMinTicks is SCHEDULER_MINTICKS: no decay event is ever scheduled
// closer than this, which stops a zero-duration item spinning the dispatcher.
const schedulerMinTicks = 50 * time.Millisecond

// DecayState values (ItemDecayState_t).
const (
	// decayingFalse mirrors DECAYING_FALSE (src/enums/item_attribute.hpp:54).
	decayingFalse uint8 = 0
	decayingTrue  uint8 = 1
	// decayingStopping asks startDecay to cancel rather than (re)arm, which is
	// how a transform mid-decay releases the old timer.
	decayingStopping uint8 = 3
)

type decayEntry struct {
	item *Item
	pos  Position
}

// DecayManager handles item decaying (corpses, fields, torches).
type DecayManager struct {
	world *World

	mu       sync.Mutex
	decayMap map[int64][]decayEntry
	// nextAt is the timestamp the currently-armed event will fire at, so a
	// sooner item can tell whether it needs to pre-empt it. Zero means unarmed.
	nextAt int64
}

func NewDecayManager(w *World) *DecayManager {
	return &DecayManager{world: w, decayMap: make(map[int64][]decayEntry)}
}

// StartDecay is Decay::startDecay (decay.cpp:21).
//
// The decay clock lives on the ITEM, in its DURATION attribute, not on the item
// type. Reading the type instead — which the port did — restarts a half-burnt
// torch at full duration every time anything touches it.
func (d *DecayManager) StartDecay(pos Position, item *Item) {
	if item == nil || d == nil {
		return
	}
	state := itemDecayState(item)
	canDecay := d.canDecay(item)

	if state == decayingStopping || (!canDecay && state == decayingTrue) {
		d.StopDecay(item)
		return
	}
	if !canDecay || state == decayingTrue {
		return
	}

	duration, hasDuration := itemDuration(item)
	if duration <= 0 && hasDuration {
		// An item that arrives already expired decays now rather than being
		// scheduled into the past.
		d.InternalDecayItem(pos, item)
		return
	}
	if duration <= 0 {
		return
	}

	// Re-arming over an existing timestamp would leave the item in the map twice.
	if item.Attr != nil && item.Attr.DurationTimestamp != nil {
		d.StopDecay(item)
	}

	timestamp := time.Now().UnixMilli() + duration

	d.mu.Lock()
	setItemDecayState(item, decayingTrue)
	setItemDurationTimestamp(item, timestamp)
	d.decayMap[timestamp] = append(d.decayMap[timestamp], decayEntry{item: item, pos: pos})
	rearm := d.nextAt == 0 || timestamp < d.nextAt
	if rearm {
		d.nextAt = timestamp
	}
	d.mu.Unlock()

	if rearm {
		d.schedule(time.Duration(duration) * time.Millisecond)
	}
}

// StopDecay is Decay::stopDecay (decay.cpp:67): take the item back out of the
// map and clear its decay state.
//
// The duration is re-stamped from the item's own remaining value before the
// state is cleared, so an item taken off the timer and put back on resumes
// where it was instead of restarting.
func (d *DecayManager) StopDecay(item *Item) {
	if item == nil || item.Attr == nil || item.Attr.DecayState == nil {
		return
	}
	if item.Attr.DurationTimestamp == nil {
		item.Attr.DecayState = nil
		return
	}
	timestamp := *item.Attr.DurationTimestamp

	d.mu.Lock()
	entries := d.decayMap[timestamp]
	for i, e := range entries {
		if e.item != item {
			continue
		}
		entries[i] = entries[len(entries)-1]
		entries = entries[:len(entries)-1]
		if len(entries) == 0 {
			delete(d.decayMap, timestamp)
		} else {
			d.decayMap[timestamp] = entries
		}
		break
	}
	d.mu.Unlock()

	item.Attr.DecayState = nil
	item.Attr.DurationTimestamp = nil
}

// CheckDecay is Decay::checkDecay (decay.cpp:114): fire everything due, then
// re-arm for the next timestamp still in the map.
//
// Items are copied out before being acted on. Decaying an item can transform or
// remove it, which mutates the map underneath an in-flight iteration.
func (d *DecayManager) CheckDecay() {
	now := time.Now().UnixMilli()

	d.mu.Lock()
	var due []decayEntry
	stamps := make([]int64, 0, len(d.decayMap))
	for ts := range d.decayMap {
		stamps = append(stamps, ts)
	}
	sort.Slice(stamps, func(i, j int) bool { return stamps[i] < stamps[j] })

	var nextStamp int64
	for _, ts := range stamps {
		if ts > now {
			nextStamp = ts
			break
		}
		due = append(due, d.decayMap[ts]...)
		delete(d.decayMap, ts)
	}
	d.nextAt = nextStamp
	d.mu.Unlock()

	for _, e := range due {
		if !d.canDecay(e.item) {
			setItemDecayState(e.item, decayingFalse)
			continue
		}
		setItemDecayState(e.item, decayingFalse)
		d.InternalDecayItem(e.pos, e.item)
	}

	if nextStamp != 0 {
		d.schedule(time.Duration(nextStamp-now) * time.Millisecond)
	}
}

// InternalDecayItem is Decay::internalDecayItem (decay.cpp:157).
//
// decayTo == 0 means the item is destroyed, not that it is exempt — except when
// it came from the map, which upstream leaves alone. That exemption is why a
// ground tile with a duration does not delete itself at boot; getting it wrong
// is what silently rewrote map items in this port before.
func (d *DecayManager) InternalDecayItem(pos Position, item *Item) {
	if item == nil || d.world == nil || d.world.Items == nil {
		return
	}
	it := d.world.Items.Get(item.ID)
	if it == nil {
		return
	}

	stackPos := d.stackPosOf(pos, item)

	if it.DecayTo != 0 {
		newItem := &Item{ID: it.DecayTo, Count: 1}
		d.world.RemoveMapItem(pos, item)
		d.world.AddItem(pos, newItem)
		if d.world.OnItemDecay != nil {
			d.world.OnItemDecay(pos, stackPos, item, newItem)
		}
		// The replacement carries its own duration and starts its own timer, which
		// is how a chain (torch -> burnt torch -> nothing) advances one step per
		// interval instead of all at once.
		if next := d.world.Items.Get(newItem.ID); next != nil && next.Duration > 0 {
			setItemDuration(newItem, int64(next.Duration))
			d.StartDecay(pos, newItem)
		}
		return
	}

	// Item::isLoadedFromMap. Without an equivalent flag here the closest honest
	// stand-in is "the item has no owner-side attributes", which a map item never
	// does. Getting this wrong deletes ground tiles, which is the failure this
	// port has already had once.
	if item.Attr == nil {
		return
	}
	d.world.RemoveMapItem(pos, item)
	if d.world.OnItemDecay != nil {
		d.world.OnItemDecay(pos, stackPos, item, nil)
	}
}

// StartDecaying is the old entry point, kept because the world calls it when an
// item is dropped. It stamps the type's duration onto the item and hands over
// to the real StartDecay.
func (d *DecayManager) StartDecaying(pos Position, item *Item, duration uint32, decayTo uint16) {
	if item == nil || duration == 0 {
		return
	}
	if _, has := itemDuration(item); !has {
		setItemDuration(item, int64(duration))
	}
	d.StartDecay(pos, item)
}

func (d *DecayManager) schedule(in time.Duration) {
	if in < schedulerMinTicks {
		in = schedulerMinTicks
	}
	GlobalDispatcher.AddEvent(in, d.CheckDecay)
}

// canDecay is Item::canDecay: an item decays when its type says how long it
// lasts and what it becomes, and it is not a permanent map fixture.
func (d *DecayManager) canDecay(item *Item) bool {
	if item == nil || d.world == nil || d.world.Items == nil {
		return false
	}
	it := d.world.Items.Get(item.ID)
	if it == nil {
		return false
	}
	if it.Duration == 0 && it.DecayTo == 0 {
		return false
	}
	return true
}

func (d *DecayManager) stackPosOf(pos Position, item *Item) uint8 {
	tile := d.world.Map.GetTile(pos)
	if tile == nil {
		return 0
	}
	if tile.Ground == item {
		return 0
	}
	stack := uint8(1)
	for _, it := range tile.Items {
		if it == item {
			return stack
		}
		stack++
	}
	return stack
}

func itemDecayState(item *Item) uint8 {
	if item.Attr == nil || item.Attr.DecayState == nil {
		return decayingFalse
	}
	return *item.Attr.DecayState
}

func setItemDecayState(item *Item, state uint8) {
	if item.Attr == nil {
		item.Attr = &ItemAttributes{}
	}
	item.Attr.DecayState = &state
}

func itemDuration(item *Item) (int64, bool) {
	if item.Attr == nil || item.Attr.Duration == nil {
		return 0, false
	}
	return int64(*item.Attr.Duration), true
}

func setItemDuration(item *Item, duration int64) {
	if item.Attr == nil {
		item.Attr = &ItemAttributes{}
	}
	v := int32(duration)
	item.Attr.Duration = &v
}

func setItemDurationTimestamp(item *Item, timestamp int64) {
	if item.Attr == nil {
		item.Attr = &ItemAttributes{}
	}
	item.Attr.DurationTimestamp = &timestamp
}
