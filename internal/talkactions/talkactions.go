package talkactions

import (
	"strings"
	"sync"
	lua "github.com/yuin/gopher-lua"
)

type TalkAction struct {
	Words      string
	Separator  string
	// GroupType is the minimum player group required to use the command
	// (e.g. "god", "gm", "normal"). Recorded on registration; access
	// enforcement is not wired in this slice yet.
	GroupType string
	OnSayFunc lua.LValue
}

var (
	talkActionsMu sync.RWMutex
	all           []*TalkAction
)

// Register adds the talkaction to the global registry.
func Register(t *TalkAction) {
	talkActionsMu.Lock()
	defer talkActionsMu.Unlock()
	if t.Words != "" {
		all = append(all, t)
	}
}

// FindByWords finds the longest matching talkaction and returns it along with the unparsed param.
func FindByWords(words string) (*TalkAction, string) {
	talkActionsMu.RLock()
	defer talkActionsMu.RUnlock()

	lower := strings.ToLower(words)
	var result *TalkAction
	for _, t := range all {
		w := strings.ToLower(t.Words)
		if len(lower) >= len(w) && lower[:len(w)] == w {
			if result == nil || len(w) > len(result.Words) {
				// Must either match exactly, or have the talkaction's separator next
				if len(lower) == len(w) {
					result = t
					break
				}
				sepLen := len(t.Separator)
				if sepLen == 0 {
					// Fallback: If separator is empty string (""), default to space in Lua, but here just assume it requires space to separate.
					// Actually, Canary defaults separator to `""` in Lua which means " ". Let's check:
					// wait, if separator is "", it means no separator is enforced, it just takes the rest.
					// But let's check t.Separator. If it's " ", it expects a space.
					result = t
				} else if len(lower) >= len(w)+sepLen && lower[len(w):len(w)+sepLen] == t.Separator {
					result = t
				}
			}
		}
	}

	if result == nil {
		return nil, ""
	}

	param := ""
	prefix := result.Words
	if len(words) > len(prefix) {
		paramStr := words[len(prefix):]
		sep := result.Separator
		if sep == `""` || sep == "" {
			sep = " " // Default separator is space if empty
		}
		if strings.HasPrefix(paramStr, sep) {
			paramStr = paramStr[len(sep):]
		}
		param = strings.TrimSpace(paramStr)
	}

	return result, param
}
