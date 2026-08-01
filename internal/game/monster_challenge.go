package game

// Challenge, ported from src/creatures/monsters/monster.cpp.
//
// Two separate timers with two separate effects, and the port had neither:
//
//	challengeFocusDuration  pins the monster on whoever challenged it, blocking
//	                        onThinkTarget from re-rolling
//	challengeMeleeDuration  forces targetDistance down to melee, so a distance
//	                        monster is dragged into hitting range
//
// ChangeTargetDistance already existed as a one-line setter with no duration and
// no guards, which meant a script could permanently melee-lock a reward boss —
// something upstream refuses outright.

// SelectTarget is Monster::selectTarget (monster.cpp:1467): make this creature
// the monster's target, if it is a legal one.
func (m *Monster) SelectTarget(w *World, c Creature) bool {
	if c == nil || !m.IsOpponent(c) {
		return false
	}
	if c.GetHealth() == 0 {
		return false
	}
	if m.Targets == nil {
		m.Targets = make(map[uint32]Creature)
	}
	m.Targets[c.GetID()] = c
	m.SetTarget(c)
	m.Idle = false
	return true
}

// ChallengeCreature is Monster::challengeCreature (monster.cpp:3469): force the
// monster onto a target and hold it there for targetChangeCooldown ms.
//
// A summon cannot be challenged — it follows its master's target — and the hold
// only starts if the target was actually accepted.
func (m *Monster) ChallengeCreature(w *World, c Creature, targetChangeCooldown int) bool {
	if m.Master != nil {
		return false
	}
	if !m.SelectTarget(w, c) {
		return false
	}
	m.challengeFocusDuration = targetChangeCooldown
	m.targetChangeTicks = 0
	return true
}

// IsChallenged is Monster::isChallenged (monster.cpp:3506).
func (m *Monster) IsChallenged() bool { return m.challengeFocusDuration > 0 }

// ChangeTargetDistance is Monster::changeTargetDistance (monster.cpp:3487):
// override the monster's fighting distance for a while.
//
// The two refusals are the point. A summon is not draggable, and neither is a
// reward boss — otherwise a single challenge rune would pull a boss designed to
// fight at range into melee for the whole fight.
//
// duration of 0 means the caller wants upstream's default of 12000ms.
func (m *Monster) ChangeTargetDistance(distance int32, duration int) bool {
	if m.Master != nil {
		return false
	}
	if m.Type != nil && m.Type.Flags.RewardBoss {
		return false
	}
	if duration == 0 {
		duration = defaultChallengeMeleeDuration
	}
	m.challengeMeleeDuration = duration
	m.TargetDistance = distance
	return true
}

// IsTurnedMelee reports whether a challenge is currently holding the monster
// closer than its type wants. It is the condition behind the TurnedMelee icon in
// Monster::getIcons (monster.cpp:3519) and the only part of that list this port
// can currently answer.
func (m *Monster) IsTurnedMelee() bool {
	if m.challengeMeleeDuration <= 0 || m.Type == nil {
		return false
	}
	return m.Type.Flags.TargetDistance > int(m.TargetDistance)
}

// tickChallenge advances the melee-challenge timer and restores the type's own
// fighting distance when it lapses. Monster::onThink (monster.cpp:1608-1615)
// does this before anything else, so the monster is back at range on the very
// tick the challenge expires.
func (m *Monster) tickChallenge(w *World, interval uint32) {
	if m.challengeMeleeDuration == 0 {
		return
	}
	m.challengeMeleeDuration -= int(interval)
	if m.challengeMeleeDuration > 0 {
		return
	}
	m.challengeMeleeDuration = 0
	if m.Type != nil {
		m.TargetDistance = int32(m.Type.Flags.TargetDistance)
	}
	if w != nil && w.OnIconsUpdate != nil {
		if p, ok := m.GetTarget().(*Player); ok {
			w.OnIconsUpdate(p)
		}
	}
}

// defaultChallengeMeleeDuration is the 12000ms default of
// Monster::changeTargetDistance.
const defaultChallengeMeleeDuration = 12000
