package moveevents

import (
	lua "github.com/yuin/gopher-lua"
)

// MoveEvent represents a Lua move event script.
type MoveEvent struct {
	Type      string // e.g. "stepin", "stepout"
	ItemIDs   []uint16
	ActionIDs []uint16
	UniqueIDs []uint16
	OnStepIn  *lua.LFunction
	OnStepOut *lua.LFunction
}

var (
	stepInByItemID   = make(map[uint16]*MoveEvent)
	stepOutByItemID  = make(map[uint16]*MoveEvent)
)

// Register stores the move event in the maps.
func Register(m *MoveEvent) {
	for _, id := range m.ItemIDs {
		if m.Type == "stepin" || m.OnStepIn != nil {
			stepInByItemID[id] = m
		}
		if m.Type == "stepout" || m.OnStepOut != nil {
			stepOutByItemID[id] = m
		}
	}
	// Note: We might want to register by ActionIDs and UniqueIDs similarly.
}

// FindStepInByItemID looks up a step-in event by item ID.
func FindStepInByItemID(id uint16) *MoveEvent {
	return stepInByItemID[id]
}

// FindStepOutByItemID looks up a step-out event by item ID.
func FindStepOutByItemID(id uint16) *MoveEvent {
	return stepOutByItemID[id]
}
