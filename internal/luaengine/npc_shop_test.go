package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
)

// TestNpcOpenShopWindowTable is the empty-shop bug: luaNpcOpenShopWindowTable
// (npc_functions.cpp:400) reads clientId/itemName, but the port read id/itemId/name,
// so every entry yielded ID 0 and the hireling's potions/equipment windows opened
// with nothing to sell.
func TestNpcOpenShopWindowTable(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	npc := game.NewNpc(e.world.GenerateCreatureID(), "Trader", &creatures.NpcType{Name: "Trader"})
	e.world.TypeRegistry.Npcs["trader"] = &creatures.NpcType{Name: "Trader"}
	e.pushNpcUserdata(npc)
	e.L.SetGlobal("npc", e.L.Get(-1))
	e.L.Pop(1)

	p := &game.Player{Name: "Buyer", DBID: 42}
	e.pushPlayerUserdata(p)
	e.L.SetGlobal("p", e.L.Get(-1))
	e.L.Pop(1)

	if err := e.L.DoString(`
		local potions = {
			{ itemName = "great health potion", clientId = 239, buy = 225 },
			{ itemName = "mana potion", clientId = 268, buy = 56, sell = 40 },
			{ itemName = "health potion", clientId = 266, buy = 50, subType = 50 },
		}
		local ok = npc:openShopWindowTable(p, potions)
		assert(ok == true, "openShopWindowTable should return true")
	`); err != nil {
		t.Fatalf("openShopWindowTable failed: %v", err)
	}

	items := npc.GetShopItemVector(p.DBID)
	if len(items) != 3 {
		t.Fatalf("expected 3 shop items, got %d", len(items))
	}
	if items[0].ID != 239 || items[0].Name != "great health potion" || items[0].BuyPrice != 225 {
		t.Fatalf("item 1 wrong: %+v", items[0])
	}
	if items[1].SellPrice != 40 {
		t.Fatalf("item 2 sell price wrong: %+v", items[1])
	}
	if items[2].SubType != 50 {
		t.Fatalf("item 3 subtype wrong: %+v", items[2])
	}

	// Switching category: luaNpcOpenShopWindowTable closes first, so a same-length
	// list is NOT treated as "already in shop". Without the close this returned
	// false and the hireling's category switch never happened.
	if err := e.L.DoString(`
		local equipment = {
			{ itemName = "chain armor", clientId = 3358, buy = 200 },
			{ itemName = "brass helmet", clientId = 3354, buy = 120 },
			{ itemName = "wooden shield", clientId = 3412, buy = 15 },
		}
		local ok = npc:openShopWindowTable(p, equipment)
		assert(ok == true, "same-length category switch must re-open the shop")
	`); err != nil {
		t.Fatalf("category switch failed: %v", err)
	}
	items = npc.GetShopItemVector(p.DBID)
	if len(items) != 3 || items[0].ID != 3358 {
		t.Fatalf("category switch did not replace the shop list: %+v", items)
	}
}
