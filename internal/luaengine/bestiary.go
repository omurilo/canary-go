package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/bestiary"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// resolveMonsterByRaceID finds a monster type by its bestiary race id.
func (e *Engine) resolveMonsterByRaceID(raceID uint16) *creatures.MonsterType {
	if e.world == nil || e.world.TypeRegistry == nil || raceID == 0 {
		return nil
	}
	for _, m := range e.world.TypeRegistry.Monsters {
		if m.RaceID == raceID {
			return m
		}
	}
	return nil
}

func bestiaryThresholds(mt *creatures.MonsterType) bestiary.Thresholds {
	return bestiary.Thresholds{
		FirstUnlock:  mt.BestiaryFirstUnlock,
		SecondUnlock: mt.BestiarySecondUnlock,
		ToKill:       mt.BestiaryToKill,
	}
}

// playerAddbestiarykill implements Player:addBestiaryKill(name[, amount=1]) —
// credits kills of a (non-boss) bestiary monster, awarding charm points on
// completion and refreshing the entry on a stage change.
func (e *Engine) playerAddbestiarykill(L *lua.LState) int {
	p := checkPlayer(L)
	mt := e.resolveMonsterType(L.CheckString(2))
	if p == nil || mt == nil || mt.RaceID == 0 || mt.BestiaryToKill == 0 {
		L.Push(lua.LFalse)
		return 1
	}
	amount := uint32(1)
	if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTNumber {
		amount = uint32(L.CheckInt(3))
	}
	if p.AddBestiaryKill(mt.RaceID, bestiaryThresholds(mt), mt.BestiaryCharmsPoints, amount) {
		e.sendBestiaryEntryChanged(p, mt.RaceID)
	}
	L.Push(lua.LTrue)
	return 1
}

// playerIsmonsterbestiaryunlocked implements Player:isMonsterBestiaryUnlocked(raceId)
// — true when the player has fully unlocked (completed) that monster. Mirrors
// Player::isMonsterBestiaryUnlocked.
func (e *Engine) playerIsmonsterbestiaryunlocked(L *lua.LState) int {
	p := checkPlayer(L)
	mt := e.resolveMonsterByRaceID(uint16(L.CheckInt(2)))
	if p == nil || mt == nil {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LBool(p.IsBestiaryComplete(mt.RaceID, bestiaryThresholds(mt))))
	return 1
}

// playerAddcharmpoints implements Player:addCharmPoints(amount).
func (e *Engine) playerAddcharmpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	p.AddCharmPoints(uint32(L.CheckInt(2)))
	L.Push(lua.LTrue)
	return 1
}

// playerGetcharmpoints implements Player:getCharmPoints().
func (e *Engine) playerGetcharmpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.GetCharmPoints()))
	return 1
}

// sendBestiaryEntryChanged notifies the client a bestiary entry changed (via the
// session, narrow-interface assertion to avoid a protocol import cycle).
func (e *Engine) sendBestiaryEntryChanged(p *game.Player, raceID uint16) {
	if p == nil || p.Session == nil {
		return
	}
	if s, ok := p.Session.(interface{ SendBestiaryEntryChanged(uint16) }); ok {
		s.SendBestiaryEntryChanged(raceID)
	}
}

// registerBestiaryPlayerMethods overrides the stub bestiary/charm player
// bindings with the real engine-backed ones.
func (e *Engine) registerBestiaryPlayerMethods() {
	mt := e.L.GetTypeMetatable("Player")
	if mt.Type() != lua.LTTable {
		return
	}
	tbl := mt.(*lua.LTable)
	e.L.SetField(tbl, "addBestiaryKill", e.L.NewFunction(e.playerAddbestiarykill))
	e.L.SetField(tbl, "isMonsterBestiaryUnlocked", e.L.NewFunction(e.playerIsmonsterbestiaryunlocked))
	e.L.SetField(tbl, "addCharmPoints", e.L.NewFunction(e.playerAddcharmpoints))
	e.L.SetField(tbl, "getCharmPoints", e.L.NewFunction(e.playerGetcharmpoints))
}
