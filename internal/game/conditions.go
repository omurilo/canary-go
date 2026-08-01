package game

import (
	"sync"

	"github.com/omurilo/canary-go/internal/game/combat"
)

// conditionStore is an embeddable, concurrency-safe holder for the combat
// conditions applied to a creature (poison, haste, etc.). It mirrors, at a
// minimal level, the CreatureCondition list in src/creatures/creature.cpp.
//
// It is intentionally decoupled from the game.Creature interface: only the
// concrete creature types embed it, and the combat adapter reaches it through
// the conditionHolder interface. This keeps game.Creature unchanged while still
// giving the combat/spells code a real place to store conditions.
type conditionStore struct {
	condMu     sync.Mutex
	conditions []combat.Condition
}

// AddCondition stores (or refreshes) a condition on the creature. If a
// condition of the same type already exists its duration is extended, matching
// Creature::addCondition's merge behaviour.
func (s *conditionStore) AddCondition(creature combat.Creature, c combat.Condition) {
	if c == nil {
		return
	}
	// IMPORTANT: never call a condition callback (StartCondition / merge
	// AddCondition / EndCondition / ExecuteCondition) while holding condMu.
	// Those callbacks re-enter the condition store (e.g. Haste recomputes speed
	// by iterating conditions) and/or take the Lua engine / world locks, which
	// creates a lock cycle: a spell adding a condition holds e.mu and wants
	// condMu, while the combat tick holds condMu and wants e.mu -> deadlock that
	// freezes the whole combat-tick goroutine. So we only mutate the slice under
	// the lock and run every callback after releasing it.
	s.condMu.Lock()
	var merge combat.Condition
	for _, existing := range s.conditions {
		if existing.GetType() == c.GetType() {
			merge = existing
			break
		}
	}
	if merge == nil {
		s.conditions = append(s.conditions, c)
	}
	s.condMu.Unlock()

	if merge != nil {
		merge.AddCondition(creature, c)
	} else {
		c.StartCondition(creature)
	}
}

// RemoveCondition removes every condition of the given type, calling EndCondition first to revert bonuses.
func (s *conditionStore) RemoveCondition(creature combat.Creature, t combat.ConditionType) {
	s.condMu.Lock()
	kept := s.conditions[:0]
	var removed []combat.Condition
	for _, existing := range s.conditions {
		if existing.GetType() == t {
			removed = append(removed, existing)
			continue
		}
		kept = append(kept, existing)
	}
	s.conditions = kept
	s.condMu.Unlock()

	for _, cond := range removed { // EndCondition outside condMu (see AddCondition)
		cond.EndCondition(creature)
	}
}

// HasCondition reports whether a condition of the given type is present.
func (s *conditionStore) HasCondition(t combat.ConditionType) bool {
	s.condMu.Lock()
	defer s.condMu.Unlock()
	for _, existing := range s.conditions {
		if existing.GetType() == t {
			return true
		}
	}
	return false
}

// ClearConditions removes every active condition (used on death), calling EndCondition on all of them first.
func (s *conditionStore) ClearConditions(creature combat.Creature) {
	s.condMu.Lock()
	snapshot := s.conditions
	s.conditions = nil
	s.condMu.Unlock()
	for _, existing := range snapshot { // EndCondition outside condMu (see AddCondition)
		existing.EndCondition(creature)
	}
}

// Conditions returns a snapshot of the creature's active conditions.
func (s *conditionStore) Conditions() []combat.Condition {
	s.condMu.Lock()
	defer s.condMu.Unlock()
	out := make([]combat.Condition, len(s.conditions))
	copy(out, s.conditions)
	return out
}

// ExecuteConditions ticks and executes all active conditions on a creature.
func (s *conditionStore) ExecuteConditions(creature combat.Creature, interval int32) {
	// Snapshot under the lock, then run all condition callbacks WITHOUT holding
	// condMu — see AddCondition for why (avoids the condMu<->e.mu deadlock that
	// froze the combat-tick goroutine). Only re-acquire the lock to drop the
	// conditions that ended this tick.
	s.condMu.Lock()
	snapshot := make([]combat.Condition, len(s.conditions))
	copy(snapshot, s.conditions)
	s.condMu.Unlock()

	if len(snapshot) == 0 {
		return
	}

	ended := make(map[combat.Condition]bool)
	speedChanged := false
	iconsChanged := false

	for _, cond := range snapshot {
		if cond.GetEndTime() == 0 {
			cond.StartCondition(creature)
			if cond.GetType() == combat.ConditionHaste || cond.GetType() == combat.ConditionParalyze {
				speedChanged = true
			}
			iconsChanged = true
		}

		if !cond.ExecuteCondition(creature, interval) {
			cond.EndCondition(creature)
			ended[cond] = true
			if cond.GetType() == combat.ConditionHaste || cond.GetType() == combat.ConditionParalyze {
				speedChanged = true
			}
			iconsChanged = true
		}
	}

	// Rebuild the live list, dropping the conditions that ended. Conditions
	// added concurrently (not in the snapshot) are preserved.
	if len(ended) > 0 {
		s.condMu.Lock()
		out := s.conditions[:0]
		for _, cond := range s.conditions {
			if !ended[cond] {
				out = append(out, cond)
			}
		}
		s.conditions = out
		s.condMu.Unlock()
	}

	if iconsChanged || speedChanged {
		if notifier, ok := creature.(interface{ NotifyIconsChange() }); ok {
			notifier.NotifyIconsChange()
		}
	}
}
