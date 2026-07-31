package db

import (
	"context"
	"time"
)

// market_history is the record of what happened to every offer. Nothing wrote it,
// so the client's history tab was hardcoded to two zero counts: a player had no way
// to see what they had sold, bought, cancelled or lost to expiry.

// MarketOfferState_t (src/creatures/creatures_definitions.hpp:358).
const (
	OfferStateActive    uint8 = 0
	OfferStateCancelled uint8 = 1
	OfferStateExpired   uint8 = 2
	OfferStateAccepted  uint8 = 3
	// OfferStateAcceptedEx marks the COUNTERPARTY's row of an accepted trade — the
	// person who clicked accept, as opposed to the one whose offer it was. It is
	// collapsed back to Accepted when the history is read, which is why the two
	// exist at all.
	OfferStateAcceptedEx uint8 = 255
)

// HistoryOffer is one row of a player's market history.
type HistoryOffer struct {
	ItemID uint16
	Amount uint16
	Price  uint64
	// Timestamp is the `expires_at` column, which for a completed offer is when it
	// left the market rather than when it would have expired.
	Timestamp uint32
	State     uint8
	Tier      uint8
}

// AppendMarketHistory records one completed offer, mirroring
// IOMarket::appendHistory (src/io/iomarket.cpp:309).
func (d *DB) AppendMarketHistory(ctx context.Context, playerID uint32, sale uint8,
	itemID uint16, amount uint16, price uint64, timestamp int64, tier uint8, state uint8,
) error {
	_, err := d.SQL.ExecContext(ctx,
		"INSERT INTO `market_history` (`player_id`, `sale`, `itemtype`, `amount`, `price`, `expires_at`, `inserted`, `state`, `tier`) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		playerID, sale, itemID, amount, price, timestamp, time.Now().Unix(), state, tier)
	return err
}

// MoveOfferToHistory takes an offer off the market and records how it ended,
// mirroring IOMarket::moveOfferToHistory. The read, the delete and the insert are
// one transaction: a crash between them would either lose the offer with no record
// of it, or leave it on the market as well as in the history.
func (d *DB) MoveOfferToHistory(ctx context.Context, offerID uint32, state uint8) (bool, error) {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var playerID uint32
	var sale uint8
	var itemType, amount uint16
	var price uint64
	var created int64
	var tier uint8
	err = tx.QueryRowContext(ctx,
		"SELECT `player_id`, `sale`, `itemtype`, `amount`, `price`, `created`, `tier` FROM `market_offers` WHERE `id` = ?",
		offerID).Scan(&playerID, &sale, &itemType, &amount, &price, &created, &tier)
	if err != nil {
		// A missing offer is not an error: it was already moved or cancelled.
		return false, nil
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM `market_offers` WHERE `id` = ?", offerID); err != nil {
		return false, err
	}
	// C++ stamps the moment it left the market, not the moment it was created.
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO `market_history` (`player_id`, `sale`, `itemtype`, `amount`, `price`, `expires_at`, `inserted`, `state`, `tier`) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		playerID, sale, itemType, amount, price, time.Now().Unix(), time.Now().Unix(), state, tier); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// GetOwnMarketHistory returns a player's history for one side of the market,
// mirroring IOMarket::getOwnHistory. sale is 0 for buy history and 1 for sell.
//
// ACCEPTEDEX is collapsed to ACCEPTED here, exactly as upstream does: the extended
// value exists only to tell the two sides of a trade apart when writing, and the
// client has no state for it.
func (d *DB) GetOwnMarketHistory(ctx context.Context, playerID uint32, sale uint8) ([]HistoryOffer, error) {
	rows, err := d.SQL.QueryContext(ctx,
		"SELECT `itemtype`, `amount`, `price`, `expires_at`, `state`, `tier` FROM `market_history` "+
			"WHERE `player_id` = ? AND `sale` = ? ORDER BY `inserted` DESC",
		playerID, sale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HistoryOffer
	for rows.Next() {
		var h HistoryOffer
		if err := rows.Scan(&h.ItemID, &h.Amount, &h.Price, &h.Timestamp, &h.State, &h.Tier); err != nil {
			return nil, err
		}
		if h.State == OfferStateAcceptedEx {
			h.State = OfferStateAccepted
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
