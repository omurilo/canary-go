package game

import (
	"math/rand"

	"github.com/omurilo/canary-go/internal/game/combat"
)

// The Monster:: event handlers, ported from src/creatures/monsters/monster.cpp.
//
// Every one of these is the C++ hook that keeps a monster's own bookkeeping in
// step with the world around it, plus the script callback the datapack attaches
// to it. The port had the callbacks (wired from main.go) but none of the
// bookkeeping, so a monster's target list, idle state and summon list drifted
// out of step with reality and were only ever corrected by the periodic sweep.

// OnThink is Monster::onThink (monster.cpp:1580). It is the ordering that
// matters here: the challenge timer, then the despawn check, then idleness, and
// only then the four independent think timers. A monster teleported home by the
// despawn check does not go on to yell from where it used to be.
func (m *Monster) OnThink(w *World, interval uint32) {
	m.tickChallenge(w, interval)

	if !m.IsInSpawnRange(m.GetPosition()) {
		if w != nil {
			w.TeleportCreature(m, m.SpawnPosition)
		}
		m.SetIdle(true)
		return
	}

	m.UpdateIdleStatus()

	m.OnThinkTarget(w, interval)
	m.OnThinkDefense(w, interval)
	m.OnThinkYell(w, interval)
	m.OnThinkSound(w, interval)
}

// OnThinkSound is Monster::onThinkSound (monster.cpp:2295), the sibling of
// onThinkYell for ambient audio. No datapack monster ships a sounds block
// today, so this does nothing until one does — which is exactly what upstream
// does with an empty soundVector.
func (m *Monster) OnThinkSound(w *World, interval uint32) {
	if m.Type == nil || m.Type.SoundInterval == 0 {
		return
	}
	m.soundTicks += int(interval)
	if m.soundTicks < m.Type.SoundInterval {
		return
	}
	m.soundTicks = 0

	if len(m.Type.Sounds) == 0 || m.Type.SoundChance < rand.Intn(100)+1 {
		return
	}
	if w != nil && w.OnSoundEffect != nil {
		w.OnSoundEffect(m.GetPosition(), m.Type.Sounds[rand.Intn(len(m.Type.Sounds))])
	}
}

// OnCreatureAppear is Monster::onCreatureAppear (monster.cpp:347). A monster
// seeing itself appear rebuilds its whole view; seeing anyone else appear is a
// single list insertion.
func (m *Monster) OnCreatureAppear(w *World, c Creature) {
	if c == nil {
		return
	}
	if c.GetID() == m.GetID() {
		m.UpdateTargetList(w)
		m.UpdateIdleStatus()
		return
	}
	m.OnCreatureEnter(c)
}

// OnRemoveCreature is Monster::onRemoveCreature (monster.cpp:390). A monster
// being removed goes idle so its spawn can start counting down to a respawn.
func (m *Monster) OnRemoveCreature(w *World, c Creature) {
	if c == nil {
		return
	}
	if c.GetID() == m.GetID() {
		m.SetIdle(true)
		return
	}
	m.OnCreatureLeave(c)
}

// OnCreatureMove is Monster::onCreatureMove (monster.cpp:433): someone moved,
// so re-sort them. Moving out of view removes them from both lists; moving into
// view adds them and can wake an idle monster.
func (m *Monster) OnCreatureMove(w *World, c Creature, oldPos, newPos Position) {
	if c == nil || c.GetID() == m.GetID() {
		return
	}

	// Creature::onCreatureMove (creature.cpp:562): when the thing we are
	// attacking moves, either it left our sight — drop it — or it is still there
	// and we get a free swing if one is owed.
	//
	// The free swing is the whole reason extraMeleeAttack exists. Without it a
	// monster whose target stepped away and back paid the full 1500ms melee
	// cooldown again, so a player could dodge every second hit by stepping.
	if target := m.GetTarget(); target != nil && target.GetID() == c.GetID() {
		if newPos.Z != oldPos.Z || !m.GetPosition().InRangeOf(newPos) {
			m.OnAttackedCreatureDisappear()
		} else if m.HasExtraSwing() && w != nil {
			m.DoAttacking(w, 0)
		}
	}

	if m.GetPosition().InRangeOf(newPos) {
		m.OnCreatureEnter(c)
		return
	}
	m.OnCreatureLeave(c)
}

// OnCreatureSay is Monster::onCreatureSay (monster.cpp:567). Upstream's own
// body is only the script callback — a monster has no built-in reaction to
// speech — so this exists to give the datapack's onCreatureSay somewhere to
// hang, and to keep the audit honest about which hooks are reachable.
func (m *Monster) OnCreatureSay(w *World, speaker Creature, talkType byte, text string) {
	if w != nil && w.OnMonsterCreatureSay != nil {
		w.OnMonsterCreatureSay(m, speaker, talkType, text)
	}
}

// OnAttackedByPlayer is Monster::onAttackedByPlayer (monster.cpp:599).
func (m *Monster) OnAttackedByPlayer(w *World, attacker *Player) {
	if w != nil && w.OnMonsterAttackedByPlayer != nil {
		w.OnMonsterAttackedByPlayer(m, attacker)
	}
}

// OnSpawn is Monster::onSpawn (monster.cpp:626), fired once when the spawn
// engine places the monster.
func (m *Monster) OnSpawn(w *World, pos Position) {
	if w != nil && w.OnMonsterSpawn != nil {
		w.OnMonsterSpawn(m, pos)
	}
}

// OnAttackedCreatureDisappear is Monster::onAttackedCreatureDisappear
// (monster.cpp:342). Resetting attackTicks and arming the extra swing is what
// lets a monster hit immediately when its target reappears, instead of waiting
// out the remainder of an interval it spent staring at nothing.
func (m *Monster) OnAttackedCreatureDisappear() {
	m.attackTicks = 0
	m.extraMeleeAttack = true
}

// OnAddCondition is Monster::onAddCondition (monster.cpp:1568). A condition is
// enough on its own to keep a monster out of idle — upstream's updateIdleStatus
// checks `conditions.empty()` before anything else — so a poisoned monster
// alone at its spawn keeps thinking until the poison runs out.
func (m *Monster) OnAddCondition(t combat.ConditionType) {
	m.OnConditionStatusChange(t)
}

// OnEndCondition is Monster::onEndCondition (monster.cpp:1576).
func (m *Monster) OnEndCondition(t combat.ConditionType) {
	m.OnConditionStatusChange(t)
}

// AddCondition and RemoveCondition are the Monster overrides that fire those
// two hooks, the way Creature::addCondition and removeCondition call the
// virtual onAddCondition / onEndCondition (creature.cpp:1390, :1431).
//
// Go has no virtual dispatch, so the override has to be spelled out here.
// Without it a monster kept its idle state across being poisoned or hasted —
// which is what onConditionStatusChange exists to correct.
func (m *Monster) AddCondition(c combat.Condition) {
	if c == nil {
		return
	}
	m.BaseCreature.AddCondition(c)
	m.OnAddCondition(c.GetType())
}

func (m *Monster) RemoveCondition(t combat.ConditionType) {
	m.BaseCreature.RemoveCondition(t)
	m.OnEndCondition(t)
}

// OnConditionStatusChange is Monster::onConditionStatusChange (monster.cpp:1572).
func (m *Monster) OnConditionStatusChange(combat.ConditionType) { m.UpdateIdleStatus() }

// ChangeHealth is Monster::changeHealth (monster.cpp:3458). Two things happen
// besides the health change: an ambient sound rolls, and the monster is taken
// out of idle unconditionally.
//
// The un-idling is not incidental. A player with the ignore-by-monsters flag is
// not in any target list, so a monster it attacks would otherwise stay idle and
// never fight back.
func (m *Monster) ChangeHealth(w *World, healthChange int32) {
	if m.Type != nil && len(m.Type.Sounds) > 0 && m.Type.SoundChance >= rand.Intn(100)+1 {
		if w != nil && w.OnSoundEffect != nil {
			w.OnSoundEffect(m.GetPosition(), m.Type.Sounds[rand.Intn(len(m.Type.Sounds))])
		}
	}
	m.SetIdle(false)
	m.AddHealth(healthChange)
}

// DrainHealth is Monster::drainHealth (monster.cpp:3441).
//
// The field-damage clause is the interesting half. A monster that is taking
// damage and either wandering or locked onto something it cannot path to is
// allowed to walk through harmful fields. Without it, a melee monster behind a
// magic wall stands in a fire bomb it could have stepped out of.
func (m *Monster) DrainHealth(w *World, attacker Creature, damage int32) {
	if damage > 0 && (m.randomStepping || m.GetTarget() != nil) {
		m.ignoreFieldDamage = true
	}
	m.ChangeHealth(w, -damage)
}

// ResolveKiller is the player-resolution at the top of Monster::death
// (monster.cpp:3231-3251): the last hitter, or the player behind it if a summon
// landed the blow, falling back to whatever the monster was attacking.
//
// The fallback matters. A monster killed by a field or by its own reflect has no
// last hitter, and without it the kill credits nobody — no charm, no weapon
// proficiency, no bestiary progress.
func (m *Monster) ResolveKiller(lastHitCreature Creature) *Player {
	if p := playerBehind(lastHitCreature); p != nil {
		return p
	}
	return playerBehind(m.GetTarget())
}

// Death is Monster::death (monster.cpp:3226): the state teardown a monster does
// for itself when it dies, distinct from the corpse and loot the combat engine
// produces.
//
// Killing the summons is upstream behaviour and not a convenience — a summon
// outliving its master is a permanently orphaned monster with no spawn to
// return to.
//
// Two things upstream does here are deliberately NOT repeated:
//
//   - The Carnage charm splash. CombatEngine.applyCarnageOnDeath already runs
//     it from handleDeath; doing it here as well would apply it twice.
//   - weaponProficiency().applyOn(KILL) and the bosstiary/bestiary weapon
//     experience. WeaponProficiency exists here, but only its stats and
//     augments half — it has no experience model to add to yet, so there is
//     nothing to call.
func (m *Monster) Death(w *World, lastHitCreature Creature) {
	for _, summon := range m.Summons {
		if summon == nil {
			continue
		}
		summon.SetHealth(0)
		summon.Master = nil
		if w != nil && w.OnCreatureDied != nil {
			w.OnCreatureDied(summon)
		}
	}
	m.Summons = nil

	m.ClearTargetList()
	m.ClearFriendList()
	m.SetTarget(nil)
	m.Idle = true

	// The kill credit, resolved the way upstream does before anything is awarded.
	killer := m.ResolveKiller(lastHitCreature)
	if killer == nil || m.Type == nil {
		return
	}
	m.awardWeaponProficiency(w, killer)
}

// awardWeaponProficiency is the tail of Monster::death (monster.cpp:3279-3300):
// the life and mana a proficiency returns on a kill, then the weapon experience
// the monster was worth.
//
// Bosstiary and bestiary experience are separate awards and both apply — a boss
// that is also in the bestiary pays twice, which is upstream.
func (m *Monster) awardWeaponProficiency(w *World, killer *Player) {
	wp := killer.WeaponProficiency
	if wp == nil {
		return
	}
	wp.ApplyOn(killer, WeaponProfLife, WeaponProfOnKill)
	wp.ApplyOn(killer, WeaponProfMana, WeaponProfOnKill)

	var weaponID uint16
	if w != nil {
		if weapon := killer.GetWeapon(w.Items, true); weapon != nil {
			weaponID = weapon.ID
		}
	}
	if weaponID == 0 {
		return
	}
	if exp := wp.GetBosstiaryExperience(m.Type.BosstiaryRace); exp > 0 && m.Type.BosstiaryRaceID != 0 {
		wp.AddExperience(exp, weaponID)
	}
	if exp := wp.GetBestiaryExperience(m.Type.BestiaryStars); exp > 0 {
		wp.AddExperience(exp, weaponID)
	}
}

// GetCorpse is Monster::getCorpse (monster.cpp:3304): stamp the corpse with its
// owner so nobody else can loot it during the protection window.
//
// The owner is whoever dealt the most damage, or that creature's master when a
// summon landed the damage — otherwise a player whose summon did the work would
// be locked out of the corpse.
func (m *Monster) GetCorpse(corpse *Item, mostDamageCreature Creature) *Item {
	if corpse == nil || mostDamageCreature == nil {
		return corpse
	}
	owner := playerBehind(mostDamageCreature)
	if owner == nil {
		return corpse
	}
	if corpse.Attr == nil {
		corpse.Attr = &ItemAttributes{}
	}
	id := owner.GetID()
	corpse.Attr.Owner = &id
	return corpse
}

// DropLoot is Monster::dropLoot (monster.cpp:3414). The core drops nothing by
// itself — the loot table is rolled by the monsterOnDropLoot event — except the
// forge sliver, which only a fiendish monster gives.
func (m *Monster) DropLoot(w *World, corpse *Item) {
	if corpse == nil || !m.CanDropLoot() {
		return
	}
	if m.ForgeClassification != ForgeClassifications_Fiendish {
		return
	}
	count := forgeMinSlivers + rand.Intn(forgeMaxSlivers-forgeMinSlivers+1)
	sliver := &Item{ID: itemForgeSliver, Count: uint16(count)}
	corpse.Contents = append(corpse.Contents, sliver)
}

// SetSoulPitStack is Monster::setSoulPitStack (monster.cpp:3200). A stack of 40
// marks the boss of the pit; everything else is a wave monster. Neither drops
// loot, and only the boss grants skill progress.
func (m *Monster) SetSoulPitStack(stack uint8, isSummon bool) {
	isBoss := stack == 40
	m.ForgeStack = uint16(stack)
	m.SoulPit = true
	m.SoulPitBoss = isBoss
	m.SkillLoss = isBoss && !isSummon
	if m.Type != nil {
		m.Type.Flags.LootDrop = false
	}
}

// forge sliver drop range (FORGE_MIN_SLIVERS / FORGE_MAX_SLIVERS in config.lua)
// and the sliver's item id.
const (
	forgeMinSlivers = 3
	forgeMaxSlivers = 5
	itemForgeSliver = 37109
)
