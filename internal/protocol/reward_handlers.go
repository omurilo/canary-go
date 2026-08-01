package protocol

import "github.com/omurilo/canary-go/internal/netmsg"

// parseOpenRewardChest handles client request to open the reward chest (Opcode 0xD0).
func (g *GameProtocol) parseOpenRewardChest(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	chest := g.player.RewardChest
	if chest == nil {
		return
	}

	// In Tibia 13+, the client automatically sends 0xD0 on login to request
	// the Daily Reward Wall UI. Replying with a 0x6E physical container opens
	// a "ghost" Reward Chest window for the player, allowing them to view
	// boss loot deposited into the reward chest.
	if g.player.Session != nil {
		g.player.Session.OpenContainer(chest)
	}
}
