package game

import (
	"encoding/xml"
	"fmt"
	"os"
	"sync"
)

// AttachedEffectType represents the type of attached effect.
type AttachedEffectType uint8

const (
	EffectNone         AttachedEffectType = 0
	EffectBlood        AttachedEffectType = 1
	EffectBlueBubble   AttachedEffectType = 2
	EffectPoisonBubble AttachedEffectType = 3
	EffectFire         AttachedEffectType = 4
	EffectEnergy       AttachedEffectType = 5
	EffectPoison       AttachedEffectType = 6
	EffectMagic        AttachedEffectType = 7
	EffectMagicAlt     AttachedEffectType = 8
	EffectDrown        AttachedEffectType = 9
	EffectFreeze       AttachedEffectType = 10
	EffectDazzle       AttachedEffectType = 11
	EffectCurse        AttachedEffectType = 12
	EffectSparkling    AttachedEffectType = 13
)

// AttachedEffectSlot represents the OTCR effect slot type (aura, shader, effect, wing).
type AttachedEffectSlot uint8

const (
	SlotAura   AttachedEffectSlot = 1
	SlotShader AttachedEffectSlot = 2
	SlotEffect AttachedEffectSlot = 3
	SlotWing   AttachedEffectSlot = 4
)

// AttachedEffectRegistryEntry is one entry from the XML (aura, shader, effect, wing).
type AttachedEffectRegistryEntry struct {
	ID   uint16 `xml:"id,attr"`
	Name string `xml:"name,attr"`
	Slot AttachedEffectSlot
}

// AttachedEffectRegistry loads and serves effect definitions from attachedeffects.xml.
type AttachedEffectRegistry struct {
	mu      sync.RWMutex
	entries []AttachedEffectRegistryEntry
	byID    map[uint16]*AttachedEffectRegistryEntry
}

// xmlAttachedEffects is the root XML structure.
type xmlAttachedEffects struct {
	XMLName xml.Name         `xml:"attachedeffects"`
	Auras   []xmlEffectEntry `xml:"aura"`
	Shaders []xmlEffectEntry `xml:"shader"`
	Effects []xmlEffectEntry `xml:"effect"`
	Wings   []xmlEffectEntry `xml:"wing"`
}

type xmlEffectEntry struct {
	ID   uint16 `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

// LoadAttachedEffectRegistry parses the attachedeffects.xml file.
func LoadAttachedEffectRegistry(path string) (*AttachedEffectRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("attached effects: read %s: %w", path, err)
	}
	var xmlDef xmlAttachedEffects
	if err := xml.Unmarshal(data, &xmlDef); err != nil {
		return nil, fmt.Errorf("attached effects: unmarshal %s: %w", path, err)
	}
	reg := &AttachedEffectRegistry{
		byID: make(map[uint16]*AttachedEffectRegistryEntry),
	}
	for _, a := range xmlDef.Auras {
		e := &AttachedEffectRegistryEntry{ID: a.ID, Name: a.Name, Slot: SlotAura}
		reg.entries = append(reg.entries, *e)
		reg.byID[a.ID] = e
	}
	for _, s := range xmlDef.Shaders {
		e := &AttachedEffectRegistryEntry{ID: s.ID, Name: s.Name, Slot: SlotShader}
		reg.entries = append(reg.entries, *e)
		reg.byID[s.ID] = e
	}
	for _, ef := range xmlDef.Effects {
		e := &AttachedEffectRegistryEntry{ID: ef.ID, Name: ef.Name, Slot: SlotEffect}
		reg.entries = append(reg.entries, *e)
		reg.byID[ef.ID] = e
	}
	for _, w := range xmlDef.Wings {
		e := &AttachedEffectRegistryEntry{ID: w.ID, Name: w.Name, Slot: SlotWing}
		reg.entries = append(reg.entries, *e)
		reg.byID[w.ID] = e
	}
	return reg, nil
}

// GetShaderNameByID resolves a shader name from the registry by ID.
func (r *AttachedEffectRegistry) GetShaderNameByID(id uint16) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.byID[id]; ok && e.Slot == SlotShader {
		return e.Name
	}
	return ""
}

// GetNameByID returns the name of any effect entry by ID.
func (r *AttachedEffectRegistry) GetNameByID(id uint16) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.byID[id]; ok {
		return e.Name
	}
	return ""
}

// AttachedEffect represents a visual effect attached to a creature.
type AttachedEffect struct {
	Type       AttachedEffectType
	DurationMs uint32
	StartedAt  int64
}

// AttachedEffectsManager manages visual effects on a creature.
type AttachedEffectsManager struct {
	mu      sync.RWMutex
	effects []AttachedEffect
	owner   *Creature
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
	m.effects = append(m.effects, AttachedEffect{
		Type:       effectType,
		DurationMs: durationMs,
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
