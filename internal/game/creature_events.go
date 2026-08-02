package game

// Spectator notification: telling the creatures around a change that it
// happened. This is Game::internalPlaceCreature / removeCreature /
// map.moveCreature calling onCreatureAppear, onRemoveCreature and
// onCreatureMove on every spectator (src/game/game.cpp).
//
// The Monster:: and Npc:: handlers for those events were all ported and none of
// them was reachable — nothing called them. A monster's target list was only
// ever corrected by its own periodic sweep, and an NPC never learned that
// someone walked up to it until the next tick recomputed everything.
//
// Doing it here rather than at each call site is what upstream does: the
// notification belongs to the world mutation, not to whoever asked for it.

// notifyCreatureAppear tells everyone who can see pos that c arrived.
//
// The creature is notified about ITSELF too — Monster::onCreatureAppear checks
// for that case and rebuilds its whole view from it, which is how a freshly
// spawned monster gets a target list at all.
func (w *World) notifyCreatureAppear(c Creature) {
	if c == nil {
		return
	}
	for _, spectator := range w.SpectatorCreatures(c.GetPosition()) {
		switch s := spectator.(type) {
		case *Monster:
			s.OnCreatureAppear(w, c)
		case *Npc:
			s.OnCreatureAppear(w, c)
		}
	}
	switch self := c.(type) {
	case *Monster:
		self.OnCreatureAppear(w, self)
	case *Npc:
		self.OnPlacedCreature(w)
	}
}

// notifyCreatureRemove tells everyone who could see c that it is gone.
//
// The spectator list is taken BEFORE the creature leaves the tile, which is why
// this is called from inside RemoveCreature rather than after it: afterwards
// there is no position to gather spectators around.
func (w *World) notifyCreatureRemove(c Creature) {
	if c == nil {
		return
	}
	for _, spectator := range w.SpectatorCreatures(c.GetPosition()) {
		if spectator == c {
			continue
		}
		switch s := spectator.(type) {
		case *Monster:
			s.OnRemoveCreature(w, c)
		case *Npc:
			s.OnRemoveCreature(w, c)
		}
	}
}

// notifyCreatureMove tells the creatures around both ends of a step.
//
// Both ends matter and they are different sets: whoever the mover left behind
// has to drop it from their lists, and whoever it arrived next to has to pick it
// up. Notifying only the destination leaves stale entries behind the mover.
func (w *World) notifyCreatureMove(c Creature, oldPos, newPos Position) {
	if c == nil {
		return
	}
	seen := make(map[uint32]bool)
	notify := func(spectator Creature) {
		if spectator == nil || spectator == c || seen[spectator.GetID()] {
			return
		}
		seen[spectator.GetID()] = true
		switch s := spectator.(type) {
		case *Monster:
			s.OnCreatureMove(w, c, oldPos, newPos)
		case *Npc:
			s.OnCreatureMove(w, c, oldPos, newPos)
		}
	}
	for _, spectator := range w.SpectatorCreatures(oldPos) {
		notify(spectator)
	}
	for _, spectator := range w.SpectatorCreatures(newPos) {
		notify(spectator)
	}

	// The mover's own view changed too. A wandering NPC drops the spectators it
	// can no longer see; a monster re-sorts everyone around its new position.
	switch self := c.(type) {
	case *Npc:
		self.OnCreatureWalk()
	case *Monster:
		self.OnCreatureMove(w, self, oldPos, newPos)
	}
}
