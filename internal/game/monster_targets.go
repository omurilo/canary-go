package game

// Target and friend lists, ported from src/creatures/monsters/monster.cpp.
//
// Upstream keeps two lists on every monster: who it may attack (targetList) and
// who it must not (friendList). They are maintained by spectator events —
// onCreatureEnter adds, onCreatureLeave removes — and everything else reads
// them: searchTarget picks from targetList, updateIdleStatus goes idle when it
// is empty, area spells skip friendList.
//
// Neither list was ever populated here. Monster.Targets and Monster.Friends
// existed as empty maps, so searchTarget scanned raw spectators instead (which
// meant a monster could only ever attack a player) and updateIdleStatus saw an
// empty list for every monster on the map.

// Faction_t values. FACTION_DEFAULT is the "no faction" case that most monsters
// have, and it is what makes them fight players and nothing else.
const (
	FactionDefault uint8 = 0
	FactionPlayer  uint8 = 1
)

// CreatureFaction is Creature::getFaction. Players are FACTION_PLAYER, a summon
// inherits its master's faction, and everything else reads it off its type.
func CreatureFaction(c Creature) uint8 {
	switch v := c.(type) {
	case *Player:
		return FactionPlayer
	case *Monster:
		if v.Master != nil {
			return CreatureFaction(v.Master)
		}
		if v.Type != nil {
			return v.Type.Faction
		}
	}
	return FactionDefault
}

// IsEnemyFaction is Monster::isEnemyFaction (monster.cpp:266).
func (m *Monster) IsEnemyFaction(faction uint8) bool {
	if m.Type == nil {
		return false
	}
	for _, f := range m.Type.EnemyFactions {
		if f == faction {
			return true
		}
	}
	return false
}

// IsFriend is Monster::isFriend (monster.cpp:811). A player's summon counts its
// master and the master's party as friends; every other monster counts
// non-summoned monsters as friends.
func (m *Monster) IsFriend(c Creature) bool {
	if c == nil {
		return false
	}
	if masterPlayer, ok := m.Master.(*Player); ok {
		other := playerBehind(c)
		if other != nil && (other == masterPlayer || masterPlayer.IsPartner(other)) {
			return true
		}
	}
	other, isMonster := c.(*Monster)
	return isMonster && other.Master == nil
}

// IsOpponent is Monster::isOpponent (monster.cpp:839): who this monster is
// allowed to attack.
//
// The faction branch is the one that lets monsters fight each other. With
// enemyFactions unread it could never be reached, so two hostile factions
// standing next to each other simply ignored one another.
func (m *Monster) IsOpponent(c Creature) bool {
	if c == nil {
		return false
	}
	if _, ok := m.Master.(*Player); ok {
		return c != m.Master
	}
	if p, ok := c.(*Player); ok && p.CannotBeAttacked() {
		return false
	}
	if m.Type != nil && m.Type.Faction != FactionDefault {
		f := CreatureFaction(c)
		return m.IsEnemyFaction(f) || f == FactionPlayer
	}
	// No faction: players, and anything a player owns.
	return playerBehind(c) != nil
}

// playerBehind resolves a creature to the player driving it — itself if it is a
// player, its master if it is a player's summon, nil otherwise.
func playerBehind(c Creature) *Player {
	switch v := c.(type) {
	case *Player:
		return v
	case *Monster:
		if p, ok := v.Master.(*Player); ok {
			return p
		}
	}
	return nil
}

// UpdateTargetList rebuilds both lists from who is currently on screen. It is
// Monster::updateTargetList (monster.cpp:734) plus the onCreatureEnter path that
// fills them, collapsed into one pass.
//
// Upstream maintains the lists incrementally from spectator events; rebuilding
// them once per think reaches the same state and is what the port can do
// without creature-movement hooks. The cost is one spectator scan per monster
// per second, which is the scan searchTarget was already doing.
func (m *Monster) UpdateTargetList(w *World) {
	if w == nil {
		return
	}
	targets := make(map[uint32]Creature)
	friends := make(map[uint32]Creature)

	for _, c := range w.SpectatorCreatures(m.GetPosition()) {
		if c == nil || c.GetID() == m.GetID() || c.GetHealth() == 0 {
			continue
		}
		switch {
		case m.IsFriend(c):
			friends[c.GetID()] = c
		case m.IsOpponent(c):
			targets[c.GetID()] = c
		}
	}
	m.Targets = targets
	m.Friends = friends
}
