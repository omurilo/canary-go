package game

// Player-side party helpers, ported from Player::getPartyShield / isInviting /
// isPartner (src/creatures/players/player.cpp).

// addPartyInvitation records that party pt has invited this player.
func (p *Player) addPartyInvitation(pt *Party) {
	for _, i := range p.partyInvitations {
		if i == pt {
			return
		}
	}
	p.partyInvitations = append(p.partyInvitations, pt)
}

// removePartyInvitation drops a party from this player's pending invitations.
func (p *Player) removePartyInvitation(pt *Party) {
	out := p.partyInvitations[:0]
	for _, i := range p.partyInvitations {
		if i != pt {
			out = append(out, i)
		}
	}
	p.partyInvitations = out
}

// isInviting reports whether this player (as a leader) has invited other.
func (p *Player) isInviting(other *Player) bool {
	if p.Party == nil || p.Party.Leader() != p {
		return false
	}
	return p.Party.isInvited(other)
}

// isPartner reports whether other is in the same party as this player.
func (p *Player) isPartner(other *Player) bool {
	if p.Party == nil || other == p {
		return false
	}
	return p.Party == other.Party
}

// invitedByParty reports whether other's party has invited this player.
func (p *Player) invitedByParty(other *Player) bool {
	if other.Party == nil || other.Party.Leader() != other {
		return false
	}
	for _, pt := range p.partyInvitations {
		if pt == other.Party {
			return true
		}
	}
	return false
}

// PartyShield returns the shield byte this player (the viewer) should render
// over `other`, mirroring Player::getPartyShield. Exactly one byte 0..11.
func (p *Player) PartyShield(other *Player) uint8 {
	if other == nil {
		return ShieldNone
	}
	otherParty := other.Party

	if p.Party != nil {
		leader := p.Party.Leader()
		if leader == other {
			// The viewer's own leader.
			return leaderShield(p.Party)
		}
		if p.isPartner(other) {
			// A fellow party member.
			return memberShield(p.Party)
		}
	}
	// Invitation relationships.
	if p.isInviting(other) {
		return ShieldWhiteBlue
	}
	if p.invitedByParty(other) {
		return ShieldWhiteYellow
	}
	// Someone in an unrelated party.
	if otherParty != nil {
		return ShieldGray
	}
	return ShieldNone
}

func leaderShield(pt *Party) uint8 {
	if pt.IsSharedExperienceActive() {
		if pt.IsSharedExperienceEnabled() {
			return ShieldYellowSharedExp
		}
		return ShieldYellowNoSharedExp
	}
	return ShieldYellow
}

func memberShield(pt *Party) uint8 {
	if pt.IsSharedExperienceActive() {
		if pt.IsSharedExperienceEnabled() {
			return ShieldBlueSharedExp
		}
		return ShieldBlueNoSharedExp
	}
	return ShieldBlue
}
