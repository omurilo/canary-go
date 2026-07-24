package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayerBosstiary loads the player's bosstiary prowess slots (selected boss
// per slot + removal counter) from player_bosstiary. Missing row => defaults
// (empty slots, removeTimes 1).
func (d *DB) LoadPlayerBosstiary(ctx context.Context, p *game.Player) error {
	const q = `SELECT bossIdSlotOne, bossIdSlotTwo, removeTimes FROM player_bosstiary WHERE player_id = ?`
	var one, two uint32
	var removeTimes uint8
	err := d.SQL.QueryRowContext(ctx, q, p.DBID).Scan(&one, &two, &removeTimes)
	if errors.Is(err, sql.ErrNoRows) {
		p.BossRemoveTimes = 1
		return nil
	}
	if err != nil {
		return err
	}
	p.BossSlotOne, p.BossSlotTwo, p.BossRemoveTimes = one, two, removeTimes
	return nil
}

// SavePlayerBosstiary persists the player's bosstiary slots. player_bosstiary
// has no unique key on player_id, so we DELETE+INSERT (as player_items/storage
// do). The tracker blob is not modelled yet; we write an empty blob (the column
// is NOT NULL).
func (d *DB) SavePlayerBosstiary(ctx context.Context, p *game.Player) error {
	if _, err := d.SQL.ExecContext(ctx, `DELETE FROM player_bosstiary WHERE player_id = ?`, p.DBID); err != nil {
		return err
	}
	const q = `INSERT INTO player_bosstiary (player_id, bossIdSlotOne, bossIdSlotTwo, removeTimes, tracker)
	           VALUES (?, ?, ?, ?, ?)`
	_, err := d.SQL.ExecContext(ctx, q, p.DBID, p.BossSlotOne, p.BossSlotTwo, p.GetRemoveTimes(), []byte{})
	return err
}
