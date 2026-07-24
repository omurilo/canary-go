package protocol

import (
	"sort"

	"github.com/opentibiabr/canary-go/internal/bestiary"
	"github.com/opentibiabr/canary-go/internal/charms"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

func bestiaryThresholds(mt *creatures.MonsterType) bestiary.Thresholds {
	return bestiary.Thresholds{
		FirstUnlock:  mt.BestiaryFirstUnlock,
		SecondUnlock: mt.BestiarySecondUnlock,
		ToKill:       mt.BestiaryToKill,
	}
}

// lootDifficulty maps a loot drop chance (out of 100000) to a difficulty tier
// 0..4. Mirrors IOBestiary::calculateDifficult.
func lootDifficulty(chance uint32) uint8 {
	pct := float64(chance) / 1000.0
	switch {
	case pct < 0.2:
		return 4
	case pct < 1:
		return 3
	case pct < 5:
		return 2
	case pct < 25:
		return 1
	default:
		return 0
	}
}

// parseBestiarySendCreatures handles recv 0xE2 and replies with the 0xD6
// overview: the monsters of a bestiary class (search==else) or a set of race
// ids (search==1), each with the player's progress. Mirrors
// ProtocolGame::parseBestiarySendCreatures.
func (g *GameProtocol) parseBestiarySendCreatures(r *netmsg.Reader) {
	if g.player == nil || g.deps == nil || g.deps.World == nil || g.deps.World.TypeRegistry == nil {
		return
	}
	reg := g.deps.World.TypeRegistry
	search := r.GetByte()

	var text string
	var monsters []*creatures.MonsterType
	if search == 1 {
		amount := int(r.GetU16())
		for i := 0; i < amount; i++ {
			raceID := r.GetU16()
			if g.player.GetBestiaryKillCount(raceID) > 0 {
				if mt := reg.MonsterByRaceID(raceID); mt != nil {
					monsters = append(monsters, mt)
				}
			}
		}
	} else {
		text = r.GetString()
		for _, mt := range reg.Monsters {
			if mt.RaceID > 0 && mt.BestiaryToKill > 0 && mt.BestiaryClass == text {
				monsters = append(monsters, mt)
			}
		}
		if len(monsters) == 0 {
			return
		}
	}
	sort.Slice(monsters, func(i, j int) bool { return monsters[i].RaceID < monsters[j].RaceID })

	w := netmsg.NewWriter()
	w.AddByte(0xD6)
	w.AddString(text)
	w.AddU16(uint16(len(monsters)))
	for _, mt := range monsters {
		w.AddU16(mt.RaceID)
		if kills := g.player.GetBestiaryKillCount(mt.RaceID); kills > 0 {
			w.AddByte(bestiary.KillStatus(kills, bestiaryThresholds(mt)))
			w.AddByte(mt.BestiaryOccurrence)
		} else {
			w.AddByte(0)
		}
		w.AddU16(0) // creature animus mastery bonus (client reads at >= 1340)
	}
	w.AddU16(0) // animus mastery points (>= 1340)
	g.SendToClient(w)
}

// parseBestiaryMonsterData handles recv 0xE3 and replies with the 0xD7 detail:
// progress, kill thresholds, difficulty, loot (gated by unlock level), and —
// once unlocked — combat stats. Mirrors ProtocolGame::parseBestiarysendMonsterData.
func (g *GameProtocol) parseBestiaryMonsterData(r *netmsg.Reader) {
	if g.player == nil || g.deps == nil || g.deps.World == nil || g.deps.World.TypeRegistry == nil {
		return
	}
	raceID := r.GetU16()
	mt := g.deps.World.TypeRegistry.MonsterByRaceID(raceID)
	if mt == nil {
		return
	}
	th := bestiaryThresholds(mt)
	kills := g.player.GetBestiaryKillCount(raceID)
	level := bestiary.KillStatus(kills, th)

	w := netmsg.NewWriter()
	w.AddByte(0xD7)
	w.AddU16(raceID)
	w.AddString(mt.BestiaryClass)
	w.AddByte(level)
	w.AddU16(0) // animus mastery bonus
	w.AddU16(0) // animus mastery points
	w.AddU32(kills)
	w.AddU16(uint16(mt.BestiaryFirstUnlock))
	w.AddU16(uint16(mt.BestiarySecondUnlock))
	w.AddU16(uint16(mt.BestiaryToKill))
	w.AddByte(mt.BestiaryStars)
	w.AddByte(mt.BestiaryOccurrence)

	w.AddByte(uint8(len(mt.Loot)))
	for _, loot := range mt.Loot {
		diff := lootDifficulty(loot.Chance)
		show := false
		switch level {
		case 2:
			show = diff < 2
		case 3:
			show = diff < 3
		case 4:
			show = true
		}
		if show {
			w.AddU16(loot.ID)
		} else {
			w.AddU16(0)
		}
		w.AddByte(diff)
		w.AddByte(0) // special-event flag
		if show {
			w.AddString(loot.Name)
			if loot.CountMax > 0 {
				w.AddByte(1)
			} else {
				w.AddByte(0)
			}
		}
	}

	if level > 1 {
		w.AddU16(mt.BestiaryCharmsPoints)
		attackMode := uint8(0)
		if !mt.Flags.Hostile {
			attackMode = 2
		} else if mt.TargetDistance > 0 {
			attackMode = 1
		}
		w.AddByte(attackMode)
		w.AddByte(0x02)
		w.AddU32(mt.MaxHealth)
		w.AddU32(uint32(mt.Experience))
		w.AddU16(uint16(mt.Speed))
		w.AddU16(0)      // armor (not modelled)
		w.AddDouble(0, 3) // mitigation (not modelled)
	}

	if level > 2 {
		w.AddByte(0)  // elements count (resistances not modelled)
		w.AddU16(1)   // "1" difficulty/location marker (C++ constant)
		w.AddString("") // bestiary locations text (not modelled)
	}

	g.SendToClient(w)
}

// bestyRaceLast is BESTY_RACE_INKBORN (creatures_definitions.hpp) — the highest
// bestiary race id and the count of race entries the races packet carries.
const bestyRaceLast = 21

// SendBestiaryRaces sends the bestiary class list (0xD5): u16 race count, then
// per race (1..BESTY_RACE_LAST) the class name, total monsters and how many the
// player has unlocked (>=1 kill). Mirrors ProtocolGame::parseBestiarySendRaces;
// layout matches otclient parseBestiaryRaces.
func (g *GameProtocol) SendBestiaryRaces() {
	if g.player == nil || g.deps == nil || g.deps.World == nil || g.deps.World.TypeRegistry == nil {
		return
	}
	type agg struct {
		class    string
		total    uint16
		unlocked uint16
	}
	byRace := make(map[uint8]*agg)
	for _, mt := range g.deps.World.TypeRegistry.Monsters {
		if mt.RaceID == 0 || mt.BestiaryToKill == 0 || mt.BestiaryRace == 0 {
			continue
		}
		a := byRace[mt.BestiaryRace]
		if a == nil {
			a = &agg{}
			byRace[mt.BestiaryRace] = a
		}
		a.class = mt.BestiaryClass
		a.total++
		if g.player.GetBestiaryKillCount(mt.RaceID) > 0 {
			a.unlocked++
		}
	}

	w := netmsg.NewWriter()
	w.AddByte(0xD5)
	w.AddU16(bestyRaceLast)
	for i := uint8(1); i <= bestyRaceLast; i++ {
		a := byRace[i]
		if a == nil {
			a = &agg{}
		}
		w.AddString(a.class)
		w.AddU16(a.total)
		w.AddU16(a.unlocked)
	}
	g.SendToClient(w)
}

// charmResetAllCost returns the gold cost to reset all charm assignments:
// 100000 + level*11000 above level 100, discounted 25% with charm expansion.
func (g *GameProtocol) charmResetAllCost() uint64 {
	level := uint64(g.player.Level)
	cost := uint64(100000)
	if level > 100 {
		cost += level * 11000
	}
	if g.player.CharmExpansion {
		cost = cost * 75 / 100
	}
	return cost
}

// charmRemoveCost returns the gold cost to clear one charm's monster
// assignment: level*100, discounted 25% with charm expansion.
func (g *GameProtocol) charmRemoveCost() uint32 {
	cost := uint32(g.player.Level) * 100
	if g.player.CharmExpansion {
		cost = cost * 75 / 100
	}
	return cost
}

// buildBestiaryCharms builds the 0xD8 charms window: reset cost, every charm
// with its unlock tier / monster assignment / remove cost, the available slot
// count, and the set of bestiary-complete monsters still assignable. Mirrors
// ProtocolGame::sendBestiaryCharms (>= 1410 layout).
func (g *GameProtocol) buildBestiaryCharms() *netmsg.Writer {
	reg := g.deps.World.Charms
	w := netmsg.NewWriter()
	w.AddByte(0xD8)
	w.AddU64(g.charmResetAllCost())

	w.AddByte(uint8(reg.Len()))
	for _, c := range reg.List {
		w.AddByte(c.ID)
		if charms.HasBit(int32(g.player.UnlockedRunesBit), c.ID) {
			w.AddByte(g.player.GetCharmTier(c.ID))
			if raceID := g.player.GetCharmRace(c.ID); raceID > 0 {
				w.AddByte(0x01)
				w.AddU16(raceID)
				w.AddU32(g.charmRemoveCost())
			} else {
				w.AddByte(0x00)
			}
		} else {
			w.AddByte(0x00)
			w.AddByte(0x00)
		}
	}

	// Available slots: 0xFF with charm expansion, else (2 - used). No premium
	// concept in the port, so free players get the base 2 slots.
	used := charms.UsedRunes(int32(g.player.UsedRunesBit))
	if g.player.CharmExpansion {
		w.AddByte(0xFF)
	} else {
		total := uint8(2)
		if uint8(len(used)) > total {
			w.AddByte(0)
		} else {
			w.AddByte(total - uint8(len(used)))
		}
	}

	// Finished monsters available for assignment: kills >= second unlock,
	// dropping any race already assigned to 2+ charms.
	assigned := make(map[uint16]int)
	for _, id := range used {
		if raceID := g.player.GetCharmRace(id); raceID > 0 {
			assigned[raceID]++
		}
	}
	var finished []uint16
	for _, mt := range g.deps.World.TypeRegistry.Monsters {
		if mt.RaceID == 0 || mt.BestiarySecondUnlock == 0 {
			continue
		}
		if g.player.GetBestiaryKillCount(mt.RaceID) >= mt.BestiarySecondUnlock {
			if assigned[mt.RaceID] < 2 {
				finished = append(finished, mt.RaceID)
			}
		}
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i] < finished[j] })
	w.AddU16(uint16(len(finished)))
	for _, raceID := range finished {
		w.AddU32(uint32(raceID))
	}
	return w
}

// SendBestiaryCharms sends the charms window (0xD8) followed by the charm
// resource balances. Mirrors ProtocolGame::sendBestiaryCharms.
func (g *GameProtocol) SendBestiaryCharms() {
	if g.player == nil || g.deps == nil || g.deps.World == nil ||
		g.deps.World.Charms == nil || g.deps.World.TypeRegistry == nil {
		return
	}
	g.SendToClient(g.buildBestiaryCharms())
	g.SendCharmResourcesBalance()
}

// charm resource-balance types (CharmResource_t, server_definitions.hpp).
const (
	resourceCharm         = 0x1E
	resourceMinorCharm    = 0x1F
	resourceMaxCharm      = 0x20
	resourceMaxMinorCharm = 0x21
)

// sendCharmResourceBalance sends one 0xEE resource-balance packet.
// Mirrors ProtocolGame::sendCharmResourceBalance.
func (g *GameProtocol) sendCharmResourceBalance(resourceType uint8, value uint32) {
	w := netmsg.NewWriter()
	w.AddByte(0xEE)
	w.AddByte(resourceType)
	w.AddU32(value)
	g.SendToClient(w)
}

// SendCharmResourcesBalance sends the four charm resource balances (points,
// minor echoes, and their lifetime maxima). Mirrors
// ProtocolGame::sendCharmResourcesBalance.
func (g *GameProtocol) SendCharmResourcesBalance() {
	if g.player == nil {
		return
	}
	g.sendCharmResourceBalance(resourceCharm, g.player.CharmPoints)
	g.sendCharmResourceBalance(resourceMinorCharm, g.player.MinorCharmEchoes)
	g.sendCharmResourceBalance(resourceMaxCharm, g.player.MaxCharmPoints)
	g.sendCharmResourceBalance(resourceMaxMinorCharm, g.player.MaxMinorCharmEchoes)
}

// parseSendBuyCharmRune handles recv 0xE4: unlock/upgrade (action 0), assign to
// a monster (1), clear one assignment (2), or reset all (3). Mirrors
// ProtocolGame::parseSendBuyCharmRune -> IOBestiary::sendBuyCharmRune.
func (g *GameProtocol) parseSendBuyCharmRune(r *netmsg.Reader) {
	if g.player == nil || g.deps == nil || g.deps.World == nil ||
		g.deps.World.Charms == nil || g.deps.World.TypeRegistry == nil {
		return
	}
	action := r.GetByte()
	charmID := r.GetByte()
	raceID := r.GetU16()

	c := g.deps.World.Charms.Get(charmID)
	if action != 3 && c == nil {
		return
	}
	p := g.player

	switch action {
	case 0: // unlock or upgrade a tier
		tier := p.GetCharmTier(charmID)
		if tier > 2 {
			p.SendTextMessage(0x14, "Charm at max level.")
			return
		}
		cost := uint32(c.TierCost(tier))
		switch c.Category {
		case charms.CategoryMajor:
			if p.GetCharmPoints() < cost {
				p.SendTextMessage(0x14, "You don't have enough charm points to unlock this rune.")
				return
			}
			p.SpendCharmPoints(cost)
			p.AddMinorCharmEchoes(charms.MinorEchoesGain(tier), false)
			p.SetCharmTier(charmID, tier+1)
		case charms.CategoryMinor:
			if p.GetMinorCharmEchoes() < cost {
				p.SendTextMessage(0x14, "You don't have enough minor charm echoes to unlock this rune.")
				return
			}
			p.AddMinorCharmEchoes(cost, true)
			p.SetCharmTier(charmID, tier+1)
		default:
			return
		}
		p.UnlockedRunesBit = uint32(charms.SetBit(int32(p.UnlockedRunesBit), charmID))

	case 1: // assign the charm to a monster race
		used := charms.UsedRunes(int32(p.UsedRunesBit))
		limit := 2
		if p.CharmExpansion {
			limit = 25
		}
		if limit <= len(used) {
			p.SendTextMessage(0x14, "You don't have any charm slots available.")
			return
		}
		mt := g.deps.World.TypeRegistry.MonsterByRaceID(raceID)
		if mt != nil && c.Category == charms.CategoryMajor {
			if p.GetBestiaryKillCount(raceID) < mt.BestiaryToKill {
				return
			}
		}
		// A monster may only be set on one Major and one Minor charm.
		for _, id := range used {
			if other := g.deps.World.Charms.Get(id); other != nil &&
				other.Category == c.Category && p.GetCharmRace(id) == raceID {
				cat := "Minor"
				if c.Category == charms.CategoryMajor {
					cat = "Major"
				}
				p.SendTextMessage(0x14, "You already have this monster set on another "+cat+" Charm!")
				return
			}
		}
		p.SetCharmRace(charmID, raceID)
		p.UsedRunesBit = uint32(charms.SetBit(int32(p.UsedRunesBit), charmID))

	case 2: // clear one charm's assignment (paid)
		fee := uint64(g.charmRemoveCost())
		if !p.RemoveMoney(fee, true) {
			p.SendTextMessage(0x14, "You don't have enough gold.")
			return
		}
		p.UsedRunesBit = uint32(charms.ClearBit(int32(p.UsedRunesBit), charmID))
		p.SetCharmRace(charmID, 0)

	case 3: // reset all assignments and refund points (paid)
		if !p.RemoveMoney(g.charmResetAllCost(), true) {
			p.SendTextMessage(0x14, "You don't have enough gold.")
			return
		}
		for _, ch := range g.deps.World.Charms.List {
			p.SetCharmRace(ch.ID, 0)
			p.SetCharmTier(ch.ID, 0)
		}
		p.SetCharmPoints(p.MaxCharmPoints)
		p.SetMinorCharmEchoes(0)
		p.MaxMinorCharmEchoes = 0
		p.UsedRunesBit = 0
		p.UnlockedRunesBit = 0
	}

	g.SendBestiaryCharms()
}

// SendBestiaryEntryChanged tells the client a bestiary monster entry changed
// (new unlock stage) so it refreshes it. Mirrors
// ProtocolGame::sendBestiaryEntryChanged (0xD9 + u16 raceid).
func (g *GameProtocol) SendBestiaryEntryChanged(raceID uint16) {
	w := netmsg.NewWriter()
	w.AddByte(0xD9)
	w.AddU16(raceID)
	g.SendToClient(w)
}
