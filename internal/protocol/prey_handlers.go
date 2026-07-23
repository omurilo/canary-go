package protocol

import (
	"time"

	"github.com/opentibiabr/canary-go/internal/config"
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

	// Fallback map for common Tibia monsters
	switch raceID {
	case 5:
		return "Orc", 5, 0, 0, 0, 0, 0
	case 10:
		return "Spider", 30, 0, 0, 0, 0, 0
	case 12:
		return "Snake", 28, 0, 0, 0, 0, 0
	case 13:
		return "Elven Scout", 64, 0, 0, 0, 0, 0
	case 14:
		return "Amazon", 63, 0, 0, 0, 0, 0
	case 15:
		return "Troll", 15, 0, 0, 0, 0, 0
	case 21:
		return "Minotaur", 25, 0, 0, 0, 0, 0
	case 22:
		return "Cyclops", 22, 0, 0, 0, 0, 0
	case 23:
		return "Minotaur Archer", 24, 0, 0, 0, 0, 0
	case 24:
		return "Minotaur Mage", 23, 0, 0, 0, 0, 0
	case 25:
		return "Orc Warrior", 7, 0, 0, 0, 0, 0
	case 26:
		return "Rotworm", 26, 0, 0, 0, 0, 0
	case 27:
		return "Orc Spearman", 6, 0, 0, 0, 0, 0
	case 28:
		return "Orc Shaman", 8, 0, 0, 0, 0, 0
	case 29:
		return "Orc Berserker", 9, 0, 0, 0, 0, 0
	case 30:
		return "Orc Leader", 59, 0, 0, 0, 0, 0
	case 31:
		return "Orc Warlord", 2, 0, 0, 0, 0, 0
	case 32:
		return "Dragon Hatchling", 287, 0, 0, 0, 0, 0
	case 33:
		return "Skeleton", 33, 0, 0, 0, 0, 0
	case 34:
		return "Dragon", 34, 0, 0, 0, 0, 0
	case 35:
		return "Demon", 35, 0, 0, 0, 0, 0
	case 36:
		return "Monk", 57, 0, 0, 0, 0, 0
	case 37:
		return "Hero", 73, 0, 0, 0, 0, 0
	case 38:
		return "Giant Spider", 38, 0, 0, 0, 0, 0
	case 39:
		return "Black Knight", 131, 0, 0, 0, 0, 0
	case 40:
		return "Crypt Shambler", 100, 0, 0, 0, 0, 0
	case 41:
		return "Mummy", 65, 0, 0, 0, 0, 0
	case 42:
		return "Ghoul", 18, 0, 0, 0, 0, 0
	case 43:
		return "Ghost", 48, 0, 0, 0, 0, 0
	case 44:
		return "Vampire", 68, 0, 0, 0, 0, 0
	case 45:
		return "Bonelord", 17, 0, 0, 0, 0, 0
	case 48:
		return "Dragon Lord", 39, 0, 0, 0, 0, 0
	case 49:
		return "Fire Devil", 40, 0, 0, 0, 0, 0
	case 50:
		return "Lion", 125, 0, 0, 0, 0, 0
	case 51:
		return "Bear", 16, 0, 0, 0, 0, 0
	case 52:
		return "Wolf", 27, 0, 0, 0, 0, 0
	case 53:
		return "War Wolf", 243, 0, 0, 0, 0, 0
	case 54:
		return "Dwarf", 66, 0, 0, 0, 0, 0
	case 55:
		return "Behemoth", 55, 0, 0, 0, 0, 0
	case 56:
		return "Dwarf Guard", 70, 0, 0, 0, 0, 0
	case 57:
		return "Dwarf Soldier", 67, 0, 0, 0, 0, 0
	case 60:
		return "Stone Golem", 69, 0, 0, 0, 0, 0
	case 61:
		return "Fire Elemental", 49, 0, 0, 0, 0, 0
	case 62:
		return "Water Elemental", 11, 0, 0, 0, 0, 0
	case 63:
		return "Earth Elemental", 12, 0, 0, 0, 0, 0
	case 64:
		return "Energy Elemental", 13, 0, 0, 0, 0, 0
	case 70:
		return "Necromancer", 91, 0, 0, 0, 0, 0
	case 80:
		return "Hydra", 121, 0, 0, 0, 0, 0
	case 85:
		return "Serpent Spawn", 220, 0, 0, 0, 0, 0
	case 90:
		return "Medusa", 330, 0, 0, 0, 0, 0
	case 95:
		return "Wyrm", 291, 0, 0, 0, 0, 0
	case 100:
		return "Grim Reaper", 300, 0, 0, 0, 0, 0
	case 110:
		return "Hellhound", 240, 0, 0, 0, 0, 0
	case 120:
		return "Dark Torturer", 234, 0, 0, 0, 0, 0
	case 130:
		return "Destroyer", 236, 0, 0, 0, 0, 0
	case 140:
		return "Nightstalker", 320, 0, 0, 0, 0, 0
	case 150:
		return "Nightmare", 245, 0, 0, 0, 0, 0
	case 160:
		return "Plaguesmith", 238, 0, 0, 0, 0, 0
	case 180:
		return "Spectre", 235, 0, 0, 0, 0, 0
	case 200:
		return "Undead Dragon", 231, 0, 0, 0, 0, 0
	default:
		return "Monster", 35, 0, 0, 0, 0, 0
	}
}

func (g *GameProtocol) getAllBestiaryRaceIDs() []uint16 {
	ids := []uint16{}
	if g.deps != nil && g.deps.World != nil && g.deps.World.TypeRegistry != nil {
		for _, m := range g.deps.World.TypeRegistry.Monsters {
			if m != nil && m.RaceID != 0 {
				ids = append(ids, m.RaceID)
			}
		}
	}
	if len(ids) == 0 {
		return []uint16{
			5, 10, 12, 13, 14, 15, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
			41, 42, 43, 44, 45, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 60, 61, 62, 63, 64, 70, 80, 85, 90, 95, 100, 110, 120, 130, 140, 150, 160, 180, 200,
		}
	}
	return ids
}

func addPreyOutfit(w *netmsg.Writer, lookType uint16, head, body, legs, feet, addons byte) {
	w.AddU16(lookType)
	if lookType == 0 {
		w.AddU16(0) // lookTypeEx
		return
	}
	w.AddByte(head)
	w.AddByte(body)
	w.AddByte(legs)
	w.AddByte(feet)
	w.AddByte(addons)
}

func (g *GameProtocol) reloadPreyGrid(slot *game.PreySlot) {
	allIDs := g.getAllBestiaryRaceIDs()
	if len(allIDs) < 9 {
		slot.ReloadMonsterGrid()
		return
	}

	shuffled := make([]uint16, len(allIDs))
	copy(shuffled, allIDs)

	seed := time.Now().UnixNano()
	for i := len(shuffled) - 1; i > 0; i-- {
		seed = seed*1103515245 + 12345
		j := int((seed>>16)&0x7FFF) % (i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	slot.RaceIDList = make([]uint16, 9)
	copy(slot.RaceIDList, shuffled[:9])
}

// parsePreyAction handles Opcode 0xEB (Prey Action / Reroll / Selection).
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
	case 2: // PreyAction_MonsterSelection
		index = int8(r.GetByte())
	case 4: // PreyAction_ListAll_Selection
		raceID = r.GetU16()
	case 5: // PreyAction_Option
		option = r.GetByte()
	}

	prey := g.player.GetPrey()
	slot := prey.GetSlot(slotID)
	if slot == nil {
		return
	}

	switch action {
	case 0: // PreyAction_ListReroll
		g.reloadPreyGrid(slot)
		slot.SelectedRaceID = 0
		slot.State = game.PreyDataState_Selection
	case 1: // PreyAction_BonusReroll
		if slot.BonusRarity == 0 {
			slot.BonusRarity = 5
		} else if slot.BonusRarity < 10 {
			slot.BonusRarity++
		}
		slot.Bonus = game.PreyBonusType((uint8(time.Now().UnixNano()) + slot.ID) % 4)
		slot.BonusPercentage = game.PreyBonusPercentage(slot.Bonus, slot.BonusRarity)
		slot.BonusTimeLeft = 7200
	case 2: // PreyAction_MonsterSelection
		if len(slot.RaceIDList) == 0 {
			g.reloadPreyGrid(slot)
		}
		if index >= 0 && int(index) < len(slot.RaceIDList) {
			slot.SelectedRaceID = slot.RaceIDList[index]
			slot.State = game.PreyDataState_Active
			slot.BonusTimeLeft = 7200
			if slot.BonusPercentage == 0 {
				slot.Bonus = game.PreyBonusType((uint8(time.Now().UnixNano()) + slot.ID) % 4)
				slot.BonusRarity = 5
				slot.BonusPercentage = game.PreyBonusPercentage(slot.Bonus, slot.BonusRarity)
			}
		}
	case 3: // PreyAction_ListAll_Cards
		slot.State = game.PreyDataState_ListSelection
	case 4: // PreyAction_ListAll_Selection
		slot.SelectedRaceID = raceID
		slot.State = game.PreyDataState_Active
		slot.BonusTimeLeft = 7200
		if slot.BonusPercentage == 0 {
			slot.Bonus = game.PreyBonusType((uint8(time.Now().UnixNano()) + slot.ID) % 4)
			slot.BonusRarity = 5
			slot.BonusPercentage = game.PreyBonusPercentage(slot.Bonus, slot.BonusRarity)
		}
	case 5: // PreyAction_Option
		slot.Option = option
	}

	g.SendPreyPrices()
	g.SendAllPreyData()
}

// SendPreyData sends Opcode 0xE8 for a single Prey Slot.
func (g *GameProtocol) SendPreyData(slot *game.PreySlot) {
	if g.player == nil || slot == nil {
		return
	}

	if len(slot.RaceIDList) == 0 {
		g.reloadPreyGrid(slot)
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
		addPreyOutfit(w, lookType, head, body, legs, feet, addons)

		w.AddByte(byte(slot.Bonus))
		w.AddU16(slot.BonusPercentage)
		w.AddByte(slot.BonusRarity)
		w.AddU16(slot.BonusTimeLeft)

	case game.PreyDataState_Selection:
		w.AddByte(byte(len(slot.RaceIDList)))
		for _, rID := range slot.RaceIDList {
			name, lookType, head, body, legs, feet, addons := g.getPreyMonsterInfo(rID)
			w.AddString(name)
			addPreyOutfit(w, lookType, head, body, legs, feet, addons)
		}

	case game.PreyDataState_SelectionChangeMonster:
		w.AddByte(byte(slot.Bonus))
		w.AddU16(slot.BonusPercentage)
		w.AddByte(slot.BonusRarity)
		w.AddByte(byte(len(slot.RaceIDList)))
		for _, rID := range slot.RaceIDList {
			name, lookType, head, body, legs, feet, addons := g.getPreyMonsterInfo(rID)
			w.AddString(name)
			addPreyOutfit(w, lookType, head, body, legs, feet, addons)
		}

	case game.PreyDataState_ListSelection:
		allIDs := g.getAllBestiaryRaceIDs()
		w.AddU16(uint16(len(allIDs)))
		for _, rID := range allIDs {
			w.AddU16(rID)
		}
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

// SendAllPreyData sends Opcode 0xE8 for all 3 slots and resource balance.
func (g *GameProtocol) SendAllPreyData() {
	if g.player == nil {
		return
	}
	prey := g.player.GetPrey()
	for i := byte(0); i < 3; i++ {
		slot := prey.GetSlot(i)
		if slot != nil {
			g.SendPreyData(slot)
		}
	}
	g.sendResourceBalance(0x02, uint64(g.player.GetPreyCards()))
}

// SendPreyPrices sends Opcode 0xE9 (Prey Prices and Card Costs).
func (g *GameProtocol) SendPreyPrices() {
	if g.player == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0xE9)
	w.AddU32(g.player.GetPreyRerollPrice())
	w.AddByte(byte(config.Number("preyBonusRerollPrice", 1))) // bonus reroll (wildcards)
	w.AddByte(byte(config.Number("preySelectListPrice", 5)))  // selection list (wildcards)

	w.AddU32(g.player.GetTaskHuntingRerollPrice())
	w.AddU32(g.player.GetTaskHuntingRerollPrice())
	w.AddByte(byte(config.Number("taskHuntingSelectListPrice", 5)))  // task selection list
	w.AddByte(byte(config.Number("taskHuntingBonusRerollPrice", 1))) // task bonus reroll

	g.SendToClient(w)
}
