package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/items"
)

func aiWorld(t *testing.T) *World {
	t.Helper()
	w := NewWorld()
	w.Items = items.NewCatalog(
		&items.ItemType{ID: 1, Name: "ground"},
		&items.ItemType{ID: 2, Name: "wall", BlockSolid: true, BlockProjectile: true},
	)
	for x := 100; x <= 120; x++ {
		for y := 100; y <= 110; y++ {
			w.Map.SetTile(Position{X: uint16(x), Y: uint16(y), Z: 7}, &Tile{Ground: &Item{ID: 1}})
		}
	}
	return w
}

func aiMonster(w *World, pos Position, mt *creatures.MonsterType) *Monster {
	m := NewMonster(1, "Test", mt)
	m.SetPosition(pos)
	w.AddCreature(m)
	return m
}

// A monster with runAwayHealth set stops fighting below it. Without isFleeing a
// wounded monster walked into melee and died where upstream would have run.
func TestIsFleeingHonoursRunHealth(t *testing.T) {
	mt := &creatures.MonsterType{Name: "Test"}
	mt.Flags.RunHealth = 50
	m := &Monster{Type: mt}
	m.MaxHealth, m.Health = 200, 200

	if m.IsFleeing() {
		t.Errorf("a healthy monster must not flee")
	}
	m.Health = 50
	if !m.IsFleeing() {
		t.Errorf("at exactly runAwayHealth the monster must flee")
	}

	// runAwayHealth 0 is the default and means never — not "always", which a bare
	// health check would produce.
	noRun := &creatures.MonsterType{Name: "Brave"}
	b := &Monster{Type: noRun}
	b.MaxHealth, b.Health = 200, 1
	if b.IsFleeing() {
		t.Errorf("runAwayHealth 0 must mean the monster never flees")
	}
}

// targetDistance is what separates a caster from a melee monster. It defaults to
// 1 so anything without the flag still closes in.
func TestTargetDistance(t *testing.T) {
	plain := &Monster{Type: &creatures.MonsterType{Name: "Rat"}}
	if got := plain.TargetDistanceOf(); got != 1 {
		t.Errorf("default target distance = %d, want 1", got)
	}
	mt := &creatures.MonsterType{Name: "Caster"}
	mt.Flags.TargetDistance = 4
	if got := (&Monster{Type: mt}).TargetDistanceOf(); got != 4 {
		t.Errorf("target distance = %d, want 4", got)
	}
}

// A wall between them means no shot: canUseAttack gates on line of sight, so a
// distance monster cannot fire through stone.
func TestCanUseAttackNeedsSight(t *testing.T) {
	w := aiWorld(t)
	mt := &creatures.MonsterType{Name: "Caster"}
	mt.Flags.TargetDistance = 5
	m := aiMonster(w, Position{X: 100, Y: 100, Z: 7}, mt)

	victim := &Player{Name: "Victim", DBID: 1, GroupID: 1}
	victim.SetPosition(Position{X: 104, Y: 100, Z: 7})
	w.AddPlayer(victim, nil)

	if !m.CanUseAttack(m.GetPosition(), victim, w) {
		t.Fatalf("a clear line within range must be attackable")
	}

	wall := w.Map.GetTile(Position{X: 102, Y: 100, Z: 7})
	wall.Items = append(wall.Items, &Item{ID: 2})
	if m.CanUseAttack(m.GetPosition(), victim, w) {
		t.Errorf("a blockprojectile tile in the way must stop the attack")
	}

	// Out of reach is refused regardless of sight.
	victim.SetPosition(Position{X: 115, Y: 100, Z: 7})
	if m.CanUseAttack(m.GetPosition(), victim, w) {
		t.Errorf("beyond targetDistance must be refused")
	}
}

// The strategy roll: with strategiesTargetHealth at 100 the monster must take the
// weakest reachable creature, not the closest. Always-nearest was the only
// behaviour before.
func TestSearchTargetStrategies(t *testing.T) {
	newVictim := func(w *World, name string, id uint32, hp uint32, pos Position) *Player {
		p := &Player{Name: name, DBID: id, GroupID: 1}
		p.MaxHealth, p.Health = 500, hp
		p.SetPosition(pos)
		w.AddPlayer(p, nil)
		return p
	}

	t.Run("nearest", func(t *testing.T) {
		w := aiWorld(t)
		mt := &creatures.MonsterType{Name: "Rat"}
		mt.Flags.Hostile = true
		mt.Flags.StrategiesTargetNearest = 100
		m := aiMonster(w, Position{X: 100, Y: 100, Z: 7}, mt)

		newVictim(w, "Far", 1, 10, Position{X: 105, Y: 100, Z: 7})
		near := newVictim(w, "Near", 2, 500, Position{X: 101, Y: 100, Z: 7})

		if !m.SearchTarget(w, TargetSearchDefault) {
			t.Fatalf("a reachable creature must be found")
		}
		if m.GetTarget() != Creature(near) {
			t.Errorf("nearest strategy picked %v, want Near", m.GetTarget())
		}
	})

	t.Run("weakest", func(t *testing.T) {
		w := aiWorld(t)
		mt := &creatures.MonsterType{Name: "Hunter"}
		mt.Flags.Hostile = true
		mt.Flags.StrategiesTargetHealth = 100
		m := aiMonster(w, Position{X: 100, Y: 100, Z: 7}, mt)

		weak := newVictim(w, "Weak", 1, 10, Position{X: 105, Y: 100, Z: 7})
		newVictim(w, "Strong", 2, 500, Position{X: 101, Y: 100, Z: 7})

		if !m.SearchTarget(w, TargetSearchDefault) {
			t.Fatalf("a reachable creature must be found")
		}
		if m.GetTarget() != Creature(weak) {
			t.Errorf("health strategy picked %v, want Weak — this is the case that always-nearest got wrong", m.GetTarget())
		}
	})

	t.Run("staff and ghosts are never targets", func(t *testing.T) {
		w := aiWorld(t)
		mt := &creatures.MonsterType{Name: "Rat"}
		mt.Flags.Hostile = true
		m := aiMonster(w, Position{X: 100, Y: 100, Z: 7}, mt)

		ghost := newVictim(w, "Ghost", 1, 100, Position{X: 101, Y: 100, Z: 7})
		ghost.Ghost = true

		if m.SearchTarget(w, TargetSearchDefault) {
			t.Errorf("a ghost must not be targeted, got %v", m.GetTarget())
		}
	})
}

// The dance step keeps the monster at the same distance from its target and
// refuses to run when it is closer than its fighting distance — a caster that has
// been walked into has to back off, not circle.
func TestDanceStep(t *testing.T) {
	w := aiWorld(t)
	mt := &creatures.MonsterType{Name: "Caster"}
	mt.Flags.Hostile = true
	mt.Flags.TargetDistance = 3
	m := aiMonster(w, Position{X: 103, Y: 100, Z: 7}, mt)

	victim := &Player{Name: "Victim", DBID: 1, GroupID: 1}
	victim.MaxHealth, victim.Health = 100, 100
	victim.SetPosition(Position{X: 100, Y: 100, Z: 7})
	w.AddPlayer(victim, nil)
	m.SetTarget(victim)

	dir, ok := m.GetDanceStep(w, true, true)
	if !ok {
		t.Fatalf("at its fighting distance the monster should have somewhere to dance")
	}
	// North and south both hold the chebyshev distance at 3; east and west change it.
	if dir != DirNorth && dir != DirSouth {
		t.Errorf("dance step went %v, which does not hold the distance", dir)
	}

	// Standing closer than targetDistance: no dancing.
	m.SetPosition(Position{X: 101, Y: 100, Z: 7})
	if _, ok := m.GetDanceStep(w, true, true); ok {
		t.Errorf("a monster inside its fighting distance must not dance, it must back off")
	}
}

// Sight is symmetric and stops at a blockprojectile tile; the straight-line arms
// are the ones ported exactly from C++.
func TestIsSightClear(t *testing.T) {
	w := aiWorld(t)
	a := Position{X: 100, Y: 100, Z: 7}
	b := Position{X: 105, Y: 100, Z: 7}

	if !w.IsSightClear(a, b) {
		t.Fatalf("open ground must be clear")
	}
	w.Map.GetTile(Position{X: 103, Y: 100, Z: 7}).Items = []*Item{{ID: 2}}
	if w.IsSightClear(a, b) {
		t.Errorf("a wall on the line must block it")
	}
	if w.IsSightClear(b, a) {
		t.Errorf("sight must block in both directions")
	}
	// The endpoints themselves never block.
	w.Map.GetTile(a).Items = []*Item{{ID: 2}}
	if !w.IsSightClear(a, Position{X: 101, Y: 100, Z: 7}) {
		t.Errorf("the shooter's own tile must not block its shot")
	}
	// Different floors never see each other.
	if w.IsSightClear(a, Position{X: 100, Y: 100, Z: 6}) {
		t.Errorf("sight must not cross floors")
	}
}
