package game

import "sync"

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
		case *Player:
			if s.GetFollowTarget() == c {
				w.StepFollow(s)
			}
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
	case *Player:
		if self.GetFollowTarget() != nil {
			w.StepFollow(self)
		}
	}
}

// EventRegistrar is implemented by creatures that carry a per-creature set of
// registered CreatureEvent names (*Player directly; *Monster and *Npc via
// BaseCreature). It mirrors Creature::registeredEvents in C++: a monster's
// events come from its type's `monster.events` plus any dynamic
// registerEvent/unregisterEvent calls, a player's from login.lua's
// player:registerEvent(...). An event's callback only fires for the creature
// that holds its name.
type EventRegistrar interface {
	RegisterEvent(name string)
	UnregisterEvent(name string)
	HasEvent(name string) bool
	RegisteredEvents() []string
}

// EventRegistrarOf returns the registrar for c, or nil when the creature has
// no per-creature event set.
func EventRegistrarOf(c Creature) EventRegistrar {
	if r, ok := c.(EventRegistrar); ok {
		return r
	}
	return nil
}

// creatureEventSet is the per-creature set of registered event names. It is
// embedded in BaseCreature and Player. It carries its own lock because the
// writers and readers live on different goroutines (Lua on the dispatcher, the
// seed copy in NewMonster on the game thread), and a creature can be registered
// on from a protocol goroutine during login.
type creatureEventSet struct {
	mu    sync.RWMutex
	names map[string]struct{}
}

func (s *creatureEventSet) register(name string) {
	if name == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.names == nil {
		s.names = make(map[string]struct{})
	}
	s.names[name] = struct{}{}
}

func (s *creatureEventSet) unregister(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.names, name)
}

func (s *creatureEventSet) has(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.names[name]
	return ok
}

// snapshot returns a copy of the registered names. The death dispatch loop must
// iterate a snapshot rather than the live map: the Lua handler runs while the
// engine lock is held, and a handler calling registerEvent would deadlock on a
// held set lock.
func (s *creatureEventSet) snapshot() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.names))
	for name := range s.names {
		out = append(out, name)
	}
	return out
}
