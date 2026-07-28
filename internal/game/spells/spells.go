package spells

import (
	"sync"
)

type SpellType uint8
const (
	SpellTypeInstant SpellType = iota
	SpellTypeRune
	SpellTypeConjure
)

type SpellGroup uint8
const (
	SpellGroupNone SpellGroup = iota
	SpellGroupAttack
	SpellGroupHealing
	SpellGroupSupport
)

type Spell struct {
	mu            sync.RWMutex
	name          string
	words         string
	spellType     SpellType
	group         SpellGroup
	level         int32
	magicLevel    int32
	mana          int32
	cooldown      int32
	groupCooldown int32
}

func NewSpell(name, words string, st SpellType) *Spell {
	return &Spell{name: name, words: words, spellType: st}
}

type SpellManager struct {
	mu     sync.RWMutex
	spells map[string]*Spell
}

func NewSpellManager() *SpellManager {
	return &SpellManager{spells: make(map[string]*Spell)}
}

func (sm *SpellManager) Register(s *Spell) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.spells[s.words] = s
}
