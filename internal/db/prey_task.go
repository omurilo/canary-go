package db

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/io/propstream"
)

// encodeRaceList serializes a monster grid (race ids) to the player_prey /
// player_taskhunt monster_list BLOB as a flat u16 stream.
func encodeRaceList(ids []uint16) []byte {
	ws := propstream.NewPropWriteStream()
	for _, id := range ids {
		ws.WriteUint16(id)
	}
	return ws.GetStream()
}

func decodeRaceList(blob []byte) []uint16 {
	if len(blob) == 0 {
		return nil
	}
	ps := propstream.NewPropStream(blob)
	var out []uint16
	for ps.Size() >= 2 {
		v, err := ps.ReadUint16()
		if err != nil {
			break
		}
		out = append(out, v)
	}
	return out
}

func parseU16(s string) uint16 {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0
	}
	return uint16(n)
}

// LoadPlayerPrey restores the 3 prey slots from player_prey.
func (d *DB) LoadPlayerPrey(ctx context.Context, p *game.Player) error {
	const q = `SELECT slot, state, raceid, ` + "`option`" + `, bonus_type, bonus_rarity,
	                  bonus_percentage, bonus_time, free_reroll, monster_list
	           FROM player_prey WHERE player_id = ?`
	rows, err := d.SQL.QueryContext(ctx, q, p.DBID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	defer rows.Close()

	prey := p.GetPrey()
	for rows.Next() {
		var slot, state, option, bonusType, bonusRarity uint8
		var raceid, bonusPct, bonusTime string
		var freeReroll int64
		var blob []byte
		if err := rows.Scan(&slot, &state, &raceid, &option, &bonusType, &bonusRarity,
			&bonusPct, &bonusTime, &freeReroll, &blob); err != nil {
			return err
		}
		s := prey.GetSlot(slot)
		if s == nil {
			continue
		}
		s.State = game.PreyState(state)
		s.SelectedRaceID = parseU16(raceid)
		s.Option = option
		s.Bonus = game.PreyBonusType(bonusType)
		s.BonusRarity = bonusRarity
		s.BonusPercentage = parseU16(bonusPct)
		s.BonusTimeLeft = parseU16(bonusTime)
		s.FreeRerollTimeStamp = freeReroll
		if ids := decodeRaceList(blob); len(ids) > 0 {
			s.RaceIDList = ids
		}
	}
	return rows.Err()
}

// SavePlayerPrey persists the 3 prey slots to player_prey.
func (d *DB) SavePlayerPrey(ctx context.Context, p *game.Player) error {
	if p.Prey == nil {
		return nil
	}
	const q = `INSERT INTO player_prey
	             (player_id, slot, state, raceid, ` + "`option`" + `, bonus_type, bonus_rarity,
	              bonus_percentage, bonus_time, free_reroll, monster_list)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	           ON DUPLICATE KEY UPDATE state=VALUES(state), raceid=VALUES(raceid),
	              ` + "`option`" + `=VALUES(` + "`option`" + `), bonus_type=VALUES(bonus_type),
	              bonus_rarity=VALUES(bonus_rarity), bonus_percentage=VALUES(bonus_percentage),
	              bonus_time=VALUES(bonus_time), free_reroll=VALUES(free_reroll),
	              monster_list=VALUES(monster_list)`
	for i := byte(0); i < 3; i++ {
		s := p.Prey.GetSlot(i)
		if s == nil {
			continue
		}
		if _, err := d.SQL.ExecContext(ctx, q, p.DBID, s.ID, uint8(s.State),
			strconv.Itoa(int(s.SelectedRaceID)), s.Option, uint8(s.Bonus), s.BonusRarity,
			strconv.Itoa(int(s.BonusPercentage)), strconv.Itoa(int(s.BonusTimeLeft)),
			s.FreeRerollTimeStamp, encodeRaceList(s.RaceIDList)); err != nil {
			return err
		}
	}
	return nil
}

// LoadPlayerTaskHunter restores the task-hunting slots from player_taskhunt.
func (d *DB) LoadPlayerTaskHunter(ctx context.Context, p *game.Player) error {
	const q = `SELECT slot, state, raceid, upgrade, rarity, kills, free_reroll, monster_list
	           FROM player_taskhunt WHERE player_id = ?`
	rows, err := d.SQL.QueryContext(ctx, q, p.DBID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	defer rows.Close()

	th := p.GetTaskHunter()
	for rows.Next() {
		var slot, state, upgrade, rarity uint8
		var raceid, kills string
		var freeReroll int64
		var blob []byte
		if err := rows.Scan(&slot, &state, &raceid, &upgrade, &rarity, &kills, &freeReroll, &blob); err != nil {
			return err
		}
		s := th.GetSlot(slot)
		if s == nil {
			continue
		}
		s.State = game.TaskHuntingState(state)
		s.SelectedRaceID = parseU16(raceid)
		s.Upgrade = upgrade != 0
		s.Rarity = rarity
		s.CurrentKills = parseU16(kills)
		s.FreeRerollTimeStamp = freeReroll
		if ids := decodeRaceList(blob); len(ids) > 0 {
			s.RaceIDList = ids
		}
		// TargetKills isn't stored (no column); recompute it from the option table
		// (difficulty defaults to Easy until bestiary stars are modeled).
		opt := game.TaskHuntingOptionFor(s.Difficulty, s.Rarity)
		if s.Upgrade {
			s.TargetKills = opt.SecondKills
		} else {
			s.TargetKills = opt.FirstKills
		}
	}
	return rows.Err()
}

// SavePlayerTaskHunter persists the task-hunting slots to player_taskhunt.
func (d *DB) SavePlayerTaskHunter(ctx context.Context, p *game.Player) error {
	if p.TaskHunter == nil {
		return nil
	}
	const q = `INSERT INTO player_taskhunt
	             (player_id, slot, state, raceid, upgrade, rarity, kills, disabled_time, free_reroll, monster_list)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	           ON DUPLICATE KEY UPDATE state=VALUES(state), raceid=VALUES(raceid),
	              upgrade=VALUES(upgrade), rarity=VALUES(rarity), kills=VALUES(kills),
	              disabled_time=VALUES(disabled_time), free_reroll=VALUES(free_reroll),
	              monster_list=VALUES(monster_list)`
	for i := byte(0); i < 9; i++ {
		s := p.TaskHunter.GetSlot(i)
		if s == nil {
			continue
		}
		upgrade := uint8(0)
		if s.Upgrade {
			upgrade = 1
		}
		if _, err := d.SQL.ExecContext(ctx, q, p.DBID, s.ID, uint8(s.State),
			strconv.Itoa(int(s.SelectedRaceID)), upgrade, s.Rarity,
			strconv.Itoa(int(s.CurrentKills)), int64(0), s.FreeRerollTimeStamp,
			encodeRaceList(s.RaceIDList)); err != nil {
			return err
		}
	}
	return nil
}
