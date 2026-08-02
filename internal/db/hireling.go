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

// There is no Go-side save for player_hirelings: the Lua hireling system owns
// that table (PersistHireling INSERT / SaveHirelings UPDATE), and the C++ spec
// never writes it from a player save. A DELETE-and-reinsert here wiped rows the
// Lua added mid-session, so do not reintroduce one — see the note at the
// SavePlayer subsystem loop (db/player.go).
