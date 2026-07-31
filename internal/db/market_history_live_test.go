package db

import (
	"context"
	"testing"
	"time"
)

func TestMarketHistoryAgainstLiveDB(t *testing.T) {
	d, ctx := liveDB(t)

	var guid uint32
	if err := d.SQL.QueryRowContext(ctx, "SELECT id FROM players LIMIT 1").Scan(&guid); err != nil {
		t.Skipf("no player rows to attach history to: %v", err)
	}
	clean := func() {
		d.SQL.ExecContext(context.Background(), "DELETE FROM `market_history` WHERE player_id = ?", guid)
		d.SQL.ExecContext(context.Background(), "DELETE FROM `market_offers` WHERE player_id = ?", guid)
	}
	clean()
	t.Cleanup(clean)

	now := time.Now().Unix()

	// The counterparty's row is written with ACCEPTEDEX and must read back as
	// ACCEPTED: the extended value exists only to tell the two sides apart while
	// writing, and the client has no state for it.
	if err := d.AppendMarketHistory(ctx, guid, 0, 3031, 100, 500, now, 0, OfferStateAcceptedEx); err != nil {
		t.Fatalf("AppendMarketHistory: %v", err)
	}
	buys, err := d.GetOwnMarketHistory(ctx, guid, 0)
	if err != nil {
		t.Fatalf("GetOwnMarketHistory: %v", err)
	}
	if len(buys) != 1 {
		t.Fatalf("buy history has %d rows, want 1", len(buys))
	}
	if buys[0].State != OfferStateAccepted {
		t.Errorf("state = %d, want %d — ACCEPTEDEX must collapse to ACCEPTED on read",
			buys[0].State, OfferStateAccepted)
	}
	if buys[0].ItemID != 3031 || buys[0].Amount != 100 || buys[0].Price != 500 {
		t.Errorf("row did not round trip: %+v", buys[0])
	}

	// The two sides are separate lists.
	if sells, err := d.GetOwnMarketHistory(ctx, guid, 1); err != nil || len(sells) != 0 {
		t.Errorf("a buy row must not appear in the sell history: %v %v", sells, err)
	}

	// MoveOfferToHistory deletes the offer and records it, atomically.
	res, err := d.SQL.ExecContext(ctx,
		"INSERT INTO `market_offers` (player_id, sale, itemtype, amount, created, anonymous, price, tier) VALUES (?, 1, ?, ?, ?, 0, ?, 0)",
		guid, 2160, 5, now, 1000)
	if err != nil {
		t.Fatalf("insert offer: %v", err)
	}
	offerID64, _ := res.LastInsertId()
	offerID := uint32(offerID64)

	moved, err := d.MoveOfferToHistory(ctx, offerID, OfferStateCancelled)
	if err != nil {
		t.Fatalf("MoveOfferToHistory: %v", err)
	}
	if !moved {
		t.Fatalf("MoveOfferToHistory reported no move")
	}

	var remaining int
	if err := d.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM `market_offers` WHERE id = ?", offerID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Errorf("the offer is still on the market after being moved to history")
	}
	sells, err := d.GetOwnMarketHistory(ctx, guid, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(sells) != 1 {
		t.Fatalf("sell history has %d rows, want the cancelled offer", len(sells))
	}
	if sells[0].State != OfferStateCancelled || sells[0].ItemID != 2160 || sells[0].Amount != 5 {
		t.Errorf("cancelled offer recorded wrong: %+v", sells[0])
	}

	// Moving an offer that no longer exists is not an error — it was already
	// cancelled or bought out — and must not invent a history row.
	moved, err = d.MoveOfferToHistory(ctx, 99999999, OfferStateExpired)
	if err != nil {
		t.Errorf("moving a missing offer must not error: %v", err)
	}
	if moved {
		t.Errorf("moving a missing offer must report false")
	}
	if after, _ := d.GetOwnMarketHistory(ctx, guid, 1); len(after) != 1 {
		t.Errorf("a missing offer must not add a history row, now %d", len(after))
	}
}
