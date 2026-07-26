package db

import (
	"context"
	"encoding/json"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayerHazard loads the player's hazard data from the players table.
func (d *DB) LoadPlayerHazard(ctx context.Context, p *game.Player) error {
	const q = `SELECT hazard_points FROM players WHERE id = ?`
	var points uint32
	err := d.SQL.QueryRowContext(ctx, q, p.DBID).Scan(&points)
	if err != nil {
		return err
	}
	p.HazardPoints = points
	return nil
}

// SavePlayerHazard persists the player's hazard points.
func (d *DB) SavePlayerHazard(ctx context.Context, p *game.Player) error {
	_, err := d.SQL.ExecContext(ctx, `UPDATE players SET hazard_points = ? WHERE id = ?`, p.HazardPoints, p.DBID)
	return err
}

// EnsureHazardTable ensures the hazard column exists.
func (d *DB) EnsureHazardTable(ctx context.Context) error {
	_, err := d.SQL.ExecContext(ctx, `ALTER TABLE players ADD COLUMN IF NOT EXISTS hazard_points INT UNSIGNED NOT NULL DEFAULT 0`)
	if err != nil {
		// Column might already exist — ignore error
		_ = err
	}
	return nil
}

// LoadPlayerConcoctions loads the player's concoctions from the players table.
func (d *DB) LoadPlayerConcoctions(ctx context.Context, p *game.Player) error {
	var blob []byte
	err := d.SQL.QueryRowContext(ctx, `SELECT concoctions FROM players WHERE id = ?`, p.DBID).Scan(&blob)
	if err != nil || len(blob) == 0 {
		p.Concoctions = make(map[string]int64)
		return nil
	}
	p.Concoctions = make(map[string]int64)
	_ = json.Unmarshal(blob, &p.Concoctions)
	return nil
}

// SavePlayerConcoctions persists the player's concoctions as JSON blob.
func (d *DB) SavePlayerConcoctions(ctx context.Context, p *game.Player) error {
	if p.Concoctions == nil {
		return nil
	}
	blob, err := json.Marshal(p.Concoctions)
	if err != nil {
		return err
	}
	_, err = d.SQL.ExecContext(ctx, `UPDATE players SET concoctions = ? WHERE id = ?`, blob, p.DBID)
	return err
}

// EnsureConcoctionsTable ensures the concoctions column exists.
func (d *DB) EnsureConcoctionsTable(ctx context.Context) error {
	_, err := d.SQL.ExecContext(ctx, `ALTER TABLE players ADD COLUMN IF NOT EXISTS concoctions blob DEFAULT NULL`)
	if err != nil {
		_ = err
	}
	return nil
}

// LoadPlayerAnimusMastery loads the player's animus mastery blob.
func (d *DB) LoadPlayerAnimusMastery(ctx context.Context, p *game.Player) error {
	err := d.SQL.QueryRowContext(ctx, `SELECT animus_mastery FROM players WHERE id = ?`, p.DBID).Scan(&p.AnimusMastery)
	if err != nil {
		p.AnimusMastery = nil
	}
	return nil
}

// SavePlayerAnimusMastery persists the player's animus mastery blob.
func (d *DB) SavePlayerAnimusMastery(ctx context.Context, p *game.Player) error {
	_, err := d.SQL.ExecContext(ctx, `UPDATE players SET animus_mastery = ? WHERE id = ?`, p.AnimusMastery, p.DBID)
	return err
}
