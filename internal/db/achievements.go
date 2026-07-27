package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayerAchievements loads the player's unlocked achievements from
// player_achievements. Each row maps achievement_id → unlock timestamp.
func (d *DB) LoadPlayerAchievements(ctx context.Context, p *game.Player) error {
	const q = `SELECT achievement_id, unlock_time FROM player_achievements WHERE player_id = ?`
	rows, err := d.SQL.QueryContext(ctx, q, p.DBID)
	if err != nil {
		return err
	}
	defer rows.Close()

	p.Achievements = make(map[uint16]int64)
	for rows.Next() {
		var id uint16
		var ts int64
		if err := rows.Scan(&id, &ts); err == nil {
			p.Achievements[id] = ts
		}
	}
	return nil
}

// SavePlayerAchievements persists the player's unlocked achievements.
// player_achievements has a composite PK (player_id, achievement_id), so we
// DELETE + INSERT like other player sub-tables.
func (d *DB) SavePlayerAchievements(ctx context.Context, p *game.Player) error {
	if len(p.Achievements) == 0 {
		return nil
	}
	if _, err := d.SQL.ExecContext(ctx, `DELETE FROM player_achievements WHERE player_id = ?`, p.DBID); err != nil {
		return err
	}
	const q = `INSERT INTO player_achievements (player_id, achievement_id, unlock_time) VALUES (?, ?, ?)`
	for id, ts := range p.Achievements {
		if _, err := d.SQL.ExecContext(ctx, q, p.DBID, id, ts); err != nil {
			return err
		}
	}
	return nil
}

// EnsureAchievementsTable creates player_achievements if it doesn't exist.
func (d *DB) EnsureAchievementsTable(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS player_achievements (
		player_id INT UNSIGNED NOT NULL,
		achievement_id SMALLINT UNSIGNED NOT NULL,
		unlock_time INT UNSIGNED NOT NULL DEFAULT 0,
		PRIMARY KEY (player_id, achievement_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	_, err := d.SQL.ExecContext(ctx, ddl)
	return err
}

// LoadPlayerTitles loads the player's unlocked titles from player_titles.
func (d *DB) LoadPlayerTitles(ctx context.Context, p *game.Player) error {
	const q = `SELECT title FROM player_titles WHERE player_id = ?`
	rows, err := d.SQL.QueryContext(ctx, q, p.DBID)
	if err != nil {
		// Table might not exist yet — treat as empty
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	defer rows.Close()

	p.TitleStrings = nil
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err == nil {
			p.TitleStrings = append(p.TitleStrings, title)
		}
	}
	return nil
}

// SavePlayerTitles persists the player's titles.
func (d *DB) SavePlayerTitles(ctx context.Context, p *game.Player) error {
	if len(p.TitleStrings) == 0 {
		return nil
	}
	if _, err := d.SQL.ExecContext(ctx, `DELETE FROM player_titles WHERE player_id = ?`, p.DBID); err != nil {
		return err
	}
	const q = `INSERT INTO player_titles (player_id, title) VALUES (?, ?)`
	for _, title := range p.TitleStrings {
		if _, err := d.SQL.ExecContext(ctx, q, p.DBID, title); err != nil {
			return err
		}
	}
	return nil
}

// EnsureTitlesTable creates player_titles if it doesn't exist.
func (d *DB) EnsureTitlesTable(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS player_titles (
		player_id INT UNSIGNED NOT NULL,
		title VARCHAR(64) NOT NULL,
		PRIMARY KEY (player_id, title)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	_, err := d.SQL.ExecContext(ctx, ddl)
	return err
}
