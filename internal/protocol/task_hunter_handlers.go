package protocol

import (
	"time"

	"github.com/omurilo/canary-go/internal/config"
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// parseTaskHuntingAction handles Opcode 0xBA (Task Hunting Action).
func (g *GameProtocol) parseTaskHuntingAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	slotID := r.GetByte()
	action := r.GetByte()
	upgrade := r.GetByte() != 0
	raceID := r.GetU16()

	taskHunter := g.player.GetTaskHunter()
	slot := taskHunter.GetSlot(slotID)
	if slot == nil {
		return
	}

	switch action {
	case 0: // ListReroll — free once per taskHuntingFreeRerollTime, else costs gold.
		now := time.Now().Unix()
		if slot.FreeRerollTimeStamp > now {
			if !g.player.RemoveMoney(uint64(g.player.GetTaskHuntingRerollPrice()), true) {
				g.player.SendTextMessage(0x14, "You don't have enough money to reroll the task slot.")
				return
			}
		} else {
			slot.FreeRerollTimeStamp = now + config.Number("taskHuntingFreeRerollTime", 20*60*60)
		}
		slot.State = game.PreyTaskDataState_Selection
	case 1: // RewardsReroll — costs prey cards.
		if !g.player.UsePreyCards(uint32(config.Number("taskHuntingBonusRerollPrice", 1))) {
			g.player.SendTextMessage(0x14, "You don't have enough prey cards to reroll this task.")
			return
		}
		slot.Upgrade = upgrade
	case 2: // ListAll (Cards) — costs prey cards to browse the full list.
		if !g.player.UsePreyCards(uint32(config.Number("taskHuntingSelectListPrice", 5))) {
			g.player.SendTextMessage(0x14, "You don't have enough prey cards to choose a task from the list.")
			return
		}
		slot.State = game.PreyTaskDataState_Selection
	case 3: // MonsterSelection
		// Difficulty and rarity come from the monster's bestiary stars, which set
		// the kill target and reward tier via the option table.
		difficulty, rarity := g.taskDifficultyForRace(raceID)
		slot.StartTask(raceID, difficulty, rarity, upgrade)
	case 4: // Cancel
		slot.State = game.PreyTaskDataState_Selection
		slot.CurrentKills = 0
	case 5: // Claim
		if slot.State == game.PreyTaskDataState_Completed {
			if reward, ok := slot.ClaimReward(); ok {
				g.player.AddTaskHuntingPoints(uint32(reward))
			}
			slot.State = game.PreyTaskDataState_Selection
			slot.CurrentKills = 0
		}
	}

	g.SendTaskHuntingData(slot)
	g.sendResourceBalance(24, uint64(g.player.GetTaskHuntingPoints()))
}

// taskDifficultyForRace maps a monster's bestiary stars to a task difficulty
// (≤1 Easy, ≤3 Medium, else Hard) and rarity (the star count, clamped 1..5),
// mirroring the star→difficulty split in IOPrey::initializeTaskHuntOptions.
func (g *GameProtocol) taskDifficultyForRace(raceID uint16) (byte, byte) {
	var stars byte
	if g.deps != nil && g.deps.World != nil && g.deps.World.TypeRegistry != nil {
		for _, m := range g.deps.World.TypeRegistry.Monsters {
			if m != nil && m.RaceID == raceID {
				stars = m.BestiaryStars
				break
			}
		}
	}
	rarity := stars
	if rarity < 1 {
		rarity = 1
	}
	if rarity > 5 {
		rarity = 5
	}
	switch {
	case stars <= 1:
		return game.TaskDifficultyEasy, rarity
	case stars <= 3:
		return game.TaskDifficultyMedium, rarity
	default:
		return game.TaskDifficultyHard, rarity
	}
}

// SendTaskHuntingData sends Opcode 0xBB (Task Hunting Data).
func (g *GameProtocol) SendTaskHuntingData(slot *game.TaskHuntingSlot) {
	if g.player == nil || slot == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0xBB)
	w.AddByte(slot.ID)
	w.AddByte(byte(slot.State))

	switch slot.State {
	case game.PreyTaskDataState_Locked:
		w.AddByte(1) // is premium
	case game.PreyTaskDataState_Inactive:
		// empty
	case game.PreyTaskDataState_Selection:
		w.AddU16(0) // count of raceIds
	case game.PreyTaskDataState_ListSelection:
		w.AddU16(0) // count of bestiary list
	case game.PreyTaskDataState_Active, game.PreyTaskDataState_Completed:
		w.AddU16(slot.SelectedRaceID)
		if slot.Upgrade {
			w.AddByte(1)
		} else {
			w.AddByte(0)
		}
		w.AddU16(slot.TargetKills)
		w.AddU16(slot.CurrentKills)
		w.AddByte(slot.Rarity)
	}

	freeRerollSecs := uint32(0)
	now := time.Now().Unix()
	if slot.FreeRerollTimeStamp > now {
		freeRerollSecs = uint32(slot.FreeRerollTimeStamp - now)
	}
	w.AddU32(freeRerollSecs)

	g.SendToClient(w)
}
