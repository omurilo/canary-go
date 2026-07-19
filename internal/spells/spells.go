package spells

import (
	"strings"
	"sync"
	lua "github.com/yuin/gopher-lua"
)

type Spell struct {
	Name         string
	Words        string
	Separator    string
	Level        int
	Mana         int
	Exhaustion   int
	Group        string
	NeedTarget   bool
	NeedPremium  bool
	NeedWeapon   bool
	NeedLearn    bool
	Vocation     []string
	OnCastSpell  lua.LValue
}

var (
	spellsMu sync.RWMutex
	byName   = make(map[string]*Spell)
	byWords  = make(map[string]*Spell)
)

// Register adds the spell to the global registries.
func Register(s *Spell) {
	spellsMu.Lock()
	defer spellsMu.Unlock()
	if s.Name != "" {
		byName[strings.ToLower(s.Name)] = s
	}
	if s.Words != "" {
		byWords[strings.ToLower(s.Words)] = s
	}
}

// FindByName returns the spell registered for the given name, or nil.
func FindByName(name string) *Spell {
	spellsMu.RLock()
	defer spellsMu.RUnlock()
	return byName[strings.ToLower(name)]
}

// FindByWords returns the spell registered for the given words, or nil.
func FindByWords(words string) *Spell {
	spellsMu.RLock()
	defer spellsMu.RUnlock()
	wordsLow := strings.ToLower(words)
	// Try exact match first
	if s, ok := byWords[wordsLow]; ok {
		return s
	}
	// Try prefix match (some spells might have arguments)
	for k, s := range byWords {
		if strings.HasPrefix(wordsLow, k+" ") {
			return s
		}
	}
	return nil
}
