package game

import (
	"encoding/xml"
	"testing"
	"time"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game/spawns"
)

// newSpawnTestEngine wires an engine over a world with three monster slots in a
// single spawn group, all due to spawn (zero lastSpawn) and with no player near.
func newSpawnTestEngine(t *testing.T, slots int) (*SpawnEngine, *SpawnBlock) {
	t.Helper()
	w := NewWorld()
	reg := creatures.NewTypeRegistry()
	// Blockable, so the tests below exercise the synchronous spawn path.
	// A non-blockable monster is scheduled 4.2s out through the dispatcher
	// (TestNonBlockableSpawnIsDelayed covers that arm).
	rat := &creatures.MonsterType{Name: "Rat"}
	rat.Flags.IsBlockable = true
	reg.Monsters["rat"] = rat

	e := NewSpawnEngine(w, reg)
	block := &SpawnBlock{
		centerPos: Position{X: 1000, Y: 1000, Z: 7},
		interval:  defaultSpawnInterval,
		blocks:    make(map[uint32]*spawnBlock),
		spawned:   make(map[uint32]Creature),
		engine:    e,
		// SpawnEngine.Start arms every group; a hand-built one has to say so or
		// the sweep skips it.
		checkActive: true,
	}
	for i := 0; i < slots; i++ {
		block.addMonster("rat", Position{X: uint16(1000 + i), Y: 1000, Z: 7}, DirSouth, 30*time.Second)
	}
	if len(block.blocks) != slots {
		t.Fatalf("set up %d slots, want %d", len(block.blocks), slots)
	}
	e.blocks = append(e.blocks, block)
	return e, block
}

// The group used to carry one shared state flag, flipped to "alive" as soon as
// any slot spawned and only cleared by a death. A slot that was not due on the
// pass that filled its siblings — the normal case, since a player standing in
// the spawn pushes its timer forward — was then skipped on every later pass, so
// it stayed empty for as long as any sibling lived. C++ gates each slot on its
// own spawnedMonsterMap entry and nothing else.
func TestCheckSpawnsRefillsASlotWhileSiblingsLive(t *testing.T) {
	e, block := newSpawnTestEngine(t, 2)
	start := time.Now()

	// Hold one slot back on the first pass: not due yet.
	var held *spawnBlock
	for _, sb := range block.blocks {
		held = sb
		break
	}
	held.lastSpawn = start

	e.checkSpawnsOnce(start)
	block.stateMu.Lock()
	first := len(block.spawned)
	block.stateMu.Unlock()
	if first != 1 {
		t.Fatalf("first pass spawned %d, want 1 (one slot was held back)", first)
	}

	// Its interval has now elapsed and its sibling is still alive.
	e.checkSpawnsOnce(start.Add(31 * time.Second))
	block.stateMu.Lock()
	second := len(block.spawned)
	block.stateMu.Unlock()
	if second != 2 {
		t.Fatalf("held slot never spawned (occupancy %d, want 2): a living sibling is suppressing it", second)
	}
}

// An occupied slot must be skipped while its creature lives, and reopen once it
// dies — the per-slot half of the same gate.
func TestCheckSpawnsSkipsOccupiedSlotUntilDeath(t *testing.T) {
	e, block := newSpawnTestEngine(t, 2)
	e.RegisterHooks()

	e.checkSpawnsOnce(time.Now())
	block.stateMu.Lock()
	first := len(block.spawned)
	var victim Creature
	for _, c := range block.spawned {
		victim = c
		break
	}
	block.stateMu.Unlock()
	if first != 2 {
		t.Fatalf("first pass spawned %d, want 2", first)
	}

	// Another pass must not duplicate anything while both live.
	e.checkSpawnsOnce(time.Now())
	block.stateMu.Lock()
	again := len(block.spawned)
	block.stateMu.Unlock()
	if again != 2 {
		t.Fatalf("second pass changed occupancy to %d, want 2 (occupied slots must be skipped)", again)
	}

	e.CreatureDied(victim)
	block.stateMu.Lock()
	afterDeath := len(block.spawned)
	block.stateMu.Unlock()
	if afterDeath != 1 {
		t.Fatalf("after death occupancy is %d, want 1", afterDeath)
	}

	// The freed slot only refills once its interval elapses.
	e.checkSpawnsOnce(time.Now())
	block.stateMu.Lock()
	tooSoon := len(block.spawned)
	block.stateMu.Unlock()
	if tooSoon != 1 {
		t.Fatalf("slot refilled before its respawn interval (occupancy %d, want 1)", tooSoon)
	}

	e.checkSpawnsOnce(time.Now().Add(31 * time.Second))
	block.stateMu.Lock()
	refilled := len(block.spawned)
	block.stateMu.Unlock()
	if refilled != 2 {
		t.Fatalf("slot did not refill after its interval (occupancy %d, want 2)", refilled)
	}
}

// getMonsterType returns the boss outright and otherwise honours weights, per
// spawnBlock_t::getMonsterType.
func TestSpawnBlockGetMonsterType(t *testing.T) {
	rat := &creatures.MonsterType{Name: "Rat"}
	if got := (&spawnBlock{monsterTypes: map[*creatures.MonsterType]uint32{rat: 1}}).getMonsterType(); got != rat {
		t.Errorf("single type: got %v, want the only type", got)
	}
	if got := (&spawnBlock{monsterTypes: map[*creatures.MonsterType]uint32{}}).getMonsterType(); got != nil {
		t.Errorf("empty block: got %v, want nil", got)
	}
	// A zero total weight has nothing to pick from.
	if got := (&spawnBlock{monsterTypes: map[*creatures.MonsterType]uint32{rat: 0, {Name: "Cave Rat"}: 0}}).getMonsterType(); got != nil {
		t.Errorf("zero weights: got %v, want nil", got)
	}
}

// Spawn XML uses direction="N"; reading "dir" left every creature facing north.
func TestSpawnParserReadsDirectionAttribute(t *testing.T) {
	var node spawns.CreatureNode
	raw := `<monster name="Rat" x="1" y="2" z="7" spawntime="60" direction="2"/>`
	if err := xml.Unmarshal([]byte(raw), &node); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if node.Direction != 2 {
		t.Errorf("Direction = %d, want 2 (the XML attribute is \"direction\", not \"dir\")", node.Direction)
	}
}

// The spawn decision branches on isBlockable and the port had it backwards for
// both halves. A nearby player holds back only a BLOCKABLE monster; everything
// else spawns regardless, after a delay that shows a teleport effect first.
//
// 1647 of the 1655 datapack monsters are non-blockable, so the old behaviour —
// player nearby suppresses the respawn, and the spawn is instant when it fires
// — was wrong for essentially every monster in the game.
func TestNonBlockableSpawnIsDelayedAndIgnoresNearbyPlayers(t *testing.T) {
	e, block := newSpawnTestEngine(t, 1)
	// Undo the fixture's blockable flag for this one.
	for _, sb := range block.blocks {
		for mType := range sb.monsterTypes {
			mType.Flags.IsBlockable = false
		}
	}
	t.Cleanup(func() {
		for _, sb := range block.blocks {
			for mType := range sb.monsterTypes {
				mType.Flags.IsBlockable = true
			}
		}
	})

	// A player standing right on the spawn must not hold it back.
	p := &Player{Name: "Camper", DBID: 1}
	p.MaxHealth, p.Health = 100, 100
	p.SetPosition(Position{X: 1000, Y: 1000, Z: 7})
	e.world.AddPlayer(p, nil)

	var effects int
	e.world.OnMagicEffect = func(Position, uint16) { effects++ }

	e.checkSpawnsOnce(time.Now())

	block.stateMu.Lock()
	spawned := len(block.spawned)
	block.stateMu.Unlock()
	if spawned != 0 {
		t.Errorf("non-blockable spawn was immediate (%d), want it scheduled", spawned)
	}
	if effects == 0 {
		t.Error("no teleport effect — the player gets no warning the monster is coming")
	}

	// And the slot's timer was NOT pushed forward by the nearby player.
	for _, sb := range block.blocks {
		if !sb.lastSpawn.IsZero() {
			t.Error("a nearby player pushed a non-blockable slot's respawn clock")
		}
	}
}

// A blockable monster is the opposite: held back while a player is there.
func TestBlockableSpawnIsHeldBackByANearbyPlayer(t *testing.T) {
	e, block := newSpawnTestEngine(t, 1)

	p := &Player{Name: "Camper", DBID: 1}
	p.MaxHealth, p.Health = 100, 100
	p.SetPosition(Position{X: 1000, Y: 1000, Z: 7})
	e.world.AddPlayer(p, nil)

	now := time.Now()
	e.checkSpawnsOnce(now)

	block.stateMu.Lock()
	spawned := len(block.spawned)
	block.stateMu.Unlock()
	if spawned != 0 {
		t.Errorf("spawned %d with a player standing there, want 0", spawned)
	}
	for _, sb := range block.blocks {
		if sb.lastSpawn.IsZero() {
			t.Error("the respawn clock must restart while a player is in the spawn")
		}
	}
}

// isInZone is a square, not a circle. A circular test rejects the corners of the
// spawn area, so monsters never appear there.
func TestIsInZoneIsASquare(t *testing.T) {
	center := Position{X: 1000, Y: 1000, Z: 7}

	if !IsInZone(center, 10, Position{X: 1010, Y: 1010, Z: 7}) {
		t.Error("the corner of a radius-10 box is inside the zone")
	}
	if IsInZone(center, 10, Position{X: 1011, Y: 1000, Z: 7}) {
		t.Error("one tile past the edge is outside")
	}
	if !IsInZone(center, -1, Position{X: 9999, Y: 9999, Z: 7}) {
		t.Error("radius -1 is unbounded")
	}
}
