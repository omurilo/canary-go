package game

import (
	"testing"
	"time"
)

// pvpPair builds two players in a world, with the tiles they stand on.
func pvpPair(t *testing.T) (*World, *Player, *Player) {
	t.Helper()
	w := NewWorld()
	pos := Position{X: 100, Y: 100, Z: 7}
	w.Map.SetTile(pos, &Tile{Ground: &Item{ID: 1}})

	killer := &Player{Name: "Killer", DBID: 10, GroupID: 1}
	victim := &Player{Name: "Victim", DBID: 20, GroupID: 1}
	killer.SetPosition(pos)
	victim.SetPosition(pos)
	w.AddPlayer(killer, nil)
	w.AddPlayer(victim, nil)
	return w, killer, victim
}

// The rule is a conjunction, so every clause needs its own case: a single wrong &&
// here writes a skull onto someone who did not earn it, and player_kills now
// persists that mistake across restarts.
func TestOnKilledPlayerJustificationRules(t *testing.T) {
	t.Run("aggressor killing a passive victim is unjustified", func(t *testing.T) {
		_, killer, victim := pvpPair(t)
		killer.AddAttacked(victim)
		if !killer.OnKilledPlayer(victim, true) {
			t.Errorf("an unprovoked kill must be unjustified")
		}
		if len(killer.UnjustifiedKills) != 1 {
			t.Errorf("the frag was not recorded: %+v", killer.UnjustifiedKills)
		}
	})

	t.Run("a killer who never attacked is not unjustified", func(t *testing.T) {
		_, killer, victim := pvpPair(t)
		// No AddAttacked: finishing someone you never engaged does not count.
		if killer.OnKilledPlayer(victim, true) {
			t.Errorf("without having attacked, the kill must be justified")
		}
	})

	t.Run("a victim who fought back makes it justified", func(t *testing.T) {
		_, killer, victim := pvpPair(t)
		killer.AddAttacked(victim)
		victim.AddAttacked(killer)
		if killer.OnKilledPlayer(victim, true) {
			t.Errorf("a mutual fight must be justified")
		}
		if len(killer.UnjustifiedKills) != 0 {
			t.Errorf("no frag should be recorded for a mutual fight")
		}
	})

	t.Run("party members never frag each other", func(t *testing.T) {
		_, killer, victim := pvpPair(t)
		party := &Party{}
		killer.Party, victim.Party = party, party
		killer.AddAttacked(victim)
		if killer.OnKilledPlayer(victim, true) {
			t.Errorf("killing a partner must be justified")
		}
	})

	t.Run("staff are outside the system", func(t *testing.T) {
		_, killer, victim := pvpPair(t)
		killer.GroupID = 3 // NotGainInFight
		killer.AddAttacked(victim)
		if killer.OnKilledPlayer(victim, true) {
			t.Errorf("a gamemaster kill must never be unjustified")
		}
	})

	t.Run("a victim who already has a skull is fair game", func(t *testing.T) {
		_, killer, victim := pvpPair(t)
		victim.Skull = SkullRed
		killer.AddAttacked(victim)
		if killer.OnKilledPlayer(victim, true) {
			t.Errorf("killing a skulled player must be justified")
		}
	})

	t.Run("a pvp zone exempts the fight entirely", func(t *testing.T) {
		w, killer, victim := pvpPair(t)
		w.Map.GetTile(victim.Pos).Flags |= TileFlagPvpZone
		killer.AddAttacked(victim)
		victim.SkillLoss = true
		if killer.OnKilledPlayer(victim, true) {
			t.Errorf("a kill inside a pvp zone must be justified")
		}
		if victim.SkillLoss {
			t.Errorf("a pvp-zone death must not cost skills")
		}
	})

	t.Run("self kill", func(t *testing.T) {
		_, killer, _ := pvpPair(t)
		if killer.OnKilledPlayer(killer, true) {
			t.Errorf("killing yourself must never be unjustified")
		}
	})
}

// Revenge: dying to someone you murdered clears that frag instead of creating a new
// one, which is what the unavenged flag is for.
func TestRevengeClearsTheFragInsteadOfAddingOne(t *testing.T) {
	_, avenger, murderer := pvpPair(t)

	// The murderer previously killed the avenger, unavenged.
	murderer.UnjustifiedKills = []Kill{
		{Target: avenger.DBID, Time: time.Now().Unix() - 60, Unavenged: true},
	}
	// Note the murderer does NOT attack back in this fight: the same
	// !target->hasAttacked(killer) clause that gates an unjustified kill also gates
	// the revenge branch, so a mutual brawl is simply justified and clears nothing.

	// Now the avenger kills the murderer.
	avenger.AddAttacked(murderer)
	unjustified := avenger.OnKilledPlayer(murderer, true)

	if unjustified {
		t.Errorf("avenging a murder must not be unjustified")
	}
	if len(avenger.UnjustifiedKills) != 0 {
		t.Errorf("the avenger must not gain a frag: %+v", avenger.UnjustifiedKills)
	}
	if murderer.UnjustifiedKills[0].Unavenged {
		t.Errorf("the murderer's frag should have been marked avenged")
	}
}

// The escalation thresholds, including that a black skull needs DOUBLE the red ones
// and that an existing black skull is never downgraded.
func TestUnjustifiedKillsEscalateTheSkull(t *testing.T) {
	_, killer, victim := pvpPair(t)
	now := time.Now().Unix()

	// Two prior kills today; the third crosses dayKillsToRedSkull (3).
	killer.UnjustifiedKills = []Kill{
		{Target: 91, Time: now - 60, Unavenged: true},
		{Target: 92, Time: now - 120, Unavenged: true},
	}
	if killer.Skull != SkullNone {
		t.Fatalf("precondition: killer starts unskulled")
	}
	killer.AddUnjustifiedDead(victim)
	if killer.Skull != SkullRed {
		t.Fatalf("3 kills in the day window must give a red skull, got %d", killer.Skull)
	}

	// Up to six, which is 2 * dayKillsToRedSkull, and it goes black.
	killer.UnjustifiedKills = append(killer.UnjustifiedKills,
		Kill{Target: 93, Time: now - 30, Unavenged: true},
		Kill{Target: 94, Time: now - 40, Unavenged: true},
	)
	killer.AddUnjustifiedDead(&Player{Name: "Sixth", DBID: 95})
	if killer.Skull != SkullBlack {
		t.Fatalf("6 kills in the day window must give a black skull, got %d", killer.Skull)
	}

	// A further kill must not downgrade an existing black skull.
	killer.AddUnjustifiedDead(&Player{Name: "Seventh", DBID: 96})
	if killer.Skull != SkullBlack {
		t.Errorf("a black skull must not be downgraded, got %d", killer.Skull)
	}
}

// Kills outside the counting window must not escalate. The day bucket is four
// hours, not twenty-four — the name says day but the code says 4 * 60 * 60.
func TestOldKillsDoNotEscalate(t *testing.T) {
	_, killer, victim := pvpPair(t)
	now := time.Now().Unix()
	// Two kills five hours ago: inside the week and month windows, outside the day one.
	killer.UnjustifiedKills = []Kill{
		{Target: 91, Time: now - 5*60*60, Unavenged: true},
		{Target: 92, Time: now - 5*60*60, Unavenged: true},
	}
	killer.AddUnjustifiedDead(victim)
	// 3 in the week window is below weekKillsToRedSkull (5), and only 1 is inside
	// the 4h day window, so no skull yet.
	if killer.Skull != SkullNone {
		t.Errorf("kills past the 4h day window must not trigger a red skull, got %d", killer.Skull)
	}
}

// An enforced-PvP world has no frags at all.
func TestEnforcedPvpWorldRecordsNoFrags(t *testing.T) {
	w, killer, victim := pvpPair(t)
	w.WorldType = 3 // WORLD_TYPE_PVP_ENFORCED
	killer.AddAttacked(victim)
	killer.OnKilledPlayer(victim, true)
	if len(killer.UnjustifiedKills) != 0 {
		t.Errorf("an enforced-pvp world must record no frags, got %+v", killer.UnjustifiedKills)
	}
	if killer.Skull == SkullRed || killer.Skull == SkullBlack {
		t.Errorf("no frag skull in an enforced-pvp world, got %d", killer.Skull)
	}
}

// The white skull marks the aggressor, but only on the last hit and only while in
// fight — it is the visible "this one started it" flag.
func TestWhiteSkullOnLastHit(t *testing.T) {
	_, killer, victim := pvpPair(t)
	killer.AddAttacked(victim)
	killer.AddInFightTicks()

	killer.OnKilledPlayer(victim, true)
	if killer.Skull != SkullWhite && killer.Skull != SkullRed && killer.Skull != SkullBlack {
		t.Errorf("the last hitter in a fight must be skulled, got %d", killer.Skull)
	}

	// Not the last hit: no white skull from this call.
	_, other, victim2 := pvpPair(t)
	other.AddAttacked(victim2)
	other.AddInFightTicks()
	other.Skull = SkullNone
	other.OnKilledPlayer(victim2, false)
	if other.Skull == SkullWhite {
		t.Errorf("a non-last hit must not apply the white skull")
	}
}

func TestHasAttackedAndHasKilled(t *testing.T) {
	_, a, b := pvpPair(t)

	if a.HasAttacked(b) {
		t.Errorf("nobody has attacked yet")
	}
	a.AddAttacked(b)
	if !a.HasAttacked(b) {
		t.Errorf("AddAttacked did not register")
	}
	// Attacking yourself is not recorded.
	a.AddAttacked(a)
	if a.HasAttacked(a) {
		t.Errorf("self-attack must not register")
	}

	// hasKilled only counts unavenged kills inside the orange-skull window.
	now := time.Now().Unix()
	a.UnjustifiedKills = []Kill{{Target: b.DBID, Time: now - 60, Unavenged: true}}
	if !a.HasKilled(b) {
		t.Errorf("a recent unavenged kill must count")
	}
	a.UnjustifiedKills[0].Unavenged = false
	if a.HasKilled(b) {
		t.Errorf("an avenged kill must not count")
	}
	a.UnjustifiedKills[0] = Kill{Target: b.DBID, Time: now - 8*24*60*60, Unavenged: true}
	if a.HasKilled(b) {
		t.Errorf("a kill past the orange-skull window must not count")
	}
}
