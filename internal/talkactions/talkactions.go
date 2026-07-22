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

// FindByWords finds the matching talkaction and returns it along with the unparsed param.
func FindByWords(words string) (*TalkAction, string) {
	talkActionsMu.RLock()
	defer talkActionsMu.RUnlock()

	lower := strings.ToLower(words)
	var bestMatch *TalkAction
	var bestMatchedWord string
	bestLen := 0

	for _, t := range all {
		wordList := strings.Split(t.Words, ",")
		for _, w := range wordList {
			w = strings.TrimSpace(w)
			if w == "" {
				continue
			}
			lowerW := strings.ToLower(w)
			if len(lower) >= len(lowerW) && lower[:len(lowerW)] == lowerW {
				if len(lower) == len(lowerW) {
					if len(lowerW) > bestLen {
						bestMatch = t
						bestMatchedWord = w
						bestLen = len(lowerW)
					}
				} else {
					sep := t.Separator
					if sep == "" || sep == `""` {
						sep = " "
					}
					rest := words[len(w):]
					if strings.HasPrefix(rest, sep) || strings.HasPrefix(rest, " ") {
						if len(lowerW) > bestLen {
							bestMatch = t
							bestMatchedWord = w
							bestLen = len(lowerW)
						}
					}
				}
			}
		}
	}

	if bestMatch == nil {
		return nil, ""
	}

	param := ""
	if len(words) > len(bestMatchedWord) {
		paramStr := words[len(bestMatchedWord):]
		sep := bestMatch.Separator
		if sep == "" || sep == `""` {
			sep = " "
		}
		if strings.HasPrefix(paramStr, sep) {
			paramStr = paramStr[len(sep):]
		}
		param = strings.TrimSpace(paramStr)
	}

	return bestMatch, param
}
