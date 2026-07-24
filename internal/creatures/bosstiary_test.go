package creatures

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/bosstiary"
)

func TestBosstiaryRegistry(t *testing.T) {
	r := NewTypeRegistry()
	boss := &MonsterType{Name: "Bibby Bloodbath", BosstiaryRaceID: 900, BosstiaryRace: bosstiary.RarityArchfoe}
	plain := &MonsterType{Name: "Rat"}
	r.Monsters["bibby bloodbath"] = boss
	r.Monsters["rat"] = plain

	if !boss.IsBoss() {
		t.Error("boss with race id 900 + Archfoe should be IsBoss")
	}
	if plain.IsBoss() {
		t.Error("plain monster (no bossRaceId) should not be IsBoss")
	}
	if got := r.MonsterByBossRaceID(900); got != boss {
		t.Errorf("MonsterByBossRaceID(900) = %v, want the boss", got)
	}
	if got := r.MonsterByBossRaceID(0); got != nil {
		t.Errorf("MonsterByBossRaceID(0) = %v, want nil", got)
	}
	if got := r.MonsterByBossRaceID(123); got != nil {
		t.Errorf("MonsterByBossRaceID(unknown) = %v, want nil", got)
	}
	all := r.BosstiaryMonsters()
	if len(all) != 1 || all[900] != boss {
		t.Errorf("BosstiaryMonsters() = %v, want {900: boss}", all)
	}
}
