package protocol

import (
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/bosstiary"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// SendBosstiaryCooldownTimer sends the per-boss fight cooldowns (0xBD): u16
// count, then per boss u32 raceId + u64 seconds remaining. Read from the
// player's boss cooldown store (keyed by boss name). Sent on login and mirrors
// ProtocolGame::sendBosstiaryCooldownTimer; empty (count 0) when no cooldowns.
func (g *GameProtocol) SendBosstiaryCooldownTimer() {
	if g.player == nil || g.deps == nil || g.deps.World == nil || g.deps.World.TypeRegistry == nil {
		return
	}
	now := time.Now().Unix()
	type entry struct {
		raceID  uint16
		seconds uint64
	}
	var entries []entry
	for raceID, mt := range g.deps.World.TypeRegistry.BosstiaryMonsters() {
		if ts := g.player.GetBossCooldown(mt.Name); ts > now {
			entries = append(entries, entry{raceID, uint64(ts - now)})
		}
	}
	w := netmsg.NewWriter()
	w.AddByte(0xBD)
	w.AddU16(uint16(len(entries)))
	for _, e := range entries {
		w.AddU32(uint32(e.raceID))
		w.AddU64(e.seconds)
	}
	g.SendToClient(w)
}

// SendBosstiaryEntryChanged notifies the client that a boss's cyclopedia entry
// changed (new level/points) so it refreshes that entry. Mirrors
// ProtocolGame::sendBosstiaryEntryChanged (0xE6 + u32 bossId).
func (g *GameProtocol) SendBosstiaryEntryChanged(bossID uint32) {
	w := netmsg.NewWriter()
	w.AddByte(0xE6)
	w.AddU32(bossID)
	g.SendToClient(w)
}

// SendBosstiaryData sends the static Boss Cyclopedia rules: the kill thresholds
// and points for each rarity (Bane/Archfoe/Nemesis) at each level
// (Prowess/Expertise/Mastery). Mirrors ProtocolGame::sendBosstiaryData
// (0x61 + 9 u16 kill thresholds + 9 u16 point rewards); layout matches the
// otclient parseBosstiaryData reader exactly.
func (g *GameProtocol) SendBosstiaryData() {
	g.SendToClient(buildBosstiaryData())
}

// SendBosstiaryInfo sends the Boss Cyclopedia boss list (0x73): every boss with
// this player's kill count and unlock category. Mirrors the 0x73 block of
// ProtocolGame::parseSendBosstiary; layout matches otclient parseBosstiaryInfo
// (u16 count, then per boss: u32 raceId, u8 race, u32 kills, u8 reserved, u8
// tracker — the tracker flag is read at protocol >= 1320).
// bossListEntry is one boss row in the cyclopedia list (0x73).
type bossListEntry struct {
	RaceID  uint16
	Race    uint8
	Kills   uint32
	Tracked bool
}

func (g *GameProtocol) SendBosstiaryInfo() {
	if g.player == nil || g.deps == nil || g.deps.World == nil || g.deps.World.TypeRegistry == nil {
		return
	}
	bosses := g.deps.World.TypeRegistry.BosstiaryMonsters()
	entries := make([]bossListEntry, 0, len(bosses))
	for raceID, mt := range bosses {
		entries = append(entries, bossListEntry{
			RaceID: raceID,
			Race:   uint8(mt.BosstiaryRace),
			Kills:  g.player.GetBestiaryKillCount(raceID),
		})
	}
	g.SendToClient(buildBosstiaryInfo(entries))
}

// buildBosstiaryInfo builds the 0x73 Boss Cyclopedia list packet.
func buildBosstiaryInfo(entries []bossListEntry) *netmsg.Writer {
	w := netmsg.NewWriter()
	w.AddByte(0x73)
	w.AddU16(uint16(len(entries)))
	for _, e := range entries {
		w.AddU32(uint32(e.RaceID))
		w.AddByte(e.Race)
		w.AddU32(e.Kills)
		w.AddByte(0) // reserved
		if e.Tracked {
			w.AddByte(1)
		} else {
			w.AddByte(0)
		}
	}
	return w
}

// bosstiarySlotView is one filled prowess slot's display data.
type bosstiarySlotView struct {
	filled      bool
	bossID      uint32
	race        uint8
	kills       uint32
	lootBonus   uint16
	killBonus   uint8
	removePrice uint32
	inactive    bool
}

// bosstiarySlotsView is everything the 0x62 slots packet needs.
type bosstiarySlotsView struct {
	playerPoints        uint32
	pointsNextBonus     uint32
	currentBonus        uint16
	slotOneUnlocked     bool
	slotOne             bosstiarySlotView
	slotTwoUnlocked     bool
	slotTwoLockPoints   uint32 // shown as the u32 when slot two is still locked
	slotTwo             bosstiarySlotView
	todayUnlocked       bool
	boostedBossID       uint32
	todaySlot           bosstiarySlotView
	unlocked            []bossListEntry // bosses selectable into a slot (raceId, race)
}

func writeBosstiarySlot(w *netmsg.Writer, s bosstiarySlotView) {
	w.AddByte(s.race)
	w.AddU32(s.kills)
	w.AddU16(s.lootBonus)
	w.AddByte(s.killBonus)
	w.AddByte(s.race) // race repeated (client reads it twice)
	if s.inactive {
		w.AddU32(0)
	} else {
		w.AddU32(s.removePrice)
	}
	if s.inactive {
		w.AddByte(1)
	} else {
		w.AddByte(0)
	}
}

// buildBosstiarySlots builds the 0x62 prowess-slots packet. Layout matches
// otclient parseBosstiarySlots.
func buildBosstiarySlots(v bosstiarySlotsView) *netmsg.Writer {
	w := netmsg.NewWriter()
	w.AddByte(0x62)
	w.AddU32(v.playerPoints)
	w.AddU32(v.pointsNextBonus)
	w.AddU16(v.currentBonus)
	w.AddU16(v.currentBonus + 1)

	w.AddByte(boolByte(v.slotOneUnlocked))
	if v.slotOneUnlocked {
		w.AddU32(v.slotOne.bossID)
	} else {
		w.AddU32(0)
	}
	if v.slotOneUnlocked && v.slotOne.filled {
		writeBosstiarySlot(w, v.slotOne)
	}

	w.AddByte(boolByte(v.slotTwoUnlocked))
	if v.slotTwoUnlocked {
		w.AddU32(v.slotTwo.bossID)
	} else {
		w.AddU32(v.slotTwoLockPoints)
	}
	if v.slotTwoUnlocked && v.slotTwo.filled {
		writeBosstiarySlot(w, v.slotTwo)
	}

	w.AddByte(boolByte(v.todayUnlocked))
	w.AddU32(v.boostedBossID)
	if v.todayUnlocked && v.boostedBossID != 0 {
		writeBosstiarySlot(w, v.todaySlot)
	}

	w.AddByte(boolByte(len(v.unlocked) != 0))
	if len(v.unlocked) != 0 {
		w.AddU16(uint16(len(v.unlocked)))
		for _, e := range v.unlocked {
			w.AddU32(uint32(e.RaceID))
			w.AddByte(e.Race)
		}
	}
	return w
}

// parseBosstiarySlot handles recv 0xB0: set or remove a boss in a prowess slot.
// Payload: u8 slotId, u32 selectedBossId (0 = remove). Removing a slotted boss
// costs RemoveBossPrice gold (first removal free) and bumps the removal counter.
// Mirrors Game::playerBosstiarySlot.
func (g *GameProtocol) parseBosstiarySlot(r *netmsg.Reader) {
	slotID := r.GetByte()
	selectedBossID := r.GetU32()
	if g.player == nil {
		return
	}
	if g.player.IsUIExhausted(250) {
		g.sendCancelMessage("You are exhausted.")
		return
	}
	g.player.UpdateUIExhausted()

	current := g.player.GetSlotBossId(slotID)
	if selectedBossID == 0 && current != 0 {
		// (Boosted-boss removal is free in C++, but we don't model a boosted boss
		// yet, so the removal always charges.)
		price := uint64(bosstiary.RemoveBossPrice(g.player.GetRemoveTimes()))
		if price > 0 {
			if g.player.GetMoney()+g.player.BankBalance < price {
				g.sendCancelMessage("You do not have enough money.")
				return
			}
			g.player.RemoveMoney(price, true)
		}
		g.player.AddRemoveTime()
	}
	g.player.SetSlotBossId(slotID, selectedBossID)

	// Refresh the slots view.
	g.SendBosstiaryData()
	g.SendBosstiarySlots()
}

// SendBosstiarySlots gathers the player's prowess-slot state and sends 0x62.
func (g *GameProtocol) SendBosstiarySlots() {
	if g.player == nil || g.deps == nil || g.deps.World == nil || g.deps.World.TypeRegistry == nil {
		return
	}
	reg := g.deps.World.TypeRegistry
	points := g.player.GetBossPoints()
	currentBonus := bosstiary.CalculateLootBonus(points)
	removePrice := bosstiary.RemoveBossPrice(g.player.GetRemoveTimes())

	// Today's boosted boss (resolved from its name). It gets its own "today"
	// slot and is excluded from the regular unlocked list (mirrors C++).
	boostedBossID := uint32(0)
	if bb := g.deps.World.GetBoostedBoss(); bb != "" {
		if mt := reg.Monsters[strings.ToLower(bb)]; mt != nil && mt.IsBoss() {
			boostedBossID = uint32(mt.BosstiaryRaceID)
		}
	}

	// Unlocked bosses = every boss the player has reached level >= 1 on,
	// excluding the boosted boss (it occupies the today slot).
	var unlocked []bossListEntry
	for raceID, mt := range reg.BosstiaryMonsters() {
		if uint32(raceID) == boostedBossID {
			continue
		}
		if bosstiary.Level(mt.BosstiaryRace, g.player.GetBestiaryKillCount(raceID)) >= 1 {
			unlocked = append(unlocked, bossListEntry{RaceID: raceID, Race: uint8(mt.BosstiaryRace)})
		}
	}

	view := bosstiarySlotsView{
		playerPoints:      points,
		pointsNextBonus:   bosstiary.CalculateBossPoints(currentBonus + 1),
		currentBonus:      currentBonus,
		slotOneUnlocked:   len(unlocked) > 0,
		slotTwoUnlocked:   points >= 1500,
		slotTwoLockPoints: 1500,
		todayUnlocked:     true, // config boostedBossSlot defaults true
		boostedBossID:     boostedBossID,
	}

	// Today slot: the boosted boss with its fixed boosted bonuses (config
	// defaults boostedBossLootBonus=250, bosstiaryKillMultiplier=1 +
	// boostedBossKillBonus=3).
	if boostedBossID != 0 {
		if mt := reg.MonsterByBossRaceID(uint16(boostedBossID)); mt != nil {
			view.todaySlot = bosstiarySlotView{
				filled:    true,
				bossID:    boostedBossID,
				race:      uint8(mt.BosstiaryRace),
				kills:     g.player.GetBestiaryKillCount(uint16(boostedBossID)),
				lootBonus: 250,
				killBonus: 4,
			}
		}
	}

	slotFor := func(bossID uint32) bosstiarySlotView {
		mt := reg.MonsterByBossRaceID(uint16(bossID))
		if mt == nil {
			return bosstiarySlotView{}
		}
		kills := g.player.GetBestiaryKillCount(uint16(bossID))
		level := bosstiary.Level(mt.BosstiaryRace, kills)
		bonus := currentBonus
		if level == 3 {
			bonus += 25 // mastery gives +25% loot bonus on the slotted boss
		}
		return bosstiarySlotView{
			filled: true, bossID: bossID, race: uint8(mt.BosstiaryRace),
			kills: kills, lootBonus: bonus, removePrice: removePrice,
		}
	}

	usedFromUnlocked := 0
	if view.slotOneUnlocked && g.player.GetSlotBossId(1) != 0 {
		view.slotOne = slotFor(g.player.GetSlotBossId(1))
		usedFromUnlocked++
	}
	if view.slotTwoUnlocked && g.player.GetSlotBossId(2) != 0 {
		view.slotTwo = slotFor(g.player.GetSlotBossId(2))
		usedFromUnlocked++
	}

	// The selectable list excludes bosses already placed in a slot.
	slotted := map[uint16]bool{
		uint16(g.player.GetSlotBossId(1)): g.player.GetSlotBossId(1) != 0,
		uint16(g.player.GetSlotBossId(2)): g.player.GetSlotBossId(2) != 0,
	}
	filtered := unlocked[:0]
	for _, e := range unlocked {
		if slotted[e.RaceID] {
			continue
		}
		filtered = append(filtered, e)
	}
	view.unlocked = filtered
	_ = usedFromUnlocked

	if g.deps.Log != nil {
		g.deps.Log.Info("bosstiary slots",
			"points", points, "currentBonus", currentBonus,
			"unlockedCount", len(unlocked), "slotOneUnlocked", view.slotOneUnlocked,
			"slotTwoUnlocked", view.slotTwoUnlocked, "boostedBossID", boostedBossID,
			"totalBosses", len(reg.BosstiaryMonsters()))
	}

	g.SendToClient(buildBosstiarySlots(view))
}

// buildBosstiaryData builds the 0x61 Boss Cyclopedia rules packet.
func buildBosstiaryData() *netmsg.Writer {
	w := netmsg.NewWriter()
	w.AddByte(0x61)
	order := []bosstiary.Rarity{bosstiary.RarityBane, bosstiary.RarityArchfoe, bosstiary.RarityNemesis}
	// First all kill thresholds (rarity-major, level-minor), then all points.
	for _, r := range order {
		s, _ := bosstiary.Stages(r)
		for _, stage := range s {
			w.AddU16(uint16(stage.Kills))
		}
	}
	for _, r := range order {
		s, _ := bosstiary.Stages(r)
		for _, stage := range s {
			w.AddU16(stage.Points)
		}
	}
	return w
}
