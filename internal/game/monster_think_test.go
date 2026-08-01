package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
)

// The Monster::onThink timers. The arithmetic is upstream's and the failure mode
// of getting it wrong is silent — a monster that yells every tick instead of
// every interval reads as "working" until you watch it for ten seconds.

const thinkTick = uint32(1000)

// yellSpeedTicks of 0 disables yelling. Every monster had it at 0 before the
// voices block was read, so this is the case that used to hold everywhere.
func TestYellDisabledWhenIntervalIsZero(t *testing.T) {
	w := aiWorld(t)
	said := 0
	w.OnCreatureSay = func(Creature, byte, string) { said++ }

	mt := &creatures.MonsterType{Name: "Mute", YellChance: 100}
	mt.Voices = []creatures.MonsterVoice{{Text: "hello"}}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)

	for i := 0; i < 10; i++ {
		m.OnThinkYell(w, thinkTick)
	}
	if said != 0 {
		t.Errorf("yelled %d times with interval 0, want 0", said)
	}
}

// The counter accumulates and fires on the interval, once — not on every tick
// after the first, which is what a `>=` without the reset would do.
func TestYellFiresOncePerInterval(t *testing.T) {
	w := aiWorld(t)
	var lines []string
	var types []byte
	w.OnCreatureSay = func(_ Creature, talkType byte, text string) {
		lines = append(lines, text)
		types = append(types, talkType)
	}

	mt := &creatures.MonsterType{Name: "Dragon", YellInterval: 5000, YellChance: 100}
	mt.Voices = []creatures.MonsterVoice{{Text: "GROOAAARRR", Yell: true}}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)

	for i := 0; i < 4; i++ {
		m.OnThinkYell(w, thinkTick)
	}
	if len(lines) != 0 {
		t.Fatalf("yelled after %d ticks, want silence until 5000ms", 4)
	}
	m.OnThinkYell(w, thinkTick) // 5000ms
	if len(lines) != 1 {
		t.Fatalf("yelled %d times at the interval, want 1", len(lines))
	}
	if types[0] != talkTypeMonsterYell {
		t.Errorf("talk type = %d, want %d for yell = true", types[0], talkTypeMonsterYell)
	}

	// And the counter restarts rather than firing every tick from here on.
	for i := 0; i < 4; i++ {
		m.OnThinkYell(w, thinkTick)
	}
	if len(lines) != 1 {
		t.Errorf("yelled %d times total, want 1 — the counter did not reset", len(lines))
	}
}

// yell = false is a say, not a yell. The two render differently on the client.
func TestVoiceWithoutYellSays(t *testing.T) {
	w := aiWorld(t)
	var got byte
	w.OnCreatureSay = func(_ Creature, talkType byte, _ string) { got = talkType }

	mt := &creatures.MonsterType{Name: "Whisperer", YellInterval: 1000, YellChance: 100}
	mt.Voices = []creatures.MonsterVoice{{Text: "psst", Yell: false}}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)

	m.OnThinkYell(w, thinkTick)
	if got != talkTypeMonsterSay {
		t.Errorf("talk type = %d, want %d", got, talkTypeMonsterSay)
	}
}

// A defensive healing block restores the monster's health on its own interval.
// This is the whole point of the defenses block, and it never ran.
func TestDefenseBlockHealsOnItsInterval(t *testing.T) {
	w := aiWorld(t)
	mt := &creatures.MonsterType{Name: "Healer"}
	mt.Defenses = []creatures.MonsterAttack{{
		Name: "combat", Interval: 2000, Chance: 100,
		CombatType: "healing", MinDamage: 40, MaxDamage: 40,
	}}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)
	m.MaxHealth, m.Health = 1000, 500

	m.OnThinkDefense(w, thinkTick) // 1000ms — not yet
	if m.Health != 500 {
		t.Fatalf("health = %d after 1000ms, want 500", m.Health)
	}
	m.OnThinkDefense(w, thinkTick) // 2000ms
	if m.Health != 540 {
		t.Fatalf("health = %d after 2000ms, want 540", m.Health)
	}
}

// Healing must not overshoot the monster's maximum.
func TestDefenseHealIsCappedAtMaxHealth(t *testing.T) {
	w := aiWorld(t)
	mt := &creatures.MonsterType{Name: "Healer"}
	mt.Defenses = []creatures.MonsterAttack{{
		Name: "combat", Interval: 1000, Chance: 100,
		CombatType: "healing", MinDamage: 500, MaxDamage: 500,
	}}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)
	m.MaxHealth, m.Health = 100, 90

	m.OnThinkDefense(w, thinkTick)
	if m.Health != 100 {
		t.Errorf("health = %d, want it clamped to 100", m.Health)
	}
}

// changeTargetSpeed of 0 means the monster never re-picks a target on its own.
// That was every monster before the changeTarget block was read.
func TestTargetChangeDisabledWhenIntervalIsZero(t *testing.T) {
	w := aiWorld(t)
	mt := &creatures.MonsterType{Name: "Fixed", ChangeTargetChance: 100}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)

	for i := 0; i < 10; i++ {
		m.OnThinkTarget(w, thinkTick)
	}
	if m.targetChangeTicks != 0 {
		t.Errorf("targetChangeTicks = %d, want the timer never to run", m.targetChangeTicks)
	}
}

// After a change the cooldown blocks the next one for a full interval, which is
// what stops a monster with a short interval thrashing between two players.
func TestTargetChangeCooldownBlocksTheNextRoll(t *testing.T) {
	w := aiWorld(t)
	mt := &creatures.MonsterType{Name: "Switcher", ChangeTargetInterval: 2000, ChangeTargetChance: 100}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)

	m.OnThinkTarget(w, thinkTick) // 1000
	m.OnThinkTarget(w, thinkTick) // 2000 -> fires, arms the cooldown
	if m.targetChangeCooldown != 2000 {
		t.Fatalf("cooldown = %d after firing, want 2000", m.targetChangeCooldown)
	}

	// While the cooldown runs the tick counter must stay put.
	m.OnThinkTarget(w, thinkTick)
	if m.targetChangeTicks != 0 {
		t.Errorf("targetChangeTicks = %d during cooldown, want 0", m.targetChangeTicks)
	}
}

// A summon follows its master's target; onThinkTarget is skipped entirely for it.
func TestSummonsDoNotChangeTargetOnTheirOwn(t *testing.T) {
	w := aiWorld(t)
	mt := &creatures.MonsterType{Name: "Minion", ChangeTargetInterval: 1000, ChangeTargetChance: 100}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)
	m.Master = &Monster{}

	m.OnThinkTarget(w, thinkTick)
	if m.targetChangeTicks != 0 {
		t.Errorf("a summon ran its own target timer (%d)", m.targetChangeTicks)
	}
}

// updateLookDirection picks the dominant axis. The interesting case is the exact
// diagonal, where upstream keeps the axis the monster already faces instead of
// snapping — so the same offset yields different results from different facings.
func TestLookDirectionFollowsTheDominantAxis(t *testing.T) {
	w := aiWorld(t)
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Looker"})
	target := aiMonster(w, Position{X: 110, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Prey"})
	target.ID = 2
	m.SetTarget(target)

	if got := m.UpdateLookDirection(); got != DirEast {
		t.Errorf("dx > dy to the east: got %v, want %v", got, DirEast)
	}

	target.SetPosition(Position{X: 105, Y: 101, Z: 7})
	if got := m.UpdateLookDirection(); got != DirNorth {
		t.Errorf("dy > dx to the north: got %v, want %v", got, DirNorth)
	}
}

func TestLookDirectionKeepsAxisOnAnExactDiagonal(t *testing.T) {
	w := aiWorld(t)
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Looker"})
	target := aiMonster(w, Position{X: 103, Y: 103, Z: 7}, &creatures.MonsterType{Name: "Prey"})
	target.ID = 2
	m.SetTarget(target)

	// Up-left on the diagonal: facing south turns west, facing east turns north,
	// and any other facing is left alone (monster.cpp:3383-3389).
	m.SetDirection(DirSouth)
	if got := m.UpdateLookDirection(); got != DirWest {
		t.Errorf("facing south: got %v, want %v", got, DirWest)
	}
	m.SetDirection(DirEast)
	if got := m.UpdateLookDirection(); got != DirNorth {
		t.Errorf("facing east: got %v, want %v", got, DirNorth)
	}
	m.SetDirection(DirWest)
	if got := m.UpdateLookDirection(); got != DirWest {
		t.Errorf("facing west: got %v, want it unchanged", got)
	}
}

// A monster with no target does not turn, rather than snapping to a default.
func TestLookDirectionIsUnchangedWithoutATarget(t *testing.T) {
	w := aiWorld(t)
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Idle"})
	m.SetDirection(DirSouth)

	if got := m.UpdateLookDirection(); got != DirSouth {
		t.Errorf("got %v, want the facing left alone", got)
	}
}

// A monster with no spawn — summoned, scripted, GM-placed — is always in range.
// Without this it would be teleported to position zero on its first think.
func TestSpawnlessMonsterIsAlwaysInRange(t *testing.T) {
	m := &Monster{}
	if !m.IsInSpawnRange(Position{X: 500, Y: 500, Z: 7}) {
		t.Error("a monster with no spawn position must never be out of range")
	}
}

func TestSpawnRangeUsesRadiusAndFloor(t *testing.T) {
	m := &Monster{SpawnPosition: Position{X: 100, Y: 100, Z: 7}}

	if !m.IsInSpawnRange(Position{X: 140, Y: 100, Z: 7}) {
		t.Error("40 tiles away is inside the 50-tile despawn radius")
	}
	if m.IsInSpawnRange(Position{X: 160, Y: 100, Z: 7}) {
		t.Error("60 tiles away is outside the despawn radius")
	}
	// The floor check is separate: within the radius but too many floors up.
	if m.IsInSpawnRange(Position{X: 100, Y: 100, Z: 2}) {
		t.Error("5 floors up is outside the despawn range")
	}
}

// A monster away from its spawn with nothing to fight heads home instead of
// wandering further; one standing on its spawn goes idle.
func TestIdleStatusMarksWalkBackAwayFromSpawn(t *testing.T) {
	home := Position{X: 100, Y: 100, Z: 7}

	atHome := &Monster{SpawnPosition: home}
	atHome.Pos = home
	atHome.UpdateIdleStatus()
	if !atHome.Idle || atHome.IsWalkingBack() {
		t.Errorf("at spawn: Idle = %v walkingBack = %v, want true/false", atHome.Idle, atHome.IsWalkingBack())
	}

	away := &Monster{SpawnPosition: home}
	away.Pos = Position{X: 110, Y: 100, Z: 7}
	away.UpdateIdleStatus()
	if away.Idle || !away.IsWalkingBack() {
		t.Errorf("away from spawn: Idle = %v walkingBack = %v, want false/true", away.Idle, away.IsWalkingBack())
	}
}

// A monster with a target is neither idle nor walking back, whatever its
// distance from home.
func TestMonsterWithATargetIsNotIdle(t *testing.T) {
	w := aiWorld(t)
	m := aiMonster(w, Position{X: 110, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Busy"})
	m.SpawnPosition = Position{X: 100, Y: 100, Z: 7}
	target := aiMonster(w, Position{X: 111, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Prey"})
	target.ID = 2
	m.SetTarget(target)

	m.UpdateIdleStatus()
	if m.Idle || m.IsWalkingBack() {
		t.Errorf("Idle = %v walkingBack = %v, want both false while fighting", m.Idle, m.IsWalkingBack())
	}
}

// maxSummons caps the total; each block's own count caps that block. Both are
// checked inside the loop because a block can fire while another is on cooldown.
func TestSummonsRespectMaxSummons(t *testing.T) {
	w := aiWorld(t)
	w.TypeRegistry = testRegistryWith(&creatures.MonsterType{Name: "fire elemental", MaxHealth: 100})

	mt := &creatures.MonsterType{Name: "Summoner", MaxSummons: 2}
	mt.Summons = []creatures.MonsterSummon{
		{Name: "fire elemental", Chance: 100, Interval: 1000, Count: 5},
	}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)

	for i := 0; i < 10; i++ {
		m.OnThinkDefense(w, thinkTick)
	}
	if len(m.Summons) > 2 {
		t.Errorf("summoned %d, want at most maxSummons = 2", len(m.Summons))
	}
	if len(m.Summons) == 0 {
		t.Error("summoned nothing at chance 100")
	}
	for _, s := range m.Summons {
		if s.Master != Creature(m) {
			t.Error("summon is not linked to its master")
		}
	}
}

// A summon does not summon. Without the master check a chain of summons would
// multiply without bound.
func TestSummonsDoNotSummon(t *testing.T) {
	w := aiWorld(t)
	w.TypeRegistry = testRegistryWith(&creatures.MonsterType{Name: "fire elemental", MaxHealth: 100})

	mt := &creatures.MonsterType{Name: "Summoner", MaxSummons: 2}
	mt.Summons = []creatures.MonsterSummon{
		{Name: "fire elemental", Chance: 100, Interval: 1000, Count: 5},
	}
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, mt)
	m.Master = &Monster{}

	for i := 0; i < 5; i++ {
		m.OnThinkDefense(w, thinkTick)
	}
	if len(m.Summons) != 0 {
		t.Errorf("a summon spawned %d summons of its own", len(m.Summons))
	}
}

func testRegistryWith(types ...*creatures.MonsterType) *creatures.TypeRegistry {
	r := creatures.NewTypeRegistry()
	for _, t := range types {
		r.Monsters[t.Name] = t
	}
	return r
}
