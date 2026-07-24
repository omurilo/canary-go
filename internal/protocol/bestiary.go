package protocol

import (
	"sort"

	"github.com/opentibiabr/canary-go/internal/bestiary"
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

// SendBestiaryCharms sends the charms window (0xD8). The charm-definition list
// is not modelled yet, so this sends a valid empty list (reset cost, 0 charms,
// available slots, no assignable monsters) — enough for the bestiary window to
// open with an empty Charms tab. Mirrors the >= 1410 layout of
// ProtocolGame::sendBestiaryCharms. Full charm defs/assignment is a follow-up.
func (g *GameProtocol) SendBestiaryCharms() {
	if g.player == nil {
		return
	}
	level := uint64(g.player.Level)
	resetCost := uint64(100000)
	if level > 100 {
		resetCost += level * 11000
	}
	if g.player.CharmExpansion {
		resetCost = resetCost * 75 / 100
	}
	w := netmsg.NewWriter()
	w.AddByte(0xD8)
	w.AddU64(resetCost)
	w.AddByte(0) // charm count (charm definitions not modelled yet)
	w.AddByte(2) // available charm slots (non-premium default)
	w.AddU16(0)  // finished monsters available for assignment
	g.SendToClient(w)
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
