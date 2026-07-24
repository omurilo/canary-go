package game

import "testing"

func TestBestiaryKillCount(t *testing.T) {
	p := &Player{}
	if got := p.GetBestiaryKillCount(900); got != 0 {
		t.Fatalf("initial kill count = %d, want 0", got)
	}
	p.AddBestiaryKillCount(900, 1)
	p.AddBestiaryKillCount(900, 4)
	if got := p.GetBestiaryKillCount(900); got != 5 {
		t.Fatalf("after +1+4 = %d, want 5", got)
	}
	// distinct races are independent, and stored under the 61305000+raceid key
	if got := p.GetBestiaryKillCount(901); got != 0 {
		t.Fatalf("other race = %d, want 0", got)
	}
	if got := p.GetStorageValue(storageBestiaryKillCount + 900); got != 5 {
		t.Fatalf("stored under storage key = %d, want 5 (persists via player_storage)", got)
	}
}

func TestEnsureBoostedBoss(t *testing.T) {
	w := NewWorld()
	w.TypeRegistry = creaturesRegistryWithArchfoe()
	if w.BoostedBoss != "" {
		t.Fatal("expected empty boosted boss initially")
	}
	w.EnsureBoostedBoss()
	if w.BoostedBoss == "" || w.BoostedBoss == "default" {
		t.Fatalf("EnsureBoostedBoss did not pick a boss, got %q", w.BoostedBoss)
	}
	// idempotent: second call keeps the same pick
	first := w.BoostedBoss
	w.EnsureBoostedBoss()
	if w.BoostedBoss != first {
		t.Fatalf("boosted boss changed on second call: %q -> %q", first, w.BoostedBoss)
	}
}
