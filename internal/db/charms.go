package db

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"

	"github.com/opentibiabr/canary-go/internal/game"
)

// charmsBlobSize is the fixed on-disk size of the per-charm state blob:
// 25 charms x (u16 raceId + u8 tier).
const charmsBlobSize = 25 * 3

// encodeCharms serialises a player's per-charm state (tier + assigned race) as
// a fixed-width blob for the player_charms.charms column.
func encodeCharms(p *game.Player) []byte {
	buf := make([]byte, charmsBlobSize)
	for i := range p.Charms {
		off := i * 3
		binary.LittleEndian.PutUint16(buf[off:], p.Charms[i].RaceID)
		buf[off+2] = p.Charms[i].Tier
	}
	return buf
}

// decodeCharms restores per-charm state from the blob (no-op if empty/short).
func decodeCharms(p *game.Player, blob []byte) {
	for i := range p.Charms {
		off := i * 3
		if off+3 > len(blob) {
			return
		}
		p.Charms[i].RaceID = binary.LittleEndian.Uint16(blob[off:])
		p.Charms[i].Tier = blob[off+2]
	}
}

// LoadPlayerCharms loads the player's charm-point balances, rune bitmasks and
// per-charm assignments from player_charms. Missing row => defaults (all zero).
func (d *DB) LoadPlayerCharms(ctx context.Context, p *game.Player) error {
	const q = `SELECT charm_points, max_charm_points, minor_charm_echoes,
	                  max_minor_charm_echoes, charm_expansion, UsedRunesBit,
	                  UnlockedRunesBit, charms
	           FROM player_charms WHERE player_id = ?`
	var blob []byte
	err := d.SQL.QueryRowContext(ctx, q, p.DBID).Scan(
		&p.CharmPoints, &p.MaxCharmPoints, &p.MinorCharmEchoes,
		&p.MaxMinorCharmEchoes, &p.CharmExpansion, &p.UsedRunesBit,
		&p.UnlockedRunesBit, &blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	decodeCharms(p, blob)
	return nil
}

// SavePlayerCharms persists the player's charm state. player_charms has no
// unique key on player_id, so DELETE+INSERT.
func (d *DB) SavePlayerCharms(ctx context.Context, p *game.Player) error {
	if _, err := d.SQL.ExecContext(ctx, `DELETE FROM player_charms WHERE player_id = ?`, p.DBID); err != nil {
		return err
	}
	const q = `INSERT INTO player_charms
	           (player_id, charm_points, minor_charm_echoes, max_charm_points,
	            max_minor_charm_echoes, charm_expansion, UsedRunesBit,
	            UnlockedRunesBit, charms)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := d.SQL.ExecContext(ctx, q, p.DBID, p.CharmPoints, p.MinorCharmEchoes,
		p.MaxCharmPoints, p.MaxMinorCharmEchoes, p.CharmExpansion, p.UsedRunesBit,
		p.UnlockedRunesBit, encodeCharms(p))
	return err
}
