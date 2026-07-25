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

	g.player.Session.OpenContainer(chest)
}
