package game

import (
	"math"
	"math/rand"
	"sync"

	"github.com/opentibiabr/canary-go/internal/config"
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

// Task difficulty tiers (PreyTaskDifficult_t: Easy=1, Medium=2, Hard=3).
const (
	TaskDifficultyEasy   byte = 1
	TaskDifficultyMedium byte = 2
	TaskDifficultyHard   byte = 3
)

type TaskHuntingSlot struct {
	ID                  byte
	State               TaskHuntingState
	SelectedRaceID      uint16
	RaceIDList          []uint16
	CurrentKills        uint16
	TargetKills         uint16
	Difficulty          byte
	Rarity              byte
	Upgrade             bool
	FreeRerollTimeStamp int64
}

// TaskHuntingOption is one (difficulty, rarity) reward tier, mirroring C++
// TaskHuntingOption populated by IOPrey::initializeTaskHuntOptions.
type TaskHuntingOption struct {
	Difficulty   byte
	Rarity       byte
	FirstKills   uint16
	FirstReward  uint16
	SecondKills  uint16
	SecondReward uint16
}

// taskHuntingOptions[difficulty][rarity] is generated exactly like the C++ loop:
// kills = 25, ×4 per difficulty; reward = round(10*kills/25) then compounded per
// star by (115 + difficulty*5)%.
var taskHuntingOptions = buildTaskHuntingOptions()

func buildTaskHuntingOptions() map[byte]map[byte]TaskHuntingOption {
	const killStage = 25
	const limitOfStars = 5
	out := map[byte]map[byte]TaskHuntingOption{}
	kills := uint16(killStage)
	for difficulty := byte(1); difficulty <= 3; difficulty++ {
		reward := uint16(math.Round(float64(10*kills) / killStage))
		out[difficulty] = map[byte]TaskHuntingOption{}
		for star := byte(1); star <= limitOfStars; star++ {
			out[difficulty][star] = TaskHuntingOption{
				Difficulty:   difficulty,
				Rarity:       star,
				FirstKills:   kills,
				FirstReward:  reward,
				SecondKills:  kills * 2,
				SecondReward: reward * 2,
			}
			reward = uint16(math.Round(float64(reward) * float64(115+int(difficulty)*limitOfStars) / 100))
		}
		kills *= 4
	}
	return out
}

// TaskHuntingOptionFor returns the reward tier for (difficulty, rarity),
// clamping to the valid ranges.
func TaskHuntingOptionFor(difficulty, rarity byte) TaskHuntingOption {
	if difficulty < 1 {
		difficulty = 1
	}
	if difficulty > 3 {
		difficulty = 3
	}
	if rarity < 1 {
		rarity = 1
	}
	if rarity > 5 {
		rarity = 5
	}
	return taskHuntingOptions[difficulty][rarity]
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
			Difficulty:          TaskDifficultyEasy,
			Rarity:              1,
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

// StartTask configures a slot for a newly selected monster: it stores the
// difficulty/rarity/upgrade and derives the kill target from the option table
// (first or second tier depending on the upgrade), mirroring the C++ selection
// path. difficulty/rarity would come from the monster's bestiary stars; until
// that data is modeled they default to Easy/1 via TaskHuntingOptionFor.
func (s *TaskHuntingSlot) StartTask(raceID uint16, difficulty, rarity byte, upgrade bool) {
	opt := TaskHuntingOptionFor(difficulty, rarity)
	s.SelectedRaceID = raceID
	s.Difficulty = opt.Difficulty
	s.Rarity = opt.Rarity
	s.Upgrade = upgrade
	s.CurrentKills = 0
	if upgrade {
		s.TargetKills = opt.SecondKills
	} else {
		s.TargetKills = opt.FirstKills
	}
	s.State = PreyTaskDataState_Active
}

// ClaimReward computes and returns the hunting-task points for a completed slot,
// mirroring the PreyTaskAction_Claim math: a base reward (first/second tier by
// upgrade) scaled by a rarity-dependent boost (10/15/20 → ×1.0/1.5/2.0, ceil).
// Returns 0 and false when the slot has not met its kill target.
func (s *TaskHuntingSlot) ClaimReward() (uint64, bool) {
	opt := TaskHuntingOptionFor(s.Difficulty, s.Rarity)
	var base uint16
	switch {
	case s.Upgrade && s.CurrentKills >= opt.SecondKills:
		base = opt.SecondReward
	case s.CurrentKills >= opt.FirstKills:
		base = opt.FirstReward
	default:
		return 0, false
	}
	boost := rand.Intn(101) // 0..100
	switch {
	case s.Rarity >= 4 && boost <= 5:
		boost = 20
	case s.Rarity >= 4 && boost <= 10:
		boost = 15
	default:
		boost = 10
	}
	reward := uint64(math.Ceil(float64(base) * float64(boost) / 10))
	return reward, true
}

// GetTaskHuntingRerollPrice returns the gold cost to reroll the task list,
// mirroring level * taskHuntingRerollPricePerLevel (config.lua, default 200).
func (p *Player) GetTaskHuntingRerollPrice() uint32 {
	return uint32(p.Level) * uint32(config.Number("taskHuntingRerollPricePerLevel", 200))
}
