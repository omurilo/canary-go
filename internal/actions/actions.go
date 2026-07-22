package actions

import (
	"fmt"
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// Action represents a Lua action script.
type Action struct {
	ItemIDs     []uint16
	ActionIDs   []uint16
	UniqueIDs   []uint16
	OnUse       *lua.LFunction
	AllowFarUse bool
}

// Engine represents the global Actions Engine
type ActionsEngine struct{
	L *lua.LState
}

var Engine *ActionsEngine

func NewEngine(L *lua.LState) *ActionsEngine {
	Engine = &ActionsEngine{L: L}
	return Engine
}

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
		fmt.Printf("RegisterAction: id %d\n", id)
		byItemID[id] = a
	}
	for _, id := range a.ActionIDs {
		byActionID[id] = a
	}
	for _, id := range a.UniqueIDs {
		byUniqueID[id] = a
	}
}

func Count() int {
	return len(byItemID) + len(byActionID) + len(byUniqueID)
}

// FindByItemID looks up an action by item ID.
func FindByItemID(id uint16) *Action {
	a := byItemID[id]
	if a == nil {
		fmt.Printf("FindByItemID: %d -> nil\n", id)
	}
	return a
}

// FindByActionID looks up an action by action ID.
func FindByActionID(id uint16) *Action {
	return byActionID[id]
}

// FindByUniqueID looks up an action by unique ID.
func FindByUniqueID(id uint16) *Action {
	return byUniqueID[id]
}

// FindAction looks up an action for an item, respecting UniqueID and ActionID.
func FindAction(item *game.Item) *Action {
	if item.Attr != nil {
		if item.Attr.UniqueID != nil {
			if a := FindByUniqueID(*item.Attr.UniqueID); a != nil {
				return a
			}
		}
		if item.Attr.ActionID != nil {
			if a := FindByActionID(*item.Attr.ActionID); a != nil {
				return a
			}
		}
	}
	return FindByItemID(item.ID)
}

// Execute looks up and executes an action for an item.
func (e *ActionsEngine) ExecuteUse(player *game.Player, item *game.Item, fromPos game.Position, target interface{}, toPos game.Position, isHotkey bool) bool {
	action := FindAction(item)
	if action == nil || action.OnUse == nil {
		return false
	}

	L := e.L
	L.Push(action.OnUse)
	
	// Create Player userdata
	pUd := L.NewUserData()
	pUd.Value = player
	L.SetMetatable(pUd, L.GetTypeMetatable("Player"))
	L.Push(pUd)
	
	// Create Item userdata
	iUd := L.NewUserData()
	iUd.Value = item
	L.SetMetatable(iUd, L.GetTypeMetatable("Item"))
	L.Push(iUd)
	
	// Create fromPos table
	fPos := L.NewTable()
	L.SetField(fPos, "x", lua.LNumber(fromPos.X))
	L.SetField(fPos, "y", lua.LNumber(fromPos.Y))
	L.SetField(fPos, "z", lua.LNumber(fromPos.Z))
	L.Push(fPos)
	
	// Target (Item or Creature)
	if tItem, ok := target.(*game.Item); ok {
		tUd := L.NewUserData()
		tUd.Value = tItem
		L.SetMetatable(tUd, L.GetTypeMetatable("Item"))
		L.Push(tUd)
	} else if tCreature, ok := target.(game.Creature); ok {
		tUd := L.NewUserData()
		tUd.Value = tCreature
		L.SetMetatable(tUd, L.GetTypeMetatable("Creature"))
		L.Push(tUd)
	} else {
		L.Push(lua.LNil)
	}

	// Create toPos table
	tPos := L.NewTable()
	L.SetField(tPos, "x", lua.LNumber(toPos.X))
	L.SetField(tPos, "y", lua.LNumber(toPos.Y))
	L.SetField(tPos, "z", lua.LNumber(toPos.Z))
	L.Push(tPos)
	
	L.Push(lua.LBool(isHotkey))
	
	if err := L.PCall(6, 1, nil); err != nil {
		fmt.Printf("Lua action onUse error: %v\n", err)
		return false
	}
	
	ret := L.Get(-1)
	L.Pop(1)
	
	if b, ok := ret.(lua.LBool); ok {
		return bool(b)
	}
	return true // default success if returns nil
}
