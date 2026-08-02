package game

import (
	"time"

	"github.com/omurilo/canary-go/internal/creatures"
)

type ForgeClassification byte

const (
	ForgeClassifications_None       ForgeClassification = 0
	ForgeClassifications_Influenced ForgeClassification = 1
	ForgeClassifications_Fiendish   ForgeClassification = 2
)

type Monster struct {
	BaseCreature
	TargetDistance int32
	Master         Creature
	// CorpseID is the item id dropped on death. 0 means "unknown" and the
	// combat engine falls back to a default. Populated from MonsterType.Corpse.
	CorpseID uint16

	// SpawnPosition is where this monster was spawned (for IsInSpawnRange).
	SpawnPosition Position

	// Friends and Targets track relationships for AI targeting.
	Friends map[uint32]Creature
	Targets map[uint32]Creature

	// Idle indicates this monster has been idled (no AI processing).
	Idle bool

	// Summons this monster has out, capped by MonsterType.MaxSummons.
	Summons []*Monster

	// Per-timer tick accumulators for the Monster::onThink pipeline
	// (monster.cpp:2140-2310). Each counter advances by the think interval and
	// fires independently; they live on the monster because upstream keeps them
	// there and they must survive a tick where the monster is skipped.
	yellTicks            int
	defenseTicks         int
	targetChangeTicks    int
	targetChangeCooldown int
	// challengeFocusDuration pins the monster on a challenging player, blocking
	// onThinkTarget from re-rolling until it expires.
	challengeFocusDuration int
	// walkingBack is set when the monster is outside its spawn and heading home.
	walkingBack bool
	// stepDuration counts up while the target is adjacent and down while it is
	// not, saturating at 2. Monster::isTargetNearby reads it; getDistanceStep is
	// the only writer.
	stepDuration int
	// randomStepping distinguishes a monster wandering from one following, so a
	// wandering monster is not treated as having a path.
	randomStepping bool
	// walkTicks and pendingStepCost pace the monster's steps. Upstream schedules
	// one walk event per creature at exactly getStepDuration; here the walk sweep
	// accumulates on the server beat and steps when the clock is due. The cost is
	// charged AFTER the direction is chosen, because a diagonal costs three times
	// a straight step.
	walkTicks       uint32
	pendingStepCost uint32
	// ignoreFieldDamage is set while a monster is walking a path that crosses a
	// magic field, and cleared the moment the path runs out. It is what lets a
	// monster follow a player through its own fire wall.
	ignoreFieldDamage bool
	// challengeFocusDuration's melee sibling: while it runs, targetDistance is
	// forced to melee range by a challenge, and getIcons shows the icon.
	challengeMeleeDuration int
	// soundTicks is onThinkSound.s counter, the audio sibling of yellTicks.
	soundTicks int
	// lastMoveMs backs getTimeSinceLastMove for the wander gate.
	lastMoveMs int64
	// attackTicks is the single counter every attack block is gated against;
	// lastMeleeAttack is the separate 1500ms floor between melee swings, and
	// extraMeleeAttack a one-shot flag that bypasses both.
	attackTicks      int
	lastMeleeAttack  int64
	extraMeleeAttack bool
	// minCombatValue/maxCombatValue hold the damage range of the block currently
	// being cast, which is what Monster::getCombatValues reports.
	minCombatValue int32
	maxCombatValue int32
	// SoulPitBoss and SkillLoss are set by setSoulPitStack.
	SoulPitBoss bool
	SkillLoss   bool

	// Type is the shared, immutable monster definition (attacks, loot,
	// experience, flags). May be nil for synthetic/test monsters.
	Type *creatures.MonsterType

	ForgeClassification  ForgeClassification
	ForgeStack           uint16
	TimeToChangeFiendish int64

	// spellCooldowns tracks per-spell cooldowns keyed by spell name. Each
	// MonsterSpell converted from Type.Attacks is tracked independently so the
	// AI engine can respect per-attack intervals.
	spellCooldowns map[string]int64

	// Defense is the monster's base defense value (armor/damage reduction).
	Defense int32

	// AttackSpells holds runtime-added attack spells registered via Lua.
	AttackSpells []MonsterSpell

	// DefenseSpells holds runtime-added defense spells registered via Lua.
	DefenseSpells []MonsterSpell

	// RespawnType indicates the monster's respawn category (normal, raid, script).
	RespawnType int32

	// HazardPoints is the hazard system tier value.
	HazardPoints int32

	// CritChance is the critical hit chance (0-100).
	CritChance uint8

	// CritDamage is the critical hit damage multiplier percentage.
	CritDamage uint8

	// ReflectElements maps combat type to reflection percentage.
	ReflectElements map[uint32]int16

	// HazardCrit indicates the monster has hazard system crit enabled.
	HazardCrit bool

	// HazardDodge indicates the monster has hazard system dodge enabled.
	HazardDodge bool

	// HazardDamageBoost indicates the monster has hazard system damage boost enabled.
	HazardDamageBoost bool

	// HazardDefenseBoost indicates the monster has hazard system defense boost enabled.
	HazardDefenseBoost bool

	// SoulPit indicates the monster is affected by the soul pit system.
	SoulPit bool
}

func NewMonster(id uint32, name string, mType *creatures.MonsterType) *Monster {
	maxHealth := uint32(100)
	speed := uint32(200)
	outfit := Outfit{}
	corpse := uint16(0)

	if mType != nil {
		maxHealth = mType.MaxHealth
		speed = mType.Speed
		corpse = mType.Corpse
		outfit = Outfit{
			LookType:  mType.Outfit.LookType,
			Head:      mType.Outfit.Head,
			Body:      mType.Outfit.Body,
			Legs:      mType.Outfit.Legs,
			Feet:      mType.Outfit.Feet,
			Addons:    mType.Outfit.Addons,
			LookMount: mType.Outfit.LookMount,
		}
	}

	m := &Monster{
		BaseCreature: BaseCreature{
			ID:        id,
			Name:      name,
			Health:    maxHealth,
			MaxHealth: maxHealth,
			Speed:     uint16(speed),
			Outfit:    outfit,
		},
		CorpseID:       corpse,
		Type:           mType,
		spellCooldowns: make(map[string]int64),
	}
	// Monster::Monster seeds internalLight from the type (monster.cpp:100), and
	// setNormalCreatureLight restores that same value when a light condition
	// ends. Neither happened, so a demon lit nothing around it.
	m.SetNormalCreatureLight()
	return m
}

func (m *Monster) GetCreatureType() uint8 { return 1 } // CREATURETYPE_MONSTER

// MeleeAttack returns the monster's basic melee attack block, or nil if it has
// none. Mirrors selecting the name=="melee" spellBlock in Monster::doAttacking
// (src/creatures/monsters/monster.cpp:1753).
func (m *Monster) MeleeAttack() *creatures.MonsterAttack {
	if m.Type == nil {
		return nil
	}
	for i := range m.Type.Attacks {
		if m.Type.Attacks[i].IsMelee() {
			return &m.Type.Attacks[i]
		}
	}
	return nil
}

// AttackInterval is the cadence of the monster's melee attack (ms). Falls back
// to the MonsterType default of 2000ms (src/creatures/monsters/monsters.hpp).
func (m *Monster) AttackInterval() time.Duration {
	if atk := m.MeleeAttack(); atk != nil && atk.Interval > 0 {
		return time.Duration(atk.Interval) * time.Millisecond
	}
	return defaultMonsterAttackSpeed
}

// Experience is the exp awarded to the killer.
func (m *Monster) Experience() uint64 {
	if m.Type == nil {
		return 0
	}
	return m.Type.Experience
}

// CanBeForgeMonster checks if the monster is eligible to become influenced or fiendish.
func (m *Monster) CanBeForgeMonster() bool {
	return m.ForgeStack == 0 && m.Type != nil && m.Type.RaceID > 0
}

// ApplyStacks scales health based on forge stack level.
func (m *Monster) ApplyStacks() {
	if m.ForgeStack == 0 {
		return
	}
	percent := 1.0 + float64(15*m.ForgeStack+35)/100.0
	newMax := uint32(float64(m.MaxHealth) * percent)
	if newMax == 0 {
		newMax = 100
	}
	m.MaxHealth = newMax
	m.Health = newMax
}

// ConfigureForgeSystem sets up stacks and HP scaling for Influenced or Fiendish monsters.
func (m *Monster) ConfigureForgeSystem(stack uint16) {
	if m.ForgeClassification == ForgeClassifications_Fiendish {
		m.ForgeStack = 15
	} else if m.ForgeClassification == ForgeClassifications_Influenced {
		if stack == 0 {
			stack = uint16(1 + (time.Now().UnixNano() % 5))
		}
		m.ForgeStack = stack
	}
	m.ApplyStacks()
}

// ClearFiendishStatus resets fiendish classification and stacks.
func (m *Monster) ClearFiendishStatus() {
	m.ForgeClassification = ForgeClassifications_None
	m.ForgeStack = 0
	m.TimeToChangeFiendish = 0
}

// ---------------------------------------------------------------------------
// Monster spell helpers (for AI engine)
// ---------------------------------------------------------------------------

// Spells returns the monster's non-melee attacks as MonsterSpell objects that
// the AI engine can attempt to cast.
func (m *Monster) Spells() []MonsterSpell {
	if m.Type == nil {
		return nil
	}
	out := make([]MonsterSpell, 0, len(m.Type.Attacks))
	for i := range m.Type.Attacks {
		atk := m.Type.Attacks[i]
		if atk.IsMelee() {
			continue
		}
		spell := MonsterSpellFromAttack(atk)
		// Restore per-spell cooldown from the monster's local tracker.
		if lastUsed, ok := m.spellCooldowns[atk.Name]; ok {
			spell.lastUsedMs = lastUsed
		}
		out = append(out, spell)
	}
	return out
}

// HasSpells returns true if the monster has at least one non-melee attack
// configured.
func (m *Monster) HasSpells() bool {
	if m.Type == nil {
		return false
	}
	for i := range m.Type.Attacks {
		if !m.Type.Attacks[i].IsMelee() {
			return true
		}
	}
	return false
}

// MaxSpellRange returns the maximum range among all of the monster's
// non-melee attacks. Returns 1 if no spells are configured (so the monster
// defaults to adjacent melee range).
func (m *Monster) MaxSpellRange() int {
	if m.Type == nil {
		return 1
	}
	maxR := 1
	for i := range m.Type.Attacks {
		atk := &m.Type.Attacks[i]
		if atk.IsMelee() {
			continue
		}
		if atk.Range > maxR {
			maxR = atk.Range
		}
	}
	return maxR
}

// MarkSpellUsed records the current time for the given spell name in the
// monster's local cooldown tracker. Called by the AI engine after a
// successful cast.
func (m *Monster) MarkSpellUsed(name string) {
	if m.spellCooldowns == nil {
		m.spellCooldowns = make(map[string]int64)
	}
	m.spellCooldowns[name] = time.Now().UnixMilli()
}

// ResetSpellCooldowns clears all per-spell cooldown entries.
func (m *Monster) ResetSpellCooldowns() {
	m.spellCooldowns = make(map[string]int64)
}

// HealingSpells returns spells that have the IsHealing flag set.
func (m *Monster) HealingSpells() []MonsterSpell {
	all := m.Spells()
	if len(all) == 0 {
		return nil
	}
	out := make([]MonsterSpell, 0, len(all))
	for _, s := range all {
		if s.IsHealing {
			out = append(out, s)
		}
	}
	return out
}
