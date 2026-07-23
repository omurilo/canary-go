package moveevents

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// MoveEvent represents a Lua move event script.
type MoveEvent struct {
	Type      string // e.g. "stepin", "stepout"
	ItemIDs   []uint16
	ActionIDs []uint16
	UniqueIDs []uint16
	Positions []game.Position
	OnStepIn  *lua.LFunction
	OnStepOut *lua.LFunction
}

var (
	stepInByItemID  = make(map[uint16]*MoveEvent)
	stepOutByItemID = make(map[uint16]*MoveEvent)
	// Movements are also keyed by the tile item's unique id and action id.
	// Many map-placed movements (e.g. the citizen/temple "set town" tiles in
	// data-otservbr-global/scripts/movements/teleport/citizen.lua) register only
	// by unique id, so indexing by item id alone would never fire them.
	stepInByUniqueID   = make(map[uint16]*MoveEvent)
	stepOutByUniqueID  = make(map[uint16]*MoveEvent)
	stepInByActionID   = make(map[uint16]*MoveEvent)
	stepOutByActionID  = make(map[uint16]*MoveEvent)
	stepInByPosition  = make(map[game.Position]*MoveEvent)
	stepOutByPosition = make(map[game.Position]*MoveEvent)
)

// Register stores the move event indexed by item id, unique id, action id, and position.
func Register(m *MoveEvent) {
	isStepIn := m.Type == "stepin" || m.OnStepIn != nil
	isStepOut := m.Type == "stepout" || m.OnStepOut != nil
	index := func(in, out map[uint16]*MoveEvent, ids []uint16) {
		for _, id := range ids {
			if id == 0 {
				continue
			}
			if isStepIn {
				in[id] = m
			}
			if isStepOut {
				out[id] = m
			}
		}
	}
	index(stepInByItemID, stepOutByItemID, m.ItemIDs)
	index(stepInByUniqueID, stepOutByUniqueID, m.UniqueIDs)
	index(stepInByActionID, stepOutByActionID, m.ActionIDs)

	for _, pos := range m.Positions {
		if isStepIn {
			stepInByPosition[pos] = m
		}
		if isStepOut {
			stepOutByPosition[pos] = m
		}
	}
}

// FindStepInByItemID looks up a step-in event by item ID.
func FindStepInByItemID(id uint16) *MoveEvent { return stepInByItemID[id] }

// FindStepOutByItemID looks up a step-out event by item ID.
func FindStepOutByItemID(id uint16) *MoveEvent { return stepOutByItemID[id] }

// FindStepInByUniqueID looks up a step-in event by the tile item's unique id.
func FindStepInByUniqueID(uid uint16) *MoveEvent { return stepInByUniqueID[uid] }

// FindStepOutByUniqueID looks up a step-out event by the tile item's unique id.
func FindStepOutByUniqueID(uid uint16) *MoveEvent { return stepOutByUniqueID[uid] }

// FindStepInByActionID looks up a step-in event by the tile item's action id.
func FindStepInByActionID(aid uint16) *MoveEvent { return stepInByActionID[aid] }

// FindStepOutByActionID looks up a step-out event by the tile item's action id.
func FindStepOutByActionID(aid uint16) *MoveEvent { return stepOutByActionID[aid] }

// FindStepInByPosition looks up a step-in event by tile position.
func FindStepInByPosition(pos game.Position) *MoveEvent { return stepInByPosition[pos] }

// FindStepOutByPosition looks up a step-out event by tile position.
func FindStepOutByPosition(pos game.Position) *MoveEvent { return stepOutByPosition[pos] }

