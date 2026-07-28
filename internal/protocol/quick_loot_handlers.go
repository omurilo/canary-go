package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseQuickLoot handles opcode 0x8F (Quick Loot request).
func (g *GameProtocol) parseQuickLoot(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	variant := r.GetByte()
	if variant == 2 {
		// Loot nearby corpses
		g.deps.World.PlayerQuickLoot(g.player.ID, g.player.Pos, 0, 0, true)
		return
	}

	netPos := r.GetPosition()
	pos := game.Position{X: netPos.X, Y: netPos.Y, Z: netPos.Z}
	itemID := r.GetU16()
	stackPos := r.GetByte()
	lootAllCorpses := variant == 1

	g.deps.World.PlayerQuickLoot(g.player.ID, pos, itemID, stackPos, lootAllCorpses)
}

// parseLootContainer handles opcode 0x90 (Manage Loot Containers / Categories).
func (g *GameProtocol) parseLootContainer(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	action := r.GetByte()
	switch action {
	case 0: // Set managed container (Loot)
		category := r.GetByte()
		netPos := r.GetPosition()
		pos := game.Position{X: netPos.X, Y: netPos.Y, Z: netPos.Z}
		itemID := r.GetU16()
		stackPos := r.GetByte()
		g.deps.World.PlayerSetManagedContainer(g.player.ID, category, pos, itemID, stackPos, true)
	case 1: // Clear managed container (Loot)
		category := r.GetByte()
		g.deps.World.PlayerClearManagedContainer(g.player.ID, category, true)
	case 2: // Open managed container (Loot)
		category := r.GetByte()
		g.deps.World.PlayerOpenManagedContainer(g.player.ID, category, true)
	case 3: // Set fallback to main container
		fallback := r.GetByte() == 1
		g.deps.World.PlayerSetQuickLootFallback(g.player.ID, fallback)
	case 4: // Set managed container (Obtain)
		category := r.GetByte()
		netPos := r.GetPosition()
		pos := game.Position{X: netPos.X, Y: netPos.Y, Z: netPos.Z}
		itemID := r.GetU16()
		stackPos := r.GetByte()
		g.deps.World.PlayerSetManagedContainer(g.player.ID, category, pos, itemID, stackPos, false)
	case 5: // Clear managed container (Obtain)
		category := r.GetByte()
		g.deps.World.PlayerClearManagedContainer(g.player.ID, category, false)
	case 6: // Open managed container (Obtain)
		category := r.GetByte()
		g.deps.World.PlayerOpenManagedContainer(g.player.ID, category, false)
	}
	
	// Send updated containers back to the client
	g.SendLootContainers()
}

// parseQuickLootBlackWhitelist handles opcode 0x91 (Set Skipped / Accepted items list).
func (g *GameProtocol) parseQuickLootBlackWhitelist(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	filter := r.GetByte()
	count := int(r.GetU16())
	var listedItems []uint16
	for i := 0; i < count; i++ {
		listedItems = append(listedItems, r.GetU16())
	}

	g.deps.World.PlayerSetQuickLootFilter(g.player.ID, filter, listedItems)
}

// SendLootContainers sends the client its managed container state.
func (g *GameProtocol) SendLootContainers() {
	if g.player == nil || g.player.Session == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0xC0)
	
	if g.player.QuickLootFallbackToMain {
		w.AddByte(1)
	} else {
		w.AddByte(0)
	}

	// We only loop up to 32 categories (OBJECTCATEGORY_DEFAULT is 31).
	var containerCount uint8 = 0
	
	for i := uint8(0); i <= 31; i++ {
		_, hasLoot := g.player.ManagedContainers[i]
		_, hasObtain := g.player.ManagedObtainContainers[i]
		
		if hasLoot || hasObtain {
			containerCount++
		}
	}

	w.AddByte(containerCount)
	for i := uint8(0); i <= 31; i++ {
		lootID, hasLoot := g.player.ManagedContainers[i]
		obtainID, hasObtain := g.player.ManagedObtainContainers[i]
		
		if !hasLoot && !hasObtain {
			continue
		}
		
		w.AddByte(i) // category

		// Managed containers are stored by their container item id, so report it
		// directly (no map-tile lookup — loot containers live in the inventory).
		w.AddU16(lootID)
		w.AddU16(obtainID)
	}

	g.player.Session.SendToClient(w)
}
