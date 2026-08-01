package luaengine

import (
	"log/slog"
	"os"
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
)

// buyFlowNpc mirrors the shape every merchant script in the datapack uses: a shop
// table plus an onBuyItem that delegates to npc:sellItem.
const buyFlowNpc = `
local npcType = Game.createNpcType("Shopkeeper")
local npcConfig = {}
npcConfig.name = "Shopkeeper"
npcConfig.health = 100
npcConfig.maxHealth = 100

npcConfig.shop = {
	{ itemName = "rope", clientId = 3003, buy = 50, sell = 25 },
}

npcType.onBuyItem = function(npc, player, itemId, subType, amount, ignore, inBackpacks, totalCost)
	npc:sellItem(player, itemId, amount, subType, 0, ignore, inBackpacks)
end

npcType:register(npcConfig)
`

func buyFlowWorld(t *testing.T) (*game.World, *Engine, *game.Npc, *game.Player) {
	t.Helper()

	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	w.Items = items.NewCatalog(
		&items.ItemType{ID: 3003, Name: "rope"},
		&items.ItemType{ID: 3031, Name: "gold coin", Stackable: true, StackSize: 100},
		&items.ItemType{ID: 1988, Name: "bag", Group: items.GroupContainer, SlotPosition: "backpack", Capacity: 20},
	)

	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	loadNpcRegisterShim(t, e)
	if err := e.L.DoString(buyFlowNpc); err != nil {
		t.Fatalf("register shopkeeper: %v", err)
	}

	npc := game.NewNpc(1, "Shopkeeper", w.TypeRegistry.Npcs["shopkeeper"])
	w.AddCreature(npc)

	p := &game.Player{Name: "Buyer", Level: 10, Health: 100, MaxHealth: 100}
	// A backpack with gold in it: the money helpers read from the inventory tree.
	p.Inventory[game.ConstSlotBackpack] = &game.Item{ID: 1988, Contents: []*game.Item{
		{ID: 3031, Count: 500},
	}}
	return w, e, npc, p
}

// The whole point of the architecture change: the core no longer delivers. A buy
// only completes because onBuyItem fires and calls npc:sellItem.
func TestBuyFlowThroughOnBuyItem(t *testing.T) {
	w, e, npc, p := buyFlowWorld(t)
	defer e.Close()

	moneyBefore := p.GetMoney()
	if moneyBefore != 500 {
		t.Fatalf("fixture: expected 500 gold, got %d", moneyBefore)
	}

	if !e.CallNpcOnBuyItem(npc, p, 3003, 0, 2, false, false, 100) {
		t.Fatal("onBuyItem should have been dispatched")
	}

	// 2 ropes at 50 each = 100 gold.
	if got := p.GetMoney(); got != 400 {
		t.Errorf("money after buying: got %d want 400", got)
	}
	if got := p.GetItemTypeCount(w.Items, 3003, -1); got != 2 {
		t.Errorf("ropes delivered: got %d want 2", got)
	}
}

// An NPC with no onBuyItem must report false so the caller can tell that nothing
// happened — the core deliberately does not deliver on its own.
func TestBuyFlowWithoutCallbackDoesNothing(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	w.Items = items.NewCatalog(&items.ItemType{ID: 3003, Name: "rope"})

	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	loadNpcRegisterShim(t, e)
	defer e.Close()

	script := `
		local npcType = Game.createNpcType("Silent Trader")
		npcType:register({
			name = "Silent Trader", health = 100, maxHealth = 100,
			shop = { { itemName = "rope", clientId = 3003, buy = 50 } },
		})
	`
	if err := e.L.DoString(script); err != nil {
		t.Fatalf("register: %v", err)
	}

	npc := game.NewNpc(1, "Silent Trader", w.TypeRegistry.Npcs["silent trader"])
	p := &game.Player{Name: "Buyer"}

	if e.CallNpcOnBuyItem(npc, p, 3003, 0, 1, false, false, 50) {
		t.Error("an NPC without onBuyItem must report no dispatch")
	}
	if got := p.GetItemTypeCount(w.Items, 3003, -1); got != 0 {
		t.Errorf("nothing should have been delivered, got %d", got)
	}
}

// The charge is always derived from what was delivered, never from what was asked
// for, so a full delivery bills exactly amount * price.
func TestSellItemChargesForDeliveredAmount(t *testing.T) {
	w, e, npc, p := buyFlowWorld(t)
	defer e.Close()

	result, ok := npc.SellItemTo(p, w.Items, 3003, 4, 0, false)
	if !ok {
		t.Fatal("the sale should have succeeded")
	}
	if result.Delivered != 4 {
		t.Fatalf("expected 4 delivered, got %d", result.Delivered)
	}
	if result.Charged != 200 {
		t.Errorf("charged %d want 200", result.Charged)
	}
}

// With nothing deliverable the sale fails and, critically, charges nothing.
func TestSellItemChargesNothingWhenUndeliverable(t *testing.T) {
	w, e, npc, p := buyFlowWorld(t)
	defer e.Close()

	before := p.GetMoney()
	// An item the NPC does not stock cannot be priced, so the sale is refused.
	if _, ok := npc.SellItemTo(p, w.Items, 9999, 1, 0, false); ok {
		t.Fatal("an unstocked item must not be sellable")
	}
	if got := p.GetMoney(); got != before {
		t.Errorf("money changed on a refused sale: %d -> %d", before, got)
	}
}

// Buying "in backpacks" adds 20 gold per shopping bag on top of the goods.
func TestSellItemChargesForShoppingBags(t *testing.T) {
	w, e, npc, p := buyFlowWorld(t)
	defer e.Close()

	result, ok := npc.SellItemTo(p, w.Items, 3003, 2, 0, true)
	if !ok {
		t.Fatal("the sale should have succeeded")
	}
	if result.BagsCost == 0 {
		t.Error("buying in backpacks must charge for the bags")
	}
	if result.Charged != uint64(50)*uint64(result.Delivered)+result.BagsCost {
		t.Errorf("charged %d, goods %d + bags %d",
			result.Charged, uint64(50)*uint64(result.Delivered), result.BagsCost)
	}
}

// onSellItem is a notification: the caller has already moved the items and money,
// so the dispatcher only has to reach the script.
func TestSellNotificationDispatched(t *testing.T) {
	w := game.NewWorld()
	w.TypeRegistry = creatures.NewTypeRegistry()
	w.Items = items.NewCatalog(&items.ItemType{ID: 3003, Name: "rope"})

	e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	loadNpcRegisterShim(t, e)
	defer e.Close()

	script := `
		SOLD_LOG = {}
		local npcType = Game.createNpcType("Buyer Npc")
		npcType.onSellItem = function(npc, player, itemId, subType, amount, ignore, name, totalCost)
			SOLD_LOG[#SOLD_LOG + 1] = string.format("%d:%s:%d", amount, name, totalCost)
		end
		npcType:register({
			name = "Buyer Npc", health = 100, maxHealth = 100,
			shop = { { itemName = "rope", clientId = 3003, sell = 25 } },
		})
	`
	if err := e.L.DoString(script); err != nil {
		t.Fatalf("register: %v", err)
	}

	npc := game.NewNpc(1, "Buyer Npc", w.TypeRegistry.Npcs["buyer npc"])
	p := &game.Player{Name: "Seller"}

	if !e.CallNpcOnSellItem(npc, p, 3003, 0, 3, false, "rope", 75) {
		t.Fatal("onSellItem should have been dispatched")
	}

	if err := e.L.DoString(`assert(SOLD_LOG[1] == "3:rope:75", "got " .. tostring(SOLD_LOG[1]))`); err != nil {
		t.Errorf("callback received wrong arguments: %v", err)
	}
}
