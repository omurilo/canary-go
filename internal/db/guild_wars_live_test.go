package db

import (
	"context"
	"testing"
)

// The query has two traps: a war row names both sides so the caller wants the OTHER
// column, and only status 1 counts — a pending, rejected or ended war must not
// excuse a kill. Both are easy to get wrong and silent when wrong.
func TestLoadGuildWarListAgainstLiveDB(t *testing.T) {
	d, ctx := liveDB(t)

	const g1, g2, g3, g4 = 900101, 900102, 900103, 900104
	cleanup := func() {
		d.SQL.ExecContext(context.Background(),
			"DELETE FROM `guild_wars` WHERE guild1 IN (?,?,?,?) OR guild2 IN (?,?,?,?)",
			g1, g2, g3, g4, g1, g2, g3, g4)
	}
	cleanup()
	t.Cleanup(cleanup)

	insert := func(a, b uint32, status int) {
		t.Helper()
		if _, err := d.SQL.ExecContext(ctx,
			"INSERT INTO `guild_wars` (guild1, guild2, name1, name2, status, started, ended) VALUES (?, ?, 'A', 'B', ?, 0, 0)",
			a, b, status); err != nil {
			t.Fatalf("insert war: %v", err)
		}
	}

	insert(g1, g2, 1) // active, g1 listed first
	insert(g3, g1, 1) // active, g1 listed second
	insert(g1, g4, 0) // pending: must be ignored
	insert(g1, g4, 2) // ended: must be ignored

	got, err := d.LoadGuildWarList(ctx, g1)
	if err != nil {
		t.Fatalf("LoadGuildWarList: %v", err)
	}
	set := map[uint32]bool{}
	for _, id := range got {
		set[id] = true
	}
	if len(got) != 2 {
		t.Fatalf("war list = %v, want exactly the two active opponents", got)
	}
	if !set[g2] {
		t.Errorf("missing %d, the opponent listed in guild2", g2)
	}
	if !set[g3] {
		t.Errorf("missing %d, the opponent listed in guild1", g3)
	}
	if set[g4] {
		t.Errorf("%d is only in a pending/ended war and must not appear", g4)
	}
	if set[g1] {
		t.Errorf("the guild must never list itself as an opponent")
	}

	// A guild with no wars gets an empty list, not an error.
	none, err := d.LoadGuildWarList(ctx, 900999)
	if err != nil {
		t.Fatalf("LoadGuildWarList(no wars): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("a guild with no wars must get an empty list, got %v", none)
	}
}
