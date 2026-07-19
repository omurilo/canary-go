package spells

import (
	"encoding/xml"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/luaengine"
	lua "github.com/yuin/gopher-lua"
)

type SpellsXML struct {
	XMLName  xml.Name  `xml:"spells"`
	Instants []Instant `xml:"instant"`
}

type Instant struct {
	Name       string `xml:"name,attr"`
	Words      string `xml:"words,attr"`
	Level      int    `xml:"level,attr"`
	Mana       int    `xml:"mana,attr"`
	Exhaustion int    `xml:"exhaustion,attr"`
	Script     string `xml:"script,attr"`
}

type Engine struct {
	mu     sync.RWMutex
	Words  map[string]*Instant
	lua    *luaengine.Engine
	script string
}

func NewEngine(lua *luaengine.Engine) *Engine {
	return &Engine{
		Words: make(map[string]*Instant),
		lua:   lua,
	}
}

func (e *Engine) LoadFromXML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var spellsXML SpellsXML
	if err := xml.Unmarshal(data, &spellsXML); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range spellsXML.Instants {
		inst := &spellsXML.Instants[i]
		e.Words[strings.ToLower(inst.Words)] = inst
	}
	return nil
}

func (e *Engine) ExecuteSpell(player *game.Player, words string) bool {
	e.mu.RLock()
	spell, ok := e.Words[strings.ToLower(words)]
	e.mu.RUnlock()
	if !ok {
		return false
	}

	// Check Exhaustion (we use spell words hash as ID for simplicity or just a global spell cooldown)
	// For simplicity, let's use a global exhaustion condition id for spells, say 1.
	const conditionExhaust = 1
	now := time.Now().UnixMilli()

	if player.Exhaustion == nil {
		player.Exhaustion = make(map[uint32]int64)
	}

	if expire, has := player.Exhaustion[conditionExhaust]; has && expire > now {
		// Player is exhausted
		return true // Handled, but exhausted
	}

	if int(player.Level) < spell.Level {
		return true // Handled, but insufficient level
	}

	if int(player.Mana) < spell.Mana {
		return true // Handled, but insufficient mana
	}

	// Consume mana and apply exhaustion
	player.Mana -= uint32(spell.Mana)
	player.Exhaustion[conditionExhaust] = now + int64(spell.Exhaustion)

	// Execute lua script
	if spell.Script != "" {
		_ = e.lua.Execute(func(L *lua.LState) {
			_ = L.DoFile("data/scripts/spells/" + spell.Script)
			// we could call onCastSpell(player) here
			fn := L.GetGlobal("onCastSpell")
			if fn.Type() == lua.LTFunction {
				// We should push player userdata here, but for now we just call it
				_ = L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true})
			}
		})
	}
	return true
}
