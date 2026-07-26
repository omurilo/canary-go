package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayerFamiliars loads the player's unlocked familiars from
// player_familiars and the currently active familiar from the players table.
func (d *DB) LoadPlayerFamiliars(ctx context.Context, p *game.Player) error {
	const q = `SELECT familiar_id FROM player_familiars WHERE player_id = ?`
	rows, err := d.SQL.QueryContext(ctx, q, p.DBID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	defer rows.Close()

	p.Familiars = nil
	for rows.Next() {
		var lookType uint16
		if err := rows.Scan(&lookType); err == nil {
			p.Familiars = append(p.Familiars, game.Familiar{
				LookType: lookType,
				Unlocked: true,
			})
		}
	}
	return nil
}

// SavePlayerFamiliars persists the player's unlocked familiars.
func (d *DB) SavePlayerFamiliars(ctx context.Context, p *game.Player) error {
	if len(p.Familiars) == 0 {
		return nil
	}
	if _, err := d.SQL.ExecContext(ctx, `DELETE FROM player_familiars WHERE player_id = ?`, p.DBID); err != nil {
		return err
	}
	const q = `INSERT INTO player_familiars (player_id, familiar_id) VALUES (?, ?)`
	for _, f := range p.Familiars {
		if f.Unlocked {
			if _, err := d.SQL.ExecContext(ctx, q, p.DBID, f.LookType); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureFamiliarsTable creates player_familiars if it doesn't exist.
func (d *DB) EnsureFamiliarsTable(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS player_familiars (
		player_id INT UNSIGNED NOT NULL,
		familiar_id SMALLINT UNSIGNED NOT NULL,
		PRIMARY KEY (player_id, familiar_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	_, err := d.SQL.ExecContext(ctx, ddl)
	return err
}
