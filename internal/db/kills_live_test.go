package db

import (
	"context"
	"testing"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
)

// The frag list drives the skull system, so losing it on restart means a red or
// black skull silently disappears. This runs against the live MariaDB because an
// in-memory codec test would not prove the table works.
func TestPlayerKillsPersistAgainstLiveDB(t *testing.T) {
	d, ctx := liveDB(t)

	// player_kills has a foreign key onto players, so borrow an existing character
	// rather than inventing one.
	var guid uint32
	if err := d.SQL.QueryRowContext(ctx, "SELECT id FROM players LIMIT 1").Scan(&guid); err != nil {
		t.Skipf("no player rows to attach kills to: %v", err)
	}
	if _, err := d.SQL.ExecContext(ctx, "DELETE FROM `player_kills` WHERE player_id = ?", guid); err != nil {
		t.Fatalf("clean: %v", err)
	}
	t.Cleanup(func() {
		d.SQL.ExecContext(context.Background(), "DELETE FROM `player_kills` WHERE player_id = ?", guid)
	})

	now := time.Now().Unix()
	stale := now - fragTime() - 60 // just past the window
	p := &game.Player{DBID: guid, Name: "kills-test"}
	p.UnjustifiedKills = []game.Kill{
		{Target: 1001, Time: now - 10, Unavenged: true},
		{Target: 1002, Time: now - 20, Unavenged: false},
		{Target: 1003, Time: stale, Unavenged: true}, // must be dropped on load
	}

	if err := d.SavePlayerKills(ctx, p); err != nil {
		t.Fatalf("SavePlayerKills: %v", err)
	}

	// All three rows are stored; the filtering happens on load, as upstream does.
	var stored int
	if err := d.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM `player_kills` WHERE player_id = ?", guid).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 3 {
		t.Fatalf("stored %d rows, want 3", stored)
	}

	// A fresh player, as after a restart.
	loaded := &game.Player{DBID: guid, Name: "kills-test"}
	if err := d.LoadPlayerKills(ctx, loaded); err != nil {
		t.Fatalf("LoadPlayerKills: %v", err)
	}
	if len(loaded.UnjustifiedKills) != 2 {
		t.Fatalf("loaded %d kills, want 2 (the one past FRAG_TIME must be dropped)",
			len(loaded.UnjustifiedKills))
	}
	if loaded.LastKillTime != now-10 {
		t.Errorf("LastKillTime = %d, want %d (the newest kill)", loaded.LastKillTime, now-10)
	}
	byTarget := map[uint32]game.Kill{}
	for _, k := range loaded.UnjustifiedKills {
		byTarget[k.Target] = k
	}
	if k, ok := byTarget[1001]; !ok || !k.Unavenged {
		t.Errorf("kill 1001 lost its unavenged flag: %+v", k)
	}
	if k, ok := byTarget[1002]; !ok || k.Unavenged {
		t.Errorf("kill 1002 should not be unavenged: %+v", k)
	}
	if _, ok := byTarget[1003]; ok {
		t.Errorf("the stale kill was loaded despite being past FRAG_TIME")
	}

	// Saving an emptied list must clear the rows, not leave the old ones behind.
	loaded.UnjustifiedKills = nil
	if err := d.SavePlayerKills(ctx, loaded); err != nil {
		t.Fatalf("SavePlayerKills(empty): %v", err)
	}
	if err := d.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM `player_kills` WHERE player_id = ?", guid).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Errorf("%d rows left after saving an empty list, want 0", stored)
	}
}
