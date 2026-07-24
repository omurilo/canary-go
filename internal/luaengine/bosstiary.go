package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/bosstiary"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// resolveMonsterType looks up a monster type by (case-insensitive) name.
func (e *Engine) resolveMonsterType(name string) *creatures.MonsterType {
	if e.world == nil || e.world.TypeRegistry == nil {
		return nil
	}
	return e.world.TypeRegistry.Monsters[strings.ToLower(name)]
}

// playerGetbosstiarylevel implements Player:getBosstiaryLevel(name) — the boss's
// current unlock level (0..3) for this player. Mirrors luaPlayerGetBosstiaryLevel
// (uses the boss's race id, which for bosses is bossRaceId).
func (e *Engine) playerGetbosstiarylevel(L *lua.LState) int {
	p := checkPlayer(L)
	mt := e.resolveMonsterType(L.CheckString(2))
	if p == nil || mt == nil || !mt.IsBoss() {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(bosstiary.Level(mt.BosstiaryRace, p.GetBestiaryKillCount(mt.BosstiaryRaceID))))
	return 1
}

// playerGetbosstiarykills implements Player:getBosstiaryKills(name).
func (e *Engine) playerGetbosstiarykills(L *lua.LState) int {
	p := checkPlayer(L)
	mt := e.resolveMonsterType(L.CheckString(2))
	if p == nil || mt == nil || !mt.IsBoss() {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.GetBestiaryKillCount(mt.BosstiaryRaceID)))
	return 1
}

// playerAddbosstiarykill implements Player:addBosstiaryKill(name[, amount=1]).
// Increments the boss kill count and, when the kill crosses into a new unlock
// level, awards that level's boss points and sends the cyclopedia entry-changed
// update. Mirrors IOBosstiary::addBosstiaryKill.
func (e *Engine) playerAddbosstiarykill(L *lua.LState) int {
	p := checkPlayer(L)
	mt := e.resolveMonsterType(L.CheckString(2))
	if p == nil || mt == nil || !mt.IsBoss() {
		L.Push(lua.LFalse)
		return 1
	}
	amount := uint32(1)
	if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTNumber {
		amount = uint32(L.CheckInt(3))
	}

	raceID := mt.BosstiaryRaceID
	race := mt.BosstiaryRace
	oldLevel := bosstiary.Level(race, p.GetBestiaryKillCount(raceID))
	p.AddBestiaryKillCount(raceID, amount)
	newLevel := bosstiary.Level(race, p.GetBestiaryKillCount(raceID))

	if newLevel > oldLevel {
		// Match C++: award the (new) level's stage points.
		p.AddBossPoints(uint32(bosstiary.PointsForLevel(race, newLevel)))
		e.sendBosstiaryEntryChanged(p, raceID)
	}
	L.Push(lua.LTrue)
	return 1
}

// playerSetbosspoints implements Player:setBossPoints(amount).
func (e *Engine) playerSetbosspoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	p.SetBossPoints(uint32(L.CheckInt(2)))
	L.Push(lua.LTrue)
	return 1
}

// playerGetbosspoints implements Player:getBossPoints().
func (e *Engine) playerGetbosspoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.GetBossPoints()))
	return 1
}

// sendBosstiaryEntryChanged notifies the client that a boss's cyclopedia entry
// changed (new level/points). The 0x61 kill-tracker/entry-changed packet is
// wired in phase 5; for now this is the single hook the kill path calls.
func (e *Engine) sendBosstiaryEntryChanged(p *game.Player, bossRaceID uint16) {
	// TODO(phase 5): send the S_BosstiaryEntryChanged (0x61) packet.
	_ = p
	_ = bossRaceID
}

// registerBosstiaryPlayerMethods overrides the stub bosstiary player bindings
// with the real engine-backed implementations (same pattern as getStoreInbox).
func (e *Engine) registerBosstiaryPlayerMethods() {
	mt := e.L.GetTypeMetatable("Player")
	if mt.Type() != lua.LTTable {
		return
	}
	tbl := mt.(*lua.LTable)
	e.L.SetField(tbl, "getBosstiaryLevel", e.L.NewFunction(e.playerGetbosstiarylevel))
	e.L.SetField(tbl, "getBosstiaryKills", e.L.NewFunction(e.playerGetbosstiarykills))
	e.L.SetField(tbl, "addBosstiaryKill", e.L.NewFunction(e.playerAddbosstiarykill))
	e.L.SetField(tbl, "setBossPoints", e.L.NewFunction(e.playerSetbosspoints))
	e.L.SetField(tbl, "getBossPoints", e.L.NewFunction(e.playerGetbosspoints))
}
