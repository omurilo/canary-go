package protocol

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// sendAttachedEffect sends an attached effect notification to the client (opcode 0x34).
func (g *GameProtocol) sendAttachedEffect(creatureID uint32, effectID uint16) {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x34)
	w.AddU32(creatureID)
	w.AddU16(effectID)
	g.player.Session.SendToClient(w)
}

// sendDetachEffect sends a detach effect notification (opcode 0x35).
func (g *GameProtocol) sendDetachEffect(creatureID uint32, effectID uint16) {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x35)
	w.AddU32(creatureID)
	w.AddU16(effectID)
	g.player.Session.SendToClient(w)
}

// sendShader sends a shader change for a creature (opcode 0x36).
func (g *GameProtocol) sendShader(creatureID uint32, shaderName string) {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x36)
	w.AddU32(creatureID)
	w.AddString(shaderName)
	g.player.Session.SendToClient(w)
}

// sendMapShader sends a global map shader (opcode 0x37).
func (g *GameProtocol) sendMapShader(shaderName string) {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x37)
	w.AddString(shaderName)
	g.player.Session.SendToClient(w)
}

// BroadcastAttachedEffect sends an attached effect to all spectators of a creature.
func BroadcastAttachedEffect(world *game.World, creatureID uint32, effectID uint16) {
	if world == nil {
		return
	}
	for _, p := range world.Players() {
		if gp, ok := p.Session.(*GameProtocol); ok {
			gp.sendAttachedEffect(creatureID, effectID)
		}
	}
}

// BroadcastDetachEffect sends a detach effect to all spectators.
func BroadcastDetachEffect(world *game.World, creatureID uint32, effectID uint16) {
	if world == nil {
		return
	}
	for _, p := range world.Players() {
		if gp, ok := p.Session.(*GameProtocol); ok {
			gp.sendDetachEffect(creatureID, effectID)
		}
	}
}

// BroadcastShader sends a creature shader change to all spectators.
func BroadcastShader(world *game.World, creatureID uint32, shaderName string) {
	if world == nil {
		return
	}
	for _, p := range world.Players() {
		if gp, ok := p.Session.(*GameProtocol); ok {
			gp.sendShader(creatureID, shaderName)
		}
	}
}

// BroadcastMapShader sends a map-wide shader to all online players.
func BroadcastMapShader(world *game.World, shaderName string) {
	if world == nil {
		return
	}
	for _, p := range world.Players() {
		if gp, ok := p.Session.(*GameProtocol); ok {
			gp.sendMapShader(shaderName)
		}
	}
}
