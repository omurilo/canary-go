package game

import "time"

// ConcoctionType is Concoction_t (creatures_definitions.hpp): the values are the
// concoction item ids, not a 1..20 index — C++ logs the map key as an itemId and
// sendCyclopediaCharacterMiscStats puts it straight on the wire. It used to be a
// uint8 holding 1..20, so the client received item id 1 for Kooldown Aid.
type ConcoctionType uint16

const (
	ConcoctionKooldownAid           ConcoctionType = 36723
	ConcoctionStaminaExtension      ConcoctionType = 36725
	ConcoctionStrikeEnhancement     ConcoctionType = 36724
	ConcoctionCharmUpgrade          ConcoctionType = 36726
	ConcoctionWealthDuplex          ConcoctionType = 36727
	ConcoctionBestiaryBetterment    ConcoctionType = 36728
	ConcoctionFireResilience        ConcoctionType = 36729
	ConcoctionIceResilience         ConcoctionType = 36730
	ConcoctionEarthResilience       ConcoctionType = 36731
	ConcoctionEnergyResilience      ConcoctionType = 36732
	ConcoctionHolyResilience        ConcoctionType = 36733
	ConcoctionDeathResilience       ConcoctionType = 36734
	ConcoctionPhysicalResilience    ConcoctionType = 36735
	ConcoctionFireAmplification     ConcoctionType = 36736
	ConcoctionIceAmplification      ConcoctionType = 36737
	ConcoctionEarthAmplification    ConcoctionType = 36738
	ConcoctionEnergyAmplification   ConcoctionType = 36739
	ConcoctionHolyAmplification     ConcoctionType = 36740
	ConcoctionDeathAmplification    ConcoctionType = 36741
	ConcoctionPhysicalAmplification ConcoctionType = 36742
)

// concoctionNames maps ConcoctionType to its string key used in Player.Concoctions map.
var concoctionNames = map[ConcoctionType]string{
	ConcoctionKooldownAid:           "kooldown-aid",
	ConcoctionStaminaExtension:      "stamina-extension",
	ConcoctionStrikeEnhancement:     "strike-enhancement",
	ConcoctionCharmUpgrade:          "charm-upgrade",
	ConcoctionWealthDuplex:          "wealth-duplex",
	ConcoctionBestiaryBetterment:    "bestiary-betterment",
	ConcoctionFireResilience:        "fire-resilience",
	ConcoctionIceResilience:         "ice-resilience",
	ConcoctionEarthResilience:       "earth-resilience",
	ConcoctionEnergyResilience:      "energy-resilience",
	ConcoctionHolyResilience:        "holy-resilience",
	ConcoctionDeathResilience:       "death-resilience",
	ConcoctionPhysicalResilience:    "physical-resilience",
	ConcoctionFireAmplification:     "fire-amplification",
	ConcoctionIceAmplification:      "ice-amplification",
	ConcoctionEarthAmplification:    "earth-amplification",
	ConcoctionEnergyAmplification:   "energy-amplification",
	ConcoctionHolyAmplification:     "holy-amplification",
	ConcoctionDeathAmplification:    "death-amplification",
	ConcoctionPhysicalAmplification: "physical-amplification",
}

// There is deliberately no enum-to-item-id table: the enum value IS the item id.
// The table that used to live here had StaminaExtension and StrikeEnhancement
// swapped against Concoction_t (36724/36725 instead of 36725/36724).

// UpdateConcoction sets or updates an active concoction with the given timeLeft in seconds.
// The expiry is calculated as now + timeLeft.
func UpdateConcoction(p *Player, concoctionID ConcoctionType, timeLeft int64) {
	if p == nil {
		return
	}
	name, ok := concoctionNames[concoctionID]
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
func IsConcoctionActive(p *Player, concoctionID ConcoctionType) bool {
	if p == nil || p.Concoctions == nil {
		return false
	}
	name, ok := concoctionNames[concoctionID]
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
