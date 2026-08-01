package game

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// WheelGemQuality mirrors C++ WheelGemQuality_t.
type WheelGemQuality uint8

const (
	GemQualityLesser  WheelGemQuality = 0
	GemQualityRegular WheelGemQuality = 1
	GemQualityGreater WheelGemQuality = 2
)

// WheelGemAffinity mirrors C++ WheelGemAffinity_t (domain).
type WheelGemAffinity uint8

const (
	GemAffinityCombat  WheelGemAffinity = 0
	GemAffinityDefense WheelGemAffinity = 1
	GemAffinityHealing WheelGemAffinity = 2
	GemAffinitySupport WheelGemAffinity = 3
)

// WheelGemBasicModifier mirrors C++ WheelGemBasicModifier_t (0x00..0x2D, 46 values).
type WheelGemBasicModifier uint8

// WheelGemSupremeModifier mirrors C++ WheelGemSupremeModifier_t (vocation-specific, 23 values).
type WheelGemSupremeModifier uint8

// PlayerWheelGem mirrors C++ PlayerWheelGem.
type PlayerWheelGem struct {
	UUID            string
	Locked          bool
	Affinity        WheelGemAffinity
	Quality         WheelGemQuality
	BasicModifier1  WheelGemBasicModifier
	BasicModifier2  *WheelGemBasicModifier
	SupremeModifier *WheelGemSupremeModifier
}

// generateUUID creates a simple UUID string.
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// basicModifierPool is the pool of possible basic modifier 1 values.
var basicModifierPool = []WheelGemBasicModifier{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
}

// randomInt returns a random int in [0, max).
func randomInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

// selectBasicModifier2 selects a second modifier that differs from the first.
func selectBasicModifier2(m1 WheelGemBasicModifier) WheelGemBasicModifier {
	pool := make([]WheelGemBasicModifier, 0, len(basicModifierPool))
	for _, m := range basicModifierPool {
		if m != m1 {
			pool = append(pool, m)
		}
	}
	if len(pool) == 0 {
		return m1
	}
	return pool[randomInt(len(pool))]
}

// NewRevealedGem creates a new gem with random modifiers for the given quality.
func NewRevealedGem(quality WheelGemQuality) PlayerWheelGem {
	gem := PlayerWheelGem{
		UUID:     generateUUID(),
		Locked:   false,
		Affinity: WheelGemAffinity(randomInt(4)),
		Quality:  quality,
	}
	gem.BasicModifier1 = basicModifierPool[randomInt(len(basicModifierPool))]
	if quality >= GemQualityRegular {
		gem.BasicModifier2 = new(WheelGemBasicModifier)
		*gem.BasicModifier2 = selectBasicModifier2(gem.BasicModifier1)
	}
	if quality >= GemQualityGreater {
		gem.SupremeModifier = new(WheelGemSupremeModifier)
		*gem.SupremeModifier = WheelGemSupremeModifier(randomInt(23))
	}
	return gem
}

// WheelGemPersistData holds serializable gem data for DB persistence.
type WheelGemPersistData struct {
	ActiveGems   [4]*PlayerWheelGem
	RevealedGems []PlayerWheelGem
}

// WheelGemCollection holds gem data separate from WheelOfDestiny.
type WheelGemCollection struct {
	ActiveGems   [4]*PlayerWheelGem
	RevealedGems []PlayerWheelGem
}

func (gc *WheelGemCollection) GetActiveGemCount() int {
	count := 0
	for _, g := range gc.ActiveGems {
		if g != nil {
			count++
		}
	}
	return count
}

func (gc *WheelGemCollection) DestroyGem(index uint16) {
	if int(index) >= len(gc.RevealedGems) {
		return
	}
	for i, g := range gc.ActiveGems {
		if g != nil && g.UUID == gc.RevealedGems[index].UUID {
			gc.ActiveGems[i] = nil
			break
		}
	}
	gc.RevealedGems = append(gc.RevealedGems[:index], gc.RevealedGems[index+1:]...)
}

func (gc *WheelGemCollection) SwitchGemDomain(index uint16) {
	if int(index) >= len(gc.RevealedGems) {
		return
	}
	gc.RevealedGems[index].Affinity = WheelGemAffinity((gc.RevealedGems[index].Affinity + 1) % 4)
}

func (gc *WheelGemCollection) ToggleGemLock(index uint16) {
	if int(index) >= len(gc.RevealedGems) {
		return
	}
	gc.RevealedGems[index].Locked = !gc.RevealedGems[index].Locked
}

// ImproveGemGrade upgrades a gem by one quality tier.
// Returns the new quality (or current if already max) and whether it succeeded.
func (gc *WheelGemCollection) ImproveGemGrade(index uint16) (WheelGemQuality, bool) {
	if int(index) >= len(gc.RevealedGems) {
		return 0, false
	}
	gem := &gc.RevealedGems[index]
	if gem.Locked {
		return 0, false
	}

	switch gem.Quality {
	case GemQualityLesser:
		gem.Quality = GemQualityRegular
		bm2 := selectBasicModifier2(gem.BasicModifier1)
		gem.BasicModifier2 = &bm2
		return GemQualityRegular, true

	case GemQualityRegular:
		gem.Quality = GemQualityGreater
		sm := WheelGemSupremeModifier(randomInt(23))
		gem.SupremeModifier = &sm
		return GemQualityGreater, true

	case GemQualityGreater:
		return GemQualityGreater, false
	}
	return 0, false
}
