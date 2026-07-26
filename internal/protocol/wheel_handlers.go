package protocol

import (
	"context"

	"github.com/opentibiabr/canary-go/internal/game"
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
	_ = r.GetU32() // ownerID
	g.SendWheelOfDestiny()
}

// parseSaveWheel handles opcode 0x62 (Save Wheel of Destiny allocations).
// In Tibia 13.x protocol, the client sends 36 uint16 values representing allocated points for slots 1..36.
func (g *GameProtocol) parseSaveWheel(r *netmsg.Reader) {
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
	if !wheel.ValidateAndSave(pointsMap, wheel.GetTotalPoints(g.player.Level)) {
		g.player.SendTextMessage(0x14, "Something went wrong, try relogging and try again.")
		g.SendWheelOfDestiny()
		return
	}
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

// parseWheelOfDestiny handles opcode 0xEC (Legacy / Alternative Wheel request).
func (g *GameProtocol) parseWheelOfDestiny(r *netmsg.Reader) {
	if r.Remaining() < 1 {
		return
	}
	subOp := r.GetByte()
	switch subOp {
	case 0: // Request / Open Wheel Data
		g.SendWheelOfDestiny()
	case 1: // Save Wheel Allocation / Preset
		g.applyWheelSave(r)
	}
}

// parseWheelGemAction handles opcode 0xE7 (Wheel of Destiny Gem actions / enhance mod grade).
func (g *GameProtocol) parseWheelGemAction(r *netmsg.Reader) {
	if r.Remaining() < 1 {
		return
	}
	action := r.GetByte()
	wheel := g.player.GetWheel()
	switch action {
		case 0: // Destroy gem
		if r.Remaining() >= 2 {
			slotID := r.GetU16()
			for i, g := range wheel.Gems {
				if g.Slot == slotID {
					wheel.Gems = append(wheel.Gems[:i], wheel.Gems[i+1:]...)
					break
				}
			}
		}
	case 1: // Reveal gem
		if r.Remaining() >= 1 {
			_ = r.GetByte() // quality
			wheel.RevealedGems++
		}
	case 2: // SwitchDomain
		if r.Remaining() >= 2 {
			slotID := r.GetU16()
			for i, g := range wheel.Gems {
				if g.Slot == slotID {
					wheel.Gems[i].Domain = (g.Domain + 1) % 4
					break
				}
			}
		}
	case 3: // ToggleLock
		if r.Remaining() >= 2 {
			slotID := r.GetU16()
			for i, g := range wheel.Gems {
				if g.Slot == slotID {
					wheel.Gems[i].Locked = !g.Locked
					break
				}
			}
		}
	case 4: // ImproveGrade / Enhance
		if r.Remaining() >= 2 {
			slot := r.GetByte()
			_ = r.GetByte() // extra param
			for i, g := range wheel.Gems {
				if g.Slot == uint16(slot) {
					if wheel.Gems[i].Grade < 6 {
						wheel.Gems[i].Grade++
					}
					break
				}
			}
		}	}
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

	// Gems section — send real gem data from the Wheel
	wheelGems := wheel.Gems
	revealed := uint16(wheel.RevealedGems)
	if revealed == 0 && len(wheelGems) > 0 {
		revealed = uint16(len(wheelGems))
	}
	w.AddByte(byte(len(wheelGems))) // active gems count
	w.AddU16(revealed)              // revealed gems count
	for _, gem := range wheelGems {
		w.AddU16(gem.Slot)
		w.AddByte(gem.Domain)
		w.AddByte(gem.Grade)
		if gem.Locked { w.AddByte(1) } else { w.AddByte(0) }
	}

	// Grade modifiers section — send actual grades from equipped gems
	gradeMap := make(map[byte]byte)
	for _, gem := range wheelGems {
		if gem.Grade > 0 {
			gradeMap[byte(gem.Slot)] = gem.Grade
		}
	}
	w.AddByte(46)
	for i := byte(0); i < 46; i++ {
		w.AddByte(i)
		w.AddByte(gradeMap[i])
	}

	// Supreme grade modifiers (vocation-specific 23 entries)
	supremeMods := getSupremeModifiers(g.player.Vocation)
	w.AddByte(byte(len(supremeMods)))
	for _, modPos := range supremeMods {
		w.AddByte(modPos)
		w.AddByte(gradeMap[modPos])
	}

	g.SendToClient(w)

	// Send Gift of Life cooldown state (opcode 0x5E)
	g.sendGiftOfLifeCooldown()

	// Send resource balance updates expected by Wheel UI
	g.sendResourceBalance(0x00, g.player.BankBalance)
	g.sendResourceBalance(0x01, uint64(g.player.GetMoney()))
	lesser, reg, greater := g.countInventoryGems()
		g.sendResourceBalance(0x51, uint64(lesser))    // RESOURCE_LESSER_GEMS
	g.sendResourceBalance(0x52, uint64(reg))       // RESOURCE_REGULAR_GEMS
	g.sendResourceBalance(0x53, uint64(greater))   // RESOURCE_GREATER_GEMS
	g.sendResourceBalance(0x54, uint64(wheel.LesserFragments))   // RESOURCE_LESSER_FRAGMENT
	g.sendResourceBalance(0x55, uint64(wheel.GreaterFragments))  // RESOURCE_GREATER_FRAGMENT
}

// countInventoryGems scans the player's inventory for gem items and returns counts.
func (g *GameProtocol) countInventoryGems() (lesser, regular, greater uint32) {
	if g.player == nil || g.deps.Items == nil {
		return 0, 0, 0
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
	return
}
