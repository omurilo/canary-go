package game

import (
	"sync"

	"github.com/opentibiabr/canary-go/internal/game/combat"
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
func (s *conditionStore) AddCondition(c combat.Condition) {
	if c == nil {
		return
	}
	s.condMu.Lock()
	defer s.condMu.Unlock()
	for _, existing := range s.conditions {
		if existing.GetType() == c.GetType() {
			existing.AddCondition(nil, c)
			return
		}
	}
	s.conditions = append(s.conditions, c)
}

// RemoveCondition removes every condition of the given type.
func (s *conditionStore) RemoveCondition(t combat.ConditionType) {
	s.condMu.Lock()
	defer s.condMu.Unlock()
	out := s.conditions[:0]
	for _, existing := range s.conditions {
		if existing.GetType() == t {
			continue
		}
		out = append(out, existing)
	}
	s.conditions = out
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

// ClearConditions removes every active condition (used on death).
func (s *conditionStore) ClearConditions() {
	s.condMu.Lock()
	defer s.condMu.Unlock()
	s.conditions = nil
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
	s.condMu.Lock()
	defer s.condMu.Unlock()

	out := s.conditions[:0]
	speedChanged := false
	iconsChanged := false

	for _, cond := range s.conditions {
		if cond.GetEndTime() == 0 {
			cond.StartCondition(creature)
			if cond.GetType() == combat.ConditionHaste || cond.GetType() == combat.ConditionParalyze {
				speedChanged = true
			}
			iconsChanged = true
		}

		keep := cond.ExecuteCondition(creature, interval)
		if keep {
			out = append(out, cond)
		} else {
			cond.EndCondition(creature)
			if cond.GetType() == combat.ConditionHaste || cond.GetType() == combat.ConditionParalyze {
				speedChanged = true
			}
			iconsChanged = true
		}
	}
	s.conditions = out

	if iconsChanged || speedChanged {
		if notifier, ok := creature.(interface{ NotifyIconsChange() }); ok {
			notifier.NotifyIconsChange()
		}
	}
}
