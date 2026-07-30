package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

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

	// WeaponProficiency is derived state that C++ recomputes from the persisted
	// perks (applyPerks); it is never read back from the database. The persisted
	// half lives in the KV store and is loaded by LoadPlayerWeaponProficiency.
	if p.WeaponProficiency == nil {
		p.WeaponProficiency = game.NewWeaponProficiency()
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

	// Restore open containers from persisted attributes.
	d.restoreOpenContainers(p)

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

	// Mounts and outfits are loaded from the KV store by the subsystem loop below.
	// Load player guild membership
	gQuery := `SELECT g.name, r.name, m.nick
	           FROM guild_membership m
	           JOIN guilds g ON m.guild_id = g.id
	           JOIN guild_ranks r ON m.rank_id = r.id
	           WHERE m.player_id = ? LIMIT 1`
	_ = d.SQL.QueryRowContext(ctx, gQuery, p.DBID).Scan(&p.GuildName, &p.GuildRankName, &p.GuildNick)

	// Per-subsystem loads. These used to be `_ = ...`, which hid real failures.
	// Everything not backed by a canonical column now reads from the KV store.
	for _, sub := range []struct {
		name string
		load func(context.Context, *game.Player) error
	}{
		{"wheel", d.LoadPlayerWheel},
		{"prey", d.LoadPlayerPrey},
		{"task_hunter", d.LoadPlayerTaskHunter},
		{"bosstiary", d.LoadPlayerBosstiary},
		{"charms", d.LoadPlayerCharms},
		{"spells", d.LoadPlayerSpells},
		{"vip", d.LoadPlayerVIP},
		{"achievements", d.LoadPlayerAchievements},
		{"titles", d.LoadPlayerTitles},
		{"familiars", d.LoadPlayerFamiliars},
		{"hazard", d.LoadPlayerHazard},
		{"concoctions", d.LoadPlayerConcoctions},
		{"hirelings", d.LoadPlayerHirelings},
		{"animus_mastery", d.LoadPlayerAnimusMastery},
		{"weapon_proficiency", d.LoadPlayerWeaponProficiency},
		{"mounts", d.LoadPlayerMounts},
		{"outfits", d.LoadPlayerOutfits},
	} {
		if err := sub.load(ctx, p); err != nil {
			slog.Default().Warn("load player subsystem failed",
				"subsystem", sub.name, "player", p.Name, "err", err)
		}
	}

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

	// Propstream is always 108 bytes (36 slots × 3 bytes each: [U8 slotID][U16 pts]).
	// Any data beyond 108 bytes is the JSON payload.
	const propstreamSize = 108
	propEnd := propstreamSize
	if propstreamSize > len(slotBlob) {
		propEnd = len(slotBlob)
	}

	ps := propstream.NewPropStream(slotBlob[:propEnd])
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

	// Parse JSON payload (gems, revelation stages, scrolls)
	if propEnd < len(slotBlob) {
		var extra struct {
			Gems              *game.WheelGemPersistData `json:"gems"`
			RevelationStages  map[string]uint8          `json:"revelationStages"`
			RevelationPoints  map[string]uint16         `json:"revelationPoints"`
			UsedScrolls       map[string]bool           `json:"usedScrolls"`
		}
		if err := json.Unmarshal(slotBlob[propEnd:], &extra); err == nil {
			if extra.Gems != nil {
				if p.WheelGemManager == nil {
					p.WheelGemManager = &game.WheelGemCollection{}
				}
				p.WheelGemManager.ActiveGems = extra.Gems.ActiveGems
				p.WheelGemManager.RevealedGems = extra.Gems.RevealedGems
			}
			if len(extra.RevelationStages) > 0 {
				wheel.RevelationStages = extra.RevelationStages
			}
			if len(extra.RevelationPoints) > 0 {
				wheel.RevelationPoints = extra.RevelationPoints
			}
			if len(extra.UsedScrolls) > 0 {
				wheel.UsedScrolls = extra.UsedScrolls
			}
		}
	}
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

	// Append JSON data after slot points: gems + revelation stages + scrolls.
	extra := map[string]interface{}{}
	if p.WheelGemManager != nil {
		extra["gems"] = game.WheelGemPersistData{
			ActiveGems:   p.WheelGemManager.ActiveGems,
			RevealedGems: p.WheelGemManager.RevealedGems,
		}
	}
	if len(p.Wheel.RevelationStages) > 0 {
		extra["revelationStages"] = p.Wheel.RevelationStages
	}
	if len(p.Wheel.UsedScrolls) > 0 {
		extra["usedScrolls"] = p.Wheel.UsedScrolls
	}
	if len(p.Wheel.RevelationPoints) > 0 {
		extra["revelationPoints"] = p.Wheel.RevelationPoints
	}
	if len(extra) > 0 {
		if js, err := json.Marshal(extra); err == nil {
			blob = append(blob, js...)
		}
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
	              town_id=?, posx=?, posy=?, posz=?,
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
		p.TownID, p.Pos.X, p.Pos.Y, p.Pos.Z,
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


	// Save Wheel of Destiny, Prey and Task Hunting slots.
	// Per-subsystem saves. These used to be `_ = ...`; a missing table or a broken
	// blob encoder therefore lost player state with no trace in the log.
	for _, sub := range []struct {
		name string
		save func(context.Context, *game.Player) error
	}{
		{"wheel", d.SavePlayerWheel},
		{"prey", d.SavePlayerPrey},
		{"bosstiary", d.SavePlayerBosstiary},
		{"charms", d.SavePlayerCharms},
		{"task_hunter", d.SavePlayerTaskHunter},
		{"account_coins", d.SaveAccountCoins},
		{"spells", d.SavePlayerSpells},
		{"vip", d.SavePlayerVIP},
		{"achievements", d.SavePlayerAchievements},
		{"titles", d.SavePlayerTitles},
		{"familiars", d.SavePlayerFamiliars},
		{"hazard", d.SavePlayerHazard},
		{"concoctions", d.SavePlayerConcoctions},
		{"hirelings", d.SavePlayerHirelings},
		{"animus_mastery", d.SavePlayerAnimusMastery},
		{"weapon_proficiency", d.SavePlayerWeaponProficiency},
		{"mounts", d.SavePlayerMounts},
		{"outfits", d.SavePlayerOutfits},
	} {
		if err := sub.save(ctx, p); err != nil {
			slog.Default().Warn("save player subsystem failed",
				"subsystem", sub.name, "player", p.Name, "err", err)
		}
	}

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

// highscoreOrderBy maps categoryID to the players table column to ORDER BY.
var highscoreOrderBy = map[uint8]string{
	0: "experience",
	1: "skill_fist",
	2: "skill_club",
	3: "skill_sword",
	4: "skill_axe",
	5: "skill_dist",
	6: "skill_shielding",
	7: "skill_fishing",
	8: "maglevel",
	9: "skill_fist", // loyalty — no dedicated column, use skill_fist
}

// LoadHighscore fetches a page of highscore entries from the players table.
// It returns the entries ordered by the requested category, the total number of
// pages available for the given perPage, or an error.
func (d *DB) LoadHighscore(ctx context.Context, categoryID uint8, vocationID uint32, page uint16, perPage uint8) ([]game.HighscoreEntry, int, error) {
	orderBy, ok := highscoreOrderBy[categoryID]
	if !ok {
		orderBy = "experience"
	}

	// Count total matching players for pagination.
	var totalPlayers int
	countQuery := "SELECT COUNT(*) FROM players WHERE group_id < 3" // exclude groups >= 3 (GMs, etc.)
	countArgs := []any{}
	if vocationID > 0 && vocationID <= 4 {
		countQuery += " AND vocation = ?"
		countArgs = append(countArgs, vocationID)
	}
	if err := d.SQL.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalPlayers); err != nil {
		return nil, 0, fmt.Errorf("db: count highscore: %w", err)
	}
	if totalPlayers < 1 {
		totalPlayers = 1
	}
	totalPages := (totalPlayers + int(perPage) - 1) / int(perPage)

	// Fetch the requested page.
	offset := int(page) * int(perPage)
	baseQuery := fmt.Sprintf(
		"SELECT name, level, vocation, %s, town_id FROM players WHERE group_id < 3",
		orderBy,
	)
	queryArgs := []any{}
	if vocationID > 0 && vocationID <= 4 {
		baseQuery += " AND vocation = ?"
		queryArgs = append(queryArgs, vocationID)
	}
	baseQuery += fmt.Sprintf(" ORDER BY %s DESC LIMIT ? OFFSET ?", orderBy)
	queryArgs = append(queryArgs, perPage, offset)

	rows, err := d.SQL.QueryContext(ctx, baseQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("db: query highscore: %w", err)
	}
	defer rows.Close()

	var entries []game.HighscoreEntry
	rank := uint16(offset + 1)
	for rows.Next() {
		var e game.HighscoreEntry
		if err := rows.Scan(&e.Name, &e.Level, &e.Vocation, &e.Value, &e.TownID); err != nil {
			return nil, 0, fmt.Errorf("db: scan highscore row: %w", err)
		}
		e.Rank = rank
		entries = append(entries, e)
		rank++
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("db: highscore rows iteration: %w", err)
	}

	return entries, totalPages, nil
}
