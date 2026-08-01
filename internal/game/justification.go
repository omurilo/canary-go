package game

import (
	"fmt"
	"time"

	"github.com/omurilo/canary-go/internal/config"
	"github.com/omurilo/canary-go/internal/game/combat"
)

// PvP justification, the port of Player::onKilledPlayer and
// Player::addUnjustifiedDead (src/creatures/players/player.cpp). This is what
// decides whether a kill counts against the killer, and it is the last piece of the
// skull system: the damage map already names the killers and player_kills already
// persists the frags, but every kill was recorded as justified because nothing
// evaluated the rules.

// Skulls_t (src/utils/utils_definitions.hpp:499).
const (
	SkullNone   uint8 = 0
	SkullYellow uint8 = 1
	SkullGreen  uint8 = 2
	SkullWhite  uint8 = 3
	SkullRed    uint8 = 4
	SkullBlack  uint8 = 5
	SkullOrange uint8 = 6
)

// Config defaults from config.lua.dist:43-52.
const (
	defaultDayKillsToRed    = 3
	defaultWeekKillsToRed   = 5
	defaultMonthKillsToRed  = 10
	defaultRedSkullDays     = 1
	defaultBlackSkullDays   = 3
	defaultOrangeSkullDays  = 7
	defaultWhiteSkullMillis = 15 * 60 * 1000
)

// The windows addUnjustifiedDead counts kills over. Note the day bucket is FOUR
// hours, not twenty-four — a detail easy to get wrong from the name alone.
const (
	dayKillWindow   = 4 * 60 * 60
	weekKillWindow  = 7 * 24 * 60 * 60
	monthKillWindow = 30 * 24 * 60 * 60
)

// AddAttacked records that this player attacked another, mirroring
// Player::addAttacked. The set is what separates an aggressor from someone
// defending themselves: only a player who attacked first can be unjustified.
func (p *Player) AddAttacked(target *Player) {
	if target == nil || target == p || p.CannotGainInFight() {
		return
	}
	p.attackedMu.Lock()
	if p.attackedSet == nil {
		p.attackedSet = map[uint32]struct{}{}
	}
	p.attackedSet[target.DBID] = struct{}{}
	p.attackedMu.Unlock()
}

// HasAttacked reports whether this player attacked target (Player::hasAttacked).
func (p *Player) HasAttacked(target *Player) bool {
	if target == nil || p.CannotGainInFight() {
		return false
	}
	p.attackedMu.RLock()
	defer p.attackedMu.RUnlock()
	_, ok := p.attackedSet[target.DBID]
	return ok
}

// HasKilled reports an unavenged kill of target still inside the orange-skull
// window (Player::hasKilled). It is what lets the victim's revenge cancel a frag
// instead of creating a new one.
func (p *Player) HasKilled(target *Player) bool {
	if target == nil {
		return false
	}
	window := config.Number("orangeSkullDuration", defaultOrangeSkullDays) * 24 * 60 * 60
	now := time.Now().Unix()
	for _, kill := range p.UnjustifiedKills {
		if kill.Target == target.DBID && kill.Unavenged && now-kill.Time < window {
			return true
		}
	}
	return false
}

// IsPartner reports party membership (Player::isPartner).
func (p *Player) IsPartner(other *Player) bool {
	if other == nil || other == p || p.Party == nil {
		return false
	}
	return p.Party == other.Party
}

// IsGuildMate reports shared guild membership (Player::isGuildMate).
func (p *Player) IsGuildMate(other *Player) bool {
	if other == nil || p.GuildID == 0 {
		return false
	}
	return p.GuildID == other.GuildID
}

// CannotGainInFight is PlayerFlags_t::NotGainInFight, the staff flag that keeps a
// gamemaster out of the whole frag system.
func (p *Player) CannotGainInFight() bool {
	return p.GroupID >= 3
}

// OnKilledPlayer decides whether this player's kill of target was unjustified, and
// applies the consequence. Port of Player::onKilledPlayer.
//
// A kill is unjustified only when ALL of these hold: the killer is not staff, the
// two are not partners, neither is in a PvP zone, the killer attacked the target,
// the target never attacked back, they are not guild mates, and it is not a self
// kill. Any one of them makes it a fair fight or a non-event.
//
// The revenge branch comes first: if the target had an unavenged kill on this
// player, that frag is cleared instead of a new one being created — dying to
// someone you murdered squares the account.
func (p *Player) OnKilledPlayer(target *Player, lastHit bool) bool {
	if target == nil || p.World == nil {
		return false
	}
	if target.IsInPvpZone() {
		// A death inside a PVP zone costs nothing and is never unjustified.
		target.SkillLoss = false
		return false
	}
	if p.CannotGainInFight() || p.IsPartner(target) {
		return false
	}
	if p.IsInPvpZone() || !p.HasAttacked(target) || target.HasAttacked(p) ||
		p.IsGuildMate(target) || target == p {
		return false
	}

	unjustified := false
	if target.HasKilled(p) {
		// Revenge: clear the target's unavenged frag on us rather than adding one.
		for i := range target.UnjustifiedKills {
			if target.UnjustifiedKills[i].Target == p.DBID && target.UnjustifiedKills[i].Unavenged {
				target.UnjustifiedKills[i].Unavenged = false
				p.attackedMu.Lock()
				delete(p.attackedSet, target.DBID)
				p.attackedMu.Unlock()
				break
			}
		}
	} else if target.Skull == SkullNone && !p.IsInWarWith(target) {
		unjustified = true
		p.AddUnjustifiedDead(target)
	}

	// The white skull marks an aggressor for the duration of the fight.
	if lastHit && p.HasCondition(combat.ConditionInFight) {
		p.Skull = SkullWhite
		p.SkullTime = time.Now().UnixMilli() +
			config.Number("whiteSkullTime", defaultWhiteSkullMillis)
	}
	return unjustified
}

// IsInWarWith reports an active guild war between the two players' guilds, which
// makes kills between them justified. Port of Player::isInWar.
//
// The check is deliberately SYMMETRIC — each side must carry the other's guild in
// its own war list. The lists are snapshots taken at login, so one player can be
// holding a stale one; requiring both to agree means a war that ended, or one that
// started after someone logged in, cannot silently excuse a murder in one direction
// only.
func (p *Player) IsInWarWith(other *Player) bool {
	if other == nil || p.GuildID == 0 || other.GuildID == 0 {
		return false
	}
	return p.IsInWarList(other.GuildID) && other.IsInWarList(p.GuildID)
}

// IsInWarList reports whether guildID is in this player's war list
// (Player::isInWarList).
func (p *Player) IsInWarList(guildID uint32) bool {
	for _, id := range p.GuildWarList {
		if id == guildID {
			return true
		}
	}
	return false
}

// AddUnjustifiedDead records the frag and escalates the skull, mirroring
// Player::addUnjustifiedDead.
func (p *Player) AddUnjustifiedDead(attacked *Player) {
	if p.CannotGainInFight() || attacked == p {
		return
	}
	// An enforced-PvP world has no frags at all.
	if p.World != nil && p.World.WorldType == 3 {
		return
	}

	p.SendTextMessage(messageEventAdvance,
		fmt.Sprintf("Warning! The murder of %s was not justified.", attacked.Name))

	killTime := time.Now().Unix()
	p.UnjustifiedKills = append(p.UnjustifiedKills, Kill{
		Target: attacked.DBID, Time: killTime, Unavenged: true,
	})
	if killTime > p.LastKillTime {
		p.LastKillTime = killTime
	}

	var dayKills, weekKills, monthKills int64
	now := time.Now().Unix()
	for _, kill := range p.UnjustifiedKills {
		diff := now - kill.Time
		if diff <= dayKillWindow {
			dayKills++
		}
		if diff <= weekKillWindow {
			weekKills++
		}
		if diff <= monthKillWindow {
			monthKills++
		}
	}

	dayToRed := config.Number("dayKillsToRedSkull", defaultDayKillsToRed)
	weekToRed := config.Number("weekKillsToRedSkull", defaultWeekKillsToRed)
	monthToRed := config.Number("monthKillsToRedSkull", defaultMonthKillsToRed)

	// A black skull needs DOUBLE the red thresholds, and an existing black skull is
	// never downgraded by a later kill.
	if p.Skull != SkullBlack {
		switch {
		case dayKills >= 2*dayToRed || weekKills >= 2*weekToRed || monthKills >= 2*monthToRed:
			p.Skull = SkullBlack
			p.SkullTime = time.Now().Unix() +
				config.Number("blackSkullDuration", defaultBlackSkullDays)*24*60*60
		case dayKills >= dayToRed || weekKills >= weekToRed || monthKills >= monthToRed:
			p.Skull = SkullRed
			p.SkullTime = time.Now().Unix() +
				config.Number("redSkullDuration", defaultRedSkullDays)*24*60*60
		}
	}
}

// messageEventAdvance is MESSAGE_EVENT_ADVANCE, the class the unjustified warning
// is sent with.
const messageEventAdvance = 0x15

// IsInPvpZone reports whether the player stands on a TILESTATE_PVPZONE tile, the
// Combat::isInPvpZone check that exempts a fight from the frag rules entirely.
func (p *Player) IsInPvpZone() bool {
	if p.World == nil || p.World.Map == nil {
		return false
	}
	tile := p.World.Map.GetTile(p.Pos)
	return tile != nil && tile.Flags&TileFlagPvpZone != 0
}
