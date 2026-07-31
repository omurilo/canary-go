package game

import (
	"testing"
	"time"
)

func TestAddDamagePoints(t *testing.T) {
	var d damageTracker

	// Creature::addDamagePoints ignores non-positive damage, so a heal or a fully
	// blocked hit does not make its caster a killer.
	d.AddDamagePoints(1, 0)
	d.AddDamagePoints(1, -5)
	if len(d.DamageMap()) != 0 {
		t.Errorf("non-positive damage must not register an attacker")
	}
	if d.LastHitCreatureID() != 0 {
		t.Errorf("non-positive damage must not make an attacker the last hitter")
	}
	// Nor does creature id 0, which is what an absent attacker looks like.
	d.AddDamagePoints(0, 50)
	if len(d.DamageMap()) != 0 {
		t.Errorf("id 0 must not register")
	}

	d.AddDamagePoints(7, 30)
	d.AddDamagePoints(9, 10)
	d.AddDamagePoints(7, 12) // accumulates onto the same attacker

	m := d.DamageMap()
	if len(m) != 2 {
		t.Fatalf("tracked %d attackers, want 2", len(m))
	}
	if m[7].Total != 42 {
		t.Errorf("attacker 7 total = %d, want 42 (30+12)", m[7].Total)
	}
	if m[9].Total != 10 {
		t.Errorf("attacker 9 total = %d, want 10", m[9].Total)
	}
	// The last hitter is whoever landed the most recent blow, not the biggest one.
	if d.LastHitCreatureID() != 7 {
		t.Errorf("last hitter = %d, want 7", d.LastHitCreatureID())
	}

	// The returned map is a copy: mutating it must not corrupt the tracker.
	m[7] = DamageBlock{Total: 9999}
	if d.DamageMap()[7].Total != 42 {
		t.Errorf("DamageMap must hand out a copy")
	}

	d.ClearDamage()
	if len(d.DamageMap()) != 0 || d.LastHitCreatureID() != 0 {
		t.Errorf("ClearDamage must reset both the map and the last hitter")
	}
}

// The selection rule from Creature::onDeath: the biggest total wins, but only among
// attackers whose last hit is still inside the in-fight window. A bigger total that
// has gone stale loses to a smaller recent one — that is what stops someone who left
// the fight minutes ago from being blamed for the kill.
func TestMostDamageAttackerHonoursTheInFightWindow(t *testing.T) {
	var d damageTracker
	now := time.Now().UnixMilli()
	d.damage = map[uint32]DamageBlock{
		1: {Total: 500, LastHit: now - 120_000}, // biggest, but long gone
		2: {Total: 100, LastHit: now - 1_000},   // small and recent
		3: {Total: 90, LastHit: now - 2_000},
	}

	if got := d.MostDamageAttackerID(DefaultInFightMillis); got != 2 {
		t.Errorf("most damage = %d, want 2 (attacker 1 is past the window despite dealing more)", got)
	}
	// Widen the window and the heavy hitter wins again.
	if got := d.MostDamageAttackerID(300_000); got != 1 {
		t.Errorf("with a wide window most damage = %d, want 1", got)
	}
	// Nobody qualifies at all.
	if got := d.MostDamageAttackerID(100); got != 0 {
		t.Errorf("with everyone stale most damage = %d, want 0", got)
	}

	// Entries with no damage or no timestamp are skipped, as upstream skips
	// total == 0 || ticks == 0.
	var empty damageTracker
	empty.damage = map[uint32]DamageBlock{
		4: {Total: 0, LastHit: now},
		5: {Total: 10, LastHit: 0},
	}
	if got := empty.MostDamageAttackerID(DefaultInFightMillis); got != 0 {
		t.Errorf("zeroed entries must not win, got %d", got)
	}
}

// Killers is what the death path calls. The two answers differ exactly when someone
// softens a target and someone else lands the finishing blow — the case the
// player_deaths row could not express before.
func TestWorldKillersDistinguishesLastHitFromMostDamage(t *testing.T) {
	w := NewWorld()
	victim := NewMonster(100, "Victim", nil)
	heavy := NewMonster(101, "Heavy", nil)
	finisher := NewMonster(102, "Finisher", nil)
	w.AddCreature(victim)
	w.AddCreature(heavy)
	w.AddCreature(finisher)

	victim.AddDamagePoints(heavy.GetID(), 400)
	victim.AddDamagePoints(finisher.GetID(), 5)

	lastHit, mostDamage := w.Killers(victim)
	if lastHit == nil || lastHit.GetID() != finisher.GetID() {
		t.Errorf("last hitter = %v, want the finisher", lastHit)
	}
	if mostDamage == nil || mostDamage.GetID() != heavy.GetID() {
		t.Errorf("most damage = %v, want the heavy hitter", mostDamage)
	}

	// With no damage recorded at all — drowning, a field, a script — there is no
	// killer, and the caller has to cope with nil rather than get a wrong name.
	untouched := NewMonster(103, "Untouched", nil)
	w.AddCreature(untouched)
	if lh, md := w.Killers(untouched); lh != nil || md != nil {
		t.Errorf("an untouched creature has no killers, got %v / %v", lh, md)
	}

	// When every contributor has gone stale, mostDamage falls back to the last
	// hitter rather than to nobody.
	stale := NewMonster(104, "Stale", nil)
	w.AddCreature(stale)
	stale.AddDamagePoints(heavy.GetID(), 50)
	stale.damage[heavy.GetID()] = DamageBlock{Total: 50, LastHit: time.Now().UnixMilli() - 600_000}
	lh, md := w.Killers(stale)
	if lh == nil || lh.GetID() != heavy.GetID() {
		t.Fatalf("last hitter should survive staleness, got %v", lh)
	}
	if md == nil || md.GetID() != heavy.GetID() {
		t.Errorf("most damage should fall back to the last hitter, got %v", md)
	}
}

// A player is a killer too, and Player embeds the tracker separately from
// BaseCreature — this catches the embedding being added to only one of them.
func TestPlayersAreDamageTracked(t *testing.T) {
	w := NewWorld()
	victim := &Player{Name: "Victim"}
	w.AddPlayer(victim, nil)
	attacker := NewMonster(50, "Rat", nil)
	w.AddCreature(attacker)

	victim.AddDamagePoints(attacker.GetID(), 25)
	lastHit, mostDamage := w.Killers(victim)
	if lastHit == nil || lastHit.GetID() != attacker.GetID() {
		t.Errorf("a player's last hitter = %v, want the rat", lastHit)
	}
	if mostDamage == nil || mostDamage.GetID() != attacker.GetID() {
		t.Errorf("a player's most-damage killer = %v, want the rat", mostDamage)
	}
}
