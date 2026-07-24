// Package bestiary holds the pure Bestiary (monster Cyclopedia) rules: the
// unlock stages a monster's kill count progresses through and the completion
// check that awards charm points. No world/DB/protocol deps. Mirrors
// src/io/iobestiary.cpp.
package bestiary

// Race is a bestiary class id (BESTY_RACE_*).
type Race uint8

const (
	RaceNone             Race = 0
	RaceAmphibic         Race = 1
	RaceAquatic          Race = 2
	RaceBird             Race = 3
	RaceConstruct        Race = 4
	RaceDemon            Race = 5
	RaceDragon           Race = 6
	RaceElemental        Race = 7
	RaceFey              Race = 8
	RaceGiant            Race = 9
	RaceHuman            Race = 10
	RaceHumanoid         Race = 11
	RaceLycanthrope      Race = 12
	RaceMagical          Race = 13
	RaceMammal           Race = 14
	RacePlant            Race = 15
	RaceReptile          Race = 16
	RaceSlime            Race = 17
	RaceUndead           Race = 18
	RaceVermin           Race = 19
	RaceExtraDimensional Race = 20
	RaceInkborn          Race = 21
)

// Thresholds are a monster's bestiary kill milestones.
type Thresholds struct {
	FirstUnlock  uint32
	SecondUnlock uint32
	ToKill       uint32 // completion
}

// KillStatus returns the unlock stage (1..4) for a kill count. Mirrors
// IOBestiary::getKillStatus: 1 = just started (< first unlock), 2 = first
// unlock reached, 3 = second unlock reached, 4 = completed (>= ToKill).
func KillStatus(kills uint32, t Thresholds) uint8 {
	switch {
	case kills < t.FirstUnlock:
		return 1
	case kills < t.SecondUnlock:
		return 2
	case kills < t.ToKill:
		return 3
	default:
		return 4
	}
}

// IsComplete reports whether the entry is fully unlocked (>= ToKill).
func IsComplete(kills uint32, t Thresholds) bool {
	return t.ToKill > 0 && kills >= t.ToKill
}

// CrossedStage reports whether adding `amount` kills reaches a new unlock stage
// (the initial kill, or crossing First/Second/ToKill) — when the client should
// be told the entry changed. Mirrors the boundary check in
// IOBestiary::addBestiaryKill.
func CrossedStage(old, amount uint32, t Thresholds) bool {
	if old == 0 {
		return true
	}
	newCount := old + amount
	for _, edge := range []uint32{t.FirstUnlock, t.SecondUnlock, t.ToKill} {
		if edge > 0 && old < edge && newCount >= edge {
			return true
		}
	}
	return false
}

// CrossedCompletion reports whether adding `amount` kills completes the entry
// for the first time — the moment charm points are awarded.
func CrossedCompletion(old, amount uint32, t Thresholds) bool {
	return t.ToKill > 0 && old < t.ToKill && old+amount >= t.ToKill
}
