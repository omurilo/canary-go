package combat

import (
	"math"
	"math/rand"
	"time"

	"github.com/opentibiabr/canary-go/internal/game/vocations"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// CalculateMeleeDamage returns damage value based on Tibia formulas from Canary.
func CalculateMeleeDamage(attack int, skill int, armor int, voc *vocations.Vocation) int {
	multiplier := 1.0
	if voc != nil {
		multiplier = voc.Formula.MeleeDamage
	}

	maxDamage := math.Ceil(float64(skill+4) * float64(attack) * 0.0605 * multiplier)
	if maxDamage <= 0 {
		return 0
	}

	// Damage range is [0, maxDamage]
	dmg := rand.Intn(int(maxDamage) + 1)

	armorReduction := 0
	if armor > 0 {
		minArmor := armor / 2
		armorReduction = minArmor + rand.Intn(armor-minArmor+1)
	}

	dmg -= armorReduction
	if dmg < 0 {
		dmg = 0
	}

	return dmg
}

func CalculateDistanceDamage(attack int, skill int, armor int, voc *vocations.Vocation) int {
	multiplier := 1.0
	if voc != nil {
		multiplier = voc.Formula.DistDamage
	}

	maxDamage := math.Ceil(float64(skill+4) * float64(attack) * 0.0605 * multiplier)
	if maxDamage <= 0 {
		return 0
	}

	dmg := rand.Intn(int(maxDamage) + 1)

	armorReduction := 0
	if armor > 0 {
		minArmor := armor / 2
		armorReduction = minArmor + rand.Intn(armor-minArmor+1)
	}

	dmg -= armorReduction
	if dmg < 0 {
		dmg = 0
	}

	return dmg
}
