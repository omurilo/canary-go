package db

import (
	"context"

	"github.com/omurilo/canary-go/internal/game"
)

// LoadPlayerHirelings loads all hirelings for a player from the DB.
func (d *DB) LoadPlayerHirelings(ctx context.Context, p *game.Player) error {
	if p == nil {
		return nil
	}
	rows, err := d.SQL.QueryContext(ctx,
		`SELECT id, name, active, sex, posx, posy, posz,
		        lookbody, lookfeet, lookhead, looklegs, looktype
		 FROM player_hirelings WHERE player_id = ?`, p.DBID)
	if err != nil {
		return err
	}
	defer rows.Close()

	p.Hirelings = nil
	for rows.Next() {
		h := &game.Hireling{PlayerID: p.DBID}
		var active int8
		var posx, posy, posz int32
		if err := rows.Scan(&h.ID, &h.Name, &active, &h.Sex,
			&posx, &posy, &posz,
			&h.LookBody, &h.LookFeet, &h.LookHead, &h.LookLegs, &h.LookType); err != nil {
			return err
		}
		h.Active = active != 0
		h.Pos = game.Position{X: uint16(posx), Y: uint16(posy), Z: uint8(posz)}
		p.Hirelings = append(p.Hirelings, h)
	}
	return rows.Err()
}

// SavePlayerHirelings persists all hirelings for a player.
func (d *DB) SavePlayerHirelings(ctx context.Context, p *game.Player) error {
	if p == nil {
		return nil
	}
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing and re-insert.
	if _, err := tx.ExecContext(ctx, `DELETE FROM player_hirelings WHERE player_id = ?`, p.DBID); err != nil {
		return err
	}
	for _, h := range p.Hirelings {
		active := 0
		if h.Active {
			active = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO player_hirelings (player_id, name, active, sex, posx, posy, posz,
			 lookbody, lookfeet, lookhead, looklegs, looktype)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			h.PlayerID, h.Name, active, h.Sex,
			int32(h.Pos.X), int32(h.Pos.Y), int32(h.Pos.Z),
			h.LookBody, h.LookFeet, h.LookHead, h.LookLegs, h.LookType); err != nil {
			return err
		}
	}
	return tx.Commit()
}
