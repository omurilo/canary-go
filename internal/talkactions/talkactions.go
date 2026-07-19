package talkactions

import (
	"strings"
	"sync"
	lua "github.com/yuin/gopher-lua"
)

type TalkAction struct {
	Words      string
	Separator  string
	OnSayFunc  lua.LValue
}

var (
	talkActionsMu sync.RWMutex
	byWords       = make(map[string]*TalkAction)
)

// Register adds the talkaction to the global registry.
func Register(t *TalkAction) {
	talkActionsMu.Lock()
	defer talkActionsMu.Unlock()
	if t.Words != "" {
		byWords[strings.ToLower(t.Words)] = t
	}
}

// FindByWords returns the talkaction registered for the given words, or nil.
func FindByWords(words string) *TalkAction {
	talkActionsMu.RLock()
	defer talkActionsMu.RUnlock()
	wordsLow := strings.ToLower(words)
	// Try exact match first
	if t, ok := byWords[wordsLow]; ok {
		return t
	}
	// Try prefix match (for commands like "/z 1")
	// If space is the separator, etc. For now just checking if words starts with the registered talkaction words followed by space
	for k, t := range byWords {
		if strings.HasPrefix(wordsLow, k+" ") {
			return t
		}
	}
	return nil
}
