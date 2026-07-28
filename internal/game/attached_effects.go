package game

import "sync"

// AttachedEffectType represents the type of attached effect.
type AttachedEffectType uint8

const (
	EffectNone         AttachedEffectType = 0
	EffectBlood       AttachedEffectType = 1
	EffectBlueBubble  AttachedEffectType = 2
	EffectPoisonBubble AttachedEffectType = 3
	EffectFire        AttachedEffectType = 4
	EffectEnergy      AttachedEffectType = 5
	EffectPoison       AttachedEffectType = 6
	EffectMagic        AttachedEffectType = 7
	EffectMagicAlt    AttachedEffectType = 8
	EffectDrown       AttachedEffectType = 9
	EffectFreeze       AttachedEffectType = 10
	EffectDazzle       AttachedEffectType = 11
	EffectCurse        AttachedEffectType = 12
	EffectSparkling   AttachedEffectType = 13
)

// AttachedEffect represents a visual effect attached to a creature.
type AttachedEffect struct {
	Type      AttachedEffectType
	DurationMs uint32 // 0 = permanent until removed
	StartedAt int64  // unix timestamp in ms
}

// AttachedEffectsManager manages visual effects on a creature.
type AttachedEffectsManager struct {
	mu       sync.RWMutex
	effects  []AttachedEffect
	owner    *Creature
}

// NewAttachedEffectsManager creates a new manager.
func NewAttachedEffectsManager(owner *Creature) *AttachedEffectsManager {
	return &AttachedEffectsManager{
		effects: make([]AttachedEffect, 0),
		owner:   owner,
	}
}

// AddEffect adds a visual effect to the creature.
func (m *AttachedEffectsManager) AddEffect(effectType AttachedEffectType, durationMs uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nowMs := 0 // placeholder; would use time.Now().UnixMilli() in real code
	m.effects = append(m.effects, AttachedEffect{
		Type:       effectType,
		DurationMs: durationMs,
		StartedAt:  int64(nowMs),
	})
}

// RemoveEffect removes all effects of the given type.
func (m *AttachedEffectsManager) RemoveEffect(effectType AttachedEffectType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var remaining []AttachedEffect
	for _, e := range m.effects {
		if e.Type != effectType {
			remaining = append(remaining, e)
		}
	}
	m.effects = remaining
}

// Clear removes all attached effects.
func (m *AttachedEffectsManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.effects = nil
}

// GetEffects returns a copy of the current active effects.
func (m *AttachedEffectsManager) GetEffects() []AttachedEffect {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AttachedEffect, len(m.effects))
	copy(result, m.effects)
	return result
}
