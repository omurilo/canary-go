package db

import (
	"context"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadMarketOffers loads all active market offers from the market_offers table.
func (d *DB) LoadMarketOffers(ctx context.Context, m *game.Market) error {
	const q = `SELECT id, player_id, sale, itemtype, amount, price, tier, created, anonymous, (SELECT name FROM players WHERE id = player_id) AS player_name
	           FROM market_offers`
	rows, err := d.SQL.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var offer game.MarketOffer
		var sale uint8
		var anonymous uint8
		if err := rows.Scan(&offer.ID, &offer.PlayerID, &sale, &offer.ItemID,
			&offer.Amount, &offer.Price, &offer.Tier, &offer.Timestamp, &anonymous, &offer.PlayerName); err != nil {
			continue
		}
		offer.Anonymous = anonymous > 0
		offer.Action = game.MarketAction(sale)
		m.AddOffer(&offer)
	}
	return nil
}

// CreateMarketOffer inserts a new market offer and returns its ID.
func (d *DB) CreateMarketOffer(ctx context.Context, offer *game.MarketOffer, sale uint8) (uint32, error) {
	const q = `INSERT INTO market_offers (player_id, sale, itemtype, amount, price, tier, created, anonymous)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	anonymous := uint8(0)
	if offer.Anonymous {
		anonymous = 1
	}
	result, err := d.SQL.ExecContext(ctx, q, offer.PlayerID, sale, offer.ItemID,
		offer.Amount, offer.Price, offer.Tier, offer.Timestamp, anonymous)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint32(id), nil
}

// RemoveMarketOffer deletes a market offer by ID. Returns true if any row was removed.
func (d *DB) RemoveMarketOffer(ctx context.Context, offerID uint32) (bool, error) {
	result, err := d.SQL.ExecContext(ctx, `DELETE FROM market_offers WHERE id = ?`, offerID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

// GetPlayerMarketOffers loads all offers for a specific player.
func (d *DB) GetPlayerMarketOffers(ctx context.Context, playerID uint32) ([]game.MarketOffer, error) {
	const q = `SELECT id, player_id, sale, itemtype, amount, price, tier, created, anonymous, (SELECT name FROM players WHERE id = player_id) AS player_name
	           FROM market_offers WHERE player_id = ? ORDER BY created ASC`
	rows, err := d.SQL.QueryContext(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offers []game.MarketOffer
	for rows.Next() {
		var offer game.MarketOffer
		var sale uint8
		var anonymous uint8
		if err := rows.Scan(&offer.ID, &offer.PlayerID, &sale, &offer.ItemID,
			&offer.Amount, &offer.Price, &offer.Tier, &offer.Timestamp, &anonymous, &offer.PlayerName); err != nil {
			continue
		}
		offer.Anonymous = anonymous > 0
		offer.Action = game.MarketAction(sale)
		offers = append(offers, offer)
	}
	return offers, nil
}

// EnsureMarketTable creates market_offers if it doesn't exist.
func (d *DB) EnsureMarketTable(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS market_offers (
		id INT UNSIGNED NOT NULL AUTO_INCREMENT,
		player_id INT UNSIGNED NOT NULL,
		sale TINYINT UNSIGNED NOT NULL DEFAULT 0,
		itemtype SMALLINT UNSIGNED NOT NULL,
		amount SMALLINT UNSIGNED NOT NULL,
		price BIGINT UNSIGNED NOT NULL,
		created INT UNSIGNED NOT NULL,
		anonymous TINYINT UNSIGNED NOT NULL DEFAULT 0,
		tier TINYINT UNSIGNED NOT NULL DEFAULT 0,
		PRIMARY KEY (id),
		KEY idx_player (player_id),
		KEY idx_item (itemtype, sale)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	_, err := d.SQL.ExecContext(ctx, ddl)
	return err
}
