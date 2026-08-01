package actions

import (
	"fmt"
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// Action represents a Lua action script.
type Action struct {
	ItemIDs   []uint16
	ActionIDs []uint16
	UniqueIDs []uint16
	Positions []game.Position
	OnUse     *lua.LFunction

	// The three range knobs from src/lua/creature/actions.hpp:129-131. Upstream
	// defaults CheckFloor and CheckLineOfSight to TRUE, so an Action built with a
	// bare `&Action{}` is not the upstream default — use New().
	AllowFarUse      bool
	CheckLineOfSight bool
	CheckFloor       bool
}

// New builds an Action with upstream's defaults. Zero-valuing the struct gives
// checkFloor and checkLineOfSight of false, which is the opposite of what C++
// starts from and would let every action shoot through walls and floors.
func New() *Action {
	return &Action{CheckLineOfSight: true, CheckFloor: true}
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
	byPosition = make(map[game.Position]*Action)
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
	for _, pos := range a.Positions {
		byPosition[pos] = a
	}
}

func Count() int {
	return len(byItemID) + len(byActionID) + len(byUniqueID) + len(byPosition)
}

// FindByItemID looks up an action by item ID.
func FindByItemID(id uint16) *Action {
	a := byItemID[id]
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

// FindByPosition looks up an action by map position.
func FindByPosition(pos game.Position) *Action {
	return byPosition[pos]
}

// FindAction looks up an action for an item, respecting UniqueID, ActionID, and Position.
func FindAction(item *game.Item, pos game.Position) *Action {
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
	if a := FindByPosition(pos); a != nil {
		return a
	}
	return FindByItemID(item.ID)
}

// Execute looks up and executes an action for an item.
func (e *ActionsEngine) ExecuteUse(player *game.Player, item *game.Item, fromPos game.Position, target interface{}, toPos game.Position, isHotkey bool) bool {
	action := FindAction(item, fromPos)
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
	
	// Create fromPos Position
	fUd := L.NewUserData()
	fUd.Value = fromPos
	L.SetMetatable(fUd, L.GetTypeMetatable("Position"))
	L.Push(fUd)
	
	// Target (Item or Creature)
	if tItem, ok := target.(*game.Item); ok {
		tUd := L.NewUserData()
		tUd.Value = tItem
		L.SetMetatable(tUd, L.GetTypeMetatable("Item"))
		L.Push(tUd)
	} else if tCreature, ok := target.(game.Creature); ok {
		tUd := L.NewUserData()
		tUd.Value = tCreature
		mtName := "Creature"
		switch tCreature.(type) {
		case *game.Player:
			mtName = "Player"
		case *game.Npc:
			mtName = "Npc"
		case *game.Monster:
			mtName = "Monster"
		}
		L.SetMetatable(tUd, L.GetTypeMetatable(mtName))
		L.Push(tUd)
	} else {
		L.Push(lua.LNil)
	}

	// Create toPos Position
	tUd := L.NewUserData()
	tUd.Value = toPos
	L.SetMetatable(tUd, L.GetTypeMetatable("Position"))
	L.Push(tUd)
	
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
