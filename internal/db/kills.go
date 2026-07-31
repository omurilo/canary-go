package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/game"
)

// player_kills is the unjustified-kill (frag) list that drives the skull system.
// Nothing read or wrote it, so a player's frags reset to zero on every restart:
// a red or black skull earned before a restart simply vanished, and so did the
// window in which those kills still count.

// defaultFragTime mirrors `timeToDecreaseFrags` in config.lua.dist — the age at
// which a kill stops counting (FRAG_TIME).
const defaultFragTime = 24 * 60 * 60

func fragTime() int64 {
	n := config.Number("timeToDecreaseFrags", defaultFragTime)
	if n <= 0 {
		return defaultFragTime
	}
	return n
}

// LoadPlayerKills restores the frag list, mirroring
// IOLoginDataLoad::loadPlayerKills: kills older than FRAG_TIME are dropped on load
// rather than kept and filtered later, so the list never grows without bound.
func (d *DB) LoadPlayerKills(ctx context.Context, p *game.Player) error {
	rows, err := d.SQL.QueryContext(ctx,
		"SELECT `time`, `target`, `unavenged` FROM `player_kills` WHERE `player_id` = ?", p.DBID)
	if err != nil {
		return err
	}
	defer rows.Close()

	cutoff := time.Now().Unix() - fragTime()
	var kills []game.Kill
	var lastKillTime int64
	for rows.Next() {
		var k game.Kill
		var unavenged int
		if err := rows.Scan(&k.Time, &k.Target, &unavenged); err != nil {
			return err
		}
		if k.Time < cutoff {
			continue
		}
		k.Unavenged = unavenged != 0
		kills = append(kills, k)
		if k.Time > lastKillTime {
			lastKillTime = k.Time
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	p.UnjustifiedKills = kills
	p.LastKillTime = lastKillTime
	return nil
}

// SavePlayerKills rewrites the player's frag list, mirroring
// IOLoginDataSave::savePlayerKills: delete then re-insert, so a kill that expired
// or was avenged disappears instead of lingering.
func (d *DB) SavePlayerKills(ctx context.Context, p *game.Player) error {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM `player_kills` WHERE `player_id` = ?", p.DBID); err != nil {
		return err
	}
	if len(p.UnjustifiedKills) > 0 {
		// One multi-row INSERT, like DBInsert batches upstream.
		values := make([]string, 0, len(p.UnjustifiedKills))
		args := make([]any, 0, len(p.UnjustifiedKills)*4)
		for _, k := range p.UnjustifiedKills {
			values = append(values, "(?, ?, ?, ?)")
			unavenged := 0
			if k.Unavenged {
				unavenged = 1
			}
			args = append(args, p.DBID, k.Target, k.Time, unavenged)
		}
		q := fmt.Sprintf(
			"INSERT INTO `player_kills` (`player_id`, `target`, `time`, `unavenged`) VALUES %s",
			strings.Join(values, ","))
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
