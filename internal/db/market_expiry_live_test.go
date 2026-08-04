package db

import (
	"context"
	"testing"
	"time"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
)

func expiryWorld() *game.World {
	w := game.NewWorld()
	w.Items = items.NewCatalog(
		&items.ItemType{ID: 3031, Name: "gold coin", Stackable: true, StackSize: 100},
		&items.ItemType{ID: 2160, Name: "crystal coin", Stackable: true, StackSize: 100},
		&items.ItemType{ID: 2400, Name: "magic sword"},
	)
	return w
}

// Expiring must give the value back. An expiry that only deletes the row is
// confiscation, so this checks the refund as closely as the deletion.
func TestExpireMarketOffersAgainstLiveDB(t *testing.T) {
	d, ctx := liveDB(t)

	var guid uint32
	var startBalance uint64
	if err := d.SQL.QueryRowContext(ctx, "SELECT id, balance FROM players LIMIT 1").Scan(&guid, &startBalance); err != nil {
		t.Skipf("no player rows: %v", err)
	}
	clean := func() {
		bg := context.Background()
		d.SQL.ExecContext(bg, "DELETE FROM `market_offers` WHERE player_id = ?", guid)
		d.SQL.ExecContext(bg, "DELETE FROM `market_history` WHERE player_id = ?", guid)
		d.SQL.ExecContext(bg, "DELETE FROM `player_inboxitems` WHERE player_id = ?", guid)
		d.SQL.ExecContext(bg, "UPDATE `players` SET balance = ? WHERE id = ?", startBalance, guid)
	}
	clean()
	t.Cleanup(clean)

	stale := time.Now().Add(-MarketOfferDuration() - time.Hour).Unix()
	fresh := time.Now().Unix()

	// An expired BUY offer: 10 units at 250 each, so 2500 gold comes back.
	if _, err := d.SQL.ExecContext(ctx,
		"INSERT INTO `market_offers` (player_id, sale, itemtype, amount, created, anonymous, price, tier) VALUES (?, 0, 3031, 10, ?, 0, 250, 0)",
		guid, stale); err != nil {
		t.Fatal(err)
	}
	// An expired SELL offer of 250 stackables: must come back as 100+100+50.
	if _, err := d.SQL.ExecContext(ctx,
		"INSERT INTO `market_offers` (player_id, sale, itemtype, amount, created, anonymous, price, tier) VALUES (?, 1, 2160, 250, ?, 0, 5, 0)",
		guid, stale); err != nil {
		t.Fatal(err)
	}
	// A fresh offer that must be left alone.
	if _, err := d.SQL.ExecContext(ctx,
		"INSERT INTO `market_offers` (player_id, sale, itemtype, amount, created, anonymous, price, tier) VALUES (?, 0, 3031, 1, ?, 0, 99, 0)",
		guid, fresh); err != nil {
		t.Fatal(err)
	}

	w := expiryWorld() // the owner is offline, the harder path
	n, err := d.ExpireMarketOffers(ctx, w)
	if err != nil {
		t.Fatalf("ExpireMarketOffers: %v", err)
	}
	if n != 2 {
		t.Fatalf("expired %d offers, want 2 (the fresh one must survive)", n)
	}

	var left int
	if err := d.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM `market_offers` WHERE player_id = ?", guid).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Errorf("%d offers left on the market, want only the fresh one", left)
	}

	// The gold came back to an offline player's row.
	var balance uint64
	if err := d.SQL.QueryRowContext(ctx, "SELECT balance FROM players WHERE id = ?", guid).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != startBalance+2500 {
		t.Errorf("balance = %d, want %d (a 10 x 250 buy offer must refund 2500)",
			balance, startBalance+2500)
	}

	// The unsold items came back, split into stacks of 100.
	rows, err := d.SQL.QueryContext(ctx,
		"SELECT itemtype, count FROM `player_inboxitems` WHERE player_id = ? ORDER BY sid", guid)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var counts []int
	for rows.Next() {
		var itemType, count int
		if err := rows.Scan(&itemType, &count); err != nil {
			t.Fatal(err)
		}
		if itemType != 2160 {
			t.Errorf("unexpected item %d in the inbox", itemType)
		}
		counts = append(counts, count)
	}
	if len(counts) != 3 || counts[0] != 100 || counts[1] != 100 || counts[2] != 50 {
		t.Errorf("stack split = %v, want [100 100 50]", counts)
	}

	// Both expiries were recorded, not merely deleted.
	hist, err := d.GetOwnMarketHistory(ctx, guid, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].State != OfferStateExpired {
		t.Errorf("sell history = %+v, want one expired row", hist)
	}

	// Running again must be a no-op: the rows are gone, so nothing can be refunded
	// a second time.
	n2, err := d.ExpireMarketOffers(ctx, w)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Errorf("a second sweep expired %d offers, want 0", n2)
	}
	if err := d.SQL.QueryRowContext(ctx, "SELECT balance FROM players WHERE id = ?", guid).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != startBalance+2500 {
		t.Errorf("balance changed on the second sweep: %d — the refund was applied twice", balance)
	}
}

// An online owner gets the items in the live inbox, so they appear without a relog.
func TestExpiredItemsGoToAnOnlineInbox(t *testing.T) {
	d, ctx := liveDB(t)

	var guid uint32
	if err := d.SQL.QueryRowContext(ctx, "SELECT id FROM players LIMIT 1").Scan(&guid); err != nil {
		t.Skipf("no player rows: %v", err)
	}
	clean := func() {
		bg := context.Background()
		d.SQL.ExecContext(bg, "DELETE FROM `market_offers` WHERE player_id = ?", guid)
		d.SQL.ExecContext(bg, "DELETE FROM `market_history` WHERE player_id = ?", guid)
		d.SQL.ExecContext(bg, "DELETE FROM `player_inboxitems` WHERE player_id = ?", guid)
	}
	clean()
	t.Cleanup(clean)

	stale := time.Now().Add(-MarketOfferDuration() - time.Hour).Unix()
	if _, err := d.SQL.ExecContext(ctx,
		"INSERT INTO `market_offers` (player_id, sale, itemtype, amount, created, anonymous, price, tier) VALUES (?, 1, 2400, 2, ?, 0, 100, 0)",
		guid, stale); err != nil {
		t.Fatal(err)
	}

	w := expiryWorld()
	online := &game.Player{Name: "Owner", DBID: guid}
	w.AddPlayer(online, nil)

	if _, err := d.ExpireMarketOffers(ctx, w); err != nil {
		t.Fatalf("ExpireMarketOffers: %v", err)
	}

	if online.Inbox == nil || online.Inbox.Container == nil || len(online.Inbox.Container.Contents) != 2 {
		t.Fatalf("the online inbox holds %v, want 2 magic swords", online.Inbox)
	}
	for _, it := range online.Inbox.Container.Contents {
		if it.ID != 2400 {
			t.Errorf("unexpected item %d in the live inbox", it.ID)
		}
	}
	// And nothing was written to the offline table for a player who is online.
	var offlineRows int
	if err := d.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM `player_inboxitems` WHERE player_id = ?", guid).Scan(&offlineRows); err != nil {
		t.Fatal(err)
	}
	if offlineRows != 0 {
		t.Errorf("%d rows written to the offline inbox for an online player", offlineRows)
	}
}
