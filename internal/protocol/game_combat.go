package protocol

import (
	"fmt"
	"math"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// Combat text-message classes and colors (src/utils/utils_definitions.hpp).
const (
	messageDamageDealt    = 23  // MESSAGE_DAMAGE_DEALT
	messageDamageReceived = 24  // MESSAGE_DAMAGE_RECEIVED
	textcolorWhite        = 215 // TEXTCOLOR_WHITE_EXP (experience gain)
	textcolorRed          = 180 // TEXTCOLOR_RED (physical/blood damage)
	textcolorNone         = 255 // TEXTCOLOR_NONE
)

// sendCreatureHealth mirrors ProtocolGame::sendCreatureHealth
// (src/server/network/protocol/protocolgame.cpp:8212): 0x8C, u32 creatureId,
// then a health-percent byte = min(100, ceil(health / max(maxHealth,1) * 100)).
func (g *GameProtocol) sendCreatureHealth(c game.Creature) {
	maxHealth := c.GetMaxHealth()
	if maxHealth < 1 {
		maxHealth = 1
	}
	pct := math.Ceil(float64(c.GetHealth()) / float64(maxHealth) * 100)
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}

	w := netmsg.NewWriter()
	w.AddByte(opCreatureHealth)
	w.AddU32(c.GetID())
	w.AddByte(byte(pct))
	g.SendToClient(w)
}

// sendDamageText mirrors the MESSAGE_DAMAGE_* branch of
// ProtocolGame::sendTextMessage (src/server/network/protocol/protocolgame.cpp:6040):
// 0xB4, message class byte, victim position, u32 primary value, primary color
// byte, u32 secondary value, secondary color byte, then the text.
func (g *GameProtocol) sendDamageText(class byte, pos game.Position, value uint32, color byte, text string) {
	w := netmsg.NewWriter()
	w.AddByte(opTextMessage)
	w.AddByte(class)
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddU32(value)          // primary.value
	w.AddByte(color)         // primary.color
	w.AddU32(0)              // secondary.value
	w.AddByte(textcolorNone) // secondary.color
	w.AddString(text)
	g.SendToClient(w)
}

// sendExpText mirrors the MESSAGE_EXPERIENCE branch of ProtocolGame::sendTextMessage
// (0xB4, class 26, pos, uint64 value, color white, text string).
func (g *GameProtocol) sendExpText(pos game.Position, value uint64, color byte, text string) {
	w := netmsg.NewWriter()
	w.AddByte(opTextMessage)
	w.AddByte(26) // MESSAGE_EXPERIENCE
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddU64(value)
	w.AddByte(color)
	if text == "" {
		if value == 1 {
			text = "You gained 1 experience point."
		} else {
			text = fmt.Sprintf("You gained %d experience points.", value)
		}
	}
	w.AddString(text)
	g.SendToClient(w)
}

// SendExpMessage sends an experience gained message + animated text to the player client.
func SendExpMessage(p *game.Player, exp uint64, text string) {
	if gp, ok := p.Session.(*GameProtocol); ok {
		gp.sendExpText(p.GetPosition(), exp, textcolorWhite, text)
	}
}

// sendTileAddItem tells this client an item appeared on top of a tile
// (0x6A TileAddThing), mirroring the internalAddItem broadcast used when a
// corpse is dropped in Creature::dropCorpse (src/creatures/creature.cpp).
func (g *GameProtocol) sendTileAddItem(pos game.Position, item *game.Item) {
	g.sendAddTileItem(pos, g.stackPosOfItem(pos, item), item)
}

// BroadcastAddItem tells every spectator an item appeared on a tile (used for
// dropped corpses).
func BroadcastAddItem(w *game.World, pos game.Position, item *game.Item) {
	for _, s := range w.Spectators(pos, 0) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.sendTileAddItem(pos, item)
		}
	}
}

// BroadcastItemDecay tells every spectator an item transformed due to decay.
func BroadcastItemDecay(w *game.World, pos game.Position, stackPos uint8, oldItem, newItem *game.Item) {
	for _, s := range w.Spectators(pos, 0) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.sendUpdateTileThing(pos, stackPos, newItem)
		}
	}
}

// SendCancelTarget clears the attack target on a player's client (0xA3), used
// when the current target dies or leaves.
func SendCancelTarget(p *game.Player) {
	if gp, ok := p.Session.(*GameProtocol); ok {
		gp.sendCancelTarget()
	}
}

// SendPlayerStats pushes a fresh stats packet (0xA0) to the player, used after
// experience/level changes on a monster kill. Mirrors Player::sendStats
// (src/creatures/players/player.cpp).
func SendPlayerStats(p *game.Player) {
	if gp, ok := p.Session.(*GameProtocol); ok {
		w := netmsg.NewWriter()
		gp.addStats(w)
		gp.SendToClient(w)
	}
}

// SendPartyShield sends `viewer` a party-shield packet (0x91) for `target`,
// computed from viewer.PartyShield(target). Mirrors
// ProtocolGame::sendCreatureShield.
func SendPartyShield(viewer, target *game.Player) {
	if viewer == nil || target == nil {
		return
	}
	gp, ok := viewer.Session.(*GameProtocol)
	if !ok {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x91)
	w.AddU32(target.ID)
	w.AddByte(viewer.PartyShield(target))
	gp.SendToClient(w)
}

// ucfirst upper-cases the first byte of s (matching C++ ucfirst for names).
func ucfirst(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}
