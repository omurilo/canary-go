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
	w.AddU32(value)   // primary.value
	w.AddByte(color)  // primary.color
	w.AddU32(0)       // secondary.value
	w.AddByte(textcolorNone) // secondary.color
	w.AddString(text)
	g.SendToClient(w)
}

// sendTileAddItem tells this client an item appeared on top of a tile
// (0x6A TileAddThing), mirroring the internalAddItem broadcast used when a
// corpse is dropped in Creature::dropCorpse (src/creatures/creature.cpp).
func (g *GameProtocol) sendTileAddItem(pos game.Position, item *game.Item) {
	stack := 0
	if t := g.deps.World.Map.GetTile(pos); t != nil {
		if t.Ground != nil {
			stack++
		}
		if len(t.Items) > 0 {
			stack += len(t.Items) - 1 // the item is already in the stack
		}
		stack += len(t.Creatures)
	}
	if stack < 0 {
		stack = 0
	}
	w := netmsg.NewWriter()
	w.AddByte(0x6A) // TileAddThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(byte(stack))
	g.addItem(w, item)
	g.SendToClient(w)
}

// BroadcastCreatureHealth sends a health-bar update for c to every player who
// can see it (item #3 of the combat migration).
func BroadcastCreatureHealth(w *game.World, c game.Creature) {
	for _, s := range w.Spectators(c.GetPosition(), 0) {
		if gp, ok := s.Session.(*GameProtocol); ok && gp.known[c.GetID()] {
			gp.sendCreatureHealth(c)
		}
	}
}

// BroadcastCombatHit shows the impact magic effect at the victim and the
// animated damage text to the participants (attacker sees MESSAGE_DAMAGE_DEALT,
// victim sees MESSAGE_DAMAGE_RECEIVED), mirroring Game::sendEffects /
// Game::buildMessageAsAttacker / buildMessageAsTarget (src/game/game.cpp).
func BroadcastCombatHit(w *game.World, attacker, victim game.Creature, damage int32, effect uint16) {
	pos := victim.GetPosition()

	// Impact effect for everyone who can see the tile.
	for _, s := range w.Spectators(pos, 0) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.sendMagicEffect(pos, effect)
		}
	}

	if damage <= 0 {
		return
	}

	unit := "hitpoints"
	if damage == 1 {
		unit = "hitpoint"
	}
	dmgStr := fmt.Sprintf("%d %s", damage, unit)

	// Attacker (if a player): "NAME loses X hitpoints due to your attack."
	if ap, ok := attacker.(*game.Player); ok {
		if gp, ok := ap.Session.(*GameProtocol); ok {
			text := fmt.Sprintf("%s loses %s due to your attack.", ucfirst(victim.GetName()), dmgStr)
			gp.sendDamageText(messageDamageDealt, pos, uint32(damage), textcolorRed, text)
		}
	}

	// Victim (if a player): "You lose X hitpoints due to an attack by NAME."
	if vp, ok := victim.(*game.Player); ok {
		if gp, ok := vp.Session.(*GameProtocol); ok {
			text := fmt.Sprintf("You lose %s due to an attack by %s.", dmgStr, attacker.GetName())
			gp.sendDamageText(messageDamageReceived, pos, uint32(damage), textcolorRed, text)
		}
	}
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

// SendCancelTarget clears the attack target on a player's client (0xA3), used
// when the current target dies or leaves.
func SendCancelTarget(p *game.Player) {
	if gp, ok := p.Session.(*GameProtocol); ok {
		gp.sendCancelTarget()
	}
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
