package game

import "testing"

func TestPartyLifecycle(t *testing.T) {
	leader := &Player{ID: 1, Name: "Leader"}
	m1 := &Player{ID: 2, Name: "Member1"}
	m2 := &Player{ID: 3, Name: "Member2"}

	pt := NewParty(leader, nil)
	if leader.Party != pt {
		t.Fatal("leader not linked to party")
	}

	// Invite + join m1.
	if !pt.Invite(m1) {
		t.Fatal("invite m1 failed")
	}
	if !m1.invitedByParty(leader) {
		t.Error("m1 should see leader's invitation")
	}
	if !pt.Join(m1) {
		t.Fatal("join m1 failed")
	}
	if m1.Party != pt || pt.MemberCount() != 1 {
		t.Fatalf("m1 not a member; count=%d", pt.MemberCount())
	}
	if !leader.isPartner(m1) || !m1.isPartner(leader) {
		t.Error("leader and m1 should be partners")
	}

	// Shields: leader sees blue over member, member sees yellow over leader.
	if s := leader.PartyShield(m1); s != ShieldBlue {
		t.Errorf("leader->m1 shield = %d, want ShieldBlue(%d)", s, ShieldBlue)
	}
	if s := m1.PartyShield(leader); s != ShieldYellow {
		t.Errorf("m1->leader shield = %d, want ShieldYellow(%d)", s, ShieldYellow)
	}

	// Invite + join m2, then pass leadership to m1.
	pt.Invite(m2)
	pt.Join(m2)
	if !pt.PassLeadership(m1) {
		t.Fatal("pass leadership to m1 failed")
	}
	if pt.Leader() != m1 {
		t.Errorf("leader = %s, want Member1", pt.Leader().Name)
	}
	// Old leader is now a member.
	if !m1.isPartner(leader) {
		t.Error("old leader should still be a partner")
	}

	// Shared exp toggle.
	pt.SetSharedExperience(true)
	if !pt.IsSharedExperienceActive() {
		t.Error("shared exp should be active")
	}
	if s := m1.PartyShield(leader); s != ShieldBlueSharedExp {
		t.Errorf("shared-exp member shield = %d, want ShieldBlueSharedExp(%d)", s, ShieldBlueSharedExp)
	}

	// Leave until disband.
	pt.Leave(leader)
	pt.Leave(m2)
	if pt.MemberCount() != 0 {
		t.Errorf("member count after leaves = %d, want 0", pt.MemberCount())
	}
	// Only the leader (m1) remains; leaving disbands.
	pt.Leave(m1)
	if m1.Party != nil {
		t.Error("party should be disbanded (m1.Party != nil)")
	}
}

func TestPartyShieldUnrelated(t *testing.T) {
	a := &Player{ID: 1, Name: "A"}
	b := &Player{ID: 2, Name: "B"}
	NewParty(b, nil) // b is in its own party
	if s := a.PartyShield(b); s != ShieldGray {
		t.Errorf("unrelated party shield = %d, want ShieldGray(%d)", s, ShieldGray)
	}
	c := &Player{ID: 3, Name: "C"}
	if s := a.PartyShield(c); s != ShieldNone {
		t.Errorf("no-party shield = %d, want ShieldNone(%d)", s, ShieldNone)
	}
}
