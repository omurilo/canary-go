package game

import "github.com/opentibiabr/canary-go/internal/game/combat"

// The combat package defines its own combat.Creature interface (int32 health,
// mana, conditions, combat.Position) that does not match game.Creature. Rather
// than pollute game.Creature, we bridge with a per-call adapter and reach the
// optional mana/condition capabilities through these narrow interfaces, which
// the concrete creature types implement.
type manaHolder interface {
	GetMana() uint32
	GetMaxMana() uint32
	AddMana(amount int32)
}

type conditionHolder interface {
	AddCondition(c combat.Condition)
	RemoveCondition(t combat.ConditionType)
	HasCondition(t combat.ConditionType) bool
}

// combatAdapter wraps a game.Creature so the combat engine can operate on it.
type combatAdapter struct {
	c Creature
}

// adaptCreature returns a combat.Creature view of c (nil-safe).
func adaptCreature(c Creature) combat.Creature {
	if c == nil {
		return nil
	}
	return combatAdapter{c: c}
}

func (a combatAdapter) GetId() uint32 { return a.c.GetID() }

func (a combatAdapter) GetPosition() combat.Position {
	p := a.c.GetPosition()
	return combat.Position{X: p.X, Y: p.Y, Z: uint16(p.Z)}
}

func (a combatAdapter) GetHealth() int32    { return int32(a.c.GetHealth()) }
func (a combatAdapter) GetMaxHealth() int32 { return int32(a.c.GetMaxHealth()) }

func (a combatAdapter) GetMana() int32 {
	if m, ok := a.c.(manaHolder); ok {
		return int32(m.GetMana())
	}
	return 0
}

func (a combatAdapter) GetMaxMana() int32 {
	if m, ok := a.c.(manaHolder); ok {
		return int32(m.GetMaxMana())
	}
	return 0
}

// ChangeHealth applies a signed health delta, mirroring Creature::changeHealth
// (src/creatures/creature.cpp): drainHealth calls changeHealth(-damage).
func (a combatAdapter) ChangeHealth(amount int32) { a.c.AddHealth(amount) }

func (a combatAdapter) GetBaseSpeed() uint16 { return a.c.GetBaseSpeed() }

func (a combatAdapter) ChangeSpeed(delta int32) {
	a.c.ChangeSpeed(delta)
	if p, ok := a.c.(*Player); ok {
		if p.World != nil && p.World.OnChangeSpeed != nil {
			p.World.OnChangeSpeed(p)
		}
	} else if bc, ok := a.c.(*BaseCreature); ok {
		if bc.World != nil && bc.World.OnChangeSpeed != nil {
			bc.World.OnChangeSpeed(bc)
		}
	}
}

func (a combatAdapter) NotifyIconsChange() {
	if p, ok := a.c.(*Player); ok {
		p.NotifyIconsChange()
	}
}

func (a combatAdapter) ChangeMana(amount int32) {
	if m, ok := a.c.(manaHolder); ok {
		m.AddMana(amount)
	}
}

func (a combatAdapter) AddCondition(c combat.Condition) error {
	if h, ok := a.c.(conditionHolder); ok {
		h.AddCondition(c)
	}
	return nil
}

func (a combatAdapter) RemoveCondition(t combat.ConditionType) {
	if h, ok := a.c.(conditionHolder); ok {
		h.RemoveCondition(t)
	}
}

func (a combatAdapter) HasCondition(t combat.ConditionType) bool {
	if h, ok := a.c.(conditionHolder); ok {
		return h.HasCondition(t)
	}
	return false
}

func (a combatAdapter) GetArmor() int32 {
	return a.c.GetArmor()
}

func (a combatAdapter) GetDefense() int32 {
	return a.c.GetDefense()
}

func (a combatAdapter) GetResistance(combatType combat.CombatType) int16 {
	if m, ok := a.c.(*Monster); ok {
		if m.Type != nil && m.Type.Elements != nil {
			return m.Type.Elements[uint32(combatType)]
		}
	}
	return 0
}

