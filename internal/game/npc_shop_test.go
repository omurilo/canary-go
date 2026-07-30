package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/items"
)

func shopCatalog() *items.Catalog {
	return items.NewCatalog(
		// Stackable goods with the usual 100-per-stack limit.
		&items.ItemType{ID: 3031, Name: "gold coin", Stackable: true, StackSize: 100},
		&items.ItemType{ID: 2160, Name: "crystal coin", Stackable: true, StackSize: 100},
		&items.ItemType{ID: 3155, Name: "rope"},
		// A backpack: a container that fits the backpack slot.
		&items.ItemType{ID: 1988, Name: "bag", Group: items.GroupContainer, SlotPosition: "backpack"},
		// A container that does NOT fit the backpack slot.
		&items.ItemType{ID: 1987, Name: "chest", Group: items.GroupContainer},
	)
}

// npcTypeWithShop is a merchant type: one buy-only entry and one sell-only entry.
func npcTypeWithShop() *creatures.NpcType {
	return &creatures.NpcType{
		ShopItems: []creatures.ShopItem{
			{ID: 3031, Name: "gold coin", BuyPrice: 100},
			{ID: 3155, Name: "rope", SellPrice: 50},
		},
	}
}

// calculateSlotsNeeded drives both the bag cost and the tile limit, so its four
// branches are worth pinning (npc.cpp:45).
func TestCalculateSlotsNeeded(t *testing.T) {
	cat := shopCatalog()
	stackable := cat.Get(3031)
	plain := cat.Get(3155)

	cases := []struct {
		name        string
		it          *items.ItemType
		amount      uint16
		inBackpacks bool
		want        float64
	}{
		// 250 stackable / 100 per stack = 3 stacks.
		{"stackable loose", stackable, 250, false, 3},
		// 3 stacks / 20 per bag = 1 bag.
		{"stackable in bags", stackable, 250, true, 1},
		// 2100 / 100 = 21 stacks -> 21/20 = 2 bags.
		{"stackable spilling to a second bag", stackable, 2100, true, 2},
		// Non-stackable: one slot each.
		{"non-stackable loose", plain, 5, false, 5},
		// 25 items / 20 per bag = 2 bags.
		{"non-stackable in bags", plain, 25, true, 2},
	}
	for _, c := range cases {
		if got := calculateSlotsNeeded(c.it, c.amount, c.inBackpacks); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestCalculateBagsCost(t *testing.T) {
	cat := shopCatalog()
	stackable := cat.Get(3031)

	if got := CalculateBagsCost(stackable, 250, false); got != 0 {
		t.Errorf("no bags when not buying in backpacks: got %d", got)
	}
	// 1 bag * 20 gold.
	if got := CalculateBagsCost(stackable, 250, true); got != 20 {
		t.Errorf("one bag: got %d want 20", got)
	}
	// 2 bags * 20 gold.
	if got := CalculateBagsCost(stackable, 2100, true); got != 40 {
		t.Errorf("two bags: got %d want 40", got)
	}
}

// ignore (ignoreCapacity) short-circuits the slot check entirely.
func TestIsBackpackSlotUnavailableIgnored(t *testing.T) {
	cat := shopCatalog()
	p := &Player{}
	if p.IsBackpackSlotUnavailable(cat, 3155, true) {
		t.Error("with ignore set the slot check must not refuse")
	}
}

// With no free slot and no backpack equipped, buying a backpack-slot container is
// still allowed — that is the escape hatch in isBackpackSlotUnavailable.
func TestIsBackpackSlotUnavailableAllowsBuyingABag(t *testing.T) {
	cat := shopCatalog()
	p := &Player{} // no backpack equipped, so GetFreeBackpackSlots is 0

	if p.IsBackpackSlotUnavailable(cat, 1988, false) {
		t.Error("buying a backpack must be allowed with no free slots")
	}
	// A container that does not fit the backpack slot is refused.
	if !p.IsBackpackSlotUnavailable(cat, 1987, false) {
		t.Error("a non-backpack container must be refused")
	}
	// A plain item is refused.
	if !p.IsBackpackSlotUnavailable(cat, 3155, false) {
		t.Error("a plain item must be refused with no free slots")
	}
}

// An equipped backpack with no room refuses everything, including another bag.
func TestIsBackpackSlotUnavailableWithFullBackpack(t *testing.T) {
	cat := shopCatalog()
	p := &Player{}
	// ContainerCapacity never reports 0 for a container, so the bag is filled to
	// its declared size instead of relying on a zero-capacity type.
	full := &Item{ID: 1988, MaxSize: 2, Contents: []*Item{{ID: 3155}, {ID: 3155}}}
	p.Inventory[ConstSlotBackpack] = full

	if p.GetFreeBackpackSlots(cat) != 0 {
		t.Fatalf("fixture is not actually full: %d free slots", p.GetFreeBackpackSlots(cat))
	}
	if !p.IsBackpackSlotUnavailable(cat, 1988, false) {
		t.Error("a full equipped backpack must refuse even a new bag")
	}
}

// Gold checks purse+bank against goods+bags together.
func TestHasInsufficientFundsGold(t *testing.T) {
	cat := shopCatalog()
	p := &Player{BankBalance: 50}

	if !p.HasInsufficientFunds(cat, GoldCoinID, 100, 20) {
		t.Error("50 gold must not cover a cost of 120")
	}
	p.BankBalance = 120
	if p.HasInsufficientFunds(cat, GoldCoinID, 100, 20) {
		t.Error("120 gold must cover a cost of 120")
	}
}

// A custom currency checks the currency ITEMS for the goods and gold for the bags,
// which is why they cannot be summed.
func TestHasInsufficientFundsCustomCurrency(t *testing.T) {
	cat := shopCatalog()
	const token = 2160

	// Plenty of gold, but no tokens: the goods cannot be paid for.
	p := &Player{BankBalance: 10000}
	if !p.HasInsufficientFunds(cat, token, 5, 0) {
		t.Error("no tokens must fail the goods check even with gold in the bank")
	}

	// Tokens for the goods but no gold for the bags.
	p2 := &Player{}
	p2.Inventory[ConstSlotBackpack] = &Item{ID: 1988, Contents: []*Item{
		{ID: token, Count: 10},
	}}
	if !p2.HasInsufficientFunds(cat, token, 5, 20) {
		t.Error("bags are paid in gold, so no gold must fail")
	}
	// Same tokens, no bags to pay for: fine.
	if p2.HasInsufficientFunds(cat, token, 5, 0) {
		t.Error("10 tokens must cover a cost of 5")
	}
}

func TestShopBuyAndSellPriceLookup(t *testing.T) {
	npc := NewNpc(1, "Merchant", nil)
	npc.Type = npcTypeWithShop()

	if _, ok := npc.ShopBuyPrice(9999, 0); ok {
		t.Error("an unlisted item must not be buyable")
	}
	// sell-only entry: no buy price
	if _, ok := npc.ShopBuyPrice(3155, 0); ok {
		t.Error("an entry with buy price 0 must not be buyable")
	}
	if price, ok := npc.ShopBuyPrice(3031, 0); !ok || price != 100 {
		t.Errorf("buy price: got %d ok=%v want 100", price, ok)
	}
	if price, ok := npc.ShopSellPrice(3155, 0); !ok || price != 50 {
		t.Errorf("sell price: got %d ok=%v want 50", price, ok)
	}
}

// placeItem used to fall back to ANY free equipment slot when the backpack was
// full, so a rope ended up worn on the head, the necklace slot and the legs. Worse,
// InternalAddItem then reported those bogus placements as delivered, and the shop
// charged for them.
func TestInternalAddItemDoesNotEquipUnfittingItems(t *testing.T) {
	cat := items.NewCatalog(
		&items.ItemType{ID: 3003, Name: "rope"}, // no slot: containers only
		&items.ItemType{ID: 1988, Name: "bag", Group: items.GroupContainer, SlotPosition: "backpack"},
	)
	p := &Player{}
	// A backpack with capacity 2 that already holds one item: room for exactly one
	// more, and no equipment slot may take the overflow.
	bp := &Item{ID: 1988, MaxSize: 2, Contents: []*Item{{ID: 3003}}}
	p.Inventory[ConstSlotBackpack] = bp

	placed, ok := p.InternalAddItem(cat, 3003, 5, -1, ConstSlotWhereever)

	if ok {
		t.Error("adding 5 items into room for 1 must not report full success")
	}
	if len(placed) != 1 {
		t.Errorf("expected exactly 1 placement to be reported, got %d", len(placed))
	}
	if len(bp.Contents) != 2 {
		t.Errorf("backpack should hold 2 items, holds %d", len(bp.Contents))
	}
	for s := ConstSlotFirst; s <= ConstSlotLast; s++ {
		if s == ConstSlotBackpack {
			continue
		}
		if p.Inventory[s] != nil {
			t.Errorf("slot %d must stay empty, holds item %d", s, p.Inventory[s].ID)
		}
	}
	// The reported count must match reality, or the shop overcharges.
	if got := p.GetItemTypeCount(cat, 3003, -1); got != 2 {
		t.Errorf("player should hold 2 ropes (1 pre-existing + 1 added), holds %d", got)
	}
}

// FitsSlot mirrors the per-slot rules of Player::queryAdd.
func TestFitsSlot(t *testing.T) {
	cat := items.NewCatalog(
		&items.ItemType{ID: 1, SlotPosition: "head"},
		&items.ItemType{ID: 2, SlotPosition: "two-handed"},
		&items.ItemType{ID: 3, SlotPosition: "right-hand"},
		&items.ItemType{ID: 4}, // not equippable
	)

	if !FitsSlot(cat.Get(1), ConstSlotHead) {
		t.Error("a helmet must fit the head slot")
	}
	if FitsSlot(cat.Get(1), ConstSlotLegs) {
		t.Error("a helmet must not fit the legs slot")
	}
	// Two-handed goes in either hand; the equip path enforces the other hand.
	if !FitsSlot(cat.Get(2), ConstSlotLeft) || !FitsSlot(cat.Get(2), ConstSlotRight) {
		t.Error("a two-handed weapon must fit either hand")
	}
	if FitsSlot(cat.Get(3), ConstSlotLeft) {
		t.Error("a right-hand item must not fit the left hand")
	}
	for s := ConstSlotFirst; s <= ConstSlotLast; s++ {
		if FitsSlot(cat.Get(4), s) {
			t.Errorf("an item with no slot must fit nowhere, but fits %d", s)
		}
	}
	if FitsSlot(nil, ConstSlotHead) {
		t.Error("a nil item type must not fit anywhere")
	}
}
