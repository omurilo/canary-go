package game

import "time"

// ConcoctionType matches the Lua Concoction_* enum values (1-20).
type ConcoctionType uint8

const (
	ConcoctionKooldownAid            ConcoctionType = 1
	ConcoctionStaminaExtension       ConcoctionType = 2
	ConcoctionStrikeEnhancement      ConcoctionType = 3
	ConcoctionCharmUpgrade           ConcoctionType = 4
	ConcoctionWealthDuplex           ConcoctionType = 5
	ConcoctionBestiaryBetterment     ConcoctionType = 6
	ConcoctionFireResilience         ConcoctionType = 7
	ConcoctionIceResilience          ConcoctionType = 8
	ConcoctionEarthResilience        ConcoctionType = 9
	ConcoctionEnergyResilience       ConcoctionType = 10
	ConcoctionHolyResilience         ConcoctionType = 11
	ConcoctionDeathResilience        ConcoctionType = 12
	ConcoctionPhysicalResilience     ConcoctionType = 13
	ConcoctionFireAmplification      ConcoctionType = 14
	ConcoctionIceAmplification       ConcoctionType = 15
	ConcoctionEarthAmplification     ConcoctionType = 16
	ConcoctionEnergyAmplification    ConcoctionType = 17
	ConcoctionHolyAmplification      ConcoctionType = 18
	ConcoctionDeathAmplification     ConcoctionType = 19
	ConcoctionPhysicalAmplification  ConcoctionType = 20
)

// concoctionNames maps ConcoctionType to its string key used in Player.Concoctions map.
var concoctionNames = map[ConcoctionType]string{
	ConcoctionKooldownAid:            "kooldown-aid",
	ConcoctionStaminaExtension:       "stamina-extension",
	ConcoctionStrikeEnhancement:      "strike-enhancement",
	ConcoctionCharmUpgrade:           "charm-upgrade",
	ConcoctionWealthDuplex:           "wealth-duplex",
	ConcoctionBestiaryBetterment:     "bestiary-betterment",
	ConcoctionFireResilience:         "fire-resilience",
	ConcoctionIceResilience:          "ice-resilience",
	ConcoctionEarthResilience:        "earth-resilience",
	ConcoctionEnergyResilience:       "energy-resilience",
	ConcoctionHolyResilience:         "holy-resilience",
	ConcoctionDeathResilience:        "death-resilience",
	ConcoctionPhysicalResilience:     "physical-resilience",
	ConcoctionFireAmplification:      "fire-amplification",
	ConcoctionIceAmplification:       "ice-amplification",
	ConcoctionEarthAmplification:     "earth-amplification",
	ConcoctionEnergyAmplification:    "energy-amplification",
	ConcoctionHolyAmplification:      "holy-amplification",
	ConcoctionDeathAmplification:     "death-amplification",
	ConcoctionPhysicalAmplification:  "physical-amplification",
}

// concoctionItemIDs maps the Lua Concoction_* enum (1-20) to the client item IDs.
var concoctionItemIDs = map[ConcoctionType]uint16{
	ConcoctionKooldownAid:            36723,
	ConcoctionStaminaExtension:       36724,
	ConcoctionStrikeEnhancement:      36725,
	ConcoctionCharmUpgrade:           36726,
	ConcoctionWealthDuplex:           36727,
	ConcoctionBestiaryBetterment:     36728,
	ConcoctionFireResilience:         36729,
	ConcoctionIceResilience:          36730,
	ConcoctionEarthResilience:        36731,
	ConcoctionEnergyResilience:       36732,
	ConcoctionHolyResilience:         36733,
	ConcoctionDeathResilience:        36734,
	ConcoctionPhysicalResilience:     36735,
	ConcoctionFireAmplification:      36736,
	ConcoctionIceAmplification:       36737,
	ConcoctionEarthAmplification:     36738,
	ConcoctionEnergyAmplification:    36739,
	ConcoctionHolyAmplification:      36740,
	ConcoctionDeathAmplification:     36741,
	ConcoctionPhysicalAmplification:  36742,
}

// UpdateConcoction sets or updates an active concoction with the given timeLeft in seconds.
// The expiry is calculated as now + timeLeft.
func UpdateConcoction(p *Player, concoctionID uint8, timeLeft int64) {
	if p == nil {
		return
	}
	name, ok := concoctionNames[ConcoctionType(concoctionID)]
	if !ok {
		return
	}
	if p.Concoctions == nil {
		p.Concoctions = make(map[string]int64)
	}
	if timeLeft <= 0 {
		delete(p.Concoctions, name)
	} else {
		p.Concoctions[name] = time.Now().UnixMilli() + timeLeft*1000
	}
}

// IsConcoctionActive returns true if the given concoction is currently active.
func IsConcoctionActive(p *Player, concoctionID uint8) bool {
	if p == nil || p.Concoctions == nil {
		return false
	}
	name, ok := concoctionNames[ConcoctionType(concoctionID)]
	if !ok {
		return false
	}
	expiry, ok := p.Concoctions[name]
	return ok && expiry > time.Now().UnixMilli()
}

// GetActiveConcoctions returns the list of currently active concoctions.
func GetActiveConcoctions(p *Player) []uint8 {
	if p == nil || p.Concoctions == nil {
		return nil
	}
	now := time.Now().UnixMilli()
	var active []uint8
	for id, name := range concoctionNames {
		if expiry, ok := p.Concoctions[name]; ok && expiry > now {
			active = append(active, uint8(id))
		}
	}
	return active
}
