package protocol

import "github.com/opentibiabr/canary-go/internal/game"

// HandlePlayerDeath runs the client-facing side of a player death: it announces
// the death, teleports the player to their temple, and refreshes stats. The
// model-side penalty (experience/level loss, vitals refill, condition strip)
// has already been applied by game.Player.ApplyDeathPenalty before this runs.
//
// This is the pragmatic "instant respawn" flow: rather than the 0x28 relogin
// dialog (which requires a full relog handshake), the player is moved straight
// to the temple alive, which is non-freezing and immediately testable. The
// 0x28 death window is a later refinement.
func HandlePlayerDeath(world *game.World, p *game.Player, killer game.Creature) {
	if p == nil {
		return
	}
	temple := p.TemplePosition()
	if temple.X == 0 && temple.Y == 0 {
		temple = world.DefaultSpawn
	}

	p.SendTextMessage(messageStatus, "You are dead.")

	if gp, ok := p.Session.(*GameProtocol); ok {
		gp.teleport(temple)
		gp.sendStats()
	} else {
		// Headless / no session: just relocate the model.
		world.SetPosition(p, temple)
	}
}
