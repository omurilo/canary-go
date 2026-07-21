package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// sendReLoginWindow sends the death dialog (0x28), mirroring
// ProtocolGame::sendReLoginWindow. The client shows "You are dead." with a
// button that reconnects the character, which then logs in at the temple.
// Layout (modern): byte 0x28; byte 0x00 (>=1055); unfairFightReduction byte;
// byte 0x00 death-redemption (>=1121).
func (g *GameProtocol) sendReLoginWindow(unfairFightReduction uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0x28)
	w.AddByte(0x00)
	w.AddByte(unfairFightReduction)
	w.AddByte(0x00)
	g.SendToClient(w)
}

// HandlePlayerDeath runs the client-facing side of a player death, matching the
// C++ flow: the model-side penalty (experience/level loss, vitals refill,
// condition strip) has already been applied by game.Player.ApplyDeathPenalty.
// Here we announce the death, send the relogin window, move the character to
// their temple (so the death save and the subsequent relogin place them there),
// and despawn them from the world so the client's reconnect succeeds.
func HandlePlayerDeath(world *game.World, p *game.Player, killer game.Creature) {
	if p == nil {
		return
	}
	// TemplePosition is the LoginPosition resolved (and walkability-checked)
	// from the OTBM town at enter-world; fall back to the default spawn only if
	// the tile went missing.
	temple := p.TemplePosition()
	if world.Map.GetTile(temple) == nil {
		temple = world.DefaultSpawn
	}

	if gp, ok := p.Session.(*GameProtocol); ok {
		gp.player.SendTextMessage(messageStatus, "You are dead.")
		// PvE deaths carry no unfair-fight reduction (100%).
		gp.sendReLoginWindow(100)
	}

	// Despawn at the death location (broadcasts the removal to spectators) and
	// only then relocate the model to the temple, so the tile bookkeeping and
	// the persisted position are both correct. The client shows the death
	// window and, on the button click, reconnects and logs in at the temple.
	world.RemovePlayer(p.ID)
	p.Pos = temple
	p.Dead = false
}
