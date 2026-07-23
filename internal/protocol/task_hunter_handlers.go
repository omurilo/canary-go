package protocol

import (
	"time"

	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
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
		// Difficulty/rarity would come from the monster's bestiary stars; until
		// that is modeled StartTask defaults to Easy/1 and derives TargetKills
		// from the option table (no more hardcoded 200).
		slot.StartTask(raceID, slot.Difficulty, slot.Rarity, upgrade)
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
