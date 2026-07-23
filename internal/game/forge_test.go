package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/items"
)

// forgeCatalog defines the forge item ids, money, a backpack, and two class-4
// upgradeable weapons used by the fusion/transfer tests.
func forgeCatalog() *items.Catalog {
	return items.NewCatalog(
		&items.ItemType{ID: 3031, Name: "gold coin", Stackable: true, StackSize: 100},
		&items.ItemType{ID: 3035, Name: "platinum coin", Stackable: true, StackSize: 100},
		&items.ItemType{ID: 3043, Name: "crystal coin", Stackable: true, StackSize: 100},
		&items.ItemType{ID: 1988, Name: "backpack", Group: items.GroupContainer, Capacity: 20},
		&items.ItemType{ID: ItemForgeSliver, Name: "forge sliver", Stackable: true, StackSize: 10000},
		&items.ItemType{ID: ItemForgeCore, Name: "exalted core", Stackable: true, StackSize: 10000},
		&items.ItemType{ID: ItemExaltationChest, Name: "exaltation chest", Group: items.GroupContainer, Capacity: 20},
		&items.ItemType{ID: 100, Name: "test blade", UpgradeClassification: 4, SlotPosition: "two-handed"},
		&items.ItemType{ID: 101, Name: "test axe", UpgradeClassification: 4, SlotPosition: "two-handed"},
	)
}

func forgePlayer() *Player {
	p := &Player{Capacity: 1_000_000_000, BankBalance: 100_000_000_000}
	p.Inventory[ConstSlotBackpack] = &Item{ID: 1988}
	return p
}

func addToBackpack(p *Player, it *Item) {
	bp := p.Inventory[ConstSlotBackpack]
	it.Parent = bp
	bp.Contents = append(bp.Contents, it)
}

func withTier(id uint16, tier uint8) *Item {
	it := &Item{ID: id, Count: 1}
	if tier > 0 {
		it.SetTier(tier)
	}
	return it
}

func TestForgeDustHelpers(t *testing.T) {
	p := &Player{ForgeDusts: 50, ForgeDustLevel: 100}

	if p.GetForgeDusts() != 50 {
		t.Fatalf("dusts = %d, want 50", p.GetForgeDusts())
	}
	p.AddForgeDusts(100) // clamps to level 100
	if p.GetForgeDusts() != 100 {
		t.Fatalf("dusts after add = %d, want clamp at 100", p.GetForgeDusts())
	}
	if !p.RemoveForgeDusts(30) || p.GetForgeDusts() != 70 {
		t.Fatalf("dusts after remove = %d, want 70", p.GetForgeDusts())
	}
	if p.RemoveForgeDusts(1000) {
		t.Fatalf("removing more dust than held should fail")
	}
}

func TestForgeBonusTable(t *testing.T) {
	cases := []struct {
		roll int
		want uint8
	}{
		{0, 0}, {7399, 0}, {7400, 1}, {8999, 1}, {9000, 2}, {9499, 2},
		{9500, 3}, {9524, 3}, {9525, 4}, {9549, 4}, {9550, 5}, {9949, 5},
		{9950, 6}, {9974, 6}, {9975, 7}, {10000, 7},
	}
	for _, c := range cases {
		if got := forgeBonus(c.roll); got != c.want {
			t.Errorf("forgeBonus(%d) = %d, want %d", c.roll, got, c.want)
		}
	}
}

func TestForgeResourceConversionDustToSlivers(t *testing.T) {
	cat := forgeCatalog()
	p := forgePlayer()
	p.ForgeDusts = 100

	if !p.ForgeResourceConversion(cat, ForgeActionDustToSliver) {
		t.Fatalf("dust->sliver should succeed")
	}
	// cost = ForgeCostOneSliver * ForgeSliverAmount = 20*3 = 60 dust -> 3 slivers.
	if p.GetForgeDusts() != 40 {
		t.Fatalf("dusts = %d, want 40", p.GetForgeDusts())
	}
	if got := p.GetForgeSlivers(cat); got != ForgeSliverAmount {
		t.Fatalf("slivers = %d, want %d", got, ForgeSliverAmount)
	}
}

func TestForgeResourceConversionSliversToCore(t *testing.T) {
	cat := forgeCatalog()
	p := forgePlayer()
	addToBackpack(p, &Item{ID: ItemForgeSliver, Count: ForgeCoreCost})

	if !p.ForgeResourceConversion(cat, ForgeActionSliverToCore) {
		t.Fatalf("sliver->core should succeed")
	}
	if got := p.GetForgeSlivers(cat); got != 0 {
		t.Fatalf("slivers = %d, want 0", got)
	}
	if got := p.GetForgeCores(cat); got != 1 {
		t.Fatalf("cores = %d, want 1", got)
	}
}

func TestForgeResourceConversionIncreaseLimit(t *testing.T) {
	cat := forgeCatalog()
	p := forgePlayer()
	p.ForgeDusts = 100 // level default 100, cost = 100-75 = 25

	if !p.ForgeResourceConversion(cat, ForgeActionIncreaseLimit) {
		t.Fatalf("increase limit should succeed")
	}
	if p.GetForgeDustLevel() != 101 {
		t.Fatalf("dust level = %d, want 101", p.GetForgeDustLevel())
	}
	if p.GetForgeDusts() != 75 {
		t.Fatalf("dusts = %d, want 75", p.GetForgeDusts())
	}
}

func TestForgeFusionSuccess(t *testing.T) {
	cat := forgeCatalog()
	p := forgePlayer()
	p.ForgeDusts = 500
	addToBackpack(p, withTier(100, 0))
	addToBackpack(p, withTier(100, 0))
	addToBackpack(p, &Item{ID: ItemForgeCore, Count: 3})

	// Deterministic success, bonus 0 (all costs charged), 1 core used.
	res := p.applyFusion(cat, 100, 0, 100, false, false, true, 0, 1)
	if res.Err != "" {
		t.Fatalf("unexpected error: %s", res.Err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	// Dust: 500 - 100 = 400.
	if p.GetForgeDusts() != 400 {
		t.Fatalf("dusts = %d, want 400", p.GetForgeDusts())
	}
	// Cores: 3 - 1 = 2.
	if got := p.GetForgeCores(cat); got != 2 {
		t.Fatalf("cores = %d, want 2", got)
	}
	// Gold: class 4 tier 1 regular price = 8,000,000 from the bank.
	if p.BankBalance != 100_000_000_000-8_000_000 {
		t.Fatalf("bank = %d, want %d", p.BankBalance, 100_000_000_000-8_000_000)
	}
	// The two source blades are gone; an exaltation chest holds one tier-1 blade.
	chest := findFirst(p, ItemExaltationChest)
	if chest == nil {
		t.Fatalf("no exaltation chest produced")
	}
	if len(chest.Contents) != 1 {
		t.Fatalf("chest holds %d items, want 1 (second item consumed on bonus 0)", len(chest.Contents))
	}
	if chest.Contents[0].GetTier() != 1 {
		t.Fatalf("forged item tier = %d, want 1", chest.Contents[0].GetTier())
	}
	if countInstances(p, 100, 0) != 0 {
		t.Fatalf("tier-0 source blades should be consumed")
	}
}

func TestForgeFusionConvergence(t *testing.T) {
	cat := forgeCatalog()
	p := forgePlayer()
	p.ForgeDusts = 500
	addToBackpack(p, withTier(100, 2))
	addToBackpack(p, withTier(100, 2))

	// Convergence fusion: no bonus, no cores; dust 130 + convergence gold.
	res := p.applyFusion(cat, 100, 2, 100, false, true, true, 0, 0)
	if res.Err != "" {
		t.Fatalf("unexpected error: %s", res.Err)
	}
	if p.GetForgeDusts() != 370 { // 500 - 130
		t.Fatalf("dusts = %d, want 370", p.GetForgeDusts())
	}
	// class 4, tier 3 convergence fusion price = 170,000,000.
	if p.BankBalance != 100_000_000_000-170_000_000 {
		t.Fatalf("bank = %d, want %d", p.BankBalance, 100_000_000_000-170_000_000)
	}
	chest := findFirst(p, ItemExaltationChest)
	if chest == nil || len(chest.Contents) != 1 {
		t.Fatalf("convergence should leave one forged item in the chest")
	}
	if chest.Contents[0].GetTier() != 3 { // tier+1
		t.Fatalf("convergence forged tier = %d, want 3", chest.Contents[0].GetTier())
	}
}

func TestForgeTransfer(t *testing.T) {
	cat := forgeCatalog()
	p := forgePlayer()
	p.ForgeDusts = 500
	addToBackpack(p, withTier(100, 2)) // donor, tier 2
	addToBackpack(p, withTier(101, 0)) // receiver, tier 0
	addToBackpack(p, &Item{ID: ItemForgeCore, Count: 5})

	res := p.ForgeTransferItemTier(cat, 100, 2, 101, false)
	if res.Err != "" {
		t.Fatalf("unexpected error: %s", res.Err)
	}
	// dust 100.
	if p.GetForgeDusts() != 400 {
		t.Fatalf("dusts = %d, want 400", p.GetForgeDusts())
	}
	// toTier = tier-1 = 1; class 4 tier 1 core price = 1, regular price = 8,000,000.
	if got := p.GetForgeCores(cat); got != 4 {
		t.Fatalf("cores = %d, want 4", got)
	}
	if p.BankBalance != 100_000_000_000-8_000_000 {
		t.Fatalf("bank = %d, want %d", p.BankBalance, 100_000_000_000-8_000_000)
	}
	chest := findFirst(p, ItemExaltationChest)
	if chest == nil || len(chest.Contents) != 1 {
		t.Fatalf("transfer should leave the receiver in the chest")
	}
	got := chest.Contents[0]
	if got.ID != 101 || got.GetTier() != 1 { // receiver gains tier-1
		t.Fatalf("transfer result id=%d tier=%d, want id=101 tier=1", got.ID, got.GetTier())
	}
	if countInstances(p, 100, 2) != 0 {
		t.Fatalf("donor should be consumed")
	}
}

// findFirst returns the first item with id anywhere in the inventory tree.
func findFirst(p *Player, id uint16) *Item {
	var found *Item
	p.WalkInventory(func(it *Item) {
		if found == nil && it.ID == id {
			found = it
		}
	})
	return found
}

// countInstances counts item instances matching (id, tier) in the inventory.
func countInstances(p *Player, id uint16, tier uint8) int {
	n := 0
	p.WalkInventory(func(it *Item) {
		if it.ID == id && it.GetTier() == tier {
			n++
		}
	})
	return n
}
