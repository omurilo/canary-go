package game

import (
	"github.com/opentibiabr/canary-go/internal/bosstiary"
	"github.com/opentibiabr/canary-go/internal/creatures"
)

func creaturesRegistryWithArchfoe() *creatures.TypeRegistry {
	r := creatures.NewTypeRegistry()
	r.Monsters["drume"] = &creatures.MonsterType{Name: "Drume", BosstiaryRaceID: 1957, BosstiaryRace: bosstiary.RarityArchfoe}
	return r
}
