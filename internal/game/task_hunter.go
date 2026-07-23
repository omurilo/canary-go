package game

import (
	"sync"
)

type TaskHuntingState byte

const (
	PreyTaskDataState_Locked        TaskHuntingState = 0
	PreyTaskDataState_Inactive      TaskHuntingState = 1
	PreyTaskDataState_Selection     TaskHuntingState = 2
	PreyTaskDataState_ListSelection TaskHuntingState = 3
	PreyTaskDataState_Active        TaskHuntingState = 4
	PreyTaskDataState_Completed     TaskHuntingState = 5
)

type TaskHuntingSlot struct {
	ID                  byte
	State               TaskHuntingState
	SelectedRaceID      uint16
	RaceIDList          []uint16
	CurrentKills        uint16
	TargetKills         uint16
	Rarity              byte
	Upgrade             bool
	FreeRerollTimeStamp int64
}

type PlayerTaskHunter struct {
	mu     sync.RWMutex
	Points uint32
	Slots  [9]*TaskHuntingSlot
}

func NewPlayerTaskHunter() *PlayerTaskHunter {
	pth := &PlayerTaskHunter{
		Points: 0,
	}
	for i := byte(0); i < 9; i++ {
		pth.Slots[i] = &TaskHuntingSlot{
			ID:                  i,
			State:               PreyTaskDataState_Selection,
			RaceIDList:          []uint16{},
			TargetKills:         200,
			FreeRerollTimeStamp: 0,
		}
	}
	return pth
}

func (pth *PlayerTaskHunter) GetSlot(slotID byte) *TaskHuntingSlot {
	pth.mu.RLock()
	defer pth.mu.RUnlock()
	if slotID >= 9 {
		return nil
	}
	return pth.Slots[slotID]
}

func (pth *PlayerTaskHunter) OnKillMonster(raceID uint16) []*TaskHuntingSlot {
	pth.mu.Lock()
	defer pth.mu.Unlock()

	var updated []*TaskHuntingSlot
	for _, slot := range pth.Slots {
		if slot == nil || slot.State != PreyTaskDataState_Active {
			continue
		}
		if slot.SelectedRaceID == raceID {
			slot.CurrentKills++
			if slot.CurrentKills >= slot.TargetKills {
				slot.State = PreyTaskTaskDataStateCompleted()
			}
			updated = append(updated, slot)
		}
	}
	return updated
}

func PreyTaskTaskDataStateCompleted() TaskHuntingState {
	return PreyTaskDataState_Completed
}

// GetTaskHuntingRerollPrice returns gold cost per level to reroll task list.
func (p *Player) GetTaskHuntingRerollPrice() uint32 {
	lvl := p.Level
	if lvl < 1 {
		lvl = 1
	}
	return uint32(lvl * 20)
}
