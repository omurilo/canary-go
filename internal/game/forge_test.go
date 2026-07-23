package game

import (
	"testing"
)

func TestForgeResources(t *testing.T) {
	player := &Player{
		ForgeDust:      50,
		ForgeDustLimit: 100,
		ForgeSlivers:   10,
		ForgeCores:     2,
	}

	if player.GetForgeDust() != 50 {
		t.Fatalf("expected dust 50, got %d", player.GetForgeDust())
	}

	player.AddForgeDust(100)
	if player.GetForgeDust() != 100 { // clamped at limit 100
		t.Fatalf("expected dust clamped at limit 100, got %d", player.GetForgeDust())
	}

	if !player.RemoveForgeDust(30) {
		t.Fatalf("expected dust remove true")
	}

	if player.GetForgeDust() != 70 {
		t.Fatalf("expected dust 70, got %d", player.GetForgeDust())
	}
}

func TestForgeResourceConversion(t *testing.T) {
	player := &Player{
		ForgeDust:      100,
		ForgeDustLimit: 100,
		ForgeSlivers:   0,
		ForgeCores:     0,
	}

	// Dust -> Sliver (20 dust -> 1 sliver)
	if !GlobalForge.ResourceConversion(player, ForgeActionDustToSliver) {
		t.Fatalf("expected dust to sliver conversion success")
	}

	if player.GetForgeDust() != 80 || player.GetForgeSlivers() != 1 {
		t.Fatalf("unexpected dust/sliver counts: dust %d, slivers %d", player.GetForgeDust(), player.GetForgeSlivers())
	}

	// Increase Limit (50 dust -> +3 limit)
	if !GlobalForge.ResourceConversion(player, ForgeActionIncreaseLimit) {
		t.Fatalf("expected dust limit increase success")
	}

	if player.GetForgeDustLimit() != 103 {
		t.Fatalf("expected dust limit 103, got %d", player.GetForgeDustLimit())
	}
}
