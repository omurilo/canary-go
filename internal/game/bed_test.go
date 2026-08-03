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

	w.SetBedSleeper(pos, 8) // DB id of the sleeping player
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
	w.SetBedSleeper(pos, 99) // DBID 99 slept in this bed

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
	w.SetBedSleeper(bedPos, 42) // another player (DBID 42) is asleep

	p := &Player{DBID: 99, Name: "Other"}
	p.Pos = Position{X: 200, Y: 200, Z: 7}
	if !w.AddPlayer(p, nil) {
		t.Fatal("AddPlayer failed")
	}
	if got := w.BedSleeper(bedPos); got != 42 {
		t.Fatalf("foreign bed lost its sleeper: got %d, want 42", got)
	}
}