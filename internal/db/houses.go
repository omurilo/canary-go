package db

import (
	"context"
	"database/sql"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadHouses loads all houses from the houses table into the world.
func (d *DB) LoadHouses(ctx context.Context, w *game.World) error {
	const q = `SELECT id, name, owner, rent, size, beds, town_id FROM houses`
	rows, err := d.SQL.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var h game.House
		h.RentPeriod = "monthly"
		var ownerID sql.NullInt64
		if err := rows.Scan(&h.ID, &h.Name, &ownerID, &h.Rent, &h.Size, &h.Beds, &h.TownID); err != nil {
			continue
		}
		if ownerID.Valid {
			h.OwnerID = uint32(ownerID.Int64)
		}
		w.RegisterHouse(&h)
	}
	return nil
}

// SaveHouseOwner persists the owner of a house.
func (d *DB) SaveHouseOwner(ctx context.Context, houseID uint32, ownerID uint32) error {
	_, err := d.SQL.ExecContext(ctx, `UPDATE houses SET owner = ? WHERE id = ?`, ownerID, houseID)
	return err
}

// SaveHouseAccessList persists the access list for a house.
func (d *DB) SaveHouseAccessList(ctx context.Context, houseID uint32, access game.AccessList) error {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM house_lists WHERE house_id = ?`, houseID); err != nil {
		return err
	}
	for _, player := range access.Players {
		if _, err := tx.ExecContext(ctx, `INSERT INTO house_lists (house_id, type, value) VALUES (?, 'player', ?)`, houseID, player); err != nil {
			return err
		}
	}
	for _, guild := range access.Guilds {
		if _, err := tx.ExecContext(ctx, `INSERT INTO house_lists (house_id, type, value) VALUES (?, 'guild', ?)`, houseID, guild); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// EnsureHousesTables creates the houses and house_lists tables if they don't exist.
func (d *DB) EnsureHousesTables(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS houses (
		id INT UNSIGNED NOT NULL,
		name VARCHAR(100) NOT NULL DEFAULT '',
		owner INT UNSIGNED NOT NULL DEFAULT 0,
		rent INT UNSIGNED NOT NULL DEFAULT 0,
		size INT UNSIGNED NOT NULL DEFAULT 0,
		town_id INT UNSIGNED NOT NULL DEFAULT 0,
		beds SMALLINT UNSIGNED NOT NULL DEFAULT 0,
		PRIMARY KEY (id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	if _, err := d.SQL.ExecContext(ctx, ddl); err != nil {
		return err
	}
	const ddl2 = `CREATE TABLE IF NOT EXISTS house_lists (
		house_id INT UNSIGNED NOT NULL,
		type VARCHAR(20) NOT NULL,
		value VARCHAR(100) NOT NULL,
		PRIMARY KEY (house_id, type, value)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	_, err := d.SQL.ExecContext(ctx, ddl2)
	return err
}
