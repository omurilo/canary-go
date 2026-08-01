package protocol

import (
	"strings"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/game/vocations"
	"github.com/omurilo/canary-go/internal/luaengine"
	"github.com/omurilo/canary-go/internal/netmsg"
	"github.com/omurilo/canary-go/internal/spells"
)

// Spell-cast failure message class and effect constants.
const (
	messageFailure  = 0x15 // MESSAGE_FAILURE wire value used by ProtocolGame
	spellEffectPoff = 3    // CONST_ME_POFF (failed/blocked cast)

	opSpellCooldown      = 0xA4 // ProtocolGame::sendSpellCooldown
	opSpellGroupCooldown = 0xA5 // ProtocolGame::sendSpellGroupCooldown
)

// tryCastSpell attempts to interpret text as an instant spell. It returns true
// if the text matched a spell (so the caller must NOT broadcast it as chat).
// Mirrors Spells::playerSaySpell -> InstantSpell::playerCastInstant ->
// Spell::playerSpellCheck (src/creatures/combat/spells.cpp).
func (g *GameProtocol) tryCastSpell(talkType byte, text string) bool {
	g.deps.Log.Debug("tryCastSpell checking words", "text", text)
	sp := spells.FindByWords(text)
	if sp == nil {
		g.deps.Log.Debug("tryCastSpell words did not match any registered spells", "text", text)
		return false
	}
	g.deps.Log.Debug("tryCastSpell matched spell words", "spell", sp.Name, "words", sp.Words)
	p := g.player

	if !g.spellCastCheck(sp) {
		g.deps.Log.Debug("tryCastSpell failed spellCastCheck", "spell", sp.Name)
		return true // matched a spell but failed a precondition
	}

	// Build the LuaVariant from the spell's targeting mode
	// (InstantSpell::playerCastInstant, spells.cpp:1144).
	vtype, targetID, pos, ok := g.buildSpellVariant(sp)
	if !ok {
		g.deps.Log.Debug("tryCastSpell failed to build spell variant", "spell", sp.Name)
		return true
	}

	// Execute the Lua onCastSpell, which runs the real combat.
	g.deps.Log.Debug("tryCastSpell executing onCastSpell", "spell", sp.Name, "vtype", vtype, "targetID", targetID, "pos", pos)
	success := g.deps.Lua.RunSpell(sp, p, vtype, targetID, pos)
	g.deps.Log.Debug("tryCastSpell onCastSpell returned", "spell", sp.Name, "success", success)

	// postCastSpell: spend mana, apply cooldowns (spells.cpp:876,795).
	if cost := spellManaCost(sp, p); cost > 0 {
		p.AddMana(-int32(cost))
	}
	if sp.Soul > 0 && p.Soul >= uint8(sp.Soul) {
		p.Soul -= uint8(sp.Soul)
	}
	g.applySpellCooldowns(sp)

	// Refresh the caster's own stat bars (mana/soul), mirroring the 0xA0 stats
	// packet sent after Player::changeMana (src/creatures/players/player.cpp).
	statsMsg := netmsg.NewWriter()
	g.addStats(statsMsg)
	g.SendToClient(statsMsg)

	// On success the spell words are spoken to spectators
	// (Game::playerSaySpell -> Player::saySpell, game.cpp:7475).
	g.broadcastSay(p, talkType, sp.Words, "")
	return true
}

// spellCastCheck mirrors the subset of Spell::playerSpellCheck (spells.cpp:476)
// that the Go server can evaluate. On failure it sends the cancel message and
// the POFF effect, then returns false.
func (g *GameProtocol) spellCastCheck(sp *spells.Spell) bool {
	p := g.player

	if !sp.Enabled {
		return false
	}

	if sp.Aggressive && p.IsInProtectionZone() {
		g.failCast("A protection zone blocks this action.")
		return false
	}

	cds := p.Cooldowns()
	if cds.HasGroupCooldown(uint32(sp.Group)) || cds.HasCooldown(sp.SpellID) ||
		(sp.SecondaryGroup != spells.SpellGroupNone && cds.HasGroupCooldown(uint32(sp.SecondaryGroup))) {
		g.failCast("You are exhausted.")
		return false
	}

	if int(p.Level) < sp.Level {
		g.failCast("You do not have enough level.")
		return false
	}
	if int(p.MagLevel) < sp.MagicLevel {
		g.failCast("You do not have enough magic level.")
		return false
	}
	if int(p.Mana) < spellManaCost(sp, p) {
		g.failCast("You do not have enough mana.")
		return false
	}
	if p.Soul < uint8(sp.Soul) {
		g.failCast("You do not have enough soul.")
		return false
	}

	// LEARN_SPELLS is off by default: only spells flagged needLearn require the
	// player to have learned them; otherwise a vocation restriction applies.
	if sp.NeedLearn {
		if !p.HasLearnedSpell(sp.Name) {
			g.failCast("You need to learn this spell first.")
			return false
		}
	}
	if len(sp.VocationNames) > 0 {
		voc := vocations.GetVocation(uint32(p.Vocation))
		if voc == nil {
			g.failCast("Your vocation cannot cast this spell.")
			return false
		}
		allowed := false
		playerVocName := strings.ToLower(voc.Name)
		for _, name := range sp.VocationNames {
			if name == playerVocName {
				allowed = true
				break
			}
		}
		if !allowed {
			g.failCast("Your vocation cannot cast this spell.")
			return false
		}
	}

	return true
}

// buildSpellVariant resolves the spell's target into a LuaVariant, mirroring
// InstantSpell::playerCastInstant (spells.cpp:1144). Returns ok=false when a
// required target is missing (the cancel message is already sent).
func (g *GameProtocol) buildSpellVariant(sp *spells.Spell) (luaengine.LuaVariantType, uint32, game.Position, bool) {
	p := g.player

	if sp.SelfTarget {
		return luaengine.VariantNumber, p.ID, p.Pos, true
	}

	if sp.NeedTarget || sp.CasterTargetOrDirection {
		target := g.deps.World.CreatureByID(p.TargetID)
		if target == nil || target.GetHealth() == 0 {
			if !sp.CasterTargetOrDirection {
				g.failCast("You can only use this on creatures.")
				return 0, 0, game.Position{}, false
			}
			// Fall back to casting in the facing direction.
			return luaengine.VariantPosition, 0, p.Pos.Offset(p.Direction), true
		}
		// Reachability: same floor (basic canThrowSpell, spells.cpp:1270).
		if target.GetPosition().Z != p.Pos.Z {
			g.failCast("Creature is not reachable.")
			return 0, 0, game.Position{}, false
		}
		return luaengine.VariantNumber, target.GetID(), p.Pos, true
	}

	// Positional / directional spells (spells.cpp:1237).
	pos := p.Pos
	if sp.NeedDirection {
		pos = p.Pos.Offset(p.Direction)
	}
	return luaengine.VariantPosition, 0, pos, true
}

// applySpellCooldowns starts the spell/group cooldowns and pushes the cooldown
// packets, mirroring Spell::applyCooldownConditions + Player::sendSpellCooldown
// (spells.cpp:795, protocolgame.cpp:9313).
func (g *GameProtocol) applySpellCooldowns(sp *spells.Spell) {
	cds := g.player.Cooldowns()
	if sp.Cooldown > 0 {
		cds.AddCooldown(sp.SpellID, sp.Cooldown)
		g.sendSpellCooldown(sp.SpellID, sp.Cooldown)
	}
	if sp.GroupCooldown > 0 {
		cds.AddGroupCooldown(uint32(sp.Group), sp.GroupCooldown)
		g.sendSpellGroupCooldown(uint8(sp.Group), sp.GroupCooldown)
	}
	if sp.SecondaryGroupCooldown > 0 && sp.SecondaryGroup != spells.SpellGroupNone {
		cds.AddGroupCooldown(uint32(sp.SecondaryGroup), sp.SecondaryGroupCooldown)
		g.sendSpellGroupCooldown(uint8(sp.SecondaryGroup), sp.SecondaryGroupCooldown)
	}
}

// spellManaCost mirrors Spell::getManaCost (spells.cpp:925): a flat mana cost, or
// a percentage of max mana when manaPercent is set.
func spellManaCost(sp *spells.Spell, p *game.Player) int {
	if sp.Mana != 0 {
		return sp.Mana
	}
	if sp.ManaPercent != 0 {
		return int(p.GetMaxMana()) * sp.ManaPercent / 100
	}
	return 0
}

// failCast sends the failure text plus the POFF effect at the caster, mirroring
// Player::sendCancelMessage + g_game().addMagicEffect(pos, CONST_ME_POFF).
func (g *GameProtocol) failCast(text string) {
	w := netmsg.NewWriter()
	w.AddByte(opTextMessage)
	w.AddByte(messageFailure)
	w.AddString(text)
	g.SendToClient(w)
	g.sendMagicEffect(g.player.Pos, spellEffectPoff)
}

// sendSpellCooldown mirrors ProtocolGame::sendSpellCooldown (protocolgame.cpp:9313):
// 0xA4, u16 spellId, u32 time(ms).
func (g *GameProtocol) sendSpellCooldown(spellID uint16, timeMs uint32) {
	w := netmsg.NewWriter()
	w.AddByte(opSpellCooldown)
	w.AddU16(spellID)
	w.AddU32(timeMs)
	g.SendToClient(w)
}

// sendSpellGroupCooldown mirrors ProtocolGame::sendSpellGroupCooldown
// (protocolgame.cpp:9333): 0xA5, byte groupId, u32 time(ms).
func (g *GameProtocol) sendSpellGroupCooldown(groupID uint8, timeMs uint32) {
	w := netmsg.NewWriter()
	w.AddByte(opSpellGroupCooldown)
	w.AddByte(groupID)
	w.AddU32(timeMs)
	g.SendToClient(w)
}
