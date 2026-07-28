package protocol

import (
	"fmt"
	"log/slog"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/vocations"
	"github.com/opentibiabr/canary-go/internal/mounts"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

const (
	cyclopediaCharacterInfoBaseInformation = 0
	cyclopediaCharacterInfoGeneralStats    = 1
	cyclopediaCharacterInfoAchievements    = 5
	cyclopediaCharacterInfoItemSummary     = 6
	cyclopediaCharacterInfoOutfitsMounts   = 7
	cyclopediaCharacterInfoStoreSummary    = 8
	cyclopediaCharacterInfoInspection      = 9
	cyclopediaCharacterInfoBadges          = 10
	cyclopediaCharacterInfoTitles          = 11
	cyclopediaCharacterInfoWheel           = 12
	cyclopediaCharacterInfoOffenceStats    = 13
	cyclopediaCharacterInfoDefenceStats    = 14
	cyclopediaCharacterInfoMiscStats       = 15
)

func (g *GameProtocol) parseInspectPlayer(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	action := r.GetByte()
	if action >= 1 && action <= 5 {
		targetID := r.GetU32()
		target := g.findPlayerByID(targetID)
		if target == nil {
			target = g.player
		}
		g.sendCyclopediaCharacterInspection(target)
	} else {
		g.sendCyclopediaCharacterInspection(g.player)
	}
}

func (g *GameProtocol) parseCyclopediaCharacterInfo(r *netmsg.Reader) {
	if g.player == nil {
		slog.Default().Info("parseCyclopediaCharacterInfo: player nil")
		return
	}
	characterID := r.GetU32()
	characterInfoType := r.GetByte()
	slog.Default().Info("parseCyclopediaCharacterInfo", "charID", characterID, "infoType", characterInfoType)
		
		if characterInfoType == 3 || characterInfoType == 4 {
			r.GetU16()
			r.GetU16()
		}
	if characterID == 0 {
		characterID = g.player.ID
	}
	if characterID != g.player.ID {
		g.sendCyclopediaNoData(characterInfoType, 2)
		return
	}

	switch characterInfoType {
	case cyclopediaCharacterInfoBaseInformation: // 0
		g.sendCyclopediaCharacterBaseInformation()
	case cyclopediaCharacterInfoGeneralStats: // 1
		g.sendCyclopediaCharacterGeneralStats()
	case cyclopediaCharacterInfoAchievements: // 5
		g.sendCyclopediaCharacterAchievements(g.player)
	case cyclopediaCharacterInfoItemSummary: // 6
		g.sendCyclopediaCharacterItemSummary()
	case cyclopediaCharacterInfoOutfitsMounts: // 7
		g.sendCyclopediaCharacterOutfitsMounts()
	case cyclopediaCharacterInfoStoreSummary: // 8
		g.sendCyclopediaCharacterStoreSummary()
	case cyclopediaCharacterInfoInspection: // 9
		g.sendCyclopediaCharacterInspection(g.player)
	case cyclopediaCharacterInfoBadges: // 10
		g.sendCyclopediaCharacterBadges()
	case cyclopediaCharacterInfoTitles: // 11
		g.sendCyclopediaCharacterTitles()
	case cyclopediaCharacterInfoWheel: // 12
		g.sendCyclopediaCharacterWheel()
	case cyclopediaCharacterInfoOffenceStats: // 13
		g.sendCyclopediaCharacterOffenceStats()
	case cyclopediaCharacterInfoDefenceStats: // 14
		g.sendCyclopediaCharacterDefenceStats()
	case cyclopediaCharacterInfoMiscStats: // 15
		g.sendCyclopediaCharacterMiscStats()
	default:
		g.sendCyclopediaNoData(characterInfoType, 1)
	}
}

// sendCyclopediaCharacterAchievements sends the player's achievements (cyclopedia tab).
func (g *GameProtocol) sendCyclopediaCharacterAchievements(p *game.Player) {
	if p == nil {
		return
	}
	reg := g.deps.World.Achievements
	if reg == nil {
		g.sendCyclopediaNoData(cyclopediaCharacterInfoAchievements, 1)
		return
	}

	type entry struct {
		ach       *game.Achievement
		timestamp uint32
	}
	entries := make([]entry, 0, len(p.Achievements))
	var totalPoints uint16
	var secretsUnlocked uint16

	for id, ts := range p.Achievements {
		ach := reg.GetByID(uint16(id))
		if ach == nil {
			continue
		}
		entries = append(entries, entry{ach: ach, timestamp: uint32(ts)})
		totalPoints += uint16(ach.Points)
		if ach.Secret {
			secretsUnlocked++
		}
	}

	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoAchievements) // 5
	w.AddByte(0x00)                                 // no error
	w.AddU16(totalPoints)
	w.AddU16(secretsUnlocked)
	w.AddU16(uint16(len(entries)))
	for _, e := range entries {
		w.AddU16(e.ach.ID)
		w.AddU32(e.timestamp)
		if e.ach.Secret {
			w.AddByte(1)
			w.AddString(e.ach.Name)
			w.AddString(e.ach.Description)
			w.AddByte(1) // grade (stars), default 1
		} else {
			w.AddByte(0)
		}
	}
	g.SendToClient(w)
}

// sendCyclopediaNoData sends [0xDA][u8 type][u8 errorCode] — a "no data available" response.
func (g *GameProtocol) sendCyclopediaNoData(infoType uint8, errorCode uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(infoType)
	w.AddByte(errorCode)
	g.SendToClient(w)
}

// sendCyclopediaCharacterInspection sends the 0xDA inspection packet with
// the full item serialization (matching C++ ProtocolGame::sendCyclopediaCharacterInspection).
// Mount is NOT sent in the outfit, matching the C++ addMount=false call.
func (g *GameProtocol) sendCyclopediaCharacterInspection(p *game.Player) {
	if p == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoInspection) // 9
	w.AddByte(0x00)                              // no error

	// Count inventory items first, then write.
	var invItems []*game.Item
	var invSlots []uint8
	for slot := uint8(game.ConstSlotFirst); slot <= uint8(game.ConstSlotLast); slot++ {
		if int(slot) < len(p.Inventory) && p.Inventory[slot] != nil {
			invItems = append(invItems, p.Inventory[slot])
			invSlots = append(invSlots, slot)
		}
	}
	w.AddByte(byte(len(invItems)))
	for i, item := range invItems {
		w.AddByte(invSlots[i])
		name := "Item"
		if t := g.deps.Items.Get(item.ID); t != nil && t.Name != "" {
			name = t.Name
		}
		w.AddString(name)
		g.addItem(w, item)
		w.AddByte(0) // imbuements count
		w.AddByte(0) // descriptions count
	}

	w.AddString(p.Name)
	addOutfitNoMount(w, p.Outfit) // C++ calls AddOutfit(msg, outfit, false)

	// Player descriptions (key-value pairs matching C++ sendCyclopediaCharacterInspection).
	vocName := "unknown"
	if voc := vocations.GetVocation(uint32(p.Vocation)); voc != nil {
		vocName = voc.Name
	}
	descriptions := []struct{ Key, Val string }{
		{"Level", fmt.Sprintf("%d", p.Level)},
		{"Vocation", vocName},
	}
	w.AddByte(byte(len(descriptions)))
	for _, d := range descriptions {
		w.AddString(d.Key)
		w.AddString(d.Val)
	}
	g.SendToClient(w)
}

// sendCyclopediaCharacterBaseInformation sends the character's base info (type 0).
// Matches C++ ProtocolGame::sendCyclopediaCharacterBaseInformation.
func (g *GameProtocol) sendCyclopediaCharacterBaseInformation() {
	if g.player == nil {
		return
	}
	p := g.player
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoBaseInformation)
	w.AddByte(0x00) // no error

	w.AddString(p.Name)

	vocName := "unknown"
	if voc := vocations.GetVocation(uint32(p.Vocation)); voc != nil {
		vocName = voc.Name
	}
	w.AddString(vocName)

	w.AddU16(p.Level)
	addOutfitNoMount(w, p.Outfit)

	// Store summary & Character titles flag
	w.AddByte(0x01)
	w.AddString("") // current title (empty for now)
	g.SendToClient(w)
}

// sendCyclopediaCharacterGeneralStats sends the character's general stats (type 1).
// Matches C++ ProtocolGame::sendCyclopediaCharacterGeneralStats.
func (g *GameProtocol) sendCyclopediaCharacterGeneralStats() {
	if g.player == nil {
		return
	}
	p := g.player
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoGeneralStats)
	w.AddByte(0x00) // no error

	// Experience & level
	w.AddU64(p.Experience)
	w.AddU16(p.Level)
	levelPercent := p.GetLevelPercent()
	w.AddU16(levelPercent)

	// XP gain rates (defaults — stamina/boosts not yet implemented)
	w.AddU16(150)   // BaseXPGainRate (default)
	w.AddU16(0)     // LowLevelBonus (grindingXpBoost not implemented)
	w.AddU16(0)     // XPBoost percent (xpBoostPercent not implemented)
	w.AddU16(100)   // StaminaMultiplier (100 = x1.0)
	w.AddU16(0)     // xpBoostRemainingTime
	w.AddByte(1)    // canBuyXpBoost (1 since xpBoostTime is 0)

	// Health
	w.AddU32(p.Health)
	w.AddU32(p.GetMaxHealth())

	// Mana
	w.AddU32(p.Mana)
	w.AddU32(p.GetMaxMana())

	// Soul
	w.AddByte(p.Soul)

	// Stamina
	w.AddU16(0) // staminaMinutes (not implemented yet)

	// Regeneration ticks
	regenTicks := uint16(0)
	if p.RegenTicks > 0 {
		regenTicks = uint16(p.RegenTicks / 1000)
	}
	w.AddU16(regenTicks)

	// Offline training time (milliseconds -> minutes)
	offlineMinutes := uint16(0)
	if p.OfflineTrainingTime > 0 {
		offlineMinutes = uint16(p.OfflineTrainingTime / 60000)
	}
	w.AddU16(offlineMinutes)

	// Speed
	w.AddU16(p.GetSpeed())
	w.AddU16(p.GetBaseSpeed())

	// Capacity
	w.AddU32(p.GetCapacity())
	w.AddU32(p.Capacity) // base capacity
	w.AddU32(p.GetFreeCapacity())

	// Hardcoded bytes (client version indicators)
	w.AddByte(8)
	w.AddByte(1)

	// Magic level
	w.AddU16(p.GetEffectiveMagLevel())
	w.AddU16(p.MagLevel) // base magic level
	w.AddU16(0)          // loyalty magic level (not implemented)
	magPct := p.GetMagLevelPercent()
	w.AddU16(magPct)

	// Skills: Fist, Club, Sword, Axe, Distance, Shielding, Fishing
	// Hardcoded skill IDs matching C++: {11, 9, 8, 10, 7, 6, 13}
	hardcodedSkillIDs := []uint8{11, 9, 8, 10, 7, 6, 13}
	skillOrder := []game.Skill{game.SkillFist, game.SkillClub, game.SkillSword, game.SkillAxe, game.SkillDistance, game.SkillShielding, game.SkillFishing}
	for i, sk := range skillOrder {
		w.AddByte(hardcodedSkillIDs[i])
		w.AddU16(p.GetEffectiveSkill(sk))
		w.AddU16(p.Skills[sk]) // base skill
		w.AddU16(0)            // loyalty skill (not implemented)
		skPct := p.GetSkillPercent(sk)
		w.AddU16(skPct)
	}

	// Combat stats (empty for now)
	w.AddByte(0)

	g.SendToClient(w)
}

// sendCyclopediaCharacterItemSummary sends the item summary (type 6) with
// real inventory and stash data, matching C++ format.
func (g *GameProtocol) sendCyclopediaCharacterItemSummary() {
	if g.player == nil {
		return
	}
	p := g.player

	// Helper to write one section: item entries grouped by (ID, tier) → count.
	writeSection := func(w *netmsg.Writer, items map[[2]uint32]uint32) {
		countPos := w.Pos()
		w.AddU16(0) // placeholder count
		written := uint16(0)
		for key, cnt := range items {
			itemID := uint16(key[0])
			tier := uint8(key[1])
			t := g.deps.Items.Get(itemID)
			w.AddU16(itemID)
			if t != nil && t.UpgradeClassification > 0 {
				w.AddByte(tier)
			} else if t == nil {
				} else if t == nil && tier > 0 {
					// Item not in registry but has tier — send it to keep alignment.
					w.AddByte(tier)
				} else if t == nil {
					// Unknown item, no tier info — skip
			}
			w.AddU32(cnt)
			written++
		}
		w.SetU16(countPos, written)
	}

	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoItemSummary)
	w.AddByte(0x00) // no error

	// 1. Inventory items — group by (ID, tier)
	inv := make(map[[2]uint32]uint32)
	for slot := uint8(game.ConstSlotFirst); slot <= uint8(game.ConstSlotLast); slot++ {
		if int(slot) < len(p.Inventory) && p.Inventory[slot] != nil {
			it := p.Inventory[slot]
			key := [2]uint32{uint32(it.ID), uint32(it.GetTier())}
			inv[key]++
		}
	}
	writeSection(w, inv)

	// 2. Store inbox — empty (no easy access to container tree)
	w.AddU16(0)

	// 3. Stash items — already grouped by ID with count
	stash := make(map[[2]uint32]uint32)
	for itemID, cnt := range p.Stash {
		key := [2]uint32{uint32(itemID), 0}
		stash[key] += cnt
	}
	writeSection(w, stash)

	// 4. Depot box items — empty (container traversal too complex for now)
	w.AddU16(0)

	// 5. Inbox items — empty
	w.AddU16(0)

	g.SendToClient(w)
}

// sendCyclopediaCharacterOutfitsMounts sends outfits, mounts, and familiars (type 7).
// Matches C++ ProtocolGame::sendCyclopediaCharacterOutfitsMounts.
func (g *GameProtocol) sendCyclopediaCharacterOutfitsMounts() {
	if g.player == nil {
		return
	}
	p := g.player
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoOutfitsMounts)
	w.AddByte(0x00) // no error

	currentOutfit := p.Outfit

	// -- Outfits --
	outfitsCountPos := w.Pos()
	w.AddU16(0) // placeholder

	outfitsSent := uint16(0)
	for _, lookType := range game.GetOutfitsBySex(p.Sex) {
		if !p.HasOutfit(lookType) {
			continue
		}
		info, ok := game.GetOutfitInfo(lookType)
		if !ok {
			// Skip outfits not in our registry.
			continue
		}
		addons := p.GetOutfitAddons(lookType)

		var outfitType uint8
		switch info.From {
		case "store":
			outfitType = game.OutfitTypeStore
		case "quest":
			outfitType = game.OutfitTypeQuest
		default:
			outfitType = game.OutfitTypeNone
		}

		var isCurrent uint32
		if lookType == currentOutfit.LookType {
			isCurrent = 1000
		}

		w.AddU16(lookType)
		w.AddString(info.Name)
		w.AddByte(addons)
		w.AddByte(outfitType)
		w.AddU32(isCurrent)
		outfitsSent++
	}

	if outfitsSent > 0 {
		w.AddByte(currentOutfit.Head)
		w.AddByte(currentOutfit.Body)
		w.AddByte(currentOutfit.Legs)
		w.AddByte(currentOutfit.Feet)
	}

	// -- Mounts --
	mountsCountPos := w.Pos()
	w.AddU16(0) // placeholder

	mountsSent := uint16(0)
	for _, m := range mounts.All() {
		if !p.HasMount(m.ID) {
			continue
		}
		mountType := game.OutfitTypeNone
		switch m.Type {
		case "store":
			mountType = game.OutfitTypeStore
		case "quest":
			mountType = game.OutfitTypeQuest
		}

		w.AddU16(m.ClientID)
		w.AddString(m.Name)
		w.AddByte(mountType)
		w.AddU32(1000) // isCurrent (always 1000 for mounts in C++)
		mountsSent++
	}

	if mountsSent > 0 {
		w.AddByte(currentOutfit.MountHead)
		w.AddByte(currentOutfit.MountBody)
		w.AddByte(currentOutfit.MountLegs)
		w.AddByte(currentOutfit.MountFeet)
	}

	// -- Familiars --
	familiarsCountPos := w.Pos()
	w.AddU16(0) // placeholder

	familiarsSent := uint16(0)
	for _, f := range p.Familiars {
		if !f.Unlocked {
			continue
		}
		vocationID := uint16(p.Vocation)
		info, ok := game.GetFamiliarInfo(vocationID, f.LookType)

		familiarName := f.Name
		if familiarName == "" && ok {
			familiarName = info.Name
		} else if familiarName == "" {
			familiarName = fmt.Sprintf("Familiar %d", f.LookType)
		}

		familiarType := game.OutfitTypeNone
		if ok && info.From == "quest" {
			familiarType = game.OutfitTypeQuest
		}

		w.AddU16(f.LookType)
		w.AddString(familiarName)
		w.AddByte(familiarType)
		w.AddU32(0) // isCurrent (always 0 for familiars)
		familiarsSent++
	}

	// Backfill section counts (C++ writes at end via setBufferPosition).
	w.SetU16(outfitsCountPos, outfitsSent)
	w.SetU16(mountsCountPos, mountsSent)
	w.SetU16(familiarsCountPos, familiarsSent)

	g.SendToClient(w)
}

// sendCyclopediaCharacterWheel sends the cyclopedia wheel response (type 12).
// The OTClient's CyclopediaCharacterInfo parser does nothing with the body
// for CYCLOPEDIA_CHARACTERINFO_WHEEL, so we only send the header.
func (g *GameProtocol) sendCyclopediaCharacterWheel() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoWheel)
	w.AddByte(0x00) // no error
	g.SendToClient(w)
}

// sendCyclopediaCharacterStoreSummary sends the store summary (type 8).
func (g *GameProtocol) sendCyclopediaCharacterStoreSummary() {
	if g.player == nil {
		return
	}
	p := g.player
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoStoreSummary)
	w.AddByte(0x00)

	w.AddU32(0) // xpBoostTime
	w.AddU32(0) // dailyRewardXpBoostTime

	blessings := []struct {
		name  string
		value uint8
	}{
		{"Twist Of Fate", 1},
		{"The Wisdom Of Solitude", 2},
		{"The Spark Of The Phoenix", 3},
		{"The Fire Of The Suns", 4},
		{"The Spiritual Shielding", 5},
		{"The Embrace Of Tibia", 6},
		{"Hearth Of The Mountain", 7},
		{"Blood Of The Mountain", 8},
	}
	w.AddByte(uint8(len(blessings)))
	for _, b := range blessings {
		w.AddString(b.name)
		idx := int(b.value) - 1
		if idx >= 0 && idx < len(p.Blessings) && p.Blessings[idx] > 0 {
			w.AddByte(p.Blessings[idx])
		} else {
			w.AddByte(0)
		}
	}

	w.AddByte(0) // preySlotsUnlocked
	w.AddByte(0) // preyWildcards
	w.AddByte(0) // instantRewards
	w.AddByte(0) // hasCharmExpansion
	w.AddByte(0) // hirelingsObtained
	w.AddByte(0) // reserved

	w.AddByte(0) // hirelingSkillsCount
	w.AddByte(0) // hirelingOutfitsCount
	w.AddU16(0)  // houseItemsCount

	slog.Default().Info("store: sending summary",
		"player", p.Name,
		"packetHex", fmt.Sprintf("%x", w.Bytes()),
		"packetLen", len(w.Bytes()))
	g.SendToClient(w)
}

// sendCyclopediaCharacterBadges sends the character badges (type 10).
// Matches C++ ProtocolGame::sendCyclopediaCharacterBadges.
func (g *GameProtocol) sendCyclopediaCharacterBadges() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoBadges)
	w.AddByte(0x00) // no error

	w.AddByte(0x01) // showAccountInformation (1 = yes)

	// isOnline — we are inspecting ourselves, so 1
	w.AddByte(0x01)

	// isPremium — stub with 1 for now
	w.AddByte(0x01)

	// Loyalty title (empty)
	w.AddString("")

	// Count matching badges first so we can write the correct count.
	var matching []game.BadgeInfo
	for _, badge := range game.DefaultBadges {
		if g.player.GetBadges().HasBadge(badge.ID) {
			matching = append(matching, badge)
		}
	}
	w.AddByte(byte(len(matching)))
	for _, badge := range matching {
		w.AddU32(badge.ID)
		w.AddString(badge.Name)
	}

	g.SendToClient(w)
}

// sendCyclopediaCharacterTitles sends the character titles (type 11).
// Exact wire format matching C++: 0xDA + u8 type + u8 error +
//   u8 currentTitle + u8 count +
//   for each: u8 id + string name + string desc + u8 permanent + u8 unlocked
func (g *GameProtocol) sendCyclopediaCharacterTitles() {
	if g.player == nil {
		return
	}
	p := g.player
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoTitles)
	w.AddByte(0x00) // no error

	titles := p.GetTitles()
	w.AddByte(titles.CurrentID)

	allTitles := game.DefaultTitles
	w.AddByte(uint8(len(allTitles)))
	for _, t := range allTitles {
		w.AddByte(t.ID)
		name := t.MaleName
		if p.Sex == 0 && t.FemaleName != "" {
			name = t.FemaleName
		}
		w.AddString(name)
		w.AddString(t.Description)
		if t.Permanent {
			w.AddByte(1)
		} else {
			w.AddByte(0)
		}
		if titles.IsUnlocked(t.ID) {
			w.AddByte(1)
		} else {
			w.AddByte(0)
		}
	}
	g.SendToClient(w)
}

func (g *GameProtocol) findPlayerByID(id uint32) *game.Player {
	if g.deps == nil || g.deps.World == nil {
		return nil
	}
	if p := g.deps.World.PlayerByID(id); p != nil {
		return p
	}
	for _, p := range g.deps.World.Players() {
		if p != nil && p.ID == id {
			return p
		}
	}
	return nil
}

func (g *GameProtocol) parseInspectionObject(r *netmsg.Reader) {
	inspectionType := r.GetByte()
	switch inspectionType {
	case 0: // INSPECT_NORMALOBJECT — inspect a thing on the map
		r.GetU16() // x
		r.GetU16() // y
		r.GetByte() // z
	case 1, 2: // INSPECT_NPCTRADE / INSPECT_CYCLOPEDIA — inspect item by ID
		r.GetU16() // itemId
		r.GetByte() // itemCount
	case 3: // INSPECT_PROFICIENCY
		r.GetU16() // itemId
		r.GetByte() // unknown
	}
}
