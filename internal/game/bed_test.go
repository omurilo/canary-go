package game

import "testing"

// Bed sleepers are keyed by the player's DB id (GUID), which is stable across
// sessions — the creature id changes every login, so keying on it would never
// match a wake-on-login.
func TestBedSleeperLifecycle(t *testing.T) {
	w := NewWorld()
	pos := Position{X: 100, Y: 100, Z: 7}

	if got := w.BedSleeper(pos); got != 0 {
		t.Fatalf("fresh bed has sleeper %d", got)
	}

	w.SetBedSleeper(pos, 8, 694) // DB id of the sleeping player
	if got := w.BedSleeper(pos); got != 8 {
		t.Fatalf("after sleeping, sleeper = %d, want 8", got)
	}

	w.RemoveBedSleeper(pos)
	if got := w.BedSleeper(pos); got != 0 {
		t.Fatalf("after wake, sleeper = %d, want 0", got)
	}
}

// A player who slept in a bed logs back in on the bed tile; AddPlayer must free
// the sleeper so the bed is usable again.
func TestAddPlayerFreesBedSleeper(t *testing.T) {
	w := NewWorld()
	pos := Position{X: 100, Y: 100, Z: 7}
	w.SetBedSleeper(pos, 99, 694) // DBID 99 slept in this bed

	p := &Player{DBID: 99, Name: "Sleeper"}
	p.Pos = pos // they logged back in on the bed tile
	if !w.AddPlayer(p, nil) {
		t.Fatal("AddPlayer refused the login")
	}
	if got := w.BedSleeper(pos); got != 0 {
		t.Fatalf("bed still has sleeper %d after the player woke", got)
	}
}

// A player logging in somewhere else must not free a bed they did not sleep in.
func TestAddPlayerKeepsForeignSleeper(t *testing.T) {
	w := NewWorld()
	bedPos := Position{X: 100, Y: 100, Z: 7}
	w.SetBedSleeper(bedPos, 42, 700) // another player (DBID 42) is asleep

	p := &Player{DBID: 99, Name: "Other"}
	p.Pos = Position{X: 200, Y: 200, Z: 7}
	if !w.AddPlayer(p, nil) {
		t.Fatal("AddPlayer failed")
	}
	if got := w.BedSleeper(bedPos); got != 42 {
		t.Fatalf("foreign bed lost its sleeper: got %d, want 42", got)
	}
}
// A two-tile bed records the sleeper on BOTH halves, so PlayerBedParts returns
// each part with its own free item id for the wake transform.
func TestPlayerBedPartsReturnsBothHalves(t *testing.T) {
	w := NewWorld()
	pillow := Position{X: 100, Y: 100, Z: 7}
	blanket := Position{X: 101, Y: 100, Z: 7}
	w.SetBedSleeper(pillow, 8, 694)
	w.SetBedSleeper(blanket, 8, 695)

	parts := w.PlayerBedParts(8)
	if len(parts) != 2 {
		t.Fatalf("PlayerBedParts(8) = %d entries, want 2", len(parts))
	}
	freeIDs := map[uint16]bool{}
	for _, p := range parts {
		freeIDs[p.FreeID] = true
	}
	if !freeIDs[694] || !freeIDs[695] {
		t.Errorf("free ids = %v, want both 694 and 695", freeIDs)
	}

	// Another player must not see these parts.
	if got := w.PlayerBedParts(99); len(got) != 0 {
		t.Errorf("PlayerBedParts(99) = %d entries, want 0", len(got))
	}
}
