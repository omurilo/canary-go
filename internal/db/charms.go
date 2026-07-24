package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayerCharms loads the player's charm-point balances and rune bitmasks
// from player_charms. Missing row => defaults (all zero).
func (d *DB) LoadPlayerCharms(ctx context.Context, p *game.Player) error {
	const q = `SELECT charm_points, max_charm_points, minor_charm_echoes,
	                  max_minor_charm_echoes, charm_expansion, UsedRunesBit, UnlockedRunesBit
	           FROM player_charms WHERE player_id = ?`
	err := d.SQL.QueryRowContext(ctx, q, p.DBID).Scan(
		&p.CharmPoints, &p.MaxCharmPoints, &p.MinorCharmEchoes,
		&p.MaxMinorCharmEchoes, &p.CharmExpansion, &p.UsedRunesBit, &p.UnlockedRunesBit)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

// SavePlayerCharms persists the player's charm state. player_charms has no
// unique key on player_id, so DELETE+INSERT (the charms/tracker blobs are not
// modelled yet and left NULL).
func (d *DB) SavePlayerCharms(ctx context.Context, p *game.Player) error {
	if _, err := d.SQL.ExecContext(ctx, `DELETE FROM player_charms WHERE player_id = ?`, p.DBID); err != nil {
		return err
	}
	const q = `INSERT INTO player_charms
	           (player_id, charm_points, minor_charm_echoes, max_charm_points,
	            max_minor_charm_echoes, charm_expansion, UsedRunesBit, UnlockedRunesBit)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := d.SQL.ExecContext(ctx, q, p.DBID, p.CharmPoints, p.MinorCharmEchoes,
		p.MaxCharmPoints, p.MaxMinorCharmEchoes, p.CharmExpansion, p.UsedRunesBit, p.UnlockedRunesBit)
	return err
}
