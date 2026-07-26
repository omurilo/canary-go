package protocol

import "github.com/opentibiabr/canary-go/internal/netmsg"

// parseOpenRewardChest handles client request to open the reward chest (Opcode 0xD0).
func (g *GameProtocol) parseOpenRewardChest(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	chest := g.player.RewardChest
	if chest == nil {
		return
	}

	// In Tibia 13+, the client automatically sends 0xD0 on login to request the Daily Reward Wall UI.
	// Replying with a 0x6E physical container opens a "ghost" Reward Chest window for the player.
	// If they actually right-click a Reward Chest on the map, parseUseItem (0x82) will handle opening the container.
	// For now, we do nothing on 0xD0 until the Daily Reward Wall UI is implemented.
	// g.player.Session.OpenContainer(chest)
}
