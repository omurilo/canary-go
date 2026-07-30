package protocol

import (
	"context"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// getSupremeModifiers returns the exact WheelGemSupremeModifier_t enum values per vocation matching C++ modsSupremePositionByVocation.
// getSupremeModifiers returns the WheelGemSupremeModifier_t enum positions for a
// vocation (23 entries), transcribed from modsSupreme<Voc>Position keyed by the
// vocation base id (1=Sorc, 2=Druid, 3=Paladin, 4=Knight, 9=Monk; promotions
// share the base list). Currently latent (all grades sent as 0), but the
// positions must match once gems are modeled.
func getSupremeModifiers(vocation uint16) []byte {
	switch vocation {
	case 1, 5: // Sorcerer / Master Sorcerer
		return []byte{0, 1, 2, 3, 4, 5, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58}
	case 2, 6: // Druid / Elder Druid
		return []byte{0, 1, 2, 3, 4, 5, 59, 60, 61, 62, 63, 64, 66, 65, 67, 68, 69, 70, 71, 72, 73, 74, 75}
	case 3, 7: // Paladin / Royal Paladin
		return []byte{0, 1, 2, 3, 5, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41}
	case 9, 10: // Monk / Exalted Monk
		return []byte{0, 1, 2, 3, 5, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93}
	default: // Knight / Elite Knight (and fallback)
		return []byte{0, 1, 2, 3, 5, 6, 7, 8, 9, 10, 11, 12, 13, 15, 14, 16, 17, 19, 18, 20, 21, 22, 23}
	}
}

// sendGiftOfLifeCooldown sends opcode 0x5E (Wheel of Destiny Gift of Life cooldown status).
func (g *GameProtocol) sendGiftOfLifeCooldown() {
	w := netmsg.NewWriter()
	w.AddByte(0x5E)
	w.AddByte(0x01) // Gift of life ID
	w.AddByte(0x00) // Cooldown ENUM
	w.AddU32(0)     // Remaining cooldown seconds
	w.AddU32(0)     // Total cooldown seconds
	w.AddByte(0x00) // Infight / cooldown paused flag
	g.SendToClient(w)
}

// parseOpenWheel handles opcode 0x61 (Open Wheel of Destiny window).
func (g *GameProtocol) parseOpenWheel(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetU32() // ownerID
	g.SendWheelOfDestiny()
}

// parseSaveWheel handles opcode 0x62 (Save Wheel of Destiny allocations).
// In Tibia 13.x protocol, the client sends 36 uint16 values representing allocated points for slots 1..36.
func (g *GameProtocol) parseSaveWheel(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	g.applyWheelSave(r)
}

// applyWheelSave reads the 36 slot values and applies them through the validated
// path (per-slot cap + total budget + adjacency), then persists, recomputes and
// refreshes. Mirrors PlayerWheel::saveSlotPointsOnPressSaveButton.
func (g *GameProtocol) applyWheelSave(r *netmsg.Reader) {
	pointsMap := make(map[uint16]uint16)
	for slotID := uint16(1); slotID <= 36 && r.Remaining() >= 2; slotID++ {
		pts := r.GetU16()
		if pts > 0 {
			pointsMap[slotID] = pts
		}
	}
	wheel := g.player.GetWheel()
	wheel.SetVocation(game.CIPVocation(g.player.Vocation))
	if g.player.WheelGemManager == nil {
		g.player.WheelGemManager = &game.WheelGemCollection{}
	}

	// Process gem vessels (4 affinities): each has [u8 hasGem][if 1: u16 gemIndex]
	for aff := 0; aff < 4 && r.Remaining() >= 1; aff++ {
		hasGem := r.GetByte()
		if hasGem == 0 {
			g.player.WheelGemManager.ActiveGems[aff] = nil
			continue
		}
		if r.Remaining() >= 2 {
			gemIdx := r.GetU16()
			if int(gemIdx) < len(g.player.WheelGemManager.RevealedGems) {
				g.player.WheelGemManager.RevealedGems[gemIdx].Affinity = game.WheelGemAffinity(aff)
				g.player.WheelGemManager.ActiveGems[aff] = &g.player.WheelGemManager.RevealedGems[gemIdx]
			}
		}
	}

	wheel.SaveSlotPoints(pointsMap)
	// Removing invested points lowers the wheel HP/mana bonus and therefore the
	// player's maximum; clamp the current values down to the new maxima (a max
	// decrease must never leave current above max). Mirrors the stat reload in
	// PlayerWheel::reloadPlayerData.
	if maxHP := g.player.GetMaxHealth(); g.player.Health > maxHP {
		g.player.Health = maxHP
	}
	if maxMana := g.player.GetMaxMana(); g.player.Mana > maxMana {
		g.player.Mana = maxMana
	}
	if g.deps != nil && g.deps.DB != nil && g.player != nil {
		_ = g.deps.DB.SavePlayerWheel(context.Background(), g.player)
	}
	g.SendWheelOfDestiny()
	g.SendStats()
}

// wheelOptions returns the "options" byte for the wheel window, mirroring
// PlayerWheel::getOptions: 1 = can increase AND decrease points (only inside a
// temple protection zone), 2 = can only increase points (anywhere else).
func (g *GameProtocol) wheelOptions() byte {
	if g.deps != nil && g.deps.World != nil {
		if tile := g.deps.World.Map.GetTile(g.player.Pos); tile != nil && tile.IsProtectionZone() {
			return 1
		}
	}
	return 2
}

// parseWheelOfDestiny was removed: it claimed opcode 0xEC (which is actually
// parseSetHirelingName in C++, protocolgame.cpp:2027) and was never dispatched.
// The wheel is served by parseOpenWheel (0x61) and parseSaveWheel (0x62).

// parseWheelGemAction handles opcode 0xE7 (Wheel of Destiny Gem actions / enhance mod grade).
// Wire format (C++): [u8 action][u8 param][u8 pos] — param and pos are BOTH u8!
func (g *GameProtocol) parseWheelGemAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	if r.Remaining() < 1 {
		return
	}
	if g.player.WheelGemManager == nil {
		g.player.WheelGemManager = &game.WheelGemCollection{}
	}
	action := r.GetByte()
	param := uint16(r.GetByte()) // C++ uses getByte() for param (U8), NOT getU16!
	var pos uint8
	if r.Remaining() >= 1 {
		pos = r.GetByte()
	}
	switch action {
	case 0: // Destroy gem
		g.player.WheelGemManager.DestroyGem(param)

	case 1: // Reveal gem
		catalog := g.deps.Items
		var gemNames []string
		var gemQuality game.WheelGemQuality
		switch game.WheelGemQuality(param) {
		case 2:
			gemNames = []string{"greater guardian gem", "greater marksman gem", "greater sage gem",
				"greater mystic gem", "greater spiritualist gem"}
			gemQuality = 2
		case 1:
			gemNames = []string{"guardian gem", "marksman gem", "sage gem",
				"mystic gem", "spiritualist gem"}
			gemQuality = 1
		default:
			gemNames = []string{"lesser guardian gem", "lesser marksman gem", "lesser sage gem",
				"lesser mystic gem", "lesser spiritualist gem"}
			gemQuality = 0
		}
		removed := false
		for _, name := range gemNames {
			id, ok := catalog.IDByName(name)
			if !ok {
				continue
			}
			if g.player.GetItemCount(id) > 0 {
				if g.player.RemoveItemOfType(catalog, id, 1, -1, false) {
					removed = true
					break
				}
			}
		}
		if removed {
			gem := game.NewRevealedGem(gemQuality)
			g.player.WheelGemManager.RevealedGems = append(g.player.WheelGemManager.RevealedGems, gem)
			g.sendGemAtelierGemRevealed(uint16(len(g.player.WheelGemManager.RevealedGems) - 1))
		}

	case 2:
		g.player.WheelGemManager.SwitchGemDomain(param)

	case 3:
		g.player.WheelGemManager.ToggleGemLock(param)

		case 4:
			catalog := g.deps.Items
			if catalog == nil {
				break
			}
			wheel := g.player.GetWheel()
			modPos := pos
			fragmentType := byte(param)
			var fragID uint16
			var fragCount uint32
			var goldCost uint64
			switch fragmentType {
			case 0:
				fragID = 46625
				fragCount = 5
				goldCost = 2000000
			case 1:
				fragID = 46626
				fragCount = 5
				goldCost = 5000000
			default:
				break
			}
			if fragID == 0 {
				break
			}
			if g.player.GetItemCount(fragID) < fragCount {
				break
			}
			if g.player.GetMoney() < goldCost {
				break
			}
			if !g.player.RemoveItemOfType(catalog, fragID, fragCount, -1, false) {
				break
			}
			if !g.player.RemoveMoney(goldCost, true) {
				break
			}
			if !wheel.ImproveModGrade(fragmentType, modPos) {
				break
			}
		}
		// Persist gem changes
		if g.deps.DB != nil && g.player != nil {
			_ = g.deps.DB.SavePlayerWheel(context.Background(), g.player)
		}
		g.SendWheelOfDestiny()

}
// SendWheelOfDestiny sends the full Wheel of Destiny payload (Opcode 0x5F) to client.
func (g *GameProtocol) SendWheelOfDestiny() {
	wheel := g.player.GetWheel()
	wheel.SetVocation(game.CIPVocation(g.player.Vocation))

	// Mirrors PlayerWheel::canOpenWheel: needs a vocation and level > 50. (C++
	// also requires premium + promotion; Go models neither yet, so those are
	// intentionally lenient.)
	canUse := g.player.Vocation > 0 && g.player.Level > 50

	w := netmsg.NewWriter()
	w.AddByte(0x5F) // Wheel window response opcode
	w.AddU32(g.player.ID)

	if !canUse {
		w.AddByte(0) // canUse = false
		g.SendToClient(w)
		return
	}

	w.AddByte(1)                // canUse = true
	w.AddByte(g.wheelOptions()) // options: 1 in a temple PZ (can decrease), else 2

	// Map OT vocation ID to CIP client vocation ID (handles Monk → 5), matching
	// getPlayerVocationEnum.
	w.AddByte(game.CIPVocation(g.player.Vocation))

	// First field is the base points (level-derived, excluding extras); the
	// second is the extra points. Mirrors getWheelPoints(false) + getExtraPoints.
	var basePoints uint16
	if g.player.Level > 50 {
		basePoints = uint16(g.player.Level) - 50
	}
	w.AddU16(basePoints)
	w.AddU16(wheel.BonusPoints) // extra points

	// Write slot allocations for 36 slots
	slotPoints := wheel.GetSlotPointsCopy()
	for slotID := uint16(1); slotID <= 36; slotID++ {
		w.AddU16(slotPoints[slotID])
	}

	w.AddU16(0)  // promotion scrolls count (u16 = 2 bytes)
	w.AddByte(0) // monk quest bonus flag (u8 = 1 byte)
	w.AddU16(0)  // monk quest bonus amount (u16 = 2 bytes)

	// Gems section — C++ addGems format
	if g.player.WheelGemManager == nil {
		g.player.WheelGemManager = &game.WheelGemCollection{}
	}
	revealedGems := g.player.WheelGemManager.RevealedGems
	activeGems := g.player.WheelGemManager.ActiveGems

	// Active gems: count(u8), then gemIndex(u16) for each
	var activeIndexes []uint16
	for aff := 0; aff < 4; aff++ {
		gem := activeGems[aff]
		if gem == nil {
			continue
		}
		for i, rg := range revealedGems {
			if rg.UUID == gem.UUID {
				activeIndexes = append(activeIndexes, uint16(i))
				break
			}
		}
	}
	w.AddByte(byte(len(activeIndexes)))
	for _, idx := range activeIndexes {
		w.AddU16(idx)
	}

	// Revealed gems: count(u16), then per: index(u16), locked(u8), affinity(u8), quality(u8), mod1(u8), [mod2(u8)], [supreme(u8)]
	w.AddU16(uint16(len(revealedGems)))
	for i, gem := range revealedGems {
		w.AddU16(uint16(i))
		if gem.Locked { w.AddByte(1) } else { w.AddByte(0) }
		w.AddByte(uint8(gem.Affinity))
		w.AddByte(uint8(gem.Quality))
		w.AddByte(uint8(gem.BasicModifier1))
		if gem.Quality >= game.GemQualityRegular && gem.BasicModifier2 != nil {
			w.AddByte(uint8(*gem.BasicModifier2))
		}
		if gem.Quality >= game.GemQualityGreater && gem.SupremeModifier != nil {
			w.AddByte(uint8(*gem.SupremeModifier))
		}
	}

	// Grade modifiers: read from wheel.BasicGrades / SupremeGrades (C++ addGradeModifiers)
	w.AddByte(46) // basic modifier count (0x2E)
	for i := byte(0); i < 46; i++ {
		w.AddByte(i)
		w.AddByte(wheel.BasicGrades[i])
	}

	supremeMods := getSupremeModifiers(g.player.Vocation)
	w.AddByte(byte(len(supremeMods))) // supreme modifier count
	for _, modPos := range supremeMods {
		w.AddByte(modPos)
		w.AddByte(wheel.SupremeGrades[modPos])
	}

	g.SendToClient(w)

	// Send Gift of Life cooldown state (opcode 0x5E)
	g.sendGiftOfLifeCooldown()

	// Send resource balance updates expected by Wheel UI
	g.sendResourceBalance(0x00, g.player.BankBalance)
	g.sendResourceBalance(0x01, uint64(g.player.GetMoney()))
	lesser, reg, greater, lesserFrags, greaterFrags := g.countInventoryGems()
		g.sendResourceBalance(0x51, uint64(lesser))    // RESOURCE_LESSER_GEMS
	g.sendResourceBalance(0x52, uint64(reg))       // RESOURCE_REGULAR_GEMS
	g.sendResourceBalance(0x53, uint64(greater))   // RESOURCE_GREATER_GEMS
	g.sendResourceBalance(0x54, uint64(lesserFrags))   // RESOURCE_LESSER_FRAGMENT
	g.sendResourceBalance(0x55, uint64(greaterFrags))  // RESOURCE_GREATER_FRAGMENT
}

// countInventoryGems scans the player's inventory for gem items and returns counts.
func (g *GameProtocol) countInventoryGems() (lesser, regular, greater, lesserFrags, greaterFrags uint32) {
	if g.player == nil || g.deps.Items == nil {
		return 0, 0, 0, 0, 0
	}
	catalog := g.deps.Items
	lesserNames := []string{
		"lesser guardian gem", "lesser marksman gem", "lesser sage gem",
		"lesser mystic gem", "lesser spiritualist gem",
	}
	regularNames := []string{
		"guardian gem", "marksman gem", "sage gem",
		"mystic gem", "spiritualist gem",
	}
	greaterNames := []string{
		"greater guardian gem", "greater marksman gem", "greater sage gem",
		"greater mystic gem", "greater spiritualist gem",
	}
	// Fragment items (forge dust items used as wheel fragments)
	// 46625 = lesser fragment, 46626 = greater fragment
	fragmentLesserIDs := []uint16{46625}
	fragmentGreaterIDs := []uint16{46626}
	
	for _, name := range lesserNames {
		if id, ok := catalog.IDByName(name); ok {
			lesser += uint32(g.player.GetItemCount(id))
		}
	}
	for _, name := range regularNames {
		if id, ok := catalog.IDByName(name); ok {
			regular += uint32(g.player.GetItemCount(id))
		}
	}
	for _, name := range greaterNames {
		if id, ok := catalog.IDByName(name); ok {
			greater += uint32(g.player.GetItemCount(id))
		}
	}
	for _, id := range fragmentLesserIDs {
		lesserFrags += uint32(g.player.GetItemCount(id))
	}
	for _, id := range fragmentGreaterIDs {
		greaterFrags += uint32(g.player.GetItemCount(id))
	}
	return
}

// findItemIDs looks up item IDs by name in the catalog.
func findItemIDs(catalog *items.Catalog, names ...string) []uint16 {
	var ids []uint16
	if catalog == nil {
		return ids
	}
	for _, name := range names {
		if id, ok := catalog.IDByName(name); ok {
			ids = append(ids, id)
		}
	}
	return ids
}


// sendGemAtelierGemRevealed notifies the client that a gem was revealed (opcode 0xC5).
func (g *GameProtocol) sendGemAtelierGemRevealed(gemIndex uint16) {
	w := netmsg.NewWriter()
	w.AddByte(0xC5)
	w.AddU16(gemIndex)
	g.SendToClient(w)
}
