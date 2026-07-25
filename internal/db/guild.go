package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
)

func (d *DB) LoadGuild(ctx context.Context, guildID uint32) (*game.Guild, error) {
	const q = `SELECT id, name, ownerid, creationdata, motd, balance, points, level
	           FROM guilds WHERE id = ? LIMIT 1`

	g := &game.Guild{}
	var creationTimestamp int64
	err := d.SQL.QueryRowContext(ctx, q, guildID).Scan(
		&g.ID, &g.Name, &g.OwnerID, &creationTimestamp,
		&g.MOTD, &g.Balance, &g.Points, &g.Level,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	g.CreationDate = time.Unix(creationTimestamp, 0)

	rows, err := d.SQL.QueryContext(ctx, "SELECT id, name, level FROM guild_ranks WHERE guild_id = ?", guildID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id uint32
			var name string
			var level uint8
			if err := rows.Scan(&id, &name, &level); err == nil {
				g.AddRank(id, name, level)
			}
		}
	}

	var count uint32
	_ = d.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM guild_membership WHERE guild_id = ?", guildID).Scan(&count)
	g.MemberCount = count

	return g, nil
}

func (d *DB) LoadGuildByName(ctx context.Context, name string) (*game.Guild, error) {
	var guildID uint32
	err := d.SQL.QueryRowContext(ctx, "SELECT id FROM guilds WHERE name = ? LIMIT 1", name).Scan(&guildID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return d.LoadGuild(ctx, guildID)
}

func (d *DB) SaveGuild(ctx context.Context, g *game.Guild) error {
	const q = `UPDATE guilds SET motd = ?, balance = ?, points = ?, level = ? WHERE id = ?`
	_, err := d.SQL.ExecContext(ctx, q, g.MOTD, g.Balance, g.Points, g.Level, g.ID)
	return err
}

func (d *DB) CreateGuild(ctx context.Context, name string, ownerID uint32) (uint32, error) {
	const q = `INSERT INTO guilds (name, ownerid, creationdata, motd, balance, points, level) 
	           VALUES (?, ?, ?, '', 0, 0, 1)`

	result, err := d.SQL.ExecContext(ctx, q, name, ownerID, time.Now().Unix())
	if err != nil {
		return 0, err
	}

	guildID64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	guildID := uint32(guildID64)

	rankNames := []string{"Leader", "Vice-Leader", "Member"}
	rankLevels := []uint8{3, 2, 1}

	for i, rankName := range rankNames {
		const rankQ = `INSERT INTO guild_ranks (guild_id, name, level) VALUES (?, ?, ?)`
		rankResult, err := d.SQL.ExecContext(ctx, rankQ, guildID, rankName, rankLevels[i])
		if err != nil {
			return 0, err
		}

		if i == 0 {
			rankID64, err := rankResult.LastInsertId()
			if err != nil {
				return 0, err
			}
			rankID := uint32(rankID64)

			const memberQ = `INSERT INTO guild_membership (player_id, guild_id, rank_id, nick) VALUES (?, ?, ?, '')`
			_, err = d.SQL.ExecContext(ctx, memberQ, ownerID, guildID, rankID)
			if err != nil {
				return 0, err
			}
		}
	}

	return guildID, nil
}

func (d *DB) DeleteGuild(ctx context.Context, guildID uint32) error {
	_, err := d.SQL.ExecContext(ctx, "DELETE FROM guilds WHERE id = ?", guildID)
	return err
}

func (d *DB) AddGuildMember(ctx context.Context, playerID, guildID, rankID uint32, nick string) error {
	const q = `INSERT INTO guild_membership (player_id, guild_id, rank_id, nick) VALUES (?, ?, ?, ?)`
	_, err := d.SQL.ExecContext(ctx, q, playerID, guildID, rankID, nick)
	return err
}

func (d *DB) RemoveGuildMember(ctx context.Context, playerID uint32) error {
	_, err := d.SQL.ExecContext(ctx, "DELETE FROM guild_membership WHERE player_id = ?", playerID)
	return err
}

func (d *DB) UpdateGuildMemberRank(ctx context.Context, playerID, rankID uint32) error {
	const q = `UPDATE guild_membership SET rank_id = ? WHERE player_id = ?`
	_, err := d.SQL.ExecContext(ctx, q, rankID, playerID)
	return err
}

func (d *DB) UpdateGuildMemberNick(ctx context.Context, playerID uint32, nick string) error {
	const q = `UPDATE guild_membership SET nick = ? WHERE player_id = ?`
	_, err := d.SQL.ExecContext(ctx, q, nick, playerID)
	return err
}

func (d *DB) CreateGuildRank(ctx context.Context, guildID uint32, name string, level uint8) (uint32, error) {
	const q = `INSERT INTO guild_ranks (guild_id, name, level) VALUES (?, ?, ?)`
	result, err := d.SQL.ExecContext(ctx, q, guildID, name, level)
	if err != nil {
		return 0, err
	}
	rankID64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint32(rankID64), nil
}

func (d *DB) DeleteGuildRank(ctx context.Context, rankID uint32) error {
	_, err := d.SQL.ExecContext(ctx, "DELETE FROM guild_ranks WHERE id = ?", rankID)
	return err
}

func (d *DB) GetGuildMembers(ctx context.Context, guildID uint32) ([]game.GuildMember, error) {
	const q = `SELECT player_id, guild_id, rank_id, nick FROM guild_membership WHERE guild_id = ?`
	rows, err := d.SQL.QueryContext(ctx, q, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []game.GuildMember{}
	for rows.Next() {
		var m game.GuildMember
		if err := rows.Scan(&m.PlayerID, &m.GuildID, &m.RankID, &m.Nick); err == nil {
			members = append(members, m)
		}
	}
	return members, nil
}
