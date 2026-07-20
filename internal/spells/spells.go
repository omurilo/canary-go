package spells

import (
	"strings"
	"sync"

	"github.com/opentibiabr/canary-go/internal/game/combat"
	lua "github.com/yuin/gopher-lua"
)

// SpellGroup mirrors SpellGroup_t (src/creatures/creatures_definitions.hpp).
type SpellGroup uint8

const (
	SpellGroupNone    SpellGroup = 0
	SpellGroupAttack  SpellGroup = 1
	SpellGroupHealing SpellGroup = 2
	SpellGroupSupport SpellGroup = 3
	SpellGroupSpecial SpellGroup = 4
)

// Spell mirrors the InstantSpell/Spell data loaded from Lua
// (src/creatures/combat/spells.hpp). The subset stored here is what the cast
// checks (Spell::playerSpellCheck) and variant builder (InstantSpell::
// playerCastInstant) need, plus the onCastSpell closure to run the combat.
type Spell struct {
	Name    string
	Words   string
	SpellID uint16

	Level       int
	MagicLevel  int
	Mana        int
	ManaPercent int
	Soul        int

	// Cooldowns in milliseconds.
	Cooldown               uint32
	GroupCooldown          uint32
	SecondaryGroupCooldown uint32

	Group          SpellGroup
	SecondaryGroup SpellGroup

	NeedTarget              bool
	NeedDirection           bool
	SelfTarget              bool
	CasterTargetOrDirection bool
	BlockWalls              bool
	Aggressive              bool
	AllowOnSelf             bool
	PzLock                  bool
	NeedWeapon              bool
	NeedPremium             bool
	NeedLearn               bool
	HasParam                bool
	Enabled                 bool

	// Range is the max cast range; -1 means unlimited (Spell::range default).
	Range int

	// Combat is the primary combat definition captured from the Lua closure
	// (set when the script calls Combat()); it is currently informational — the
	// combat is actually run through the onCastSpell closure.
	Combat *combat.Combat

	// VocationNames holds the vocations allowed to cast (spell:vocation("...")).
	// The map from name to vocation id is resolved at check time (best-effort).
	VocationNames []string

	OnCastSpell lua.LValue
}

// NewSpell returns a spell with C++ default field values (Spell ctor + defaults).
func NewSpell(name string) *Spell {
	return &Spell{
		Name:    name,
		Enabled: true,
		Range:   -1,
	}
}

var (
	spellsMu    sync.RWMutex
	byName      = make(map[string]*Spell)
	byWords     = make(map[string]*Spell)
	all         []*Spell
	nextSpellID uint16
)

// Register adds the spell to the global registries, mirroring
// Spells::registerInstantLuaEvent (src/creatures/combat/spells.cpp:163): a spell
// with no words is rejected, and duplicate words are rejected.
func Register(s *Spell) bool {
	spellsMu.Lock()
	defer spellsMu.Unlock()
	if s.Words == "" {
		return false
	}
	wordsLow := strings.ToLower(s.Words)
	if _, dup := byWords[wordsLow]; dup {
		return false
	}
	nextSpellID++
	s.SpellID = nextSpellID
	if s.Name != "" {
		byName[strings.ToLower(s.Name)] = s
	}
	byWords[wordsLow] = s
	all = append(all, s)
	return true
}

// Count returns the number of registered spells.
func Count() int {
	spellsMu.RLock()
	defer spellsMu.RUnlock()
	return len(all)
}

// All returns a snapshot of every registered spell.
func All() []*Spell {
	spellsMu.RLock()
	defer spellsMu.RUnlock()
	out := make([]*Spell, len(all))
	copy(out, all)
	return out
}

// FindByName returns the spell registered for the given name, or nil.
func FindByName(name string) *Spell {
	spellsMu.RLock()
	defer spellsMu.RUnlock()
	return byName[strings.ToLower(name)]
}

// FindByWords resolves the spell for a spoken phrase, mirroring
// Spells::getInstantSpell (src/creatures/combat/spells.cpp:267): choose the spell
// whose words are the longest case-insensitive prefix of the input; if the input
// is longer than the matched words, require that the spell takes a parameter and
// that a space separates the words from the argument.
func FindByWords(words string) *Spell {
	spellsMu.RLock()
	defer spellsMu.RUnlock()

	lower := strings.ToLower(words)
	var result *Spell
	for _, s := range all {
		w := strings.ToLower(s.Words)
		if len(lower) >= len(w) && lower[:len(w)] == w {
			if result == nil || len(w) > len(result.Words) {
				result = s
				if len(lower) == len(w) {
					break
				}
			}
		}
	}

	if result == nil {
		return nil
	}
	if len(words) > len(result.Words) {
		if !result.HasParam {
			return nil
		}
		spellLen := len(result.Words)
		if len(words)-spellLen < 2 || words[spellLen] != ' ' {
			return nil
		}
	}
	return result
}
