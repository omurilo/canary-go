package game

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/game/combat"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// Session is implemented by the game protocol connection so the world can push
// updates to a player's client.
type Session interface {
	SendToClient(w *netmsg.Writer)
	Player() *Player
}

// Skill indexes match the client skill order.
type Skill int

const (
	SkillFist Skill = iota
	SkillClub
	SkillSword
	SkillAxe
	SkillDistance
	SkillShielding
	SkillFishing
	SkillCount
)

// Player is a logged-in character. It embeds creature-like fields directly to
// keep the model flat for now.
type Player struct {
	conditionStore

	ID        uint32 // creature id (assigned at spawn)
	DBID      uint32 // players.id
	AccountID uint32
	Name      string

	Pos       Position
	Direction Direction

	Level      uint16
	Experience uint64
	Health     uint32
	MaxHealth  uint32
	Mana       uint32
	MaxMana    uint32
	Soul       uint8
	Capacity   uint32 // free capacity (in the client unit)
	Speed      uint16
	Vocation   uint16
	Sex        uint8

	MagLevel uint16
	Skills   [SkillCount]uint16

	Outfit Outfit

	LightLevel uint8
	LightColor uint8

	// Inventory holds equipment slots 1..10 (CONST_SLOT_HEAD..CONST_SLOT_AMMO);
	// index 0 is unused. Persistence of these is a later milestone.
	Inventory [11]*Item

	TargetID uint32

	// cooldowns tracks per-spell and per-group spell cooldowns, mirroring the
	// CONDITION_SPELLCOOLDOWN / CONDITION_SPELLGROUPCOOLDOWN conditions applied
	// by Spell::applyCooldownConditions (src/creatures/combat/spells.cpp:795).
	cooldowns *combat.CooldownManager

	// learnedSpells records the instant spells the player may cast, mirroring
	// Player::learnedInstantSpellList (src/creatures/players/player.hpp).
	learnedSpells map[string]bool

	Session Session
}

// Cooldowns returns the player's spell cooldown manager, creating it on first
// use.
func (p *Player) Cooldowns() *combat.CooldownManager {
	if p.cooldowns == nil {
		p.cooldowns = combat.NewCooldownManager()
	}
	return p.cooldowns
}

// HasLearnedSpell mirrors Player::hasLearnedInstantSpell
// (src/creatures/players/player.cpp). Spell names are compared case-insensitively.
// TODO(spells): persist learned spells and grant them on level-up / by vocation;
// this store is currently only populated via LearnSpell (e.g. GM commands/tests).
func (p *Player) HasLearnedSpell(name string) bool {
	if p.learnedSpells == nil {
		return false
	}
	return p.learnedSpells[strings.ToLower(name)]
}

// LearnSpell records that the player has learned the named spell.
func (p *Player) LearnSpell(name string) {
	if p.learnedSpells == nil {
		p.learnedSpells = make(map[string]bool)
	}
	p.learnedSpells[strings.ToLower(name)] = true
}

// GamemasterOutfit sets a default outfit if none was loaded.
func (p *Player) ensureDefaults() {
	if p.Outfit.LookType == 0 {
		p.Outfit.LookType = 128 // default male citizen
	}
	if p.MaxHealth == 0 {
		p.MaxHealth, p.Health = 150, 150
	}
	if p.Speed == 0 {
		p.Speed = 220
	}
	if p.Level == 0 {
		p.Level = 1
	}
	if p.Capacity == 0 {
		p.Capacity = 400
	}
	for i := range p.Skills {
		if p.Skills[i] == 0 {
			p.Skills[i] = 10
		}
	}
}

func (p *Player) GetID() uint32 { return p.ID }
func (p *Player) GetName() string { return p.Name }
func (p *Player) GetHealth() uint32 { return p.Health }
func (p *Player) SetHealth(health uint32) {
	p.Health = health
	if p.Health > p.MaxHealth {
		p.Health = p.MaxHealth
	}
}
func (p *Player) GetMaxHealth() uint32 { return p.MaxHealth }
func (p *Player) AddHealth(amount int32) {
	if amount > 0 {
		p.Health += uint32(amount)
		if p.Health > p.MaxHealth {
			p.Health = p.MaxHealth
		}
	} else {
		sub := uint32(-amount)
		if sub > p.Health {
			p.Health = 0
		} else {
			p.Health -= sub
		}
	}
}
// ExpForLevel returns the total experience required to reach a level, mirroring
// Player::getExpForLevel (src/creatures/players/player.cpp:4438):
// (((level-6)*level + 17)*level - 12) / 6 * 100.
func ExpForLevel(level uint64) uint64 {
	return (((level-6)*level+17)*level - 12) / 6 * 100
}

// AddExperience grants raw experience and applies any resulting level-ups,
// mirroring the core of Player::addExperience (src/creatures/players/player.cpp:3560).
// Basic only: no party sharing, stamina, VIP or bonus multipliers (left as
// TODOs for the vocations/party agents). On level-up, health/mana are refilled
// to max like the C++ path.
func (p *Player) AddExperience(exp uint64) {
	if exp == 0 {
		return
	}
	if p.Level == 0 {
		p.Level = 1
	}
	nextLevelExp := ExpForLevel(uint64(p.Level) + 1)
	currLevelExp := ExpForLevel(uint64(p.Level))
	if currLevelExp >= nextLevelExp {
		return // already at max level
	}

	p.Experience += exp

	prevLevel := p.Level
	for p.Experience >= nextLevelExp {
		p.Level++
		currLevelExp = nextLevelExp
		nextLevelExp = ExpForLevel(uint64(p.Level) + 1)
		if currLevelExp >= nextLevelExp {
			break // reached max level
		}
	}

	if p.Level != prevLevel {
		// TODO(vocations): apply per-vocation HP/mana/cap gains. Without a
		// vocation registry we just refill to the current max, matching the C++
		// "health = healthMax" refill after a level change.
		p.Health = p.MaxHealth
		p.Mana = p.MaxMana
	}
}

func (p *Player) GetTarget() Creature { return nil } // Stub for target
func (p *Player) SetTarget(target Creature) {}
func (p *Player) SetAttackTarget(id uint32) { p.TargetID = id }
func (p *Player) ChangeTargetDistance(distance int32) {}
func (p *Player) GetPosition() Position { return p.Pos }
func (p *Player) SetPosition(pos Position) { p.Pos = pos }
func (p *Player) GetDirection() Direction { return p.Direction }
func (p *Player) SetDirection(dir Direction) { p.Direction = dir }
func (p *Player) GetOutfit() Outfit { return p.Outfit }
func (p *Player) GetLightLevel() uint8 { return p.LightLevel }
func (p *Player) GetLightColor() uint8 { return p.LightColor }
func (p *Player) GetSpeed() uint16 { return p.Speed }
func (p *Player) GetCreatureType() uint8 { return 0 } // CREATURETYPE_PLAYER

// GetMana/GetMaxMana/AddMana expose the player's mana pool for the combat
// adapter. AddMana clamps like Creature::changeMana (src/creatures/creature.cpp).
func (p *Player) GetMana() uint32    { return p.Mana }
func (p *Player) GetMaxMana() uint32 { return p.MaxMana }
func (p *Player) AddMana(amount int32) {
	if amount > 0 {
		p.Mana += uint32(amount)
		if p.Mana > p.MaxMana {
			p.Mana = p.MaxMana
		}
	} else {
		sub := uint32(-amount)
		if sub > p.Mana {
			p.Mana = 0
		} else {
			p.Mana -= sub
		}
	}
}
