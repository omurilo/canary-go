package protocol

import (
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseTaskHuntingAction handles Opcode 0xED (Task Hunting Action).
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
	case 0: // Reroll Task List
		slot.State = game.PreyTaskDataState_Selection
	case 1: // Select Task Monster
		slot.SelectedRaceID = raceID
		slot.Upgrade = upgrade
		slot.CurrentKills = 0
		slot.TargetKills = 200
		slot.State = game.PreyTaskDataState_Active
	case 2: // Upgrade / Select Option
		slot.Upgrade = upgrade
	case 3: // Cancel Task
		slot.State = game.PreyTaskDataState_Selection
		slot.CurrentKills = 0
	case 4: // Claim Reward
		if slot.State == game.PreyTaskDataState_Completed {
			taskHunter.Points += 10 // Grant task hunting points
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
