package actions

import (
	lua "github.com/yuin/gopher-lua"
)

// Action represents a Lua action script.
type Action struct {
	ItemIDs   []uint16
	ActionIDs []uint16
	UniqueIDs []uint16
	OnUse     *lua.LFunction
}

// Engine represents the global Actions Engine mock
type ActionsEngine struct{}

var Engine *ActionsEngine

func (e *ActionsEngine) Register(a *Action) {
	Register(a)
}


var (
	byItemID   = make(map[uint16]*Action)
	byActionID = make(map[uint16]*Action)
	byUniqueID = make(map[uint16]*Action)
)

// Register stores the action in the maps.
func Register(a *Action) {
	for _, id := range a.ItemIDs {
		byItemID[id] = a
	}
	for _, id := range a.ActionIDs {
		byActionID[id] = a
	}
	for _, id := range a.UniqueIDs {
		byUniqueID[id] = a
	}
}

// FindByItemID looks up an action by item ID.
func FindByItemID(id uint16) *Action {
	return byItemID[id]
}

// FindByActionID looks up an action by action ID.
func FindByActionID(id uint16) *Action {
	return byActionID[id]
}

// FindByUniqueID looks up an action by unique ID.
func FindByUniqueID(id uint16) *Action {
	return byUniqueID[id]
}
