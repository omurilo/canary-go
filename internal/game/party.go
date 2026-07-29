package game

import (
	"fmt"
	"sync"
)

// messagePartyManagement is MESSAGE_PARTY_MANAGEMENT (34), the type used for
// party notices.
const messagePartyManagement = 34

func sprintf(format string, args ...any) string { return fmt.Sprintf(format, args...) }

// Party mirrors src/creatures/players/grouping/party.cpp at the core-grouping
// level: invite/join/leave/revoke/passLeadership/disband plus shared-experience
// toggling and shield rendering. The analyzer (loot/supply tracker) and the
// Monk mantra subsystem are out of scope (they depend on unported systems).
//
// Invariant (from C++): the leader is NOT in members; the full roster is
// members + leader.
type Party struct {
	mu               sync.Mutex
	leader           *Player
	members          []*Player // excludes the leader
	invites          []*Player
	sharedExpActive  bool
	sharedExpEnabled bool
	world            *World
}

// Party shield values (Shields_t, utils_definitions.hpp). Exactly one byte is
// written inside the AddCreature stream, so these must stay in range 0..11.
const (
	ShieldNone                    = 0
	ShieldWhiteYellow             = 1 // a player invited by the viewer's-target? (target invited viewer)
	ShieldWhiteBlue               = 2
	ShieldBlue                    = 3
	ShieldYellow                  = 4
	ShieldBlueSharedExp           = 5
	ShieldYellowSharedExp         = 6
	ShieldBlueNoSharedExpBlink    = 7
	ShieldYellowNoSharedExpBlink  = 8
	ShieldBlueNoSharedExp         = 9
	ShieldYellowNoSharedExp       = 10
	ShieldGray                    = 11
)

// NewParty creates a party led by leader and links it back to the world for
// shield fan-out.
func NewParty(leader *Player, world *World) *Party {
	p := &Party{leader: leader, world: world}
	leader.Party = p
	return p
}

// Leader returns the party leader.
func (pt *Party) Leader() *Player { return pt.leader }

// Members returns the member list (excluding the leader).
func (pt *Party) Members() []*Player {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	out := make([]*Player, len(pt.members))
	copy(out, pt.members)
	return out
}

// Players returns the full roster: members + leader.
func (pt *Party) Players() []*Player {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	out := make([]*Player, 0, len(pt.members)+1)
	out = append(out, pt.members...)
	if pt.leader != nil {
		out = append(out, pt.leader)
	}
	return out
}

// Invitees returns the pending-invite list.
func (pt *Party) Invitees() []*Player {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	out := make([]*Player, len(pt.invites))
	copy(out, pt.invites)
	return out
}

// MemberCount returns the number of members excluding the leader.
func (pt *Party) MemberCount() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return len(pt.members)
}

// IsSharedExperienceActive reports whether shared-exp is toggled on.
func (pt *Party) IsSharedExperienceActive() bool { return pt.sharedExpActive }

// IsSharedExperienceEnabled reports whether shared-exp is currently effective
// (active AND the roster satisfies the range/activity checks).
func (pt *Party) IsSharedExperienceEnabled() bool { return pt.sharedExpEnabled }

func (pt *Party) isInvited(p *Player) bool {
	for _, i := range pt.invites {
		if i == p {
			return true
		}
	}
	return false
}

func (pt *Party) isMember(p *Player) bool {
	for _, m := range pt.members {
		if m == p {
			return true
		}
	}
	return false
}

// Invite adds p to the invite list (leader-only is enforced by the caller).
func (pt *Party) Invite(p *Player) bool {
	pt.mu.Lock()
	if pt.leader == nil || pt.isInvited(p) {
		pt.mu.Unlock()
		return false
	}
	pt.invites = append(pt.invites, p)
	pt.mu.Unlock()
	p.addPartyInvitation(pt)
	pt.notify(pt.leader, "%s has been invited.", p.Name)
	p.SendTextMessage(messagePartyManagement, pt.leader.Name+" has invited you to "+pt.leaderPossessive()+" party.")
	pt.updateShields()
	return true
}

// Join moves an invited player into the party.
func (pt *Party) Join(p *Player) bool {
	pt.mu.Lock()
	if pt.leader == nil || !pt.isInvited(p) {
		pt.mu.Unlock()
		return false
	}
	pt.removeInviteLocked(p)
	pt.members = append(pt.members, p)
	pt.mu.Unlock()

	p.Party = pt
	p.removePartyInvitation(pt)
	pt.broadcast("%s has joined the party.", p.Name)
	p.SendTextMessage(messagePartyManagement, "You have joined "+pt.leaderPossessive()+" party.")
	pt.updateSharedExperience()
	pt.updateShields()
	return true
}

// Leave removes p from the party (member or leader). A leaving leader promotes
// the first remaining member; an empty party disbands.
func (pt *Party) Leave(p *Player) bool {
	pt.mu.Lock()
	isLeader := pt.leader == p
	if !isLeader && !pt.isMember(p) {
		pt.mu.Unlock()
		return false
	}
	if isLeader {
		if len(pt.members) == 0 {
			pt.mu.Unlock()
			pt.Disband()
			return true
		}
		// Promote the first member to leader.
		newLeader := pt.members[0]
		pt.members = pt.members[1:]
		pt.leader = newLeader
		pt.mu.Unlock()
		newLeader.SendTextMessage(messagePartyManagement, "You are now the leader of the party.")
		p.Party = nil
		pt.broadcast("%s has left the party.", p.Name)
		pt.updateSharedExperience()
		pt.updateShields()
		if pt.world != nil {
			pt.world.UpdatePlayerShield(p)
		}
		return true
	}
	pt.removeMemberLocked(p)
	empty := len(pt.members) == 0
	pt.mu.Unlock()

	p.Party = nil
	pt.broadcast("%s has left the party.", p.Name)
	if pt.world != nil {
		pt.world.UpdatePlayerShield(p)
	}
	if empty {
		pt.Disband()
		return true
	}
	pt.updateSharedExperience()
	pt.updateShields()
	return true
}

// Revoke cancels a pending invitation.
func (pt *Party) Revoke(p *Player) bool {
	pt.mu.Lock()
	if !pt.isInvited(p) {
		pt.mu.Unlock()
		return false
	}
	pt.removeInviteLocked(p)
	empty := len(pt.invites) == 0 && len(pt.members) == 0
	pt.mu.Unlock()

	p.removePartyInvitation(pt)
	if pt.world != nil {
		pt.world.UpdatePlayerShield(p)
	}
	if empty {
		pt.Disband()
	}
	return true
}

// PassLeadership transfers leadership to a current member.
func (pt *Party) PassLeadership(p *Player) bool {
	pt.mu.Lock()
	if pt.leader == nil || pt.leader == p || !pt.isMember(p) {
		pt.mu.Unlock()
		return false
	}
	pt.removeMemberLocked(p)
	old := pt.leader
	pt.members = append([]*Player{old}, pt.members...)
	pt.leader = p
	pt.mu.Unlock()

	pt.broadcast("%s is now the leader of the party.", p.Name)
	pt.updateSharedExperience()
	pt.updateShields()
	return true
}

// Disband dissolves the party. Idempotent.
func (pt *Party) Disband() {
	pt.mu.Lock()
	if pt.leader == nil {
		pt.mu.Unlock()
		return
	}
	leader := pt.leader
	members := pt.members
	invites := pt.invites
	pt.leader = nil
	pt.members = nil
	pt.invites = nil
	pt.mu.Unlock()

	all := append([]*Player{}, members...)
	all = append(all, leader)
	for _, m := range all {
		if m != nil {
			m.Party = nil
			m.SendTextMessage(messagePartyManagement, "Your party has been disbanded.")
			if pt.world != nil {
				pt.world.UpdatePlayerShield(m)
			}
		}
	}
	for _, i := range invites {
		if i != nil {
			i.removePartyInvitation(pt)
			if pt.world != nil {
				pt.world.UpdatePlayerShield(i)
			}
		}
	}
}

// SetSharedExperience toggles shared experience (leader-only, enforced by caller).
func (pt *Party) SetSharedExperience(active bool) {
	pt.sharedExpActive = active
	pt.updateSharedExperience()
	msg := "Shared Experience is now inactive."
	if pt.sharedExpEnabled {
		msg = "Shared Experience is now active."
	} else if active {
		msg = "Shared Experience is now active, but not all party members are close enough or of a similar level."
	}
	pt.broadcast("%s", msg)
	pt.updateShields()
}

// getLowestHazardPoints returns the lowest hazard points among all party members.
func (pt *Party) getLowestHazardPoints() uint32 {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	var lowest uint32
	first := true
	check := func(p *Player) {
		if p == nil {
			return
		}
		if first || p.HazardPoints < lowest {
			lowest = p.HazardPoints
			first = false
		}
	}
	check(pt.leader)
	for _, m := range pt.members {
		check(m)
	}
	return lowest
}

// ShareExperience splits experience across the whole roster, applying hazard
// experience bonus based on the lowest hazard points in the party.
func (pt *Party) ShareExperience(exp uint64) {
	lowestHazard := pt.getLowestHazardPoints()
	var hazardMultiplier float64
	if lowestHazard > 0 {
		hazardMultiplier = 1.0 + hazardExpBonus(int32(lowestHazard))
	}

	for _, m := range pt.Players() {
		if m != nil {
			finalExp := exp
			if hazardMultiplier > 0 {
				finalExp = uint64(float64(exp) * hazardMultiplier)
			}
			m.AddExperience(finalExp)
			if pt.world != nil && pt.world.OnPlayerStatsChange != nil {
				pt.world.OnPlayerStatsChange(m)
			}
		}
	}
}

// updateSharedExperience recomputes whether shared-exp is effective.
func (pt *Party) updateSharedExperience() {
	pt.sharedExpEnabled = pt.sharedExpActive && pt.MemberCount() > 0
}

// updateShields refreshes party shields for every roster member and invitee.
func (pt *Party) updateShields() {
	if pt.world == nil {
		return
	}
	for _, m := range pt.Players() {
		pt.world.UpdatePlayerShield(m)
	}
	for _, i := range pt.Invitees() {
		pt.world.UpdatePlayerShield(i)
	}
}

func (pt *Party) removeInviteLocked(p *Player) {
	out := pt.invites[:0]
	for _, i := range pt.invites {
		if i != p {
			out = append(out, i)
		}
	}
	pt.invites = out
}

func (pt *Party) removeMemberLocked(p *Player) {
	out := pt.members[:0]
	for _, m := range pt.members {
		if m != p {
			out = append(out, m)
		}
	}
	pt.members = out
}

// broadcast sends a management message to the whole roster.
func (pt *Party) broadcast(format string, args ...any) {
	for _, m := range pt.Players() {
		if m != nil {
			pt.notify(m, format, args...)
		}
	}
}

func (pt *Party) notify(to *Player, format string, args ...any) {
	to.SendTextMessage(messagePartyManagement, sprintf(format, args...))
}

func (pt *Party) leaderPossessive() string {
	if pt.leader != nil {
		return pt.leader.Name + "'s"
	}
	return "the"
}
