package protocol

import (
	"fmt"
	"log/slog"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/vocations"
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

// sendCyclopediaCharacterItemSummary sends an empty item summary (type 6).
// Each section count is 0, so no item data is sent.
func (g *GameProtocol) sendCyclopediaCharacterItemSummary() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoItemSummary)
	w.AddByte(0x00) // no error

	w.AddU16(0) // inventoryItemsCount
	w.AddU16(0) // storeInboxItemsCount
	w.AddU16(0) // stashItemsCount
	w.AddU16(0) // depotBoxItemsCount
	w.AddU16(0) // inboxItemsCount

	g.SendToClient(w)
}

// sendCyclopediaCharacterOutfitsMounts sends outfits, mounts, and familiars (type 7).
// For now sends empty lists for all three categories. Matches C++
// ProtocolGame::sendCyclopediaCharacterOutfitsMounts.
func (g *GameProtocol) sendCyclopediaCharacterOutfitsMounts() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoOutfitsMounts)
	w.AddByte(0x00) // no error

	// Outfits — empty list
	w.AddU16(0)
	// Mounts — empty list
	w.AddU16(0)
	// Familiars — empty list
	w.AddU16(0)

	g.SendToClient(w)
}

// sendCyclopediaCharacterStoreSummary sends the store summary (type 8).
// Exact match of C++ ProtocolGame::sendCyclopediaCharacterStoreSummary.
func (g *GameProtocol) sendCyclopediaCharacterStoreSummary() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoStoreSummary)
	w.AddByte(0x00) // no error

	w.AddU32(0) // xpBoostTime remaining
	w.AddU32(0) // dailyRewardXpBoostTime (deprecated)
	w.AddByte(0) // blessingCount

	// preySlotsUnlocked
	w.AddByte(0)
	// preyWildcards
	w.AddByte(0)
	// hasPermanentWeeklyTaskExpansion (GameTaskboard feature)
	w.AddByte(0)
	w.AddByte(0) // instantRewards
	w.AddByte(0) // hasCharmExpansion
	w.AddByte(0) // hirelingsObtained

	// In C++: msg.addByte(0x00) reserved; then msg.addByte(m_hSkills.size())
	// The OTClient parser reads the reserved byte as hirelingSkillsCount
	// C++ sequence: reserved(0x00) + hirelingSkillsCount(0) + hirelingOutfitsCount(0x00) + u16 houseItemsCount(0)
	// OTClient reads: reserved→hirelingSkillsCount, skillCount→hirelingOutfitsCount,
	// hirelingOutfitsCount→houseItemsCount LO, houseItemsCount LO→houseItemsCount HI.
	w.AddByte(0) // reserved (OTClient reads as hirelingSkillsCount)
	w.AddByte(0) // hirelingSkillsCount (OTClient reads as hirelingOutfitsCount)
	w.AddByte(0) // hirelingOutfitsCount (OTClient reads as u16 houseItemsCount LO)
	w.AddU16(0)  // houseItemsCount (OTClient reads LO byte as houseItemsCount HI; HI byte leftover)

	slog.Default().Info("store: sending summary",
		"player", g.player.Name,
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

	// Badges count (0 for now)
	w.AddByte(0)

	g.SendToClient(w)
}

// sendCyclopediaCharacterTitles sends the character titles (type 11).
// Matches C++ ProtocolGame::sendCyclopediaCharacterTitles.
func (g *GameProtocol) sendCyclopediaCharacterTitles() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoTitles)
	w.AddByte(0x00) // no error

	w.AddByte(0) // currentTitle (0 = none)
	w.AddByte(0) // titlesCount (0 for now)

	g.SendToClient(w)
}

// sendCyclopediaCharacterOffenceStats sends offence stats (type 13).
// Matches C++ ProtocolGame::sendCyclopediaCharacterOffenceStats.
// All values are zero/stubs — full stat tracking not implemented yet.
func (g *GameProtocol) sendCyclopediaCharacterOffenceStats() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoOffenceStats)
	w.AddByte(0x00) // no error

	// — Critical Hit Chance (addCyclopediaCriticalSkill: 6 doubles) —
	for i := 0; i < 6; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Critical Hit Damage (addCyclopediaCriticalSkill: 6 doubles) —
	for i := 0; i < 6; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Life Leech Amount (addCyclopediaSkills: 5 doubles) —
	for i := 0; i < 5; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Mana Leech Amount (addCyclopediaSkills: 5 doubles) —
	for i := 0; i < 5; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Onslaught (4 doubles) —
	for i := 0; i < 4; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Cleave (1 double) —
	w.AddDouble(0.0, 4)

	// — Perfect shot damage (7 u16, one per range 1-7) —
	for i := 0; i < 7; i++ {
		w.AddU16(0)
	}

	// — Flat damage/healing (3 u16) —
	w.AddU16(0) // flatBonus total
	w.AddU16(0) // flatBonus (equipment)
	w.AddU16(0) // unknown flat

	// — Weapon section: no weapon → fist path (matching C++ default) —
	w.AddU16(0) // attackTotal
	w.AddU16(0) // flatBonus
	w.AddU16(0) // attackValue
	w.AddByte(11) // CIPBIA_SKILL_FIST
	w.AddU16(0) // attackSkill
	w.AddU16(0) // attackTotal - attackRawTotal
	w.AddByte(0) // CIPBIA_ELEMENTAL_PHYSICAL
	w.AddDouble(0.0, 4) // elemental conversion
	w.AddByte(0) // element type
	w.AddByte(0) // accuracy count

	// — Influenced / Bosses damage (1 double) —
	w.AddDouble(0.0, 4)

	// — Bestiary damages count (u16, 0 entries) —
	w.AddU16(0)

	// — Element Critical Chance —
	w.AddByte(0) // hasElementCriticalChance (0 = no)

	// — Runes & Auto Attack Critical Chance (2 doubles) —
	w.AddDouble(0.0, 4) // runesCritical.chance
	w.AddDouble(0.0, 4) // autoAttackCritical.chance

	// — Element Critical Damage —
	w.AddByte(0) // hasElementCriticalDamage (0 = no)

	// — Runes & Auto Attack Critical Damage (2 doubles) —
	w.AddDouble(0.0, 4) // runesCritical.damage
	w.AddDouble(0.0, 4) // autoAttackCritical.damage

	// — Life/Mana Gain on Hit/Kill (4 u16) —
	w.AddU16(0) // lifeGainOnHit
	w.AddU16(0) // manaGainOnHit
	w.AddU16(0) // lifeGainOnKill
	w.AddU16(0) // manaGainOnKill

	// — Skill percentages (all stubbed: has=false) —
	w.AddByte(0) // hasAutoAttackSkill
	w.AddByte(0) // hasSpellDamage
	w.AddByte(0) // hasSpellHealing

	// — Final three doubles + elemental pierces count —
	w.AddDouble(0.0, 4) // fullHpExtraDamage
	w.AddDouble(0.0, 4) // lowHpExtraDamage
	w.AddDouble(0.0, 4) // armorPenetration
	w.AddByte(0) // elementalPiercesCount

	g.SendToClient(w)
}

// sendCyclopediaCharacterDefenceStats sends defence stats (type 14).
// Matches C++ ProtocolGame::sendCyclopediaCharacterDefenceStats.
// All values are zero/stubs.
func (g *GameProtocol) sendCyclopediaCharacterDefenceStats() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoDefenceStats)
	w.AddByte(0x00) // no error

	// — Dodge (5 doubles) —
	for i := 0; i < 5; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Magic shield capacity —
	w.AddU32(0) // total capacity
	w.AddU16(0) // flat bonus
	w.AddDouble(0.0, 4) // percent bonus

	// — Reflect physical flat —
	w.AddU16(0)

	// — Armor —
	w.AddU16(0)

	// — Mantra —
	w.AddU16(0)

	// — Defense total, defense equipment (2 u16) —
	w.AddU16(0) // defense total
	w.AddU16(0) // defense equipment
	w.AddByte(0x06) // shield count (hardcoded, matching C++)
	w.AddU16(0) // shielding skill
	w.AddU16(0) // defense wheel

	// — Mitigation (5 doubles) —
	for i := 0; i < 5; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Combat absorbs (0 combats) —
	w.AddByte(0) // combats count

	g.SendToClient(w)
}

// sendCyclopediaCharacterMiscStats sends misc stats (type 15).
// Matches C++ ProtocolGame::sendCyclopediaCharacterMiscStats.
// All values are zero/stubs.
func (g *GameProtocol) sendCyclopediaCharacterMiscStats() {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoMiscStats)
	w.AddByte(0x00) // no error

	// — Momentum (5 doubles) —
	for i := 0; i < 5; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Forge Legs (4 doubles) —
	for i := 0; i < 4; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Forge Feet (3 doubles) —
	for i := 0; i < 3; i++ {
		w.AddDouble(0.0, 4)
	}

	// — Blessings —
	w.AddByte(0) // haveBlesses
	w.AddByte(0) // totalBlesses

	// — Active concoctions —
	w.AddByte(0) // count

	// — Active foods —
	w.AddByte(0) // count

	// — Weapon proficiency augments —
	w.AddByte(0) // count

	// — Wheel augments —
	w.AddByte(0) // count

	// — Equipped augments —
	w.AddByte(0) // count

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
