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

// AddTarget is Monster::addTarget (monster.cpp:676). Adding the monster to its
// own target list is a bug upstream logs rather than tolerates.
func (m *Monster) AddTarget(c Creature) bool {
	if c == nil || c.GetID() == m.GetID() {
		return false
	}
	if m.Targets == nil {
		m.Targets = make(map[uint32]Creature)
	}
	if _, exists := m.Targets[c.GetID()]; exists {
		return false
	}
	m.Targets[c.GetID()] = c
	return true
}

// RemoveTarget is Monster::removeTarget (monster.cpp:709).
func (m *Monster) RemoveTarget(c Creature) bool {
	if c == nil || m.Targets == nil {
		return false
	}
	if _, exists := m.Targets[c.GetID()]; !exists {
		return false
	}
	delete(m.Targets, c.GetID())
	return true
}

// AddFriend is Monster::addFriend (monster.cpp:651).
func (m *Monster) AddFriend(c Creature) bool {
	if c == nil || c.GetID() == m.GetID() {
		return false
	}
	if m.Friends == nil {
		m.Friends = make(map[uint32]Creature)
	}
	if _, exists := m.Friends[c.GetID()]; exists {
		return false
	}
	m.Friends[c.GetID()] = c
	return true
}

// RemoveFriend is Monster::removeFriend (monster.cpp:661).
func (m *Monster) RemoveFriend(c Creature) bool {
	if c == nil || m.Friends == nil {
		return false
	}
	if _, exists := m.Friends[c.GetID()]; !exists {
		return false
	}
	delete(m.Friends, c.GetID())
	return true
}

// ClearTargetList is Monster::clearTargetList (monster.cpp:771).
func (m *Monster) ClearTargetList() { m.Targets = nil }

// ClearFriendList is Monster::clearFriendList (monster.cpp:779).
func (m *Monster) ClearFriendList() { m.Friends = nil }

// IsTarget is Monster::isTarget (monster.cpp:1433): whether this creature is a
// legal thing to point an attack at, as opposed to isOpponent's "is this an
// enemy". The two differ — a protected or out-of-floor enemy is still an
// opponent but not a target.
func (m *Monster) IsTarget(w *World, c Creature) bool {
	if c == nil || c.GetHealth() == 0 {
		return false
	}
	if c.GetPosition().Z != m.GetPosition().Z {
		return false
	}
	if p, ok := c.(*Player); ok && (p.CannotBeAttacked() || p.Ghost) {
		return false
	}
	if other, ok := c.(*Monster); ok && !other.IsAttackable() {
		return false
	}
	if w != nil && w.Map != nil {
		if tile := w.Map.GetTile(c.GetPosition()); tile != nil && tile.IsProtectionZone() {
			return false
		}
	}
	if m.Master == nil && m.GetFaction() != FactionDefault {
		return m.IsEnemyFaction(CreatureFaction(c))
	}
	return true
}

// OnCreatureFound is Monster::onCreatureFound (monster.cpp:787): sort one
// creature into whichever list it belongs in, and re-evaluate idleness if
// either changed.
func (m *Monster) OnCreatureFound(w *World, c Creature) {
	listChanged := false
	if m.IsFriend(c) {
		listChanged = m.AddFriend(c) || listChanged
	}
	if m.IsOpponent(c) {
		listChanged = m.AddTarget(c) || listChanged
	}
	if listChanged || m.Idle {
		m.UpdateIdleStatus(w)
	}
}

// OnCreatureEnter is Monster::onCreatureEnter (monster.cpp:802): someone came
// into view. This is the wake-up path for an idle monster.
func (m *Monster) OnCreatureEnter(w *World, c Creature) { m.OnCreatureFound(w, c) }

// OnCreatureLeave is Monster::onCreatureLeave (monster.cpp:881): someone left
// view. Idleness is only re-evaluated when the target list actually empties,
// which is what stops a monster idling while it still has other enemies around.
func (m *Monster) OnCreatureLeave(w *World, c Creature) {
	if m.IsFriend(c) {
		m.RemoveFriend(c)
	}
	if m.IsOpponent(c) {
		if m.RemoveTarget(c) && len(m.Targets) == 0 {
			m.UpdateIdleStatus(w)
		}
	}
}

// SetIdle is Monster::setIdle (monster.cpp:1498). Going idle drops both lists;
// coming out of idle does not rebuild them, because whoever woke the monster is
// about to add itself through onCreatureFound.
//
// A removed or dead monster is never re-idled, so a corpse does not clear the
// damage map the loot roll still needs.
func (m *Monster) SetIdle(idle bool) {
	if m.GetHealth() == 0 {
		return
	}
	m.Idle = idle
	if idle {
		m.ClearTargetList()
		m.ClearFriendList()
	}
}

// SetAttackedCreature is Monster::setAttackedCreature (monster.cpp:1367).
func (m *Monster) SetAttackedCreature(c Creature) bool {
	m.SetTarget(c)
	if c == nil {
		return false
	}
	m.Idle = false
	return true
}

// SetFollowCreature is Monster::setFollowCreature (monster.cpp:1376). The port
// has no separate follow target — movement follows the attacked creature — so
// this is the same assignment, kept under its own name because callers and the
// parity audit both look for it.
func (m *Monster) SetFollowCreature(c Creature) bool { return m.SetAttackedCreature(c) }

// UpdateSummonTarget is Monster::updateSummonTarget (monster.cpp:1331): a summon
// fights whatever its master is fighting, and follows its master otherwise.
func (m *Monster) UpdateSummonTarget(w *World) {
	master, ok := m.Master.(*Monster)
	if !ok {
		if p, isPlayer := m.Master.(*Player); isPlayer {
			if t := p.GetTarget(); t != nil && t != Creature(m) {
				m.SelectTarget(w, t)
				return
			}
			m.SetFollowCreature(p)
		}
		return
	}
	if t := master.GetTarget(); t != nil && t != Creature(m) {
		m.SelectTarget(w, t)
		return
	}
	m.SetFollowCreature(master)
}

// UpdateTargetList is Monster::updateTargetList (monster.cpp:734): drop whoever
// died or went out of view, then take in whoever is newly in view.
//
// Upstream maintains the lists purely from spectator events and only prunes
// here. This port has no creature-movement hook to add from, so the sweep does
// both — the resulting state is the same, at the cost of one spectator scan per
// monster per think, which is the scan searchTarget was already doing.
func (m *Monster) UpdateTargetList(w *World) {
	if w == nil {
		return
	}
	for id, c := range m.Friends {
		if c == nil || c.GetHealth() == 0 || !m.GetPosition().InRangeOf(c.GetPosition()) {
			delete(m.Friends, id)
		}
	}
	emptiedTargets := false
	for id, c := range m.Targets {
		if c == nil || c.GetHealth() == 0 || !m.GetPosition().InRangeOf(c.GetPosition()) {
			delete(m.Targets, id)
			emptiedTargets = true
		}
	}

	for _, c := range w.SpectatorCreatures(m.GetPosition()) {
		if c == nil || c.GetID() == m.GetID() || c.GetHealth() == 0 {
			continue
		}
		m.OnCreatureFound(w, c)
	}
	if emptiedTargets && len(m.Targets) == 0 {
		m.UpdateIdleStatus(w)
	}
}
