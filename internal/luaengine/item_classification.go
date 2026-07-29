package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// registerItemClassificationType registers the ItemClassification type. The
// metatable itself is used as __index, keeping the Go methods accessible on
// instances regardless of Lua script patches to the global table.
//
// The Lua script register_item_tier.lua patches ItemClassification.register on
// the global table, but that only affects the global table — instance method
// lookup goes through the metatable's __index (which is the metatable itself),
// so the Go register function is always the one that runs for userdata.
func (e *Engine) registerItemClassificationType() {
	L := e.L

	// Global table for namespacing. Lua scripts can add helpers here without
	// affecting instance method lookup.
	classTable := L.NewTable()
	L.SetGlobal("ItemClassification", classTable)

	// The metatable for userdata instances. Its methods are the canonical
	// versions — protected from Lua script overwrites via __index = mt.
	mt := L.NewTypeMetatable("ItemClassification")
	L.SetFuncs(mt, map[string]lua.LGFunction{
		"addTier":  e.itemClassificationAddTier,
		"register": e.itemClassificationRegister,
	})
	L.SetField(mt, "__index", mt)
}

// gameCreateItemClassification creates a new ItemClassification userdata with
// the given ID. Called from Game.createItemClassification(id) in Lua.
func (e *Engine) gameCreateItemClassification(L *lua.LState) int {
	id := uint8(L.CheckInt(1))
	ic := game.NewItemClassification(id)
	ud := L.NewUserData()
	ud.Value = ic
	L.SetMetatable(ud, L.GetTypeMetatable("ItemClassification"))
	L.Push(ud)
	return 1
}

// itemClassificationAddTier adds a tier upgrade price to the classification.
// Lua: classification:addTier(tier, corePrice, regularPrice, fusionPrice, transferPrice)
func (e *Engine) itemClassificationAddTier(L *lua.LState) int {
	ud := L.CheckUserData(1)
	ic, ok := ud.Value.(*game.ItemClassification)
	if !ok {
		L.Push(lua.LNil)
		return 1
	}
	tier := uint8(L.CheckInt(2))
	corePrice := uint8(L.CheckInt(3))
	regularPrice := uint64(L.CheckNumber(4))
	fusionPrice := uint64(L.CheckNumber(5))
	transferPrice := uint64(L.CheckNumber(6))

	ic.AddTier(tier, corePrice, regularPrice, fusionPrice, transferPrice)
	L.Push(ud)
	return 1
}

// itemClassificationRegister processes a classification mask (with Upgrades)
// and saves the classification to the World. This is the canonical register
// method used by instances — protected from Lua script overwrites.
// Lua: classification:register(mask)
func (e *Engine) itemClassificationRegister(L *lua.LState) int {
	ud := L.CheckUserData(1)
	ic, ok := ud.Value.(*game.ItemClassification)
	if !ok || e.world == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Process the mask table: extract Upgrades entries and add tiers.
	if L.GetTop() >= 2 {
		if mask, ok := L.Get(2).(*lua.LTable); ok {
			if upgrades := L.GetField(mask, "Upgrades"); upgrades.Type() == lua.LTTable {
				upgradesTable := upgrades.(*lua.LTable)
				var lastErr error
				upgradesTable.ForEach(func(_ lua.LValue, value lua.LValue) {
					if lastErr != nil {
						return
					}
					entry, ok := value.(*lua.LTable)
					if !ok {
						return
					}
					tierID := lua.LVAsNumber(L.GetField(entry, "TierId"))
					core := lua.LVAsNumber(L.GetField(entry, "Core"))
					regular := lua.LVAsNumber(L.GetField(entry, "RegularPrice"))
					fusion := lua.LVAsNumber(L.GetField(entry, "ConvergenceFustionPrice"))
					transfer := lua.LVAsNumber(L.GetField(entry, "ConvergenceTransferPrice"))

					ic.AddTier(uint8(tierID), uint8(core), uint64(regular), uint64(fusion), uint64(transfer))
				})
			}
		}
	}

	// Save to world.
	e.world.ItemClassifications[ic.ID] = ic
	L.Push(ud)
	return 1
}
