package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayer loads a character by name into a game.Player. The town temple is
// used when the stored position is (0,0,0).
func (d *DB) LoadPlayer(ctx context.Context, name string) (*game.Player, error) {
	const q = `SELECT p.id, p.account_id, p.name, p.level, p.vocation, p.sex,
	                  p.health, p.healthmax, p.mana, p.manamax, p.experience,
	                  p.maglevel, p.soul, p.cap,
	                  p.looktype, p.lookhead, p.lookbody, p.looklegs, p.lookfeet,
	                  p.lookaddons,
	                  p.posx, p.posy, p.posz, p.town_id,
	                  p.skill_fist, p.skill_club, p.skill_sword, p.skill_axe,
	                  p.skill_dist, p.skill_shielding, p.skill_fishing
	           FROM players p WHERE p.name = ? LIMIT 1`

	p := &game.Player{}
	var townID int
	var capValue uint32
	var lookType, lookHead, lookBody, lookLegs, lookFeet, lookAddons uint16
	var posx, posy uint16
	var posz uint8
	err := d.SQL.QueryRowContext(ctx, q, name).Scan(
		&p.DBID, &p.AccountID, &p.Name, &p.Level, &p.Vocation, &p.Sex,
		&p.Health, &p.MaxHealth, &p.Mana, &p.MaxMana, &p.Experience,
		&p.MagLevel, &p.Soul, &capValue,
		&lookType, &lookHead, &lookBody, &lookLegs, &lookFeet, &lookAddons,
		&posx, &posy, &posz, &townID,
		&p.Skills[game.SkillFist], &p.Skills[game.SkillClub], &p.Skills[game.SkillSword],
		&p.Skills[game.SkillAxe], &p.Skills[game.SkillDistance], &p.Skills[game.SkillShielding],
		&p.Skills[game.SkillFishing],
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	p.Capacity = capValue
	p.Outfit = game.Outfit{
		LookType:  lookType,
		Head:      uint8(lookHead),
		Body:      uint8(lookBody),
		Legs:      uint8(lookLegs),
		Feet:      uint8(lookFeet),
		Addons:    uint8(lookAddons),
		// The canonical Canary schema does not persist the mount creature id in
		// a single column (only mount outfit colors), so it defaults to 0.
	}
	p.Pos = game.Position{X: posx, Y: posy, Z: posz}

	if p.Pos.X == 0 && p.Pos.Y == 0 {
		if temple, err := d.TownTemple(ctx, townID); err == nil {
			p.Pos = temple
		}
	}
	return p, nil
}

// SavePlayer persists mutable player state (position, vitals, experience).
func (d *DB) SavePlayer(ctx context.Context, p *game.Player) error {
	const q = `UPDATE players SET
	              level=?, experience=?, health=?, healthmax=?,
	              mana=?, manamax=?, soul=?, cap=?,
	              posx=?, posy=?, posz=?,
	              looktype=?, lookhead=?, lookbody=?, looklegs=?,
	              lookfeet=?, lookaddons=?
	           WHERE id=?`
	_, err := d.SQL.ExecContext(ctx, q,
		p.Level, p.Experience, p.Health, p.MaxHealth,
		p.Mana, p.MaxMana, p.Soul, p.Capacity,
		p.Pos.X, p.Pos.Y, p.Pos.Z,
		p.Outfit.LookType, p.Outfit.Head, p.Outfit.Body, p.Outfit.Legs,
		p.Outfit.Feet, p.Outfit.Addons,
		p.DBID,
	)
	return err
}

// TownTemple returns the temple position of a town.
func (d *DB) TownTemple(ctx context.Context, townID int) (game.Position, error) {
	const q = `SELECT posx, posy, posz FROM towns WHERE id = ?`
	var x, y uint16
	var z uint8
	err := d.SQL.QueryRowContext(ctx, q, townID).Scan(&x, &y, &z)
	if err != nil {
		return game.Position{}, err
	}
	return game.Position{X: x, Y: y, Z: z}, nil
}
