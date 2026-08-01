package game

import (
	"sync"

	"github.com/omurilo/canary-go/internal/config"
)

type PreyBonusType byte

const (
	PreyBonus_DamageBoost     PreyBonusType = 0
	PreyBonus_DamageReduction PreyBonusType = 1
	PreyBonus_XPBonus         PreyBonusType = 2
	PreyBonus_ImprovedLoot    PreyBonusType = 3
)

type PreyState byte

const (
	PreyDataState_Locked                 PreyState = 0
	PreyDataState_Inactive               PreyState = 1
	PreyDataState_Active                 PreyState = 2
	PreyDataState_Selection              PreyState = 3
	PreyDataState_SelectionChangeMonster PreyState = 4
	PreyDataState_ListSelection          PreyState = 5
)

type PreySlot struct {
	ID                  byte
	State               PreyState
	SelectedRaceID      uint16
	RaceIDList          []uint16
	Bonus               PreyBonusType
	BonusPercentage     uint16
	BonusRarity         byte
	BonusTimeLeft       uint16 // seconds
	FreeRerollTimeStamp int64  // unix timestamp
	Option              byte
}

type PlayerPrey struct {
	mu    sync.RWMutex
	Slots [3]*PreySlot
}

var DefaultPreyRaceIDs = []uint16{15, 26, 22, 33, 35, 34, 55, 38, 5}

func (slot *PreySlot) ReloadMonsterGrid() {
	// Rotate or pick 9 default race IDs
	slot.RaceIDList = make([]uint16, len(DefaultPreyRaceIDs))
	copy(slot.RaceIDList, DefaultPreyRaceIDs)
}

func NewPlayerPrey() *PlayerPrey {
	pp := &PlayerPrey{}
	for i := byte(0); i < 3; i++ {
		slot := &PreySlot{
			ID:                  i,
			State:               PreyDataState_Selection,
			RaceIDList:          []uint16{},
			BonusTimeLeft:       7200, // 2 hours default
			FreeRerollTimeStamp: 0,
		}
		slot.ReloadMonsterGrid()
		pp.Slots[i] = slot
	}
	return pp
}

func (pp *PlayerPrey) GetSlot(slotID byte) *PreySlot {
	pp.mu.RLock()
	defer pp.mu.RUnlock()
	if slotID >= 3 {
		return nil
	}
	return pp.Slots[slotID]
}

func (pp *PlayerPrey) GetPreyBonus(raceID uint16, bonusType PreyBonusType) (uint16, bool) {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	for _, slot := range pp.Slots {
		if slot == nil || slot.State != PreyDataState_Active {
			continue
		}
		if slot.SelectedRaceID == raceID && slot.Bonus == bonusType && slot.BonusTimeLeft > 0 {
			return slot.BonusPercentage, true
		}
	}
	return 0, false
}

func (pp *PlayerPrey) TickStamina(seconds uint16) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	for _, slot := range pp.Slots {
		if slot != nil && slot.State == PreyDataState_Active && slot.BonusTimeLeft > 0 {
			if slot.BonusTimeLeft > seconds {
				slot.BonusTimeLeft -= seconds
			} else {
				slot.BonusTimeLeft = 0
				slot.State = PreyDataState_Selection
			}
		}
	}
}

// GetPreyRerollPrice returns the gold cost to reroll the prey list, mirroring
// Player::getPreyRerollPrice (level * preyRerollPricePerLevel; config.lua, 200).
func (p *Player) GetPreyRerollPrice() uint32 {
	return uint32(p.Level) * uint32(config.Number("preyRerollPricePerLevel", 200))
}

// PreyBonusPercentage returns the bonus percentage for a bonus type at a rarity,
// mirroring PreySlot::reloadBonusValue: Damage 2*r+5, Defense 2*r+10,
// XP/Loot 3*r+10.
func PreyBonusPercentage(bonusType PreyBonusType, rarity uint8) uint16 {
	r := uint16(rarity)
	switch bonusType {
	case PreyBonus_DamageBoost:
		return 2*r + 5
	case PreyBonus_DamageReduction:
		return 2*r + 10
	default: // XP / Loot
		return 3*r + 10
	}
}
