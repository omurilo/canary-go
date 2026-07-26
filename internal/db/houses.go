package db

import (
	"context"
	"database/sql"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadHouses loads all houses from the houses table into the world.
func (d *DB) LoadHouses(ctx context.Context, w *game.World) error {
	const q = `SELECT id, name, owner, rent, size, beds, posx, posy, posz FROM houses`
	rows, err := d.SQL.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var h game.House
		h.RentPeriod = "monthly"
		var ownerID sql.NullInt64
		if err := rows.Scan(&h.ID, &h.Name, &ownerID, &h.Rent, &h.Size, &h.Beds, &h.Position.X, &h.Position.Y, &h.Position.Z); err != nil {
			continue
		}
		if ownerID.Valid {
			h.OwnerID = uint32(ownerID.Int64)
		}
		w.RegisterHouse(&h)
	}

	// Load access lists for each house
	for _, h := range w.AllHouses() {
		d.loadHouseAccessList(ctx, h)
		d.loadHouseGuests(ctx, h)
	}

	return nil
}

func (d *DB) loadHouseAccessList(ctx context.Context, h *game.House) {
	const q = `SELECT type, value FROM house_lists WHERE house_id = ?`
	rows, err := d.SQL.QueryContext(ctx, q, h.ID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var listType string
		var value string
		if err := rows.Scan(&listType, &value); err == nil {
			switch listType {
			case "guild":
				h.AccessList.Guilds = append(h.AccessList.Guilds, value)
			case "player":
				h.AccessList.Players = append(h.AccessList.Players, value)
			}
		}
	}
}

func (d *DB) loadHouseGuests(ctx context.Context, h *game.House) {
	const q = `SELECT player_id FROM house_access WHERE house_id = ? AND access_type = 'guest'`
	rows, err := d.SQL.QueryContext(ctx, q, h.ID)
	if err != nil {
		return
	}
	defer rows.Close()
	// Guest list is loaded by name via player lookup; for now the access list is sufficient
	_ = rows
}

// SaveHouseOwner persists ownership changes for a house.
func (d *DB) SaveHouseOwner(ctx context.Context, houseID uint32, ownerID uint32) error {
	_, err := d.SQL.ExecContext(ctx, `UPDATE houses SET owner = ? WHERE id = ?`, ownerID, houseID)
	return err
}

// SaveHouseAccessList persists the full access list for a house (DELETE + INSERT).
func (d *DB) SaveHouseAccessList(ctx context.Context, houseID uint32, access game.AccessList) error {
	if _, err := d.SQL.ExecContext(ctx, `DELETE FROM house_lists WHERE house_id = ?`, houseID); err != nil {
		return err
	}
	const q = `INSERT INTO house_lists (house_id, type, value) VALUES (?, ?, ?)`
	for _, guild := range access.Guilds {
		if _, err := d.SQL.ExecContext(ctx, q, houseID, "guild", guild); err != nil {
			return err
		}
	}
	for _, player := range access.Players {
		if _, err := d.SQL.ExecContext(ctx, q, houseID, "player", player); err != nil {
			return err
		}
	}
	return nil
}

// EnsureHousesTables creates house-related tables if they don't exist.
func (d *DB) EnsureHousesTables(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS houses (
		id INT UNSIGNED NOT NULL,
		name VARCHAR(100) NOT NULL DEFAULT '',
		owner INT UNSIGNED NOT NULL DEFAULT 0,
		rent INT UNSIGNED NOT NULL DEFAULT 0,
		size INT UNSIGNED NOT NULL DEFAULT 0,
		town_id INT UNSIGNED NOT NULL DEFAULT 0,
		beds SMALLINT UNSIGNED NOT NULL DEFAULT 0,
		rent_period VARCHAR(20) NOT NULL DEFAULT 'monthly',
		posx SMALLINT UNSIGNED NOT NULL DEFAULT 0,
		posy SMALLINT UNSIGNED NOT NULL DEFAULT 0,
		posz TINYINT UNSIGNED NOT NULL DEFAULT 0,
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
