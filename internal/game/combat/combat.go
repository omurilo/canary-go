package combat

import (
	"github.com/opentibiabr/canary-go/internal/game"
)

type Spell struct {
	Name string
}

func CastSpell(caster game.Creature, spellName string) bool {
	// Combat logic for casting a spell
	// We can use game.GlobalDispatcher here if there is a cooldown or delay
	return true
}
