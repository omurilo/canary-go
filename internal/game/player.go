package game

import (
	"fmt"
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/bestiary"
	"github.com/opentibiabr/canary-go/internal/bosstiary"
	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game/combat"
	"github.com/opentibiabr/canary-go/internal/game/vocations"
	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// Session is implemented by the game protocol connection so the world and the
// Lua engine can push updates to a player's client after model mutations.
type Session interface {
	SendToClient(w *netmsg.Writer)
	Player() *Player
	Disconnect()

	// Inventory / stats refresh (Phase 1).
	SendInventoryItem(slot uint8, it *Item) // 0x78
	SendInventoryEmpty(slot uint8)          // 0x79
	SendInventoryIds()                      // 0xF5 aggregated id/tier/count list
	SendStats()                             // 0xA0 player stats
	SendSkills()                            // 0xA1 player skills

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

	// Exaltation Forge. Opens the forge window (0x87 + 0x86) for the player.
	SendOpenForge()
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

	ID            uint32 // creature id (assigned at spawn)
	DBID          uint32 // players.id
	AccountID     uint32
	AccountType   uint8
	GroupID       uint16 // players.group_id — staff groups 4/5/6 cannot be attacked
	Ghost         bool   // ghost mode (invisible; not targetable by monsters)
	Name          string
	GuildName     string
	GuildRankName string
	GuildNick     string

	Pos       Position
	Direction Direction

	Level       uint16
	Experience  uint64
	BankBalance uint64 // players.balance — bank money
	Health      uint32
	MaxHealth   uint32
	Mana        uint32
	MaxMana     uint32
	Soul        uint8
	// Capacity is the player's TOTAL base capacity (players.cap column), in the
	// client unit (hundredths of an oz). Free capacity = Capacity + BonusCapacity
	// - InventoryWeight (see GetCapacity/GetFreeCapacity).
	Capacity        uint32
	BonusCapacity   uint32 // additive bonus (equipment/wheel/varStats — stubbed 0)
	InventoryWeight uint32 // cached total weight of all carried items
	Speed           uint16
	Vocation        uint16
	Sex             uint8

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
	LastMount uint16
	Mounts map[uint16]bool

	LightLevel uint8
	LightColor uint8

	// Quick Loot settings
	QuickLootFilter         uint8    // 0 = Skipped/Blacklist, 1 = Accepted/Whitelist
	QuickLootList           []uint16 // List of item IDs
	QuickLootFallbackToMain bool     // Fallback to main container when no category container is set
	// ManagedContainers / ManagedObtainContainers map an ObjectCategory to the
	// item id of the inventory container assigned to it. Loot containers are
	// inventory containers (the client sends pos.x==0xffff), so they are resolved
	// by searching the inventory for a container of this id rather than by a map
	// tile position (mirrors C++ m_managedContainers holding container references).
	ManagedContainers       map[uint8]uint16 // ObjectCategory -> loot container item id
	ManagedObtainContainers map[uint8]uint16 // ObjectCategory -> obtain container item id

	// StoreInbox is the player's Store Inbox container (item id ITEM_STORE_INBOX
	// 23396) where in-game store purchases are delivered. Created lazily on first
	// access (Player:getStoreInbox).
	StoreInbox *Item

	// Wheel of Destiny progression tree
	Wheel *WheelOfDestiny

	// Prey & Task Hunting systems. PreyCards is the wildcard/prey-card resource
	// (schema players.prey_wildcard) spent on bonus rerolls and list selection.
	Prey       *PlayerPrey
	TaskHunter *PlayerTaskHunter
	PreyCards  uint32

	// Store / Tibia Coins (account-level: accounts.coins / coins_transferable).
	// CoinBalance is the total; CoinTransferable is the transferable subset.
	CoinBalance      uint32
	CoinTransferable uint32

	// BossPoints is the bosstiary points total (players.boss_points), earned by
	// reaching boss unlock levels and spent implicitly via the loot bonus.
	BossPoints uint32

	// Bosstiary prowess slots: the boss race ids selected in slot one/two, and
	// how many times the player has removed a slotted boss (drives the removal
	// price). Persisted in player_bosstiary. BossRemoveTimes defaults to 1.
	BossSlotOne     uint32
	BossSlotTwo     uint32
	BossRemoveTimes uint8

	// Bestiary/charms (player_charms). CharmPoints is the spendable currency
	// earned by completing bestiary entries; MaxCharmPoints is the lifetime
	// total. Runes bits are the unlocked/assigned charm bitmasks.
	CharmPoints         uint32
	MaxCharmPoints      uint32
	MinorCharmEchoes    uint32
	MaxMinorCharmEchoes uint32
	CharmExpansion      bool
	UsedRunesBit        uint32
	UnlockedRunesBit    uint32
	// Charms holds per-charm state (tier + assigned monster race) indexed by
	// charm id 0..24. Mirrors C++ Player::charmsArray. Persisted as the
	// player_charms.charms blob.
	Charms [numCharms]CharmInfo

	// Exaltation Forge resources
	// Forge (Exaltation Forge). ForgeDusts is the dust resource amount;
	// ForgeDustLevel is the stored-dust limit (schema forge_dust_level,
	// default 100, max ForgeMaxDust). Slivers and cores are NOT counters — they
	// are real inventory items (ItemForgeSliver / ItemForgeCore), mirroring C++.
	// See forge.go.
	ForgeDusts     uint64
	ForgeDustLevel uint16
	ForgeHistory   []ForgeHistory

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
	TownID               uint16
	LoginPosition        Position
	Dead                 bool
	IsTraining           bool
	SkillLoss            bool
	Skull                uint8
	Blessings            [8]uint8
	OfflineTrainingTime  int32
	OfflineTrainingSkill int8
	LastLogin            uint64
	LastLogout           uint64
	SkillTries           [SkillCount]uint64
	ManaSpent            uint64
	MagLevelPercent      uint8
	LevelPercent         uint8

	// Party is the party this player belongs to (as leader or member), nil when
	// ungrouped. partyInvitations are parties that have invited this player but
	// which they have not yet joined.
	Party            *Party
	partyInvitations []*Party

	TargetID    uint32
	target      Creature
	ShopOwnerID uint32 // ID of the NPC currently being traded with

	// lastUIInteraction is the unix-millis timestamp of the last rate-limited UI
	// action (forge, wheel, ...), mirroring Player::lastUIInteraction.
	lastUIInteraction int64

	// Action exhaustion timestamps (mirroring Player::nextPotionAction / nextAction).
	NextPotionAction time.Time
	NextAction       time.Time

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

	FightMode  uint8 // 1 = attack (offensive), 2 = balanced, 3 = defense
	ChaseMode  bool  // true = follow, false = stand
	SecureMode bool  // true = secure, false = unmarked attack allowed

	World   *World
	Session Session
	KVStore map[string]any
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

// SendFYIBox sends an FYI popup box (opcode 0x15) to the player's client.
func (p *Player) SendFYIBox(text string) {
	if p.Session != nil {
		w := netmsg.NewWriter()
		w.AddByte(0x15) // FYI Box opcode
		w.AddString(text)
		p.Session.SendToClient(w)
	}
}

// SendTextWindow sends a text dialog window (opcode 0x96) to the player's client.
func (p *Player) SendTextWindow(windowTextID uint32, itemID uint16, text string) {
	if p.Session != nil {
		w := netmsg.NewWriter()
		w.AddByte(0x96) // Text Window opcode
		w.AddU32(windowTextID)
		w.AddU16(itemID)
		if itemID == 2160 {
			w.AddByte(1) // count for stackable item
		}
		w.AddString(text) // AddString writes uint16(len) + string bytes
		w.AddU16(0)       // writer name (empty)
		w.AddByte(0)      // show traded
		w.AddU16(0)       // date (empty)
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
		p.Capacity = 40000
	}
	if p.FightMode == 0 {
		p.FightMode = 1 // offensive
	}
	// Note: p.ChaseMode defaults to false (stand) which is fine for 0.
	// p.SecureMode defaults to true (secure) as standard Tibia behavior.
	// Since we want default to be secure, we can explicitly set it to true if not initialized.
	// But let's check if bool default of false can mean uninitialized or not. Actually, let's just initialize it to true.
	p.SecureMode = true

	if p.LastLogin == 0 && p.OfflineTrainingTime == 0 {
		p.OfflineTrainingTime = 43200 * 1000 // default to full 12-hour training pool
	}
	if p.OfflineTrainingSkill == 0 {
		p.OfflineTrainingSkill = -1
	}

	if p.ForgeDustLevel == 0 {
		p.ForgeDustLevel = 100
	}

	for i := range p.Skills {
		if p.Skills[i] == 0 {
			p.Skills[i] = 10
		}
	}
}

// Forge dust/sliver/core resource helpers live in forge.go alongside the forge
// engine (GetForgeDusts, SetForgeDusts, AddForgeDusts, RemoveForgeDusts,
// GetForgeDustLevel, AddForgeDustLevel, and the item-backed sliver/core
// counts).

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

// storageBestiaryKillCount is the base storage key for per-race kill counts
// (bestiary AND bosstiary). The count for race N lives at base+N. Mirrors
// STORAGEVALUE_BESTIARYKILLCOUNT (src/utils/const.hpp). Reusing the storage
// system means kill counts persist via player_storage for free.
const storageBestiaryKillCount = 61305000

// GetBestiaryKillCount returns how many of race `raceid` the player has killed.
func (p *Player) GetBestiaryKillCount(raceid uint16) uint32 {
	if v := p.GetStorageValue(storageBestiaryKillCount + uint32(raceid)); v > 0 {
		return uint32(v)
	}
	return 0
}

// AddBestiaryKillCount adds `amount` kills to race `raceid`'s count.
func (p *Player) AddBestiaryKillCount(raceid uint16, amount uint32) {
	old := p.GetBestiaryKillCount(raceid)
	p.SetStorageValue(storageBestiaryKillCount+uint32(raceid), int32(old+amount))
}

// GetBossPoints returns the player's accumulated bosstiary points
// (players.boss_points). Mirrors Player::getBossPoints.
func (p *Player) GetBossPoints() uint32 { return p.BossPoints }

// SetBossPoints sets the player's bosstiary points.
func (p *Player) SetBossPoints(amount uint32) { p.BossPoints = amount }

// AddBossPoints adds to the player's bosstiary points.
func (p *Player) AddBossPoints(amount uint32) { p.BossPoints += amount }

// GetSlotBossId returns the boss race id selected in bosstiary slot 1 or 2
// (0 = empty). Mirrors Player::getSlotBossId.
func (p *Player) GetSlotBossId(slot uint8) uint32 {
	switch slot {
	case 1:
		return p.BossSlotOne
	case 2:
		return p.BossSlotTwo
	}
	return 0
}

// SetSlotBossId sets the boss in slot 1 or 2. Mirrors Player::setSlotBossId.
func (p *Player) SetSlotBossId(slot uint8, bossID uint32) {
	switch slot {
	case 1:
		p.BossSlotOne = bossID
	case 2:
		p.BossSlotTwo = bossID
	}
}

// GetRemoveTimes returns how many times the player has removed a slotted boss.
func (p *Player) GetRemoveTimes() uint8 {
	if p.BossRemoveTimes == 0 {
		return 1
	}
	return p.BossRemoveTimes
}

// AddRemoveTime increments the slot-boss removal counter (Player::addRemoveTime).
func (p *Player) AddRemoveTime() { p.BossRemoveTimes = p.GetRemoveTimes() + 1 }

// AddBosstiaryKill credits `amount` kills of a boss (bosstiary race id + rarity)
// and, when the kills cross into a new unlock level, awards that level's boss
// points. Returns true if the boss level increased (caller sends the cyclopedia
// entry-changed update). Mirrors IOBosstiary::addBosstiaryKill.
func (p *Player) AddBosstiaryKill(raceID uint16, race bosstiary.Rarity, amount uint32) bool {
	if raceID == 0 || amount == 0 || !bosstiary.IsBoss(race) {
		return false
	}
	oldLevel := bosstiary.Level(race, p.GetBestiaryKillCount(raceID))
	p.AddBestiaryKillCount(raceID, amount)
	newLevel := bosstiary.Level(race, p.GetBestiaryKillCount(raceID))
	if newLevel > oldLevel {
		p.AddBossPoints(uint32(bosstiary.PointsForLevel(race, newLevel)))
		return true
	}
	return false
}

// numCharms is the number of charm-rune ids (CHARM_WOUND..CHARM_OVERFLUX = 0..24).
const numCharms = 25

// CharmInfo is the player's per-charm state. Mirrors C++ struct CharmInfo.
type CharmInfo struct {
	RaceID uint16 // assigned monster race (0 = unassigned)
	Tier   uint8  // unlock tier (0 = not unlocked)
}

// GetCharmTier returns the unlock tier of a charm (0 if id out of range).
func (p *Player) GetCharmTier(charmID uint8) uint8 {
	if int(charmID) >= len(p.Charms) {
		return 0
	}
	return p.Charms[charmID].Tier
}

// SetCharmTier sets the unlock tier of a charm.
func (p *Player) SetCharmTier(charmID uint8, tier uint8) {
	if int(charmID) < len(p.Charms) {
		p.Charms[charmID].Tier = tier
	}
}

// GetCharmRace returns the monster race a charm is assigned to (0 = none).
// Mirrors Player::parseRacebyCharm (get).
func (p *Player) GetCharmRace(charmID uint8) uint16 {
	if int(charmID) >= len(p.Charms) {
		return 0
	}
	return p.Charms[charmID].RaceID
}

// SetCharmRace assigns a charm to a monster race (0 = unassign).
// Mirrors Player::parseRacebyCharm (set).
func (p *Player) SetCharmRace(charmID uint8, raceID uint16) {
	if int(charmID) < len(p.Charms) {
		p.Charms[charmID].RaceID = raceID
	}
}

// GetMinorCharmEchoes returns the player's spendable minor charm echoes.
func (p *Player) GetMinorCharmEchoes() uint32 { return p.MinorCharmEchoes }

// AddMinorCharmEchoes adds (or, when negative, spends) minor charm echoes,
// bumping the lifetime max when adding. Mirrors IOBestiary::addMinorCharmEchoes.
func (p *Player) AddMinorCharmEchoes(amount uint32, negative bool) {
	if negative {
		if amount > p.MinorCharmEchoes {
			p.MinorCharmEchoes = 0
		} else {
			p.MinorCharmEchoes -= amount
		}
		return
	}
	p.MinorCharmEchoes += amount
	p.MaxMinorCharmEchoes += amount
}

// SetMinorCharmEchoes sets the spendable minor charm echoes.
func (p *Player) SetMinorCharmEchoes(amount uint32) { p.MinorCharmEchoes = amount }

// GetCharmPoints returns the player's spendable charm points.
func (p *Player) GetCharmPoints() uint32 { return p.CharmPoints }

// SpendCharmPoints deducts spendable charm points without touching the max.
// Mirrors IOBestiary::addCharmPoints(negative=true).
func (p *Player) SpendCharmPoints(amount uint32) {
	if amount > p.CharmPoints {
		p.CharmPoints = 0
	} else {
		p.CharmPoints -= amount
	}
}

// AddCharmPoints adds spendable charm points (and bumps the lifetime max).
func (p *Player) AddCharmPoints(amount uint32) {
	p.CharmPoints += amount
	p.MaxCharmPoints += amount
}

// SetCharmPoints sets the spendable charm points.
func (p *Player) SetCharmPoints(amount uint32) { p.CharmPoints = amount }

// AddBestiaryKill credits `amount` kills of a (non-boss) bestiary monster and,
// when the kills complete the entry, awards its charm points. Returns true when
// the kill crossed into a new unlock stage (caller sends the entry-changed
// update). Mirrors IOBestiary::addBestiaryKill.
func (p *Player) AddBestiaryKill(raceID uint16, t bestiary.Thresholds, charmPoints uint16, amount uint32) bool {
	if raceID == 0 || amount == 0 || t.ToKill == 0 {
		return false
	}
	old := p.GetBestiaryKillCount(raceID)
	p.AddBestiaryKillCount(raceID, amount)
	if bestiary.CrossedCompletion(old, amount, t) {
		p.AddCharmPoints(uint32(charmPoints))
	}
	return bestiary.CrossedStage(old, amount, t)
}

// IsBestiaryComplete reports whether the player has fully unlocked a monster.
func (p *Player) IsBestiaryComplete(raceID uint16, t bestiary.Thresholds) bool {
	return bestiary.IsComplete(p.GetBestiaryKillCount(raceID), t)
}

func (p *Player) GetID() uint32     { return p.ID }
func (p *Player) GetName() string   { return p.Name }
func (p *Player) GetHealth() uint32 { return p.Health }
func (p *Player) SetHealth(health uint32) {
	p.Health = health
	if maxHP := p.GetMaxHealth(); p.Health > maxHP {
		p.Health = maxHP
	}
}
func (p *Player) GetWheel() *WheelOfDestiny {
	if p.Wheel == nil {
		p.Wheel = NewWheelOfDestiny()
	}
	return p.Wheel
}

func (p *Player) GetMaxHealth() uint32 {
	val := int32(p.MaxHealth)
	if p.Wheel != nil {
		val += int32(p.Wheel.GetBonusHealth())
	}
	percent := int32(100)

	for _, cond := range p.Conditions() {
		if attrCond, ok := cond.(*combat.ConditionAttributesStruct); ok {
			if bonus, ok := attrCond.Stats[27]; ok { // CONDITION_PARAM_STAT_MAXHITPOINTS
				val += bonus
			}
			if pct, ok := attrCond.StatPercent[31]; ok { // CONDITION_PARAM_STAT_MAXHITPOINTSPERCENT
				percent += pct - 100
			}
		}
	}

	total := (val * percent) / 100
	if total < 0 {
		return 0
	}
	return uint32(total)
}
func (p *Player) AddHealth(amount int32) {
	if amount > 0 {
		p.Health += uint32(amount)
		if maxHP := p.GetMaxHealth(); p.Health > maxHP {
			p.Health = maxHP
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
func (p *Player) SetAttackTarget(id uint32)           { p.TargetID = id }
func (p *Player) ChangeTargetDistance(distance int32) {}
func (p *Player) GetPosition() Position               { return p.Pos }
func (p *Player) SetPosition(pos Position)            { p.Pos = pos }
func (p *Player) GetDirection() Direction             { return p.Direction }
func (p *Player) SetDirection(dir Direction)          { p.Direction = dir }
func (p *Player) GetOutfit() Outfit                   { return p.Outfit }
func (p *Player) GetLightLevel() uint8                { return p.LightLevel }
func (p *Player) GetLightColor() uint8                { return p.LightColor }

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

// GetEffectiveMagLevel returns the player's MagLevel modified by any active ConditionAttributes
func (p *Player) GetEffectiveMagLevel() uint16 {
	val := int32(p.MagLevel)
	percent := int32(100)

	for _, cond := range p.Conditions() {
		if attrCond, ok := cond.(*combat.ConditionAttributesStruct); ok {
			if bonus, ok := attrCond.Stats[30]; ok { // CONDITION_PARAM_STAT_MAGICPOINTS
				val += bonus
			}
			if pct, ok := attrCond.StatPercent[34]; ok { // CONDITION_PARAM_STAT_MAGICPOINTSPERCENT
				percent += pct - 100
			}
		}
	}

	total := (val * percent) / 100
	if total < 0 {
		return 0
	}
	return uint16(total)
}

// GetEffectiveSkill returns the player's skill level modified by active ConditionAttributes
func (p *Player) GetEffectiveSkill(skill Skill) uint16 {
	val := int32(p.Skills[skill])
	percent := int32(100)

	var paramKey int32
	var paramPctKey int32

	switch skill {
	case SkillFist:
		paramKey = 20    // CONDITION_PARAM_SKILL_FIST
		paramPctKey = 37 // CONDITION_PARAM_SKILL_FISTPERCENT
	case SkillClub:
		paramKey = 21    // CONDITION_PARAM_SKILL_CLUB
		paramPctKey = 38 // CONDITION_PARAM_SKILL_CLUBPERCENT
	case SkillSword:
		paramKey = 22    // CONDITION_PARAM_SKILL_SWORD
		paramPctKey = 39 // CONDITION_PARAM_SKILL_SWORDPERCENT
	case SkillAxe:
		paramKey = 23    // CONDITION_PARAM_SKILL_AXE
		paramPctKey = 40 // CONDITION_PARAM_SKILL_AXEPERCENT
	case SkillDistance:
		paramKey = 24    // CONDITION_PARAM_SKILL_DISTANCE
		paramPctKey = 41 // CONDITION_PARAM_SKILL_DISTANCEPERCENT
	case SkillShielding:
		paramKey = 25    // CONDITION_PARAM_SKILL_SHIELD
		paramPctKey = 42 // CONDITION_PARAM_SKILL_SHIELDPERCENT
	case SkillFishing:
		paramKey = 26    // CONDITION_PARAM_SKILL_FISHING
		paramPctKey = 43 // CONDITION_PARAM_SKILL_FISHINGPERCENT
	}

	for _, cond := range p.Conditions() {
		if attrCond, ok := cond.(*combat.ConditionAttributesStruct); ok {
			// Check melee-generic bonuses (applies to Club, Sword, and Axe)
			if skill >= SkillClub && skill <= SkillAxe {
				if bonus, ok := attrCond.Skills[19]; ok { // CONDITION_PARAM_SKILL_MELEE
					val += bonus
				}
				if pct, ok := attrCond.SkillPercent[36]; ok { // CONDITION_PARAM_SKILL_MELEEPERCENT
					percent += pct - 100
				}
			}

			// Check specific skill bonuses
			if paramKey != 0 {
				if bonus, ok := attrCond.Skills[paramKey]; ok {
					val += bonus
				}
			}
			if paramPctKey != 0 {
				if pct, ok := attrCond.SkillPercent[paramPctKey]; ok {
					percent += pct - 100
				}
			}
		}
	}

	total := (val * percent) / 100
	if total < 0 {
		return 0
	}
	return uint16(total)
}

func (p *Player) GetCreatureType() uint8 { return 0 } // CREATURETYPE_PLAYER

func (p *Player) IsInProtectionZone() bool {
	if p == nil || p.World == nil || p.World.Map == nil {
		return false
	}
	tile := p.World.Map.GetTile(p.Pos)
	return tile.IsProtectionZone()
}

func (p *Player) GetIcons() uint64 {
	var icons uint64
	for _, cond := range p.Conditions() {
		icons |= cond.GetIcons()
	}
	if p.IsInProtectionZone() {
		icons |= (1 << 14) // Pigeon icon (Protection Zone icon, bit 14)
	}
	return icons
}

func (p *Player) AddCondition(c combat.Condition) {
	p.conditionStore.AddCondition(adaptCreature(p), c)
	if p.Session != nil {
		p.Session.SendIcons()
	}
}

func (p *Player) GetWorld() *World { return p.World }

func (p *Player) SetGhostMode(ghost bool) {
	if p.Ghost == ghost {
		return
	}
	p.Ghost = ghost
	if p.World != nil && p.World.OnGhostModeChange != nil {
		p.World.OnGhostModeChange(p)
	}
}

func (p *Player) NotifyIconsChange() {
	if p.Session != nil {
		p.Session.SendIcons()
	}
	if p.World != nil && p.World.OnIconsUpdate != nil {
		p.World.OnIconsUpdate(p)
	}
}

func (p *Player) TickConditions(interval int32) {
	p.conditionStore.ExecuteConditions(adaptCreature(p), interval)
}

func (p *Player) RemoveCondition(t combat.ConditionType) {
	p.conditionStore.RemoveCondition(adaptCreature(p), t)
	if p.Session != nil {
		p.Session.SendIcons()
	}
}

func (p *Player) ClearConditions() {
	p.conditionStore.ClearConditions(adaptCreature(p))
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
func (p *Player) GetMana() uint32 { return p.Mana }
func (p *Player) GetMaxMana() uint32 {
	val := int32(p.MaxMana)
	if p.Wheel != nil {
		val += int32(p.Wheel.GetBonusMana())
	}
	percent := int32(100)

	for _, cond := range p.Conditions() {
		if attrCond, ok := cond.(*combat.ConditionAttributesStruct); ok {
			if bonus, ok := attrCond.Stats[28]; ok { // CONDITION_PARAM_STAT_MAXMANAPOINTS
				val += bonus
			}
			if pct, ok := attrCond.StatPercent[32]; ok { // CONDITION_PARAM_STAT_MAXMANAPOINTSPERCENT
				percent += pct - 100
			}
		}
	}

	total := (val * percent) / 100
	if total < 0 {
		return 0
	}
	return uint32(total)
}
func (p *Player) AddMana(amount int32) {
	if amount > 0 {
		p.Mana += uint32(amount)
		if maxMana := p.GetMaxMana(); p.Mana > maxMana {
			p.Mana = maxMana
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

// GetWeapon returns currently equipped weapon in Left/Right slots, resolving ammunition/quivers.
func (p *Player) GetWeapon(catalog *items.Catalog, ignoreAmmo bool) *Item {
	itemLeft := p.getWeaponFromSlot(catalog, ConstSlotLeft, ignoreAmmo)
	if itemLeft != nil {
		return itemLeft
	}
	itemRight := p.getWeaponFromSlot(catalog, ConstSlotRight, ignoreAmmo)
	if itemRight != nil {
		return itemRight
	}
	return nil
}

func (p *Player) getWeaponFromSlot(catalog *items.Catalog, slot uint8, ignoreAmmo bool) *Item {
	item := p.Inventory[slot]
	if item == nil {
		return nil
	}
	wType := item.WeaponType(catalog)
	if wType == "" || wType == "shield" || wType == "ammunition" || wType == "ammo" {
		return nil
	}
	if !ignoreAmmo && (wType == "distance" || wType == "missile") {
		ammoType := item.AmmoType(catalog)
		if ammoType != "" && ammoType != "none" {
			// First, check ammo slot (ConstSlotAmmo) directly
			ammoSlotItem := p.Inventory[ConstSlotAmmo]
			if ammoSlotItem != nil && ammoSlotItem.AmmoType(catalog) == ammoType {
				return ammoSlotItem
			}
			// Fallback to quiver
			return p.GetQuiverAmmoOfType(catalog, ammoType)
		}
	}
	return item
}

// GetQuiverAmmoOfType searches for ammo matching the ammoType inside the equipped quiver.
func (p *Player) GetQuiverAmmoOfType(catalog *items.Catalog, ammoType string) *Item {
	var quiver *Item
	if q := p.Inventory[ConstSlotRight]; q != nil && q.IsQuiver(catalog) {
		quiver = q
	} else if q := p.Inventory[ConstSlotLeft]; q != nil && q.IsQuiver(catalog) {
		quiver = q
	}
	if quiver == nil {
		return nil
	}
	for _, ammoItem := range quiver.Contents {
		if ammoItem == nil {
			continue
		}
		if ammoItem.AmmoType(catalog) == ammoType {
			return ammoItem
		}
	}
	return nil
}

// GetShieldAndWeapon returns the equipped shield and weapon.
func (p *Player) GetShieldAndWeapon(catalog *items.Catalog) (*Item, *Item) {
	var shield, weapon *Item
	for slot := ConstSlotRight; slot <= ConstSlotLeft; slot++ {
		item := p.Inventory[slot]
		if item == nil {
			continue
		}
		wType := item.WeaponType(catalog)
		if wType == "shield" {
			if shield == nil || item.Defense(catalog) > shield.Defense(catalog) {
				shield = item
			}
		} else if wType != "" {
			weapon = item
		}
	}
	return shield, weapon
}

// GetWeaponSkill returns the skill value corresponding to the item's weapon type.
func (p *Player) GetWeaponSkill(catalog *items.Catalog, item *Item) uint16 {
	if item == nil {
		return p.GetEffectiveSkill(SkillFist)
	}
	wType := item.WeaponType(catalog)
	switch wType {
	case "sword":
		return p.GetEffectiveSkill(SkillSword)
	case "club":
		return p.GetEffectiveSkill(SkillClub)
	case "axe":
		return p.GetEffectiveSkill(SkillAxe)
	case "distance", "missile":
		return p.GetEffectiveSkill(SkillDistance)
	default:
		return p.GetEffectiveSkill(SkillFist)
	}
}

// GetAttackFactor returns the scaling multiplier for damage based on current fight mode.
func (p *Player) GetAttackFactor() float64 {
	switch p.FightMode {
	case 1: // offensive
		return 1.0
	case 2: // balanced
		return 0.75
	case 3: // defensive
		return 0.5
	default:
		return 1.0
	}
}

// GetArmor returns the player's total armor, scaled by vocation multiplier.
func (p *Player) GetArmor() int32 {
	var catalog *items.Catalog
	if p.World != nil {
		catalog = p.World.Items
	}
	armor := int32(0)
	for _, item := range p.Inventory {
		if item != nil {
			armor += item.Armor(catalog)
		}
	}
	if voc := vocations.GetVocation(uint32(p.Vocation)); voc != nil && voc.Formula.Armor > 0 {
		armor = int32(float64(armor) * voc.Formula.Armor)
	}
	return armor
}

// GetDefense calculates the player's total defense, reflecting skills, shield, extra weapon defense, and vocation multipliers.
func (p *Player) GetDefense() int32 {
	var catalog *items.Catalog
	if p.World != nil {
		catalog = p.World.Items
	}
	defenseSkill := int32(p.GetEffectiveSkill(SkillFist))
	defenseValue := int32(7)

	shield, weapon := p.GetShieldAndWeapon(catalog)
	if weapon != nil {
		defenseValue = weapon.Defense(catalog) + weapon.ExtraDefense(catalog)
		defenseSkill = int32(p.GetWeaponSkill(catalog, weapon))
	}

	if shield != nil {
		if weapon != nil {
			defenseValue = shield.Defense(catalog) + weapon.ExtraDefense(catalog)
		} else {
			defenseValue = shield.Defense(catalog)
		}
		defenseSkill = int32(p.GetEffectiveSkill(SkillShielding))
	}

	if defenseSkill == 0 {
		switch p.FightMode {
		case 1, 2:
			return 1
		case 3:
			return 2
		}
	}

	defenseScalingFactor := 0.15
	if shield != nil {
		defenseScalingFactor = 0.16
	} else if weapon != nil && weapon.Defense(catalog) > 0 {
		defenseScalingFactor = 0.146
	}

	// defense factor: 1.0 for defensive, 0.75 for balanced, 0.5 for offensive
	defenseFactor := 1.0
	switch p.FightMode {
	case 1:
		defenseFactor = 0.5
	case 2:
		defenseFactor = 0.75
	case 3:
		defenseFactor = 1.0
	}

	def := (float64(defenseSkill)/4.0 + 2.23) * float64(defenseValue) * defenseFactor * defenseScalingFactor
	if voc := vocations.GetVocation(uint32(p.Vocation)); voc != nil && voc.Formula.Defense > 0 {
		def *= voc.Formula.Defense
	}
	return int32(def)
}

// ConsumeAmmo reduces the count of the ammunition matching ammoType by 1, searching the ammo slot and quiver.
func (p *Player) ConsumeAmmo(catalog *items.Catalog, ammoType string) {
	// 1. Check ammo slot (ConstSlotAmmo)
	ammoSlotItem := p.Inventory[ConstSlotAmmo]
	if ammoSlotItem != nil && ammoSlotItem.AmmoType(catalog) == ammoType {
		if ammoSlotItem.Count > 1 {
			ammoSlotItem.Count--
		} else {
			p.Inventory[ConstSlotAmmo] = nil
			ammoSlotItem = nil
		}
		if p.Session != nil {
			p.Session.SendInventoryItem(ConstSlotAmmo, ammoSlotItem)
		}
		return
	}
	// 2. Check quiver
	var quiver *Item
	if q := p.Inventory[ConstSlotRight]; q != nil && q.IsQuiver(catalog) {
		quiver = q
	} else if q := p.Inventory[ConstSlotLeft]; q != nil && q.IsQuiver(catalog) {
		quiver = q
	}
	if quiver != nil {
		for i, ammoItem := range quiver.Contents {
			if ammoItem != nil && ammoItem.AmmoType(catalog) == ammoType {
				if ammoItem.Count > 1 {
					ammoItem.Count--
				} else {
					// Remove item from container contents
					quiver.Contents = append(quiver.Contents[:i], quiver.Contents[i+1:]...)
				}
				if p.Session != nil {
					p.Session.RefreshContainer(quiver)
				}
				return
			}
		}
	}
}

// ConsumeWeaponInHand reduces the count of the throwable weapon equipped in the specified hand slot.
func (p *Player) ConsumeWeaponInHand(slot uint8) {
	item := p.Inventory[slot]
	if item != nil {
		if item.Count > 1 {
			item.Count--
		} else {
			p.Inventory[slot] = nil
			item = nil
		}
		if p.Session != nil {
			p.Session.SendInventoryItem(slot, item)
		}
	}
}

func (p *Player) AddSkillTries(skill Skill, tries uint64) {
	if skill < 0 || skill >= SkillCount {
		return
	}
	p.SkillTries[skill] += tries

	for {
		currentLevel := p.Skills[skill]
		if currentLevel >= 150 {
			break
		}

		base := uint64(50)
		multiplier := 1.1
		switch skill {
		case SkillDistance:
			multiplier = 1.2
		case SkillShielding:
			multiplier = 1.1
		default:
			multiplier = 1.1
		}

		var req uint64
		if currentLevel < 10 {
			req = base
		} else {
			factor := 1.0
			for i := uint16(10); i < currentLevel; i++ {
				factor *= multiplier
			}
			req = uint64(float64(base) * factor)
		}

		if req == 0 {
			req = 50
		}

		if p.SkillTries[skill] >= req {
			p.SkillTries[skill] -= req
			p.Skills[skill]++
			p.SendTextMessage(0x13, "You advanced to skill level "+fmt.Sprintf("%d", p.Skills[skill])+" in "+skillNameOf(skill)+".")
		} else {
			break
		}
	}

	if p.Session != nil {
		p.Session.SendSkills()
	}
}

func (p *Player) AddManaSpent(amount uint64) {
	p.ManaSpent += amount

	for {
		currentLevel := p.MagLevel
		if currentLevel >= 150 {
			break
		}

		base := uint64(1600)
		multiplier := 1.1

		var req uint64
		if currentLevel < 1 {
			req = base
		} else {
			factor := 1.0
			for i := uint16(1); i < currentLevel; i++ {
				factor *= multiplier
			}
			req = uint64(float64(base) * factor)
		}

		if req == 0 {
			req = 1600
		}

		if p.ManaSpent >= req {
			p.ManaSpent -= req
			p.MagLevel++
			p.SendTextMessage(0x13, "You advanced to magic level "+fmt.Sprintf("%d", p.MagLevel)+".")
		} else {
			break
		}
	}

	if p.Session != nil {
		p.Session.SendSkills()
	}
}

// GetSkillPercent returns the percentage progress to the next skill level (0 to 10000).
func (p *Player) GetSkillPercent(skill Skill) uint16 {
	if skill < 0 || skill >= SkillCount {
		return 0
	}
	currentLevel := p.Skills[skill]
	if currentLevel >= 150 {
		return 0
	}

	base := uint64(50)
	multiplier := 1.1
	switch skill {
	case SkillDistance:
		multiplier = 1.2
	case SkillShielding:
		multiplier = 1.1
	default:
		multiplier = 1.1
	}

	var req uint64
	if currentLevel < 10 {
		req = base
	} else {
		factor := 1.0
		for i := uint16(10); i < currentLevel; i++ {
			factor *= multiplier
		}
		req = uint64(float64(base) * factor)
	}

	if req == 0 {
		req = 50
	}

	tries := p.SkillTries[skill]
	if tries >= req {
		return 10000
	}

	return uint16((float64(tries) / float64(req)) * 10000)
}

// GetMagLevelPercent returns the percentage progress to the next magic level (0 to 10000).
func (p *Player) GetMagLevelPercent() uint16 {
	currentLevel := p.MagLevel
	if currentLevel >= 150 {
		return 0
	}

	base := uint64(1600)
	multiplier := 1.1

	var req uint64
	if currentLevel < 1 {
		req = base
	} else {
		factor := 1.0
		for i := uint16(1); i < currentLevel; i++ {
			factor *= multiplier
		}
		req = uint64(float64(base) * factor)
	}

	if req == 0 {
		req = 1600
	}

	spent := p.ManaSpent
	if spent >= req {
		return 10000
	}

	return uint16((float64(spent) / float64(req)) * 10000)
}

// GetLevelPercent returns the percentage progress to the next level (0 to 10000).
func (p *Player) GetLevelPercent() uint16 {
	if p.Level == 0 {
		return 0
	}
	currExp := ExpForLevel(uint64(p.Level))
	nextExp := ExpForLevel(uint64(p.Level) + 1)
	if nextExp <= currExp {
		return 0
	}
	if p.Experience <= currExp {
		return 0
	}
	if p.Experience >= nextExp {
		return 10000
	}
	ratio := float64(p.Experience-currExp) / float64(nextExp-currExp)
	return uint16(ratio * 10000)
}

// CanDoPotionAction reports whether the potion action delay has elapsed.
func (p *Player) CanDoPotionAction() bool {
	return time.Now().After(p.NextPotionAction) || time.Now().Equal(p.NextPotionAction)
}

// SetNextPotionAction sets the timestamp until which potion actions are blocked.
func (p *Player) SetNextPotionAction(delay time.Duration) {
	t := time.Now().Add(delay)
	if t.After(p.NextPotionAction) {
		p.NextPotionAction = t
	}
}

// CanDoAction reports whether the standard action delay has elapsed.
func (p *Player) CanDoAction() bool {
	return time.Now().After(p.NextAction) || time.Now().Equal(p.NextAction)
}

// SetNextAction sets the timestamp until which standard actions are blocked.
func (p *Player) SetNextAction(delay time.Duration) {
	t := time.Now().Add(delay)
	if t.After(p.NextAction) {
		p.NextAction = t
	}
}

func skillNameOf(skill Skill) string {
	switch skill {
	case SkillFist:
		return "fist fighting"
	case SkillClub:
		return "club fighting"
	case SkillSword:
		return "sword fighting"
	case SkillAxe:
		return "axe fighting"
	case SkillDistance:
		return "distance fighting"
	case SkillShielding:
		return "shielding"
	case SkillFishing:
		return "fishing"
	}
	return "skill"
}

func (p *Player) GetPrey() *PlayerPrey {
	if p.Prey == nil {
		p.Prey = NewPlayerPrey()
	}
	return p.Prey
}

func (p *Player) GetTaskHunter() *PlayerTaskHunter {
	if p.TaskHunter == nil {
		p.TaskHunter = NewPlayerTaskHunter()
	}
	return p.TaskHunter
}

// IsUIExhausted reports whether a rate-limited UI action happened within the
// last exhaustionMS milliseconds (default 250), mirroring Player::isUIExhausted.
func (p *Player) IsUIExhausted(exhaustionMS int64) bool {
	if exhaustionMS <= 0 {
		exhaustionMS = 250
	}
	return time.Now().UnixMilli()-p.lastUIInteraction < exhaustionMS
}

// UpdateUIExhausted stamps the current time as the last UI interaction.
func (p *Player) UpdateUIExhausted() { p.lastUIInteraction = time.Now().UnixMilli() }

// GetPreyCards returns the player's prey-card (wildcard) balance.
func (p *Player) GetPreyCards() uint32 { return p.PreyCards }

// AddPreyCards credits prey cards.
func (p *Player) AddPreyCards(amount uint32) { p.PreyCards += amount }

// UsePreyCards spends `amount` prey cards, returning false (and spending
// nothing) when the balance is insufficient. Mirrors Player::usePreyCards.
func (p *Player) UsePreyCards(amount uint32) bool {
	if p.PreyCards < amount {
		return false
	}
	p.PreyCards -= amount
	return true
}

func (p *Player) GetTaskHuntingPoints() uint32 {
	return p.GetTaskHunter().Points
}

// AddTaskHuntingPoints credits hunting-task points, mirroring
// Player::addTaskHuntingPoints.
func (p *Player) AddTaskHuntingPoints(amount uint32) {
	th := p.GetTaskHunter()
	th.mu.Lock()
	th.Points += amount
	th.mu.Unlock()
}

// bossCooldownKey namespaces per-boss fight cooldowns inside KVStore.
func bossCooldownKey(boss string) string { return "boss-cooldown:" + boss }

// SetBossCooldown records the unix timestamp until which the player may not
// fight the named boss again (mirrors Player::setBossCooldown's KV store).
func (p *Player) SetBossCooldown(boss string, timestamp int64) {
	if p.KVStore == nil {
		p.KVStore = make(map[string]any)
	}
	p.KVStore[bossCooldownKey(boss)] = timestamp
}

// GetBossCooldown returns the stored fight cooldown timestamp for a boss, or 0.
func (p *Player) GetBossCooldown(boss string) int64 {
	if p.KVStore == nil {
		return 0
	}
	if v, ok := p.KVStore[bossCooldownKey(boss)].(int64); ok {
		return v
	}
	return 0
}

// CanFightBoss reports whether the boss fight cooldown has elapsed, mirroring
// Player::canFightBoss (now >= stored cooldown).
func (p *Player) CanFightBoss(boss string, now int64) bool {
	return now >= p.GetBossCooldown(boss)
}

func (p *Player) AddMount(mountID uint16) {
	if p.Mounts == nil {
		p.Mounts = make(map[uint16]bool)
	}
	p.Mounts[mountID] = true
}

func (p *Player) HasMount(mountID uint16) bool {
	if p.Mounts == nil {
		return false
	}
	return p.Mounts[mountID]
}

func (p *Player) RemoveMount(mountID uint16) {
	if p.Mounts != nil {
		delete(p.Mounts, mountID)
	}
}
