package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/io/propstream"
)

// LoadPlayer loads a character by name into a game.Player. The town temple is
// used when the stored position is (0,0,0).
func (d *DB) LoadPlayer(ctx context.Context, name string) (*game.Player, error) {
	const q = `SELECT p.id, p.account_id, a.type as account_type, p.group_id, p.name, p.level, p.vocation, p.sex,
	                  p.health, p.healthmax, p.mana, p.manamax, p.experience,
	                  p.maglevel, p.manaspent, p.soul, p.cap, p.balance,
	                  p.looktype, p.lookhead, p.lookbody, p.looklegs, p.lookfeet,
	                  p.lookaddons,
	                  p.posx, p.posy, p.posz, p.town_id,
	                  p.skill_fist, p.skill_fist_tries,
	                  p.skill_club, p.skill_club_tries,
	                  p.skill_sword, p.skill_sword_tries,
	                  p.skill_axe, p.skill_axe_tries,
	                  p.skill_dist, p.skill_dist_tries,
	                  p.skill_shielding, p.skill_shielding_tries,
	                  p.skill_fishing, p.skill_fishing_tries,
	                  p.offlinetraining_time, p.offlinetraining_skill,
	                  p.forge_dusts, p.forge_dust_level,
	                  p.task_points, p.quickloot_fallback, p.prey_wildcard,
	                  p.lastlogin, p.lastlogout,
	                  p.blessings1, p.blessings2, p.blessings3, p.blessings4,
	                  p.blessings5, p.blessings6, p.blessings7, p.blessings8
	           FROM players p JOIN accounts a ON a.id = p.account_id WHERE p.name = ? LIMIT 1`

	p := &game.Player{}
	var townID int
	var capValue uint32
	var lookType, lookHead, lookBody, lookLegs, lookFeet, lookAddons uint16
	var posx, posy uint16
	var posz uint8
	var offlineTimeSeconds int32
	var taskPoints uint32
	var quickLootFallback bool
	err := d.SQL.QueryRowContext(ctx, q, name).Scan(
		&p.DBID, &p.AccountID, &p.AccountType, &p.GroupID, &p.Name, &p.Level, &p.Vocation, &p.Sex,
		&p.Health, &p.MaxHealth, &p.Mana, &p.MaxMana, &p.Experience,
		&p.MagLevel, &p.ManaSpent, &p.Soul, &capValue, &p.BankBalance,
		&lookType, &lookHead, &lookBody, &lookLegs, &lookFeet, &lookAddons,
		&posx, &posy, &posz, &townID,
		&p.Skills[game.SkillFist], &p.SkillTries[game.SkillFist],
		&p.Skills[game.SkillClub], &p.SkillTries[game.SkillClub],
		&p.Skills[game.SkillSword], &p.SkillTries[game.SkillSword],
		&p.Skills[game.SkillAxe], &p.SkillTries[game.SkillAxe],
		&p.Skills[game.SkillDistance], &p.SkillTries[game.SkillDistance],
		&p.Skills[game.SkillShielding], &p.SkillTries[game.SkillShielding],
		&p.Skills[game.SkillFishing], &p.SkillTries[game.SkillFishing],
		&offlineTimeSeconds, &p.OfflineTrainingSkill,
		&p.ForgeDusts, &p.ForgeDustLevel,
		&taskPoints, &quickLootFallback, &p.PreyCards,
		&p.LastLogin, &p.LastLogout,
		&p.Blessings[0], &p.Blessings[1], &p.Blessings[2], &p.Blessings[3],
		&p.Blessings[4], &p.Blessings[5], &p.Blessings[6], &p.Blessings[7],
	)
	p.OfflineTrainingTime = offlineTimeSeconds * 1000
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	p.Capacity = capValue * 100
	p.GetTaskHunter().Points = taskPoints
	p.QuickLootFallbackToMain = quickLootFallback
	p.Outfit = game.Outfit{
		LookType: lookType,
		Head:     uint8(lookHead),
		Body:     uint8(lookBody),
		Legs:     uint8(lookLegs),
		Feet:     uint8(lookFeet),
		Addons:   uint8(lookAddons),
	}
	p.Pos = game.Position{X: posx, Y: posy, Z: posz}
	p.TownID = uint16(townID)

	// Resolve the town temple: it seeds the position when unset and is always
	// stored as LoginPosition so death can respawn the player there.
	if temple, err := d.TownTemple(ctx, townID); err == nil {
		p.LoginPosition = temple
		if p.Pos.X == 0 && p.Pos.Y == 0 {
			p.Pos = temple
		}
	}
	p.SkillLoss = true

	if err := d.LoadPlayerItems(ctx, p); err != nil {
		return nil, err
	}
	// Rebuild quick-loot managed containers from the per-container bitmask
	// attributes now that the inventory is loaded.
	p.RebuildManagedContainers()

	// Load player storages
	p.Storages = make(map[uint32]int32)
	sRows, err := d.SQL.QueryContext(ctx, "SELECT `key`, `value` FROM player_storage WHERE player_id = ?", p.DBID)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var k uint32
			var v int32
			if err := sRows.Scan(&k, &v); err == nil {
				p.Storages[k] = v
			}
		}
	}

	// Load player mounts
	p.Mounts = make(map[uint16]bool)
	mRows, err := d.SQL.QueryContext(ctx, "SELECT mount_id FROM player_mounts WHERE player_id = ?", p.DBID)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var mid uint16
			if err := mRows.Scan(&mid); err == nil {
				p.AddMount(mid)
			}
		}
	}
	// Load player guild membership
	gQuery := `SELECT g.name, r.name, m.nick
	           FROM guild_membership m
	           JOIN guilds g ON m.guild_id = g.id
	           JOIN guild_ranks r ON m.rank_id = r.id
	           WHERE m.player_id = ? LIMIT 1`
	_ = d.SQL.QueryRowContext(ctx, gQuery, p.DBID).Scan(&p.GuildName, &p.GuildRankName, &p.GuildNick)

	// Load Wheel of Destiny slot allocations
	_ = d.LoadPlayerWheel(ctx, p)
	_ = d.LoadPlayerPrey(ctx, p)
	_ = d.LoadPlayerTaskHunter(ctx, p)

	return p, nil
}

// LoadPlayerWheel loads Wheel of Destiny points from player_wheeldata table.
func (d *DB) LoadPlayerWheel(ctx context.Context, p *game.Player) error {
	const q = `SELECT slot FROM player_wheeldata WHERE player_id = ?`
	var slotBlob []byte
	err := d.SQL.QueryRowContext(ctx, q, p.DBID).Scan(&slotBlob)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if len(slotBlob) == 0 {
		return nil
	}

	ps := propstream.NewPropStream(slotBlob)
	pointsMap := make(map[uint16]uint16)
	for ps.Size() >= 3 {
		slotID, err1 := ps.ReadUint8()
		pts, err2 := ps.ReadUint16()
		if err1 != nil || err2 != nil {
			break
		}
		if pts > 0 {
			pointsMap[uint16(slotID)] = pts
		}
	}
	wheel := p.GetWheel()
	wheel.SetVocation(game.CIPVocation(p.Vocation))
	wheel.SaveSlotPoints(pointsMap)
	return nil
}

// SavePlayerWheel persists Wheel of Destiny points to player_wheeldata table.
func (d *DB) SavePlayerWheel(ctx context.Context, p *game.Player) error {
	if p.Wheel == nil {
		return nil
	}
	slotPoints := p.Wheel.GetSlotPointsCopy()
	ws := propstream.NewPropWriteStream()
	for slotID := uint8(1); slotID <= 36; slotID++ {
		pts := slotPoints[uint16(slotID)]
		ws.WriteUint8(slotID)
		ws.WriteUint16(pts)
	}
	blob := ws.GetStream()
	const q = `INSERT INTO player_wheeldata (player_id, slot) VALUES (?, ?)
	           ON DUPLICATE KEY UPDATE slot = VALUES(slot)`
	_, err := d.SQL.ExecContext(ctx, q, p.DBID, blob)
	return err
}

// SavePlayer persists mutable player state (position, vitals, experience, skills, storages, wheel).
func (d *DB) SavePlayer(ctx context.Context, p *game.Player) error {
	const q = `UPDATE players SET
	              level=?, experience=?, health=?, healthmax=?,
	              mana=?, manamax=?, soul=?, cap=?, balance=?,
	              posx=?, posy=?, posz=?,
	              looktype=?, lookhead=?, lookbody=?, looklegs=?,
	              lookfeet=?, lookaddons=?,
	              maglevel=?, manaspent=?,
	              skill_fist=?, skill_fist_tries=?,
	              skill_club=?, skill_club_tries=?,
	              skill_sword=?, skill_sword_tries=?,
	              skill_axe=?, skill_axe_tries=?,
	              skill_dist=?, skill_dist_tries=?,
	              skill_shielding=?, skill_shielding_tries=?,
	              skill_fishing=?, skill_fishing_tries=?,
	              offlinetraining_time=?, offlinetraining_skill=?,
	              lastlogin=?, lastlogout=?,
	              forge_dusts=?, forge_dust_level=?,
	              task_points=?, quickloot_fallback=?, prey_wildcard=?,
	              blessings1=?, blessings2=?, blessings3=?, blessings4=?,
	              blessings5=?, blessings6=?, blessings7=?, blessings8=?
	           WHERE id=?`
	_, err := d.SQL.ExecContext(ctx, q,
		p.Level, p.Experience, p.Health, p.MaxHealth,
		p.Mana, p.MaxMana, p.Soul, p.Capacity/100, p.BankBalance,
		p.Pos.X, p.Pos.Y, p.Pos.Z,
		p.Outfit.LookType, p.Outfit.Head, p.Outfit.Body, p.Outfit.Legs,
		p.Outfit.Feet, p.Outfit.Addons,
		p.MagLevel, p.ManaSpent,
		p.Skills[game.SkillFist], p.SkillTries[game.SkillFist],
		p.Skills[game.SkillClub], p.SkillTries[game.SkillClub],
		p.Skills[game.SkillSword], p.SkillTries[game.SkillSword],
		p.Skills[game.SkillAxe], p.SkillTries[game.SkillAxe],
		p.Skills[game.SkillDistance], p.SkillTries[game.SkillDistance],
		p.Skills[game.SkillShielding], p.SkillTries[game.SkillShielding],
		p.Skills[game.SkillFishing], p.SkillTries[game.SkillFishing],
		p.OfflineTrainingTime/1000, p.OfflineTrainingSkill,
		p.LastLogin, p.LastLogout,
		p.ForgeDusts, p.GetForgeDustLevel(),
		p.GetTaskHunter().Points, p.QuickLootFallbackToMain, p.PreyCards,
		p.Blessings[0], p.Blessings[1], p.Blessings[2], p.Blessings[3],
		p.Blessings[4], p.Blessings[5], p.Blessings[6], p.Blessings[7],
		p.DBID,
	)
	if err != nil {
		return err
	}

	// Save player storages
	if p.Storages != nil {
		// First delete existing storages for the player
		_, _ = d.SQL.ExecContext(ctx, "DELETE FROM player_storage WHERE player_id = ?", p.DBID)

		// Then insert current storages
		for k, v := range p.Storages {
			_, _ = d.SQL.ExecContext(ctx, "INSERT INTO player_storage (player_id, `key`, `value`) VALUES (?, ?, ?)", p.DBID, k, v)
		}
	}

	// Save player mounts
	if p.Mounts != nil {
		_, _ = d.SQL.ExecContext(ctx, "DELETE FROM player_mounts WHERE player_id = ?", p.DBID)
		for mid := range p.Mounts {
			_, _ = d.SQL.ExecContext(ctx, "INSERT INTO player_mounts (player_id, mount_id) VALUES (?, ?)", p.DBID, mid)
		}
	}

	// Save Wheel of Destiny, Prey and Task Hunting slots.
	_ = d.SavePlayerWheel(ctx, p)
	_ = d.SavePlayerPrey(ctx, p)
	_ = d.SavePlayerTaskHunter(ctx, p)

	return d.SavePlayerItems(ctx, p)
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
