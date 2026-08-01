package game

import (
	"testing"
	"time"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game/combat"
)

func meleeBlock(interval int) creatures.MonsterAttack {
	return creatures.MonsterAttack{Name: "melee", Interval: interval, Chance: 100}
}

// canUseSpell's melee floor is independent of the block's own interval: two
// swings are at least 1500ms apart even if the block asks for 500.
func TestMeleeHasA1500msFloorRegardlessOfInterval(t *testing.T) {
	m := &Monster{Type: &creatures.MonsterType{Name: "Rat"}}
	m.MaxHealth, m.Health = 100, 100
	block := meleeBlock(500)
	pos := Position{X: 100, Y: 100, Z: 7}

	m.attackTicks = 500
	inRange, reset := true, true
	if !m.CanUseSpell(pos, pos, &block, 1000, &inRange, &reset) {
		t.Fatal("the first swing must be allowed")
	}
	m.lastMeleeAttack = time.Now().UnixMilli()

	m.attackTicks = 1000
	inRange, reset = true, true
	if m.CanUseSpell(pos, pos, &block, 1000, &inRange, &reset) {
		t.Error("a second swing inside 1500ms must be refused")
	}
}

// The extra-swing flag bypasses both the floor and the interval. It is set when
// the target vanishes and comes back, so the monster does not lose the interval
// it spent staring at nothing.
func TestExtraSwingBypassesTheMeleeFloor(t *testing.T) {
	m := &Monster{Type: &creatures.MonsterType{Name: "Rat"}}
	block := meleeBlock(2000)
	pos := Position{X: 100, Y: 100, Z: 7}

	m.lastMeleeAttack = time.Now().UnixMilli()
	m.attackTicks = 0 // nowhere near the 2000ms interval either

	inRange, reset := true, true
	if m.CanUseSpell(pos, pos, &block, 1000, &inRange, &reset) {
		t.Fatal("without the flag this must be refused")
	}

	m.OnAttackedCreatureDisappear()
	if !m.HasExtraSwing() {
		t.Fatal("the target disappearing must arm the extra swing")
	}
	inRange, reset = true, true
	if !m.CanUseSpell(pos, pos, &block, 1000, &inRange, &reset) {
		t.Error("the extra swing must bypass both gates")
	}
}

// A fleeing monster does not melee at all, whatever its timers say.
func TestFleeingMonsterDoesNotMelee(t *testing.T) {
	mt := &creatures.MonsterType{Name: "Coward"}
	mt.Flags.RunHealth = 50
	m := &Monster{Type: mt}
	m.MaxHealth, m.Health = 200, 10

	block := meleeBlock(1000)
	m.attackTicks = 1000
	pos := Position{X: 100, Y: 100, Z: 7}

	inRange, reset := true, true
	if m.CanUseSpell(pos, pos, &block, 1000, &inRange, &reset) {
		t.Error("a fleeing monster must not melee")
	}
}

// A block whose interval has not elapsed holds the shared counter open, so the
// slower blocks behind it still get a chance. Reporting resetTicks = true here
// would starve every block with an interval longer than the fastest one.
func TestUnreadyBlockHoldsTheSharedCounterOpen(t *testing.T) {
	m := &Monster{Type: &creatures.MonsterType{Name: "Caster"}}
	block := creatures.MonsterAttack{Name: "fire", Interval: 5000, Chance: 100, Range: 5}
	m.attackTicks = 1000
	pos := Position{X: 100, Y: 100, Z: 7}

	inRange, reset := true, true
	if m.CanUseSpell(pos, pos, &block, 1000, &inRange, &reset) {
		t.Fatal("a block 4s short of its interval must not fire")
	}
	if reset {
		t.Error("resetTicks stayed true — the counter would reset and the block never fires")
	}
}

// Out of range reports inRange = false rather than failing silently, so the
// caller can tell "not yet" from "too far".
func TestRangeFailureIsReportedSeparately(t *testing.T) {
	m := &Monster{Type: &creatures.MonsterType{Name: "Archer"}}
	block := creatures.MonsterAttack{Name: "arrow", Interval: 1000, Chance: 100, Range: 3}
	m.attackTicks = 1000

	inRange, reset := true, true
	got := m.CanUseSpell(
		Position{X: 100, Y: 100, Z: 7},
		Position{X: 110, Y: 100, Z: 7},
		&block, 1000, &inRange, &reset)

	if got {
		t.Error("a target 10 tiles away is out of a range-3 block")
	}
	if inRange {
		t.Error("inRange must report false so the caller can close the gap")
	}
}

// getCombatValues distinguishes "no block mid-cast" from "a block dealing zero".
func TestCombatValuesReportsWhetherABlockIsActive(t *testing.T) {
	m := &Monster{}
	if _, _, ok := m.GetCombatValues(); ok {
		t.Error("no block is casting, so there are no values")
	}
	m.minCombatValue, m.maxCombatValue = 10, 20
	min, max, ok := m.GetCombatValues()
	if !ok || min != 10 || max != 20 {
		t.Errorf("got %d..%d ok=%v, want 10..20 true", min, max, ok)
	}
}

// blockHit applies the element modifier, and a modifier that zeroes the damage
// reports a block rather than a zero hit.
func TestBlockHitAppliesTheElementModifier(t *testing.T) {
	mt := &creatures.MonsterType{
		Name:     "Fire Elemental",
		Elements: map[uint32]int16{uint32(combat.CombatFire): 50, uint32(combat.CombatIce): 100},
	}
	m := &Monster{Type: mt}

	if got, blocked := m.BlockHit(nil, combat.CombatFire, 100); got != 50 || blocked {
		t.Errorf("50%% resistance: got %d blocked=%v, want 50 false", got, blocked)
	}
	if got, blocked := m.BlockHit(nil, combat.CombatIce, 100); got != 0 || !blocked {
		t.Errorf("100%% resistance: got %d blocked=%v, want 0 true", got, blocked)
	}
	if got, _ := m.BlockHit(nil, combat.CombatEarth, 100); got != 100 {
		t.Errorf("no modifier: got %d, want 100 untouched", got)
	}
}

// A fleeing monster searches out to the full server view and drops the
// clear-sight requirement, otherwise every reachable tile is closer to what it
// is running from.
func TestPathSearchParamsWidenWhenFleeing(t *testing.T) {
	mt := &creatures.MonsterType{Name: "Coward"}
	mt.Flags.RunHealth = 50
	mt.Flags.TargetDistance = 4
	m := &Monster{Type: mt}
	m.MaxHealth, m.Health = 200, 200

	calm := m.GetPathSearchParams(nil)
	if calm.MaxTargetDist != 4 || !calm.ClearSight || calm.KeepDistance {
		t.Errorf("calm: %+v, want dist 4, clear sight, no keep-distance", calm)
	}

	m.Health = 10
	fleeing := m.GetPathSearchParams(nil)
	if fleeing.MaxTargetDist != mapMaxViewPortX || fleeing.ClearSight || !fleeing.KeepDistance {
		t.Errorf("fleeing: %+v, want the full view port, no clear sight, keep distance", fleeing)
	}
}

// Death kills the summons. A summon outliving its master is an orphan with no
// spawn to return to.
func TestDeathKillsTheSummons(t *testing.T) {
	w := aiWorld(t)
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Summoner"})
	summon := aiMonster(w, Position{X: 106, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Minion"})
	summon.ID = 2
	summon.MaxHealth, summon.Health = 100, 100
	summon.Master = m
	m.Summons = []*Monster{summon}
	m.AddTarget(summon)

	m.Death(w, nil)

	if summon.GetHealth() != 0 {
		t.Errorf("summon health = %d, want it dead with its master", summon.GetHealth())
	}
	if summon.Master != nil {
		t.Error("the dead master is still set on the summon")
	}
	if len(m.Summons) != 0 || len(m.Targets) != 0 {
		t.Errorf("summons=%d targets=%d, want both cleared", len(m.Summons), len(m.Targets))
	}
}

// Taking damage while unable to reach a target lets the monster walk through
// harmful fields — otherwise a melee monster behind a magic wall stands in a
// fire bomb it could have stepped out of.
func TestDrainHealthAllowsWalkingThroughFields(t *testing.T) {
	w := aiWorld(t)
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Rat"})
	m.MaxHealth, m.Health = 100, 100
	target := aiMonster(w, Position{X: 108, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Prey"})
	target.ID = 2
	m.SetTarget(target)

	if m.IsIgnoringFieldDamage() {
		t.Fatal("an undamaged monster must respect fields")
	}
	m.DrainHealth(w, nil, 10)
	if !m.IsIgnoringFieldDamage() {
		t.Error("taking damage with an unreachable target must set the field flag")
	}
	if m.Health != 90 {
		t.Errorf("health = %d, want 90", m.Health)
	}
}

// changeHealth takes a monster out of idle unconditionally. A player with the
// ignore-by-monsters flag is in no target list, so an idle monster it attacks
// would otherwise never fight back.
func TestChangeHealthWakesAnIdleMonster(t *testing.T) {
	w := aiWorld(t)
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Rat"})
	m.MaxHealth, m.Health = 100, 100
	m.SetIdle(true)

	m.ChangeHealth(w, -5)
	if m.Idle {
		t.Error("a monster taking damage must not stay idle")
	}
}

// The corpse is stamped with the top damage dealer, or the player behind them
// when a summon landed the blow.
func TestCorpseOwnerFallsBackToTheSummonsMaster(t *testing.T) {
	m := &Monster{Type: &creatures.MonsterType{Name: "Rat"}}
	owner := &Player{Name: "Hunter"}
	owner.ID = 42
	summon := &Monster{Master: owner}
	summon.ID = 7

	corpse := m.GetCorpse(&Item{ID: 100}, summon)
	if corpse.Attr == nil || corpse.Attr.Owner == nil {
		t.Fatal("the corpse has no owner")
	}
	if *corpse.Attr.Owner != owner.ID {
		t.Errorf("owner = %d, want the summon's master %d", *corpse.Attr.Owner, owner.ID)
	}
}

// A wandering monster ambles: the one-second floor is what stops it jittering
// once per think tick.
func TestRandomStepIsRateLimited(t *testing.T) {
	w := aiWorld(t)
	m := aiMonster(w, Position{X: 105, Y: 105, Z: 7}, &creatures.MonsterType{Name: "Rat"})
	m.SpawnPosition = m.GetPosition()

	if _, ok := m.doRandomStep(w); !ok {
		t.Fatal("the first wander step must be allowed")
	}
	if _, ok := m.doRandomStep(w); ok {
		t.Error("a second wander step in the same millisecond must be refused")
	}
}
