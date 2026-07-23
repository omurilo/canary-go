package protocol

import (
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// getPreyMonsterInfo looks up a monster's name and outfit by raceID.
func (g *GameProtocol) getPreyMonsterInfo(raceID uint16) (string, uint16, byte, byte, byte, byte, byte) {
	if g.deps != nil && g.deps.World != nil && g.deps.World.TypeRegistry != nil {
		for _, m := range g.deps.World.TypeRegistry.Monsters {
			if m != nil && m.RaceID == raceID {
				return m.Name, m.Outfit.LookType, m.Outfit.Head, m.Outfit.Body, m.Outfit.Legs, m.Outfit.Feet, m.Outfit.Addons
			}
		}
	}

	// Fallback map for common monsters
	switch raceID {
	case 5:
		return "Orc", 5, 0, 0, 0, 0, 0
	case 15:
		return "Troll", 15, 0, 0, 0, 0, 0
	case 22:
		return "Cyclops", 22, 0, 0, 0, 0, 0
	case 26:
		return "Rotworm", 26, 0, 0, 0, 0, 0
	case 33:
		return "Skeleton", 33, 0, 0, 0, 0, 0
	case 34:
		return "Dragon", 34, 0, 0, 0, 0, 0
	case 35:
		return "Demon", 35, 0, 0, 0, 0, 0
	case 38:
		return "Giant Spider", 38, 0, 0, 0, 0, 0
	case 55:
		return "Behemoth", 55, 0, 0, 0, 0, 0
	default:
		return "Monster", 35, 0, 0, 0, 0, 0
	}
}

// parsePreyAction handles Opcode 0xEA (Prey Action / Reroll / Selection).
func (g *GameProtocol) parsePreyAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	slotID := r.GetByte()
	action := r.GetByte()

	var option byte
	var index int8 = -1
	var raceID uint16

	switch action {
	case 1: // PreyAction_MonsterSelection
		index = int8(r.GetByte())
	case 2: // PreyAction_Option
		option = r.GetByte()
	case 4: // PreyAction_ListAll_Selection
		raceID = r.GetU16()
	}

	prey := g.player.GetPrey()
	slot := prey.GetSlot(slotID)
	if slot == nil {
		return
	}

	switch action {
	case 0: // Reroll Monster List
		slot.ReloadMonsterGrid()
		slot.State = game.PreyDataState_Selection
		slot.Option = option
	case 1: // Select Monster from List
		if len(slot.RaceIDList) == 0 {
			slot.ReloadMonsterGrid()
		}
		if index >= 0 && int(index) < len(slot.RaceIDList) {
			slot.SelectedRaceID = slot.RaceIDList[index]
			slot.State = game.PreyDataState_Active
			slot.BonusTimeLeft = 7200
			if slot.BonusPercentage == 0 {
				slot.Bonus = game.PreyBonus_XPBonus
				slot.BonusPercentage = 25
				slot.BonusRarity = 5
			}
		}
	case 2: // Lock Option / Wildcard
		slot.State = game.PreyDataState_Active
		slot.BonusTimeLeft = 7200
	case 3: // List All Monsters
		slot.State = game.PreyDataState_ListSelection
	case 4: // Select From List
		slot.SelectedRaceID = raceID
		slot.State = game.PreyDataState_Active
		slot.BonusTimeLeft = 7200
		if slot.BonusPercentage == 0 {
			slot.Bonus = game.PreyBonus_XPBonus
			slot.BonusPercentage = 25
			slot.BonusRarity = 5
		}
	}

	g.SendPreyData(slot)
}

// SendPreyData sends Opcode 0xE8 (Prey Window Data).
func (g *GameProtocol) SendPreyData(slot *game.PreySlot) {
	if g.player == nil || slot == nil {
		return
	}

	if len(slot.RaceIDList) == 0 {
		slot.ReloadMonsterGrid()
	}

	w := netmsg.NewWriter()
	w.AddByte(0xE8)
	w.AddByte(slot.ID)
	w.AddByte(byte(slot.State))

	switch slot.State {
	case game.PreyDataState_Locked:
		w.AddByte(1) // is premium
	case game.PreyDataState_Inactive:
		// empty
	case game.PreyDataState_Active:
		name, lookType, head, body, legs, feet, addons := g.getPreyMonsterInfo(slot.SelectedRaceID)
		w.AddString(name)
		w.AddU16(lookType)
		w.AddByte(head)
		w.AddByte(body)
		w.AddByte(legs)
		w.AddByte(feet)
		w.AddByte(addons)

		w.AddByte(byte(slot.Bonus))
		w.AddU16(slot.BonusPercentage)
		w.AddByte(slot.BonusRarity)
		w.AddU16(slot.BonusTimeLeft)

	case game.PreyDataState_Selection:
		w.AddByte(byte(len(slot.RaceIDList)))
		for _, rID := range slot.RaceIDList {
			name, lookType, head, body, legs, feet, addons := g.getPreyMonsterInfo(rID)
			w.AddString(name)
			w.AddU16(lookType)
			w.AddByte(head)
			w.AddByte(body)
			w.AddByte(legs)
			w.AddByte(feet)
			w.AddByte(addons)
		}

	case game.PreyDataState_SelectionChangeMonster:
		w.AddByte(byte(slot.Bonus))
		w.AddU16(slot.BonusPercentage)
		w.AddByte(slot.BonusRarity)
		w.AddByte(byte(len(slot.RaceIDList)))
		for _, rID := range slot.RaceIDList {
			name, lookType, head, body, legs, feet, addons := g.getPreyMonsterInfo(rID)
			w.AddString(name)
			w.AddU16(lookType)
			w.AddByte(head)
			w.AddByte(body)
			w.AddByte(legs)
			w.AddByte(feet)
			w.AddByte(addons)
		}

	case game.PreyDataState_ListSelection:
		w.AddU16(0) // count of bestiary list
	}

	// Remaining seconds for free reroll
	freeRerollSecs := uint32(0)
	now := time.Now().Unix()
	if slot.FreeRerollTimeStamp > now {
		freeRerollSecs = uint32(slot.FreeRerollTimeStamp - now)
	}
	w.AddU32(freeRerollSecs)
	w.AddByte(slot.Option)

	g.SendToClient(w)
}

// SendPreyPrices sends Opcode 0xE9 (Prey Prices and Card Costs).
func (g *GameProtocol) SendPreyPrices() {
	if g.player == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0xE9)
	w.AddU32(g.player.GetPreyRerollPrice())
	w.AddByte(5) // Bonus Reroll Price (wildcards)
	w.AddByte(5) // Selection List Price (wildcards)

	w.AddU32(g.player.GetTaskHuntingRerollPrice())
	w.AddU32(g.player.GetTaskHuntingRerollPrice())
	w.AddByte(2) // Task Selection List Price
	w.AddByte(1) // Task Bonus Reroll Price

	g.SendToClient(w)
}
