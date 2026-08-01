package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/bestiary"
)

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

func TestBestiaryKillCharmPoints(t *testing.T) {
	p := &Player{}
	th := bestiary.Thresholds{FirstUnlock: 100, SecondUnlock: 1000, ToKill: 2500}
	// below completion: no charm points, but stage crossings happen
	if !p.AddBestiaryKill(1, th, 50, 1) { // initial kill = stage crossing
		t.Fatal("initial kill should be a stage crossing")
	}
	if p.GetCharmPoints() != 0 {
		t.Fatalf("charm points before completion = %d, want 0", p.GetCharmPoints())
	}
	// jump straight to completion (2500) -> award 50 charm points once
	p.AddBestiaryKill(1, th, 50, 3000)
	if p.GetCharmPoints() != 50 {
		t.Fatalf("charm points after completion = %d, want 50", p.GetCharmPoints())
	}
	if !p.IsBestiaryComplete(1, th) {
		t.Fatal("should be complete")
	}
	// further kills award nothing more
	p.AddBestiaryKill(1, th, 100, 50)
	if p.GetCharmPoints() != 50 {
		t.Fatalf("charm points must not grow past completion, got %d", p.GetCharmPoints())
	}
}
