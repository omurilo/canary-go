package game

// The Npc:: event handlers and spectator bookkeeping, ported from
// src/creatures/npcs/npc.cpp.
//
// An NPC's whole behaviour is gated on one thing the port never tracked: the
// set of players who can currently see it. Npc::onThink runs yell, walk and
// sound only when that set is non-empty, manageIdle takes the NPC off the check
// list when it empties, and handlePlayerMove closes a conversation the moment
// the player walks out of range.
//
// Without it, every NPC on the map thought every tick whether or not anyone was
// there, and a player could walk to the other side of town with a trade window
// still open.

// interactionRange is the default `range` of Npc::canInteract (npc.cpp:535): a
// player further than this cannot talk or trade, and an open channel closes.
const interactionRange = 4

// CanInteract is Npc::canInteract (npc.cpp:535). The floor check is separate
// from the range check — a player directly above or below is out of reach no
// matter how close they look on the horizontal.
func (n *Npc) CanInteract(pos Position, rng int) bool {
	if rng <= 0 {
		rng = interactionRange
	}
	if pos.Z != n.GetPosition().Z {
		return false
	}
	return canSeeWithin(n.GetPosition(), pos, rng, rng)
}

// canSeeWithin is Creature::canSee with an explicit range, which is what
// canInteract passes rather than the full viewport.
func canSeeWithin(from, to Position, rangeX, rangeY int) bool {
	return abs(int(from.X)-int(to.X)) <= rangeX && abs(int(from.Y)-int(to.Y)) <= rangeY
}

// IsAttackable is Npc::isAttackable (npc.cpp:527): never, unconditionally.
func (n *Npc) IsAttackable() bool { return false }

// IsPushable is Npc::isPushable (npc.cpp:523).
func (n *Npc) IsPushable() bool { return n.Type != nil && n.Type.IsPushable }

// CanSeeInvisibility is Npc::canSeeInvisibility (npc.cpp:542): always true, so
// an invisible player cannot shop unseen.
func (n *Npc) CanSeeInvisibility() bool { return true }

// GetSpeechBubble is Npc::getSpeechBubble (npc.cpp:496).
func (n *Npc) GetSpeechBubble() uint8 { return n.SpeechBubble() }

// SetSpeechBubble is Npc::setSpeechBubble (npc.cpp:500). It writes to the type,
// as upstream does — the bubble is a property of the NPC kind, not the instance.
func (n *Npc) SetSpeechBubble(bubble uint8) {
	if n.Type != nil {
		n.Type.SpeechBubble = bubble
	}
}

// GetCurrency is Npc::getCurrency (npc.cpp:504).
func (n *Npc) GetCurrency() uint16 { return n.CurrencyID() }

// SetCurrency is Npc::setCurrency (npc.cpp:508).
func (n *Npc) SetCurrency(currency uint16) {
	if n.Type != nil {
		n.Type.CurrencyID = currency
	}
}

// SetNormalCreatureLight is Npc::setNormalCreatureLight (npc.cpp:1226).
func (n *Npc) SetNormalCreatureLight() {
	if n.Type == nil {
		return
	}
	n.LightLevel = n.Type.LightLevel
	n.LightColor = n.Type.LightColor
}

// ---------------------------------------------------------------------------
// Player spectators and idleness
// ---------------------------------------------------------------------------

// OnPlayerAppear is Npc::onPlayerAppear (npc.cpp:654). A player carrying the
// ignored-by-npcs flag is never added, which is what keeps a GM from waking
// every NPC they walk past.
func (n *Npc) OnPlayerAppear(p *Player) {
	if p == nil || p.CannotBeAttacked() {
		return
	}
	if n.spectators == nil {
		n.spectators = make(map[uint32]*Player)
	}
	if _, exists := n.spectators[p.GetID()]; exists {
		return
	}
	n.spectators[p.GetID()] = p
	n.ManageIdle()
}

// OnPlayerDisappear is Npc::onPlayerDisappear (npc.cpp:663). The conversation
// ends first, then the spectator is dropped — the order matters, because
// removePlayerInteraction needs the player still to be known.
func (n *Npc) OnPlayerDisappear(p *Player) {
	if p == nil {
		return
	}
	n.RemovePlayerInteraction(p.GetID())
	if _, exists := n.spectators[p.GetID()]; exists {
		delete(n.spectators, p.GetID())
		n.ManageIdle()
	}
}

// ManageIdle is Npc::manageIdle (npc.cpp:646): an NPC with nobody watching is
// taken off the creature check list, and put back on the moment someone
// arrives. It is the difference between 1033 NPCs thinking every second and
// only the handful a player is standing next to.
func (n *Npc) ManageIdle() { n.idle = len(n.spectators) == 0 }

// IsIdle reports whether the NPC is off the check list.
func (n *Npc) IsIdle() bool { return n.idle }

// HasPlayerSpectators reports whether anyone can currently see the NPC.
func (n *Npc) HasPlayerSpectators() bool { return len(n.spectators) > 0 }

// LoadPlayerSpectators is Npc::loadPlayerSpectators (npc.cpp:1106), run once
// when the NPC is placed so it starts with an accurate view rather than waiting
// for the first player to move.
func (n *Npc) LoadPlayerSpectators(w *World) {
	if w == nil {
		return
	}
	n.spectators = make(map[uint32]*Player)
	for _, p := range w.Spectators(n.GetPosition(), n.GetID()) {
		if p == nil || p.CannotBeAttacked() {
			continue
		}
		n.spectators[p.GetID()] = p
	}
	n.ManageIdle()
}

// OnPlacedCreature is Npc::onPlacedCreature (npc.cpp:1102).
func (n *Npc) OnPlacedCreature(w *World) { n.LoadPlayerSpectators(w) }

// OnCreatureWalk is Npc::onCreatureWalk (npc.cpp:1094): the NPC itself moved,
// so anyone it can no longer see stops being a spectator. Without this an NPC
// that wanders keeps a stale list and never goes idle.
func (n *Npc) OnCreatureWalk() {
	for id, p := range n.spectators {
		if p == nil || !n.GetPosition().InRangeOf(p.GetPosition()) {
			delete(n.spectators, id)
		}
	}
	n.ManageIdle()
}

// ---------------------------------------------------------------------------
// Creature events
// ---------------------------------------------------------------------------

// OnCreatureAppear is Npc::onCreatureAppear (npc.cpp:577).
func (n *Npc) OnCreatureAppear(w *World, c Creature) {
	if p, ok := c.(*Player); ok {
		n.OnPlayerAppear(p)
	}
}

// OnRemoveCreature is Npc::onRemoveCreature (npc.cpp:596). A departing player
// loses its shop registration as well as its conversation, otherwise the NPC
// keeps serving a stale per-player shop list to whoever reuses the guid.
func (n *Npc) OnRemoveCreature(w *World, c Creature) {
	p, ok := c.(*Player)
	if !ok {
		return
	}
	n.RemoveShopPlayer(p.DBID)
	n.OnPlayerDisappear(p)
}

// OnCreatureMove is Npc::onCreatureMove (npc.cpp:620).
//
// The first branch is about the NPC's own movement: an NPC that wandered out of
// interaction range of where it was drops every conversation and shop window.
// Upstream checks the OLD position, not the new one — the question is "did I
// leave the place I was talking from", not "where am I now".
func (n *Npc) OnCreatureMove(w *World, c Creature, oldPos, newPos Position) {
	if c != nil && c.GetID() == n.GetID() {
		if !n.CanInteract(oldPos, interactionRange) {
			n.resetPlayerInteractions()
			n.CloseAllShopWindows(w)
		}
		n.OnCreatureWalk()
		return
	}
	if p, ok := c.(*Player); ok {
		n.HandlePlayerMove(w, p, newPos)
	}
}

// HandlePlayerMove is Npc::handlePlayerMove (npc.cpp:1258): the two questions
// asked of every player step near an NPC.
//
// They are separate and both matter. Out of interaction range closes the
// channel but the player may still be visible; out of sight drops them from the
// spectator list, which is what lets the NPC go idle. And the NPC turns to
// follow only the player it is actually talking to — the last one to start a
// conversation — rather than swivelling at every passer-by.
func (n *Npc) HandlePlayerMove(w *World, p *Player, newPos Position) {
	if p == nil {
		return
	}
	if !n.CanInteract(newPos, interactionRange) {
		n.OnPlayerCloseChannel(w, p)
	} else if last, ok := n.lastInteraction(); ok && last == p.GetID() {
		n.TurnToCreature(p)
		if w != nil && w.OnCreatureTurn != nil {
			w.OnCreatureTurn(n)
		}
	}

	if n.GetPosition().InRangeOf(newPos) {
		n.OnPlayerAppear(p)
	} else {
		n.OnPlayerDisappear(p)
	}
}

// OnCreatureSay is Npc::onCreatureSay (npc.cpp:670). Only players are heard;
// upstream returns before the callback for anything else.
func (n *Npc) OnCreatureSay(w *World, speaker Creature, talkType byte, text string) {
	if _, ok := speaker.(*Player); !ok {
		return
	}
	if w != nil && w.OnNpcCreatureSay != nil {
		w.OnNpcCreatureSay(n, speaker, talkType, text)
	}
}

// OnPlayerCloseChannel is Npc::onPlayerCloseChannel (npc.cpp:1026). The script
// callback runs first and may keep the conversation alive; the interaction is
// only dropped once it has had its say.
func (n *Npc) OnPlayerCloseChannel(w *World, p *Player) {
	if p == nil {
		return
	}
	if w != nil && w.OnNpcCloseChannel != nil {
		w.OnNpcCloseChannel(n, p)
	}
	n.RemovePlayerInteraction(p.GetID())
}
