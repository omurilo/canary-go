package bosstiary

import "testing"

func TestLevel(t *testing.T) {
	cases := []struct {
		r     Rarity
		kills uint32
		want  uint8
	}{
		{RarityBane, 0, 0}, {RarityBane, 24, 0}, {RarityBane, 25, 1},
		{RarityBane, 99, 1}, {RarityBane, 100, 2}, {RarityBane, 299, 2}, {RarityBane, 300, 3}, {RarityBane, 5000, 3},
		{RarityArchfoe, 4, 0}, {RarityArchfoe, 5, 1}, {RarityArchfoe, 20, 2}, {RarityArchfoe, 60, 3},
		{RarityNemesis, 0, 0}, {RarityNemesis, 1, 1}, {RarityNemesis, 3, 2}, {RarityNemesis, 5, 3},
		{RarityInvalid, 1000, 0},
	}
	for _, c := range cases {
		if got := Level(c.r, c.kills); got != c.want {
			t.Errorf("Level(%d, %d) = %d, want %d", c.r, c.kills, got, c.want)
		}
	}
}

func TestPointsForCrossing(t *testing.T) {
	cases := []struct {
		r        Rarity
		old, new uint32
		want     uint16
	}{
		{RarityBane, 24, 25, 5},     // cross Prowess
		{RarityBane, 99, 100, 15},   // cross Expertise
		{RarityBane, 299, 300, 30},  // cross Mastery
		{RarityBane, 0, 300, 50},    // cross all three at once (5+15+30)
		{RarityBane, 25, 26, 0},     // no threshold crossed
		{RarityArchfoe, 0, 20, 40},  // 10+30
		{RarityNemesis, 0, 5, 100},  // 10+30+60
		{RarityNemesis, 5, 10, 0},   // already maxed
	}
	for _, c := range cases {
		if got := PointsForCrossing(c.r, c.old, c.new); got != c.want {
			t.Errorf("PointsForCrossing(%d, %d, %d) = %d, want %d", c.r, c.old, c.new, got, c.want)
		}
	}
}

func TestCalculateLootBonus(t *testing.T) {
	cases := []struct {
		points uint32
		want   uint16
	}{
		{0, 25}, {250, 50}, {1250, 100},
	}
	for _, c := range cases {
		if got := CalculateLootBonus(c.points); got != c.want {
			t.Errorf("CalculateLootBonus(%d) = %d, want %d", c.points, got, c.want)
		}
	}
}

func TestCalculateBossPoints(t *testing.T) {
	cases := []struct {
		lootBonus uint16
		want      uint32
	}{
		{25, 0}, {50, 250}, {100, 1250},
	}
	for _, c := range cases {
		if got := CalculateBossPoints(c.lootBonus); got != c.want {
			t.Errorf("CalculateBossPoints(%d) = %d, want %d", c.lootBonus, got, c.want)
		}
	}
}

func TestRemoveBossPrice(t *testing.T) {
	cases := []struct {
		times uint8
		want  uint32
	}{
		{0, 0}, {1, 0}, {2, 100000}, {3, 400000},
	}
	for _, c := range cases {
		if got := RemoveBossPrice(c.times); got != c.want {
			t.Errorf("RemoveBossPrice(%d) = %d, want %d", c.times, got, c.want)
		}
	}
}

func TestPointsForLevel(t *testing.T) {
	if got := PointsForLevel(RarityBane, 1); got != 5 {
		t.Errorf("PointsForLevel(Bane,1) = %d, want 5", got)
	}
	if got := PointsForLevel(RarityNemesis, 3); got != 60 {
		t.Errorf("PointsForLevel(Nemesis,3) = %d, want 60", got)
	}
	if got := PointsForLevel(RarityBane, 0); got != 0 {
		t.Errorf("PointsForLevel(Bane,0) = %d, want 0", got)
	}
}
