package game

import "testing"

func TestRemoveMoneyBankFallback(t *testing.T) {
	p := &Player{Capacity: 1000000, BankBalance: 1000}
	p.Inventory[ConstSlotBackpack] = &Item{ID: 1988}
	p.AddMoney(150) // 150 gold coins into the backpack

	if got := p.GetMoney(); got != 150 {
		t.Fatalf("setup GetMoney = %d, want 150", got)
	}

	// Pure-inventory removal: 100 of 150, no bank touched, 50 change remains.
	if !p.RemoveMoney(100, true) {
		t.Fatal("RemoveMoney(100) failed")
	}
	if got := p.GetMoney(); got != 50 {
		t.Errorf("after 100: inv money = %d, want 50", got)
	}
	if p.BankBalance != 1000 {
		t.Errorf("after 100: bank = %d, want 1000 (untouched)", p.BankBalance)
	}

	// Inventory (50) + bank fallback (150): inventory drained, bank debited 150.
	if !p.RemoveMoney(200, true) {
		t.Fatal("RemoveMoney(200) with bank failed")
	}
	if got := p.GetMoney(); got != 0 {
		t.Errorf("after 200: inv money = %d, want 0", got)
	}
	if p.BankBalance != 850 {
		t.Errorf("after 200: bank = %d, want 850", p.BankBalance)
	}

	// More than inventory+bank: must fail and mutate nothing.
	before := p.BankBalance
	if p.RemoveMoney(5000, true) {
		t.Error("RemoveMoney(5000) should fail")
	}
	if p.BankBalance != before {
		t.Errorf("failed removal changed bank: %d != %d", p.BankBalance, before)
	}

	// Without bank, inventory alone can't cover it.
	if p.RemoveMoney(10, false) {
		t.Error("RemoveMoney(10, useBank=false) should fail with empty inventory")
	}
}
