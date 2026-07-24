package db

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"time"

	"github.com/opentibiabr/canary-go/internal/bosstiary"
	"github.com/opentibiabr/canary-go/internal/creatures"
)

// dayStamp is the current day-of-year (1-366), stored in the boosted_* tables'
// `date` column and used to detect a day change. dateSeed derives a per-day
// deterministic RNG seed so the same calendar day always yields the same pick
// ("aleatórios pela data").
func dayStamp(now time.Time) int    { return now.YearDay() }
func dateSeed(now time.Time) int64  { return int64(now.Year())*1000 + int64(now.YearDay()) }

// RotateBoostedBoss returns the current daily boosted boss name, re-picking it
// (a date-seeded random Archfoe) and persisting it to boosted_boss when the
// stored day differs from today. Mirrors IOBosstiary::loadBoostedBoss: on a new
// day it also clears the boss from any player's prowess slots.
func (d *DB) RotateBoostedBoss(ctx context.Context, reg *creatures.TypeRegistry) (string, error) {
	now := time.Now()
	today := dayStamp(now)

	var dateStr, name, raceStr string
	err := d.SQL.QueryRowContext(ctx,
		"SELECT `date`, boostname, raceid FROM boosted_boss ORDER BY `date` DESC LIMIT 1").
		Scan(&dateStr, &name, &raceStr)
	if err == nil && name != "" && name != "default" {
		if day, e := strconv.Atoi(dateStr); e == nil && day == today {
			return name, nil // already picked for today
		}
	}

	type ent struct {
		id     uint16
		name   string
		outfit creatures.Outfit
	}
	var bosses []ent
	if reg != nil {
		for _, mt := range reg.Monsters {
			if mt.IsBoss() && mt.BosstiaryRace == bosstiary.RarityArchfoe {
				bosses = append(bosses, ent{mt.BosstiaryRaceID, mt.Name, mt.Outfit})
			}
		}
	}
	if len(bosses) == 0 {
		return name, nil // nothing to pick; keep whatever was stored
	}
	sort.Slice(bosses, func(i, j int) bool { return bosses[i].id < bosses[j].id })
	pick := bosses[rand.New(rand.NewSource(dateSeed(now))).Intn(len(bosses))]

	_, _ = d.SQL.ExecContext(ctx, "DELETE FROM boosted_boss")
	if _, err := d.SQL.ExecContext(ctx,
		"INSERT INTO boosted_boss (boostname, `date`, raceid, looktype, lookhead, lookbody, looklegs, lookfeet, lookaddons, lookmount) VALUES (?,?,?,?,?,?,?,?,?,?)",
		pick.name, today, pick.id, pick.outfit.LookType, pick.outfit.Head, pick.outfit.Body,
		pick.outfit.Legs, pick.outfit.Feet, pick.outfit.Addons, pick.outfit.LookMount); err != nil {
		return pick.name, err
	}
	// A boosted boss lives in the "today" slot, so drop it from any prowess slot.
	_, _ = d.SQL.ExecContext(ctx, "UPDATE player_bosstiary SET bossIdSlotOne = 0 WHERE bossIdSlotOne = ?", pick.id)
	_, _ = d.SQL.ExecContext(ctx, "UPDATE player_bosstiary SET bossIdSlotTwo = 0 WHERE bossIdSlotTwo = ?", pick.id)
	return pick.name, nil
}

// RotateBoostedCreature returns the current daily boosted monster name,
// re-picking a date-seeded random bestiary monster (any with a race id) and
// persisting it to boosted_creature when the stored day differs from today.
func (d *DB) RotateBoostedCreature(ctx context.Context, reg *creatures.TypeRegistry) (string, error) {
	now := time.Now()
	today := dayStamp(now)

	var dateStr, name, raceStr string
	err := d.SQL.QueryRowContext(ctx,
		"SELECT `date`, boostname, raceid FROM boosted_creature ORDER BY `date` DESC LIMIT 1").
		Scan(&dateStr, &name, &raceStr)
	if err == nil && name != "" && name != "default" {
		if day, e := strconv.Atoi(dateStr); e == nil && day == today {
			return name, nil
		}
	}

	type ent struct {
		id     uint16
		name   string
		outfit creatures.Outfit
	}
	var mons []ent
	if reg != nil {
		for _, mt := range reg.Monsters {
			if mt.RaceID > 0 {
				mons = append(mons, ent{mt.RaceID, mt.Name, mt.Outfit})
			}
		}
	}
	if len(mons) == 0 {
		return name, nil
	}
	sort.Slice(mons, func(i, j int) bool { return mons[i].id < mons[j].id })
	pick := mons[rand.New(rand.NewSource(dateSeed(now)+1)).Intn(len(mons))]

	_, _ = d.SQL.ExecContext(ctx, "DELETE FROM boosted_creature")
	if _, err := d.SQL.ExecContext(ctx,
		"INSERT INTO boosted_creature (boostname, `date`, raceid, looktype, lookhead, lookbody, looklegs, lookfeet, lookaddons, lookmount) VALUES (?,?,?,?,?,?,?,?,?,?)",
		pick.name, today, pick.id, pick.outfit.LookType, pick.outfit.Head, pick.outfit.Body,
		pick.outfit.Legs, pick.outfit.Feet, pick.outfit.Addons, pick.outfit.LookMount); err != nil {
		return pick.name, err
	}
	return pick.name, nil
}
