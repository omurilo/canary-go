package db

import (
	"context"
	"encoding/json"
	"database/sql"
	"errors"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/io/propstream"
)

// LoadPlayer loads a character by name into a game.Player. The town temple is
// used when the stored position is (0,0,0).
func (d *DB) LoadPlayer(ctx context.Context, name string) (*game.Player, error) {
	const q = `SELECT p.id, p.account_id, a.type as account_type, a.coins, a.coins_transferable, a.tournament_coins, p.group_id, p.name, p.level, p.vocation, p.sex,
	                  p.health, p.healthmax, p.mana, p.manamax, p.experience,
	                  p.maglevel, p.manaspent, p.soul, p.cap, p.balance,
	                  p.skull, p.skulltime, p.conditions,
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
	                  p.boss_points,
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
		&p.DBID, &p.AccountID, &p.AccountType, &p.CoinBalance, &p.CoinTransferable, &p.TournamentBalance, &p.GroupID, &p.Name, &p.Level, &p.Vocation, &p.Sex,
		&p.Health, &p.MaxHealth, &p.Mana, &p.MaxMana, &p.Experience,
		&p.MagLevel, &p.ManaSpent, &p.Soul, &capValue, &p.BankBalance,
		&p.Skull, &p.SkullTime, &p.ConditionsBlob,
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
		&p.BossPoints,
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
	if minExp := game.ExpForLevel(uint64(p.Level)); p.Experience < minExp {
		p.Experience = minExp
	}
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
	if err := d.LoadPlayerDepot(ctx, p); err != nil {
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

	// Load player outfits
	p.Outfits = []game.OutfitEntry{}
	oRows, err := d.SQL.QueryContext(ctx, "SELECT looktype, addons FROM player_outfits WHERE player_id = ?", p.DBID)
	if err == nil {
		defer oRows.Close()
		for oRows.Next() {
			var lookType uint16
			var addons uint8
			if err := oRows.Scan(&lookType, &addons); err == nil {
				p.Outfits = append(p.Outfits, game.OutfitEntry{LookType: lookType, Addons: addons})
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
	_ = d.LoadPlayerBosstiary(ctx, p)
	_ = d.LoadPlayerCharms(ctx, p)
	_ = d.LoadPlayerSpells(ctx, p)
	_ = d.LoadPlayerVIP(ctx, p)
	_ = d.LoadPlayerDepot(ctx, p)
	_ = d.LoadPlayerAchievements(ctx, p)
	_ = d.LoadPlayerTitles(ctx, p)
	_ = d.LoadPlayerFamiliars(ctx, p)
	_ = d.LoadPlayerHazard(ctx, p)
	_ = d.LoadPlayerConcoctions(ctx, p)
	_ = d.LoadPlayerAnimusMastery(ctx, p)

	return p, nil
}

// LoadPlayerSpells loads the instant spells a player has learned from the DB.
func (d *DB) LoadPlayerSpells(ctx context.Context, p *game.Player) error {
	const q = `SELECT name FROM player_spells WHERE player_id = ?`
	rows, err := d.SQL.QueryContext(ctx, q, p.DBID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var spellName string
		if err := rows.Scan(&spellName); err == nil {
			p.LearnSpell(spellName)
		}
	}
	return nil
}

// SavePlayerSpells persists the instant spells a player has learned to the DB.
func (d *DB) SavePlayerSpells(ctx context.Context, p *game.Player) error {
	spells := p.GetLearnedSpells()
	if len(spells) == 0 {
		return nil
	}

	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM player_spells WHERE player_id = ?", p.DBID); err != nil {
		return err
	}

	for spellName := range spells {
		if _, err := tx.ExecContext(ctx, "INSERT INTO player_spells (player_id, name) VALUES (?, ?)", p.DBID, spellName); err != nil {
			return err
		}
	}
	return tx.Commit()
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

	// Append JSON gem data after slot points
	if gemJSON, err := json.Marshal(game.WheelGemPersistData{
		ActiveGems:   p.Wheel.ActiveGems,
		RevealedGems: p.Wheel.RevealedGems,
	}); err == nil && len(gemJSON) > 2 {
		blob = append(blob, gemJSON...)
	}

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
	              boss_points=?,
	              skull=?, skulltime=?, conditions=?,
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
		p.BossPoints,
		p.Skull, p.SkullTime, p.ConditionsBlob,
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

	// Save player outfits
	if p.Outfits != nil {
		_, _ = d.SQL.ExecContext(ctx, "DELETE FROM player_outfits WHERE player_id = ?", p.DBID)
		for _, outfit := range p.Outfits {
			_, _ = d.SQL.ExecContext(ctx, "INSERT INTO player_outfits (player_id, looktype, addons) VALUES (?, ?, ?)", p.DBID, outfit.LookType, outfit.Addons)
		}
	}

	// Save Wheel of Destiny, Prey and Task Hunting slots.
	_ = d.SavePlayerWheel(ctx, p)
	_ = d.SavePlayerPrey(ctx, p)
	_ = d.SavePlayerBosstiary(ctx, p)
	_ = d.SavePlayerCharms(ctx, p)
	_ = d.SavePlayerTaskHunter(ctx, p)
	_ = d.SaveAccountCoins(ctx, p)
	_ = d.SavePlayerSpells(ctx, p)
	_ = d.SavePlayerVIP(ctx, p)
	_ = d.SavePlayerAchievements(ctx, p)
	_ = d.SavePlayerTitles(ctx, p)
	_ = d.SavePlayerFamiliars(ctx, p)
	_ = d.SavePlayerHazard(ctx, p)
	_ = d.SavePlayerConcoctions(ctx, p)
	_ = d.SavePlayerAnimusMastery(ctx, p)

	if err := d.SavePlayerDepot(ctx, p); err != nil {
		return err
	}
	return d.SavePlayerItems(ctx, p)
}

// SaveAccountCoins persists the account's Tibia Coin balances and tournament coins.
func (d *DB) SaveAccountCoins(ctx context.Context, p *game.Player) error {
	if p.AccountID == 0 {
		return nil
	}
	const q = `UPDATE accounts SET coins=?, coins_transferable=?, tournament_coins=? WHERE id=?`
	_, err := d.SQL.ExecContext(ctx, q, p.CoinBalance, p.CoinTransferable, p.TournamentBalance, p.AccountID)
	return err
}

// LoadPlayerVIP loads the account's VIP list, groups, and assignments.
func (d *DB) LoadPlayerVIP(ctx context.Context, p *game.Player) error {
	if p.AccountID == 0 {
		return nil
	}

	// 1. Load VIP Groups
	qGroups := `SELECT id, name, customizable FROM account_vipgroups WHERE account_id = ?`
	groupRows, err := d.SQL.QueryContext(ctx, qGroups, p.AccountID)
	if err == nil {
		defer groupRows.Close()
		for groupRows.Next() {
			var g game.VIPGroup
			if err := groupRows.Scan(&g.ID, &g.Name, &g.Customizable); err == nil {
				p.VIPGroups = append(p.VIPGroups, g)
			}
		}
	}

	// 2. Load VIP List entries
	qList := `SELECT vl.player_id, p.name, vl.description, vl.icon, vl.notify 
	          FROM account_viplist vl
	          JOIN players p ON vl.player_id = p.id
	          WHERE vl.account_id = ?`
	listRows, err := d.SQL.QueryContext(ctx, qList, p.AccountID)
	if err != nil {
		return err
	}
	defer listRows.Close()

	entriesMap := make(map[uint32]*game.VIPEntry)
	for listRows.Next() {
		var e game.VIPEntry
		var notifyInt uint8
		if err := listRows.Scan(&e.PlayerID, &e.PlayerName, &e.Description, &e.Icon, &notifyInt); err == nil {
			e.Notify = (notifyInt > 0)
			p.VIPList = append(p.VIPList, e)
			entriesMap[e.PlayerID] = &p.VIPList[len(p.VIPList)-1]
		}
	}

	// 3. Load Group Assignments
	qGroupList := `SELECT player_id, vipgroup_id FROM account_vipgrouplist WHERE account_id = ?`
	glistRows, err := d.SQL.QueryContext(ctx, qGroupList, p.AccountID)
	if err == nil {
		defer glistRows.Close()
		for glistRows.Next() {
			var pid, gid uint32
			if err := glistRows.Scan(&pid, &gid); err == nil {
				if entry, ok := entriesMap[pid]; ok {
					entry.Groups = append(entry.Groups, gid)
				}
			}
		}
	}

	return nil
}

// SavePlayerVIP persists the account's VIP list, groups, and assignments.
func (d *DB) SavePlayerVIP(ctx context.Context, p *game.Player) error {
	if p.AccountID == 0 {
		return nil
	}

	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Wipe existing data for this account (account_vipgrouplist is wiped via cascade or we can do it explicitly)
	if _, err := tx.ExecContext(ctx, "DELETE FROM account_vipgrouplist WHERE account_id = ?", p.AccountID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM account_viplist WHERE account_id = ?", p.AccountID); err != nil {
		return err
	}
	// Note: We don't delete groups because they have IDs that might be referenced,
	// but in a full sync we could UPSERT. To match C++ behavior safely without UPSERT,
	// we will UPSERT groups.
	for _, g := range p.VIPGroups {
		q := `INSERT INTO account_vipgroups (id, account_id, name, customizable) VALUES (?, ?, ?, ?)
		      ON DUPLICATE KEY UPDATE name = VALUES(name), customizable = VALUES(customizable)`
		if _, err := tx.ExecContext(ctx, q, g.ID, p.AccountID, g.Name, g.Customizable); err != nil {
			return err
		}
	}

	// Insert VIP entries
	for _, e := range p.VIPList {
		notifyInt := 0
		if e.Notify {
			notifyInt = 1
		}
		qList := `INSERT INTO account_viplist (account_id, player_id, description, icon, notify) VALUES (?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, qList, p.AccountID, e.PlayerID, e.Description, e.Icon, notifyInt); err != nil {
			return err
		}

		// Insert group assignments
		for _, gid := range e.Groups {
			qGroupList := `INSERT INTO account_vipgrouplist (account_id, player_id, vipgroup_id) VALUES (?, ?, ?)`
			if _, err := tx.ExecContext(ctx, qGroupList, p.AccountID, e.PlayerID, gid); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
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
