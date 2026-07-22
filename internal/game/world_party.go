package game

// Party orchestration on the world, ported from the Game::player* party
// entrypoints (src/game/game.cpp). ID resolution goes through PlayerByID; every
// mutation ends by refreshing shields for the affected players.

// UpdatePlayerShield refreshes the party shield rendered over `target` for the
// target itself and every spectator that can see it. Mirrors
// Game::updatePlayerShield.
func (w *World) UpdatePlayerShield(target *Player) {
	if target == nil || w.OnShieldUpdate == nil {
		return
	}
	w.OnShieldUpdate(target, target)
	for _, s := range w.Spectators(target.Pos, target.ID) {
		w.OnShieldUpdate(s, target)
	}
}

// PlayerInviteToParty invites invitee to inviter's party, creating the party if
// the inviter has none. Mirrors Game::playerInviteToParty.
func (w *World) PlayerInviteToParty(inviterID, inviteeID uint32) bool {
	inviter := w.PlayerByID(inviterID)
	invitee := w.PlayerByID(inviteeID)
	if inviter == nil || invitee == nil || inviter == invitee {
		return false
	}
	// Can't invite someone already in a party.
	if invitee.Party != nil {
		inviter.SendTextMessage(messagePartyManagement, invitee.Name+" is already in a party.")
		return false
	}
	if inviter.Party == nil {
		NewParty(inviter, w)
		w.UpdatePlayerShield(inviter)
	}
	// Only the leader may invite.
	if inviter.Party.Leader() != inviter {
		return false
	}
	return inviter.Party.Invite(invitee)
}

// PlayerJoinParty joins joiner to the leader's party.
func (w *World) PlayerJoinParty(joinerID, leaderID uint32) bool {
	joiner := w.PlayerByID(joinerID)
	leader := w.PlayerByID(leaderID)
	if joiner == nil || leader == nil || leader.Party == nil {
		return false
	}
	if leader.Party.Leader() != leader {
		return false
	}
	return leader.Party.Join(joiner)
}

// PlayerRevokePartyInvitation revokes the leader's invitation to target.
func (w *World) PlayerRevokePartyInvitation(leaderID, targetID uint32) bool {
	leader := w.PlayerByID(leaderID)
	target := w.PlayerByID(targetID)
	if leader == nil || target == nil || leader.Party == nil {
		return false
	}
	if leader.Party.Leader() != leader {
		return false
	}
	return leader.Party.Revoke(target)
}

// PlayerPassPartyLeadership transfers leadership from the current leader to
// newLeaderID (a current member).
func (w *World) PlayerPassPartyLeadership(leaderID, newLeaderID uint32) bool {
	leader := w.PlayerByID(leaderID)
	newLeader := w.PlayerByID(newLeaderID)
	if leader == nil || newLeader == nil || leader.Party == nil {
		return false
	}
	if leader.Party.Leader() != leader || !leader.isPartner(newLeader) {
		return false
	}
	return leader.Party.PassLeadership(newLeader)
}

// PlayerLeaveParty removes a player from their party.
func (w *World) PlayerLeaveParty(playerID uint32) bool {
	p := w.PlayerByID(playerID)
	if p == nil || p.Party == nil {
		return false
	}
	return p.Party.Leave(p)
}

// PlayerEnableSharedPartyExperience toggles shared experience (leader-only).
func (w *World) PlayerEnableSharedPartyExperience(leaderID uint32, active bool) bool {
	leader := w.PlayerByID(leaderID)
	if leader == nil || leader.Party == nil || leader.Party.Leader() != leader {
		return false
	}
	leader.Party.SetSharedExperience(active)
	return true
}
