// Package bosstiary holds the pure Boss Cyclopedia (bosstiary) rules: boss
// rarities, the kill thresholds that unlock each level (Prowess/Expertise/
// Mastery), and the boss-points <-> loot-bonus math. It has no dependency on
// the world/DB/protocol so it can be shared by game, db, luaengine and
// protocol. Mirrors src/io/io_bosstiary.{hpp,cpp}.
package bosstiary

import "math"

// Rarity is a boss's bosstiary class (C++ BosstiaryRarity_t).
type Rarity uint8

const (
	RarityBane    Rarity = 0
	RarityArchfoe Rarity = 1
	RarityNemesis Rarity = 2

	// RarityInvalid marks a monster that is not a bosstiary boss (server-only,
	// never sent to the client).
	RarityInvalid Rarity = 10
)

// LevelStage is one unlock threshold: reaching Kills kills of the boss awards
// Points boss points and advances the boss one level.
type LevelStage struct {
	Kills  uint32
	Points uint16
}

// stages maps each rarity to its three unlock stages
// (Prowess, Expertise, Mastery). Copied verbatim from IOBosstiary::levelInfos.
var stages = map[Rarity][3]LevelStage{
	RarityBane:    {{25, 5}, {100, 15}, {300, 30}},
	RarityArchfoe: {{5, 10}, {20, 30}, {60, 60}},
	RarityNemesis: {{1, 10}, {3, 30}, {5, 60}},
}

// Stages returns the three unlock stages for a rarity (zero value + false when
// the rarity is not a real boss class).
func Stages(r Rarity) ([3]LevelStage, bool) {
	s, ok := stages[r]
	return s, ok
}

// IsBoss reports whether r is a real bosstiary rarity (not RarityInvalid).
func IsBoss(r Rarity) bool {
	_, ok := stages[r]
	return ok
}

// Level returns the boss's current unlock level (0..3) for a kill count.
// 0 = not yet unlocked, 1 = Prowess, 2 = Expertise, 3 = Mastery.
// Mirrors IOBosstiary::getBossCurrentLevel.
func Level(r Rarity, kills uint32) uint8 {
	s, ok := stages[r]
	if !ok {
		return 0
	}
	var level uint8
	for _, stage := range s {
		if kills >= stage.Kills {
			level++
		}
	}
	return level
}

// PointsForLevel returns the boss points awarded for reaching level (1..3).
// Returns 0 for out-of-range levels.
func PointsForLevel(r Rarity, level uint8) uint16 {
	s, ok := stages[r]
	if !ok || level < 1 || level > 3 {
		return 0
	}
	return s[level-1].Points
}

// PointsForCrossing returns the boss points a player gains when their kill
// count on a boss goes from oldKills to newKills — the sum of the Points of
// every stage whose threshold lies in (oldKills, newKills]. This is what
// addBosstiaryKill awards on each kill that crosses one or more thresholds.
func PointsForCrossing(r Rarity, oldKills, newKills uint32) uint16 {
	s, ok := stages[r]
	if !ok || newKills <= oldKills {
		return 0
	}
	var gained uint16
	for _, stage := range s {
		if stage.Kills > oldKills && stage.Kills <= newKills {
			gained += stage.Points
		}
	}
	return gained
}

// CalculateLootBonus converts accumulated boss points into the loot-bonus
// percentage shown in the cyclopedia. Mirrors IOBosstiary::calculateLootBonus.
func CalculateLootBonus(bossPoints uint32) uint16 {
	switch {
	case bossPoints <= 250:
		return uint16(25 + bossPoints/10)
	case bossPoints < 1250:
		return uint16(37.5 + float64(bossPoints)/20)
	default:
		return uint16(100 + 0.5*(math.Sqrt(8*(float64(bossPoints-1250)/5)+81)-9))
	}
}

// CalculateBossPoints is the inverse of CalculateLootBonus (points for a given
// loot bonus). Mirrors IOBosstiary::calculateBossPoints.
func CalculateBossPoints(lootBonus uint16) uint32 {
	switch {
	case lootBonus <= 25:
		return 0
	case lootBonus <= 50:
		return uint32(10*lootBonus) - 250
	case lootBonus <= 100:
		return uint32(20*lootBonus) - 750
	default:
		lb := float64(lootBonus)
		return uint32((2.5 * lb * lb) - (477.5 * lb) + 24000)
	}
}

// RemoveBossPrice returns the gold cost to remove a boss from a bosstiary slot,
// given how many times the player has already removed one. The first removal is
// free. Mirrors IOBosstiary::calculteRemoveBoss.
func RemoveBossPrice(removeTimes uint8) uint32 {
	if removeTimes < 2 {
		return 0
	}
	return 300000*uint32(removeTimes) - 500000
}
