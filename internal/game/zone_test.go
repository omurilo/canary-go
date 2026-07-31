package game

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAreaContainsAndIntersects(t *testing.T) {
	a := Area{From: Position{X: 10, Y: 10, Z: 7}, To: Position{X: 12, Y: 11, Z: 8}}

	// Bounds are inclusive on both ends and on every axis.
	for _, in := range []Position{
		{X: 10, Y: 10, Z: 7}, {X: 12, Y: 11, Z: 8}, {X: 11, Y: 10, Z: 8},
	} {
		if !a.Contains(in) {
			t.Errorf("Contains(%v) = false, want true", in)
		}
	}
	for _, out := range []Position{
		{X: 9, Y: 10, Z: 7}, {X: 13, Y: 10, Z: 7},
		{X: 10, Y: 9, Z: 7}, {X: 10, Y: 12, Z: 7},
		{X: 10, Y: 10, Z: 6}, {X: 10, Y: 10, Z: 9},
	} {
		if a.Contains(out) {
			t.Errorf("Contains(%v) = true, want false", out)
		}
	}

	// 3 x 2 x 2
	if got := len(a.Positions()); got != 12 {
		t.Errorf("Positions() yielded %d, want 12", got)
	}
	// A reversed area is empty rather than looping forever.
	bad := Area{From: Position{X: 5, Y: 5, Z: 7}, To: Position{X: 1, Y: 1, Z: 7}}
	if got := bad.Positions(); got != nil {
		t.Errorf("a reversed area yielded %d positions, want none", len(got))
	}

	if !a.Intersects(Area{From: Position{X: 12, Y: 11, Z: 8}, To: Position{X: 20, Y: 20, Z: 8}}) {
		t.Errorf("touching areas must intersect")
	}
	if a.Intersects(Area{From: Position{X: 13, Y: 10, Z: 7}, To: Position{X: 20, Y: 20, Z: 7}}) {
		t.Errorf("disjoint areas must not intersect")
	}
}

func TestZoneAddAndSubtractArea(t *testing.T) {
	w := NewWorld()
	z, err := w.Zones.Add("test-zone", 0)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	z.AddArea(Area{From: Position{X: 100, Y: 100, Z: 7}, To: Position{X: 102, Y: 100, Z: 7}})
	if z.Size() != 3 {
		t.Fatalf("zone holds %d positions, want 3", z.Size())
	}
	mid := Position{X: 101, Y: 100, Z: 7}
	if !z.Contains(mid) {
		t.Errorf("zone must contain %v", mid)
	}
	// The position index is what Zone.getByPosition reads.
	if got := w.Zones.At(mid); len(got) != 1 || got[0] != z {
		t.Errorf("At(%v) = %v, want the zone", mid, got)
	}

	z.SubtractArea(Area{From: mid, To: mid})
	if z.Contains(mid) {
		t.Errorf("%v was subtracted and must be gone", mid)
	}
	if z.Size() != 2 {
		t.Errorf("zone holds %d positions, want 2", z.Size())
	}
	if got := w.Zones.At(mid); len(got) != 0 {
		t.Errorf("At(%v) still returns %v; the index was not updated", mid, got)
	}

	// Positions come back in a stable order — scripts index into this list.
	got := z.Positions()
	if len(got) != 2 || got[0].X != 100 || got[1].X != 102 {
		t.Errorf("Positions() = %v, want (100,..) then (102,..)", got)
	}
}

// The subtle part: the OTBM claims zone ids before the XML names them, so Add must
// rename the existing zone rather than create a second one. Zone::addZone has this
// linking branch and ZoneRegistry.ByID auto-creates for exactly this reason.
func TestZoneRegistryLinksOTBMIdsToXMLNames(t *testing.T) {
	w := NewWorld()

	// 1) The map parses first and claims id 7, unnamed.
	w.Zones.ApplyZonePositions(map[uint16][]Position{
		7: {{X: 50, Y: 50, Z: 7}, {X: 51, Y: 50, Z: 7}},
	})
	byID := w.Zones.ByID(7)
	if byID == nil {
		t.Fatal("id 7 should have been auto-created by the OTBM positions")
	}
	if byID.Name() != "" {
		t.Errorf("a map-created zone starts unnamed, got %q", byID.Name())
	}
	if byID.Size() != 2 {
		t.Errorf("zone 7 holds %d positions, want 2", byID.Size())
	}

	// 2) The XML then names it. It must be the SAME zone, positions intact.
	named, err := w.Zones.Add("hazard-area", 7)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if named != byID {
		t.Fatalf("Add created a second zone instead of naming the existing one")
	}
	if named.Size() != 2 {
		t.Errorf("naming the zone lost its positions: %d left", named.Size())
	}
	if w.Zones.ByName("hazard-area") != byID {
		t.Errorf("the zone is not reachable by its new name")
	}
	if w.Zones.Count() != 1 {
		t.Errorf("registry holds %d zones, want 1", w.Zones.Count())
	}

	// A map zone is static; a Lua-created one (id 0) is not.
	if !named.IsStatic() {
		t.Errorf("a zone with an id must be static")
	}
	dyn, _ := w.Zones.Add("script-zone", 0)
	if dyn.IsStatic() {
		t.Errorf("a zone created from Lua must not be static")
	}
}

func TestZoneRegistryRejectsReservedAndDuplicateNames(t *testing.T) {
	w := NewWorld()
	if _, err := w.Zones.Add("default", 0); err == nil {
		t.Errorf(`"default" is reserved upstream and must be refused`)
	}
	if _, err := w.Zones.Add("dup", 0); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if _, err := w.Zones.Add("dup", 0); err == nil {
		t.Errorf("a duplicate name must be refused")
	}
	// Id 0 has no zone: Zone::getZone(0) returns the null zone.
	if got := w.Zones.ByID(0); got != nil {
		t.Errorf("ByID(0) = %v, want nil", got)
	}
}

func TestZoneMembershipAndRemoval(t *testing.T) {
	w := NewWorld()
	inside := Position{X: 200, Y: 200, Z: 7}
	outside := Position{X: 300, Y: 300, Z: 7}
	w.Map.SetTile(inside, &Tile{Ground: &Item{ID: 1}})
	w.Map.SetTile(outside, &Tile{Ground: &Item{ID: 1}})

	z, _ := w.Zones.Add("arena", 0)
	z.AddArea(Area{From: inside, To: inside})

	rat := NewMonster(10, "Rat", nil)
	rat.SetPosition(inside)
	w.AddCreature(rat)
	faraway := NewMonster(11, "Cave Rat", nil)
	faraway.SetPosition(outside)
	w.AddCreature(faraway)

	p := &Player{Name: "Inside", TownID: 1}
	p.SetPosition(inside)
	w.AddPlayer(p, nil)

	if got := len(z.Monsters()); got != 1 {
		t.Errorf("Monsters() = %d, want 1 (the one outside must not count)", got)
	}
	if got := len(z.Players()); got != 1 {
		t.Errorf("Players() = %d, want 1", got)
	}
	// getCreatures covers players too, unlike World.Creatures().
	if got := len(z.Creatures()); got != 2 {
		t.Errorf("Creatures() = %d, want 2 (a monster and a player)", got)
	}

	// RemovePlayers needs somewhere to send them: an explicit destination wins over
	// the temple, as in Zone::getRemoveDestination.
	temple := Position{X: 400, Y: 400, Z: 7}
	w.Map.SetTile(temple, &Tile{Ground: &Item{ID: 1}})
	z.SetRemoveDestination(temple)
	if got := z.RemoveDestination(p); got != temple {
		t.Errorf("RemoveDestination = %v, want %v", got, temple)
	}
	// Only players are ejected; a monster has no destination.
	if got := z.RemoveDestination(rat); got != (Position{}) {
		t.Errorf("RemoveDestination(monster) = %v, want the zero position", got)
	}

	z.RemovePlayers()
	if p.GetPosition() != temple {
		t.Errorf("player was not moved: at %v, want %v", p.GetPosition(), temple)
	}
	if got := len(z.Players()); got != 0 {
		t.Errorf("Players() = %d after RemovePlayers, want 0", got)
	}

	z.RemoveMonsters()
	if got := len(z.Monsters()); got != 0 {
		t.Errorf("Monsters() = %d after RemoveMonsters, want 0", got)
	}
	// The monster outside the zone must survive.
	if w.CreatureByID(faraway.GetID()) == nil {
		t.Errorf("RemoveMonsters deleted a monster outside the zone")
	}
}

func TestLoadZonesFromXML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "world-zones.xml")
	if err := os.WriteFile(path, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<zones>
	<zone name="hazard-area" zoneid="1" />
	<zone name="ebb-and-flow" zoneid="2" />
	<zone name="default" zoneid="3" />
</zones>`), 0o600); err != nil {
		t.Fatal(err)
	}

	w := NewWorld()
	named, err := w.Zones.LoadZonesFromXML(path, 0)
	// "default" is reserved, so it is reported but must not stop the other two.
	if err == nil {
		t.Errorf("the reserved name should have been reported")
	}
	if named != 2 {
		t.Errorf("named %d zones, want 2", named)
	}
	if z := w.Zones.ByName("hazard-area"); z == nil || z.ID() != 1 {
		t.Errorf("hazard-area missing or wrong id: %v", z)
	}
	if z := w.Zones.ByName("ebb-and-flow"); z == nil || z.ID() != 2 {
		t.Errorf("ebb-and-flow missing or wrong id: %v", z)
	}

	// shiftID left-shifts the ids so a second map cannot collide with the first.
	w2 := NewWorld()
	if _, err := w2.Zones.LoadZonesFromXML(path, 4); err == nil {
		t.Errorf("expected the reserved-name error again")
	}
	if z := w2.Zones.ByName("hazard-area"); z == nil || z.ID() != 1<<4 {
		t.Errorf("shiftID not applied: %v", z)
	}

	if _, err := w.Zones.LoadZonesFromXML(filepath.Join(dir, "nope.xml"), 0); !os.IsNotExist(err) {
		t.Errorf("a missing file must report NotExist, got %v", err)
	}
}
