package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseQuickLoot handles opcode 0x8F (Quick Loot request).
func (g *GameProtocol) parseQuickLoot(r *netmsg.Reader) {
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
	action := r.GetByte()
	switch action {
	case 0: // Set managed container
		category := r.GetByte()
		netPos := r.GetPosition()
		pos := game.Position{X: netPos.X, Y: netPos.Y, Z: netPos.Z}
		itemID := r.GetU16()
		stackPos := r.GetByte()
		g.deps.World.PlayerSetLootContainer(g.player.ID, category, pos, itemID, stackPos, true)
	case 1: // Clear managed container
		category := r.GetByte()
		g.deps.World.PlayerSetLootContainer(g.player.ID, category, game.Position{}, 0, 0, false)
	case 3: // Set fallback to main container
		fallback := r.GetByte() == 1
		g.deps.World.PlayerSetQuickLootFallback(g.player.ID, fallback)
	}
}

// parseQuickLootBlackWhitelist handles opcode 0x91 (Set Skipped / Accepted items list).
func (g *GameProtocol) parseQuickLootBlackWhitelist(r *netmsg.Reader) {
	filter := r.GetByte()
	count := int(r.GetU16())
	var listedItems []uint16
	for i := 0; i < count; i++ {
		listedItems = append(listedItems, r.GetU16())
	}

	g.deps.World.PlayerSetQuickLootFilter(g.player.ID, filter, listedItems)
}
