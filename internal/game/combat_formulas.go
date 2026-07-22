package game

import (
	"math"
	"math/rand"

	"github.com/opentibiabr/canary-go/internal/game/vocations"
)

// CalculateMeleeDamage returns damage value based on Tibia formulas from Canary.
func CalculateMeleeDamage(attack int, skill int, armor int, voc *vocations.Vocation, level int) int {
	multiplier := 1.0
	if voc != nil {
		multiplier = voc.Formula.MeleeDamage
	}

	// Modern Canary formula (attack * skill * vocationMeleeMult) / 50 + (level / 5)
	baseDamage := (float64(attack) * float64(skill) * multiplier) / 50.0
	levelBonus := float64(level) / 5.0
	maxDamage := math.Ceil(baseDamage + levelBonus)

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

func CalculateDistanceDamage(attack int, skill int, armor int, voc *vocations.Vocation, level int) int {
	multiplier := 1.0
	if voc != nil {
		multiplier = voc.Formula.DistDamage
	}

	baseDamage := (float64(attack) * float64(skill) * multiplier) / 50.0
	levelBonus := float64(level) / 5.0
	maxDamage := math.Ceil(baseDamage + levelBonus)

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
