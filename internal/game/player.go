package game

import (
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game/combat"
	"github.com/opentibiabr/canary-go/internal/game/vocations"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// Session is implemented by the game protocol connection so the world and the
// Lua engine can push updates to a player's client after model mutations.
type Session interface {
	SendToClient(w *netmsg.Writer)
	Player() *Player

	// Inventory / stats refresh (Phase 1).
	SendInventoryItem(slot uint8, it *Item) // 0x78
	SendInventoryEmpty(slot uint8)          // 0x79
	SendInventoryIds()                      // 0xF5 aggregated id/tier/count list
	SendStats()                             // 0xA0 player stats

	// Container windows (Phase 2). OpenContainer allocates/reuses a client cid
	// and pushes 0x6E; RefreshContainer re-sends 0x6E for every open window
	// showing c; CloseContainer unregisters and pushes 0x6F.
	OpenContainer(c *Item)
	RefreshContainer(c *Item)
	CloseClientContainer(cid uint8)

	// Shop (Phase 4).
	SendCloseShop() // 0x7C

	// Conditions / Icons
	SendChangeSpeed(c Creature) // 0x8F
	SendIcons()                 // 0xA2
}

// Equipment slot indices (CONST_SLOT_*). Slot 0 is "wherever" (auto-place).
const (
	ConstSlotWhereever = 0
	ConstSlotHead      = 1
	ConstSlotNecklace  = 2
	ConstSlotBackpack  = 3
	ConstSlotArmor     = 4
	ConstSlotRight     = 5
	ConstSlotLeft      = 6
	ConstSlotLegs      = 7
	ConstSlotFeet      = 8
	ConstSlotRing      = 9
	ConstSlotAmmo      = 10
	ConstSlotFirst     = ConstSlotHead
	ConstSlotLast      = ConstSlotAmmo
)

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
	AccountType uint8
	GroupID   uint16 // players.group_id — staff groups 4/5/6 cannot be attacked
	Ghost     bool   // ghost mode (invisible; not targetable by monsters)
	Name      string

	Pos       Position
	Direction Direction

	Level      uint16
	Experience uint64
	BankBalance uint64 // players.balance — bank money
	Health     uint32
	MaxHealth  uint32
	Mana       uint32
	MaxMana    uint32
	Soul       uint8
	// Capacity is the player's TOTAL base capacity (players.cap column), in the
	// client unit (hundredths of an oz). Free capacity = Capacity + BonusCapacity
	// - InventoryWeight (see GetCapacity/GetFreeCapacity).
	Capacity        uint32
	BonusCapacity   uint32 // additive bonus (equipment/wheel/varStats — stubbed 0)
	InventoryWeight uint32 // cached total weight of all carried items
	Speed      uint16
	Vocation   uint16
	Sex        uint8

	MagLevel uint16
	Skills   [SkillCount]uint16

	// SpeedBonus is the temporary speed delta from conditions (haste/paralyze).
	// Added on top of the level-scaled base speed (see GetBaseSpeed).
	SpeedBonus int32

	// RegenTicks is the remaining food/regeneration time in milliseconds
	// (CONDITION_REGENERATION). Eating food adds to it; the regen ticker drains
	// it while healing HP/mana. The "You are full" cap is enforced in the food
	// script from this value.
	RegenTicks int32

	Outfit Outfit

	LightLevel uint8
	LightColor uint8

	// Inventory holds equipment slots 1..10 (CONST_SLOT_HEAD..CONST_SLOT_AMMO);
	// index 0 is unused. Slot 11 (store inbox, CONST_SLOT_LAST) is intentionally
	// omitted. Persistence of these is a later milestone.
	Inventory [11]*Item

	// openContainers mirrors C++ Player::openContainers: the client container
	// windows currently open, keyed by client container id (0..15). Index is the
	// pagination scroll offset. This is the single source of truth the protocol
	// layer reads/writes via the Session.
	openContainers map[uint8]OpenContainer

	// Death / respawn state (Phase 5). LoginPosition is the temple the player
	// returns to on death; TownID selects it. SkillLoss gates the exp/skill
	// penalty. Blessings/SkillTries/ManaSpent feed the penalty math.
	TownID          uint16
	LoginPosition   Position
	Dead            bool
	SkillLoss       bool
	Skull           uint8
	Blessings       [8]uint8
	SkillTries      [SkillCount]uint64
	ManaSpent       uint64
	MagLevelPercent uint8
	LevelPercent    uint8

	// Party is the party this player belongs to (as leader or member), nil when
	// ungrouped. partyInvitations are parties that have invited this player but
	// which they have not yet joined.
	Party            *Party
	partyInvitations []*Party

	TargetID uint32
	target   Creature
	ShopOwnerID uint32 // ID of the NPC currently being traded with


	// cooldowns tracks per-spell and per-group spell cooldowns, mirroring the
	// CONDITION_SPELLCOOLDOWN / CONDITION_SPELLGROUPCOOLDOWN conditions applied
	// by Spell::applyCooldownConditions (src/creatures/combat/spells.cpp:795).
	cooldowns *combat.CooldownManager

	// learnedSpells records the instant spells the player may cast, mirroring
	// Player::learnedInstantSpellList (src/creatures/players/player.hpp).
	learnedSpells map[string]bool

	// Storages holds the player's key/value action storages (quest progress,
	// cooldowns, NPC state). Persistence is a later milestone; for now they live
	// for the session. Absent keys read back as -1 (see GetStorageValue).
	Storages map[uint32]int32

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
func (p *Player) GamemasterOutfit() {
	if p.Outfit.LookType == 0 {
		p.Outfit.LookType = 75 // GM outfit
	}
}

// SendTextMessage sends a text message to the player's client.
func (p *Player) SendTextMessage(msgType uint8, text string) {
	if p.Session != nil {
		w := netmsg.NewWriter()
		w.AddByte(0xB4) // opTextMessage
		w.AddByte(msgType)
		w.AddString(text)
		p.Session.SendToClient(w)
	}
}

// SendOpenShop sends the shop window (opcode 0x7A) to the player's client.
func (p *Player) SendOpenShop(npc Creature, items []creatures.ShopItem) {
	if p.Session != nil {
		w := netmsg.NewWriter()
		w.AddByte(0x7A) // opOpenShop
		w.AddString(npc.GetName())
		// Modern (13.x) layout: currency item id + currency name precede the
		// item count (ProtocolGame::sendShop). Omitting them desyncs the client's
		// parser. Default to gold coin (client id 3031).
		w.AddU16(3031)
		w.AddString("")
		w.AddU16(uint16(len(items)))
		for _, item := range items {
			w.AddU16(item.ID)
			w.AddByte(item.SubType)
			w.AddString(item.Name)
			w.AddU32(0) // weight (0 for now)
			w.AddU32(item.BuyPrice)
			w.AddU32(item.SellPrice)
		}
		p.Session.SendToClient(w)

		// Resource balances so the shop shows the player's funds on open
		// (0xEE: RESOURCE_BANK=0, RESOURCE_INVENTORY_MONEY=1).
		bank := netmsg.NewWriter()
		bank.AddByte(0xEE)
		bank.AddByte(0x00)
		bank.AddU64(p.BankBalance)
		p.Session.SendToClient(bank)

		inv := netmsg.NewWriter()
		inv.AddByte(0xEE)
		inv.AddByte(0x01)
		inv.AddU64(p.GetMoney())
		p.Session.SendToClient(inv)
	}
}

// CloseShop clears the player's active shop binding. Mirrors
// Player::closeShopWindow (the NPC-side onCloseChannel is fired by the caller).
func (p *Player) CloseShop() {
	p.ShopOwnerID = 0
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

// GetStorageValue returns the player's value for a storage key, or -1 when the
// key was never set (mirroring Player::getStorageValue). Quest/NPC scripts rely
// on the -1 sentinel to detect "not started".
func (p *Player) GetStorageValue(key uint32) int32 {
	if p.Storages == nil {
		return -1
	}
	if v, ok := p.Storages[key]; ok {
		return v
	}
	return -1
}

// SetStorageValue sets (or, with value -1, clears) a storage key.
func (p *Player) SetStorageValue(key uint32, value int32) {
	if value == -1 {
		delete(p.Storages, key)
		return
	}
	if p.Storages == nil {
		p.Storages = make(map[uint32]int32)
	}
	p.Storages[key] = value
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

func (p *Player) GetTarget() Creature { return p.target }
func (p *Player) SetTarget(target Creature) {
	p.target = target
	if target != nil {
		p.TargetID = target.GetID()
	} else {
		p.TargetID = 0
	}
}
func (p *Player) SetAttackTarget(id uint32) { p.TargetID = id }
func (p *Player) ChangeTargetDistance(distance int32) {}
func (p *Player) GetPosition() Position { return p.Pos }
func (p *Player) SetPosition(pos Position) { p.Pos = pos }
func (p *Player) GetDirection() Direction { return p.Direction }
func (p *Player) SetDirection(dir Direction) { p.Direction = dir }
func (p *Player) GetOutfit() Outfit { return p.Outfit }
func (p *Player) GetLightLevel() uint8 { return p.LightLevel }
func (p *Player) GetLightColor() uint8 { return p.LightColor }
// GetBaseSpeed returns the level-scaled base speed, mirroring
// Player::updateBaseSpeed: vocation base speed + (level - 1). The vocation base
// speed defaults to 110 (Canary's "None" vocation) when the registry is empty.
func (p *Player) GetBaseSpeed() uint16 {
	base := 110
	if voc := vocations.GetVocation(uint32(p.Vocation)); voc != nil && voc.BaseSpeed > 0 {
		base = voc.BaseSpeed
	}
	lvl := int(p.Level)
	if lvl < 1 {
		lvl = 1
	}
	speed := base + (lvl - 1)
	if speed < 0 {
		speed = 0
	}
	if speed > 0xFFFF {
		speed = 0xFFFF
	}
	return uint16(speed)
}

// GetSpeed returns the player's current movement speed: the level-scaled base
// plus temporary bonuses (haste). This is what the client and the step-duration
// pacing use.
func (p *Player) GetSpeed() uint16 {
	speed := int(p.GetBaseSpeed()) + int(p.SpeedBonus)
	if speed < 0 {
		speed = 0
	}
	if speed > 0xFFFF {
		speed = 0xFFFF
	}
	return uint16(speed)
}

func (p *Player) ChangeSpeed(delta int32) {
	p.SpeedBonus += delta
}

func (p *Player) GetCreatureType() uint8 { return 0 } // CREATURETYPE_PLAYER

func (p *Player) GetIcons() uint64 {
	var icons uint64
	for _, cond := range p.Conditions() {
		icons |= cond.GetIcons()
	}
	return icons
}

func (p *Player) AddCondition(c combat.Condition) {
	p.conditionStore.AddCondition(c)
	if p.Session != nil {
		p.Session.SendIcons()
	}
}

func (p *Player) RemoveCondition(t combat.ConditionType) {
	p.conditionStore.RemoveCondition(t)
	if p.Session != nil {
		p.Session.SendIcons()
	}
}

func (p *Player) ClearConditions() {
	p.conditionStore.ClearConditions()
	if p.Session != nil {
		p.Session.SendIcons()
	}
}
func (p *Player) AttackSpeed() time.Duration {
	voc := vocations.GetVocation(uint32(p.Vocation))
	if voc != nil && voc.AttackSpeed > 0 {
		return time.Duration(voc.AttackSpeed) * time.Millisecond
	}
	return 2000 * time.Millisecond
}

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

// GetTotalWeight calculates the total weight of all items in the player's inventory
func (p *Player) GetTotalWeight(w *World) uint32 {
	total := uint32(0)
	for _, item := range p.Inventory {
		if item != nil {
			total += item.GetWeight(w.Items)
		}
	}
	return total
}
