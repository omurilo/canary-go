package luaengine

import (
	"os"
	"path/filepath"
	"testing"
)

// register() read health, speed, corpse, attacks, loot, flags and elements, and
// walked past everything else in the file. These tests pin the blocks that were
// being dropped, against the real datapack rather than a synthetic table, since
// the bug was a mismatch between what the datapack writes and what the reader
// looked for.

func loadMonster(t *testing.T, parts ...string) *Engine {
	t.Helper()
	dir := monsterDataDir()
	if dir == "" {
		t.Skip("otservbr monster data not available")
	}
	path := filepath.Join(append([]string{dir}, parts...)...)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s not found: %v", path, err)
	}
	e := newTestEngine()
	t.Cleanup(e.Close)
	if err := e.DoFile(path); err != nil {
		t.Fatalf("loading %s: %v", path, err)
	}
	return e
}

// The datapack writes monster.strategiesTarget = {nearest = 70, ...} at the top
// level. The reader looked for `strategiesTargetNearest` inside monster.flags —
// a key no monster file has — so all four weights stayed at zero for all 1633
// monsters that declare them.
//
// Zero is not a harmless default here: searchTargetImmediate starts at NEAREST
// and advances while `rnd > sum`, so all-zero weights fall through every branch
// to TARGETSEARCH_RANDOM. Every monster in the game picked its target at random.
func TestStrategiesTargetIsRead(t *testing.T) {
	e := loadMonster(t, "dragons", "dragon.lua")

	mt := e.world.TypeRegistry.Monsters["dragon"]
	if mt == nil {
		t.Fatal("dragon not registered")
	}
	f := mt.Flags
	if f.StrategiesTargetNearest != 70 {
		t.Errorf("nearest = %d, want 70", f.StrategiesTargetNearest)
	}
	if f.StrategiesTargetHealth != 10 {
		t.Errorf("health = %d, want 10", f.StrategiesTargetHealth)
	}
	if f.StrategiesTargetDamage != 10 {
		t.Errorf("damage = %d, want 10", f.StrategiesTargetDamage)
	}
	if f.StrategiesTargetRandom != 10 {
		t.Errorf("random = %d, want 10", f.StrategiesTargetRandom)
	}
}

// monster.changeTarget drives onThinkTarget. With it unread, changeTargetSpeed
// was 0 for every monster, and 0 is the "never re-pick a target" case — a
// monster stayed locked on its first victim until that victim left.
func TestChangeTargetIsRead(t *testing.T) {
	e := loadMonster(t, "dragons", "dragon.lua")

	mt := e.world.TypeRegistry.Monsters["dragon"]
	if mt == nil {
		t.Fatal("dragon not registered")
	}
	if mt.ChangeTargetInterval != 4000 {
		t.Errorf("ChangeTargetInterval = %d, want 4000", mt.ChangeTargetInterval)
	}
	if mt.ChangeTargetChance != 10 {
		t.Errorf("ChangeTargetChance = %d, want 10", mt.ChangeTargetChance)
	}
}

// monster.voices mixes two scalars with an array of {text, yell} entries on the
// same table. The array walk has to skip the scalars rather than choke on them.
func TestVoicesAreRead(t *testing.T) {
	e := loadMonster(t, "dragons", "dragon.lua")

	mt := e.world.TypeRegistry.Monsters["dragon"]
	if mt == nil {
		t.Fatal("dragon not registered")
	}
	if mt.YellInterval != 5000 || mt.YellChance != 10 {
		t.Errorf("yell interval/chance = %d/%d, want 5000/10", mt.YellInterval, mt.YellChance)
	}
	if len(mt.Voices) != 2 {
		t.Fatalf("Voices len = %d, want 2 (the scalars must not be parsed as entries)", len(mt.Voices))
	}
	for _, v := range mt.Voices {
		if !v.Yell {
			t.Errorf("voice %q: Yell = false, want true", v.Text)
		}
	}
	if mt.Voices[0].Text != "FCHHHHH" {
		t.Errorf("first voice = %q, want FCHHHHH", mt.Voices[0].Text)
	}
}

// monster.defenses is the same shape: `defense`, `armor` and `mitigation`
// scalars alongside the defensive spell blocks. None of it was read, which is
// why no monster in the port ever healed itself and getDefense saw zero.
func TestDefensesAreRead(t *testing.T) {
	e := loadMonster(t, "dragons", "dragon.lua")

	mt := e.world.TypeRegistry.Monsters["dragon"]
	if mt == nil {
		t.Fatal("dragon not registered")
	}
	if mt.Defense != 30 {
		t.Errorf("Defense = %d, want 30", mt.Defense)
	}
	if mt.Armor != 25 {
		t.Errorf("Armor = %d, want 25", mt.Armor)
	}
	if len(mt.Defenses) != 1 {
		t.Fatalf("Defenses len = %d, want 1", len(mt.Defenses))
	}
	d := mt.Defenses[0]
	if d.Interval != 2000 || d.Chance != 15 {
		t.Errorf("defense block interval/chance = %d/%d, want 2000/15", d.Interval, d.Chance)
	}
	if d.CombatType != "healing" {
		t.Errorf("defense combat type = %q, want healing", d.CombatType)
	}
	if d.MinDamage != 40 || d.MaxDamage != 70 {
		t.Errorf("defense heal range = %d..%d, want 40..70", d.MinDamage, d.MaxDamage)
	}
}

// The defensive blocks must not leak into Attacks — a self-heal fired at a
// player would heal whoever the monster is fighting.
func TestDefensesDoNotLeakIntoAttacks(t *testing.T) {
	e := loadMonster(t, "dragons", "dragon.lua")

	mt := e.world.TypeRegistry.Monsters["dragon"]
	if mt == nil {
		t.Fatal("dragon not registered")
	}
	for _, a := range mt.Attacks {
		if a.CombatType == "healing" {
			t.Errorf("healing block %+v ended up in Attacks", a)
		}
	}
}

// monster.race, manaCost and light were all skipped. race in particular is a
// top-level string and must not be confused with Bestiary.race, a number.
func TestMiscScalarsAreRead(t *testing.T) {
	e := loadMonster(t, "dragons", "dragon.lua")

	mt := e.world.TypeRegistry.Monsters["dragon"]
	if mt == nil {
		t.Fatal("dragon not registered")
	}
	if mt.BloodRace != "blood" {
		t.Errorf("BloodRace = %q, want blood", mt.BloodRace)
	}
	if mt.BestiaryRace == 0 {
		t.Error("Bestiary.race was clobbered by the top-level race string")
	}
}
