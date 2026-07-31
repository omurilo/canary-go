package db

import "context"

// LoadGuildWarList returns the guilds this one is actively at war with, the port of
// IOGuild::getWarList (src/io/ioguild.cpp:60).
//
// A war row names both sides, so the query matches on either column and the caller
// wants the OTHER one. Only status 1 counts: a pending, rejected or ended war does
// not make its kills justified.
func (d *DB) LoadGuildWarList(ctx context.Context, guildID uint32) ([]uint32, error) {
	rows, err := d.SQL.QueryContext(ctx,
		"SELECT `guild1`, `guild2` FROM `guild_wars` WHERE (`guild1` = ? OR `guild2` = ?) AND `status` = 1",
		guildID, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []uint32
	for rows.Next() {
		var g1, g2 uint32
		if err := rows.Scan(&g1, &g2); err != nil {
			return nil, err
		}
		if guildID != g1 {
			out = append(out, g1)
		} else {
			out = append(out, g2)
		}
	}
	return out, rows.Err()
}
