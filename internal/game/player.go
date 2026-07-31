package game

import (
	"fmt"
	"strings"
	"sync"
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

	// Supply Stash. Opens the stash container (0x29).
	SendOpenStash()

	// Chat channels.
	SendTextMessage(msgType uint8, text string)
	SendToChannel(statementID uint32, speakerName string, speakerLevel uint16, talkType byte, channelID uint16, text string)
	SendChannelsDialog(channels []*ChatChannel)
	SendOpenChannel(channel *ChatChannel)
	SendOpenPrivateChannel(receiver string)
	SendCreatePrivateChannel(channelID uint16, channelName string, ownerName string)
	SendClosePrivateChannel(channelID uint16)
	SendChannelEvent(channelID uint16, playerName string, event byte)

	// Market (B11). Opens the market window showing depot contents and bank balance.
	SendOpenMarket()
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

// Skill values mirror skills_t (src/creatures/creatures_definitions.hpp:466).
// The order is load-bearing: IOLoginDataLoad::loadPlayerSkill indexes
// player->skills[i] by these values against a fixed array of column names, and a
// persisted ProficiencyPerk stores its skillId as one of them.
const (
	SkillFist Skill = iota
	SkillClub
	SkillSword
	SkillAxe
	SkillDistance
	SkillShielding
	SkillFishing
	SkillCriticalHitChance
	SkillCriticalHitDamage
	SkillLifeLeechChance
	SkillLifeLeechAmount
	SkillManaLeechChance
	SkillManaLeechAmount
	// SkillCount is one past SKILL_LAST (= SKILL_MANA_LEECH_AMOUNT), so it is the
	// length of the Skills/SkillTries arrays.
	SkillCount
)

// Player is a logged-in character. It embeds creature-like fields directly to
// keep the model flat for now.
type Player struct {
	conditionStore
	damageTracker

	ID          uint32 // creature id (assigned at spawn)
	DBID        uint32 // players.id
	AccountID   uint32
	AccountType uint8
	GroupID     uint16 // players.group_id — staff groups 4/5/6 cannot be attacked
	Ghost       bool   // ghost mode (invisible; not targetable by monsters)
	Name        string
	// GuildID is the guilds.id row, needed to query guild_wars; GuildName alone
	// cannot, and looking the guild up by name only works once it is registered in
	// the world.
	GuildID   uint32
	GuildName string
	// GuildWarList holds the guild ids this player's guild is at war with, loaded at
	// login (IOGuild::getWarList). Kills between guilds at war are justified.
	GuildWarList  []uint32
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

	Outfit  Outfit
	Outfits []OutfitEntry

	LastMount uint16
	Mounts    map[uint16]bool

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

	// Advanced Combat Stats. Crit and leech are NOT stored here: they are real
	// skills in C++ (skills_t 7..12) with their own players columns, so they live
	// in Skills/SkillTries and are read through the getters below.
	ReflectPercent uint16
	AbsorbPercent  uint16

	// StoreInbox is the player's Store Inbox container (item id ITEM_STORE_INBOX
	// 23396) where in-game store purchases are delivered. Created lazily on first
	// access (Player:getStoreInbox).
	StoreInbox *Item

	// Inbox is the player's standard Inbox container.
	Inbox *Item

	// RewardChest is the player's reward chest container.
	RewardChest *Item

	// DepotLockers holds the player's depots by depot ID.
	DepotLockers map[uint16]*Item

	// DepotManager manages all depot lockers across all towns
	DepotManager *PlayerDepotManager

	// InMarket is true while the player has the market window open.
	InMarket bool

	// Stash holds the player's supply stash items (ItemID -> Count).
	Stash map[uint16]uint32
	// stashMenuAvailable is true when the stash menu is available (set by NPC).
	stashMenuAvailable bool

	// Wheel of Destiny progression tree
	Wheel *WheelOfDestiny

	// WheelGemManager holds the player gem Atelier data (separate from Wheel).
	WheelGemManager *WheelGemCollection
	// WeaponProficiency tracks per-weapon bonus stats. It is DERIVED state:
	// C++ recomputes it from Proficiency via applyPerks and never persists it.
	WeaponProficiency *WeaponProficiency

	// Proficiency is the persisted per-weapon progression, keyed by item id.
	// It lives in the KV store under player.<guid>.weapon-proficiency.<weaponId>,
	// matching WeaponProficiency::load/save (weapon_proficiency.cpp:253-291).
	Proficiency map[uint16]WeaponProficiencyData

	// Prey & Task Hunting systems. PreyCards is the wildcard/prey-card resource
	// (schema players.prey_wildcard) spent on bonus rerolls and list selection.
	Prey       *PlayerPrey
	TaskHunter *PlayerTaskHunter
	PreyCards  uint32

	// Store / Tibia Coins (account-level: accounts.coins / coins_transferable).
	// CoinBalance is the total; CoinTransferable is the transferable subset.
	CoinBalance       uint32
	CoinTransferable  uint32
	TournamentBalance uint32

	// VIP List and Groups (account-level)
	VIPList   []VIPEntry
	VIPGroups []VIPGroup

	// Achievements (B7) — map of achievement ID → unlock timestamp (unix).
	Achievements map[uint16]int64
	// TitleStrings are string labels unlocked alongside achievements.
	TitleStrings []string

	// Familiars (B15) — unlocked familiars for this player.
	Familiars      []Familiar
	ActiveFamiliar uint16 // lookType of active familiar (0 = none)

	// Badges (B10/cyclopedia) — unlocked account badges. Mirrors C++
	// PlayerBadge (player_badge.hpp).
	Badges *PlayerBadges

	// Titles (B11/cyclopedia) — unlocked character titles.
	Titles *PlayerTitles

	// Hazard (B17) — current hazard system points.
	HazardPoints uint32
	// LastHazardCriticalHit stores the unix-millis timestamp of the last hazard
	// crit received. Used to enforce the hazard crit cooldown (10s).
	LastHazardCriticalHit int64
	// reloadHazardPointsCounter tracks hazard point reload timing.
	reloadHazardPointsCounter int32

	// Concoctions (B18) — active concoctions stored as KV.
	Concoctions map[string]int64 // concoction_name -> expiry timestamp

	// ActiveFoodItems tracks consumed food items keyed by itemID with remaining
	// time in seconds. Managed via UpdateFoodItem / Lua binding, drained by the
	// regen ticker. Persisted for cyclopedia display.
	ActiveFoodItems map[uint16]uint32 // itemID -> remaining time in seconds

	// Hirelings (B19) — player-owned hirelings.
	Hirelings []*Hireling

	// Animus Mastery (B16) — unlocked animus masteries.
	AnimusMastery []byte         // raw blob from DB (for serialization)
	AnimMastery   *AnimusMastery // runtime struct (lazy init via GetAnimusMastery)

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
	TownID        uint16
	LoginPosition Position
	Dead          bool
	IsTraining    bool
	SkillLoss     bool
	// attackedSet is the set of player guids this player has attacked, the
	// aggressor record Player::hasAttacked reads. Only someone who attacked first
	// can earn an unjustified kill.
	attackedMu  sync.RWMutex
	attackedSet map[uint32]struct{}
	Skull       uint8
	// UnjustifiedKills is the frag list behind the skull system, persisted in
	// player_kills. LastKillTime is the newest entry, cached the way C++ caches it
	// (Player::updateLastKillTimeCache).
	UnjustifiedKills     []Kill
	LastKillTime         int64
	SkullTime            int64
	ConditionsBlob       []byte
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

	// ImbuingItem is the item currently being imbued, set when the player opens
	// the imbuement window with a selected item (C++ Player::m_imbuingItem).
	ImbuingItem *Item

	World   *World
	Session Session
	KVStore map[string]any

	// ViolationReports stores rule violation reports submitted by this player
	// during the current session. A full implementation would persist them
	// to the database for moderator review.
	ViolationReports []ReportViolationEntry
}

// GetHirelings returns the player's hirelings.
func (p *Player) GetHirelings() []*Hireling { return p.Hirelings }

// GetHirelingCount returns the number of hirelings owned.
func (p *Player) GetHirelingCount() int { return len(p.Hirelings) }

// GetAnimusMastery returns the player's animus mastery tracker (lazy init).
func (p *Player) GetAnimusMastery() *AnimusMastery {
	if p.AnimMastery == nil {
		p.AnimMastery = NewAnimusMastery()
	}
	return p.AnimMastery
}

// Cooldowns returns the player's spell cooldown manager, creating it on first
// use.
func (p *Player) Cooldowns() *combat.CooldownManager {
	if p.cooldowns == nil {
		p.cooldowns = combat.NewCooldownManager()
	}
	return p.cooldowns
}

// GetBadges returns the player badge state, initialising it on first use.
func (p *Player) GetBadges() *PlayerBadges {
	if p.Badges == nil {
		p.Badges = &PlayerBadges{Unlocked: make(map[uint32]bool)}
	}
	return p.Badges
}

// GetTitles returns the player title state, initialising it on first use.
func (p *Player) GetTitles() *PlayerTitles {
	if p.Titles == nil {
		p.Titles = &PlayerTitles{Unlocked: make(map[uint8]bool)}
	}
	return p.Titles
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

// GetLearnedSpells returns a copy of the learned spells map.
func (p *Player) GetLearnedSpells() map[string]bool {
	if p.learnedSpells == nil {
		return nil
	}
	res := make(map[string]bool, len(p.learnedSpells))
	for k, v := range p.learnedSpells {
		res[k] = v
	}
	return res
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
// OnPlayerStorageUpdate is the EventCallback playerOnStorageUpdate hook, set by
// the events engine at startup.
var OnPlayerStorageUpdate func(p *Player, key uint32, value, oldValue int32)

func (p *Player) SetStorageValue(key uint32, value int32) {
	oldValue, had := p.Storages[key]
	if !had {
		oldValue = -1
	}
	if value == -1 {
		delete(p.Storages, key)
	} else {
		if p.Storages == nil {
			p.Storages = make(map[uint32]int32)
		}
		p.Storages[key] = value
	}
	if OnPlayerStorageUpdate != nil && value != oldValue {
		OnPlayerStorageUpdate(p, key, value, oldValue)
	}
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
// TODOs for the party agents). On level-up, health/mana/cap are increased based
// on the vocation, and then health/mana are refilled to max like the C++ path.
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

	// Apply Animus Mastery experience multiplier.
	if am := p.GetAnimusMastery(); am != nil && am.Count() > 0 {
		multiplier := am.GetExperienceMultiplier()
		if multiplier > 1.0 {
			exp = uint64(float64(exp) * multiplier)
		}
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
		levelsGained := p.Level - prevLevel

		// Apply vocation stats if vocation is valid
		if voc := vocations.GetVocation(uint32(p.Vocation)); voc != nil {
			p.MaxHealth += voc.GainHP * uint32(levelsGained)
			p.MaxMana += voc.GainMana * uint32(levelsGained)
			p.Capacity += (voc.GainCap * 100) * uint32(levelsGained)
		}

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

// Crit and leech read straight out of the skills array, which is where C++ keeps
// them (player->skills[SKILL_CRITICAL_HIT_CHANCE] and friends). They used to be
// separate fields that nothing ever wrote, so every one of these returned 0.
func (p *Player) GetCriticalChance() uint16           { return p.Skills[SkillCriticalHitChance] }
func (p *Player) GetCriticalDamage() uint16           { return p.Skills[SkillCriticalHitDamage] }
func (p *Player) GetLifeLeechChance() uint16          { return p.Skills[SkillLifeLeechChance] }
func (p *Player) GetLifeLeechAmount() uint16          { return p.Skills[SkillLifeLeechAmount] }
func (p *Player) GetManaLeechChance() uint16          { return p.Skills[SkillManaLeechChance] }
func (p *Player) GetManaLeechAmount() uint16          { return p.Skills[SkillManaLeechAmount] }
func (p *Player) GetReflectPercent() uint16           { return p.ReflectPercent }
func (p *Player) GetAbsorbPercent() uint16            { return p.AbsorbPercent }
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

// AddInFightTicks applies or extends the CONDITION_INFIGHT (crossed swords icon)
// for the standard PZ lock duration. Mirrors C++ Player::addInFightTicks.
func (p *Player) AddInFightTicks() {
	const fightTicks = 10 * 1000 // 10 seconds, matching C++ PZ_LOCKED default
	cond := combat.CreateCondition(0, combat.ConditionInFight, fightTicks, 0, false)
	p.AddCondition(cond)
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
func (p *Player) GetWeaponSkillForItem(catalog *items.Catalog, item *Item) uint16 {
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

// GetEquippedAugmentItems returns all equipped items that have at least one
// AugmentInfo entry in their ItemType. Scans inventory slots 1..10 (head to ammo).
func (p *Player) GetEquippedAugmentItems(catalog *items.Catalog) []*Item {
	if catalog == nil {
		return nil
	}
	var result []*Item
	for i := 1; i <= 10; i++ {
		item := p.Inventory[i]
		if item == nil {
			continue
		}
		itemType := catalog.Get(item.ID)
		if itemType != nil && len(itemType.Augments) > 0 {
			result = append(result, item)
		}
	}
	return result
}

// GetEquippedAugmentItemsByType returns equipped items whose ItemType has at
// least one AugmentInfo matching the given type and optionally spellName.
// If spellName is empty, all augments of the given type are matched.
func (p *Player) GetEquippedAugmentItemsByType(catalog *items.Catalog, augType items.AugmentType, spellName string) []*Item {
	if catalog == nil {
		return nil
	}
	var result []*Item
	for i := 1; i <= 10; i++ {
		item := p.Inventory[i]
		if item == nil {
			continue
		}
		itemType := catalog.Get(item.ID)
		if itemType == nil {
			continue
		}
		for _, aug := range itemType.Augments {
			if aug.Type != augType {
				continue
			}
			if spellName != "" && !strings.EqualFold(aug.SpellName, spellName) {
				continue
			}
			result = append(result, item)
			break
		}
	}
	return result
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
		defenseSkill = int32(p.GetWeaponSkillForItem(catalog, weapon))
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

		voc := vocations.GetVocation(uint32(p.Vocation))
		multiplier := 1.1 // fallback
		if voc != nil {
			for _, s := range voc.Skills {
				if s.ID == int(skill) {
					multiplier = s.Multiplier
					break
				}
			}
		}

		var base uint64 = 50
		if skill == SkillShielding {
			base = 100
		} else if skill == SkillFishing {
			base = 20
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

func (p *Player) GetItemCount(id uint16) uint32 {
	if p == nil {
		return 0
	}
	var count uint32
	// Scan inventory slots
	for _, item := range p.Inventory {
		if item != nil {
			count += p.countItemInTree(item, id)
		}
	}
	// Scan store inbox
	if p.StoreInbox != nil {
		count += p.countItemInTree(p.StoreInbox, id)
	}
	// Scan inbox
	if p.Inbox != nil {
		count += p.countItemInTree(p.Inbox, id)
	}
	return count
}

// AddToStash adds the given count of an item to the player's supply stash.
func (p *Player) AddToStash(itemID uint16, count uint32) {
	if p.Stash == nil {
		p.Stash = make(map[uint16]uint32)
	}
	p.Stash[itemID] += count
}

// RemoveFromStash subtracts the given count from the stash. Returns false if
// the stash doesn't have enough.
func (p *Player) RemoveFromStash(itemID uint16, count uint32) bool {
	if p.Stash == nil {
		return false
	}
	have := p.Stash[itemID]
	if have < count {
		return false
	}
	have -= count
	if have == 0 {
		delete(p.Stash, itemID)
	} else {
		p.Stash[itemID] = have
	}
	return true
}

// GetStashItemCount returns the number of stashed items for the given ID.
func (p *Player) GetStashItemCount(itemID uint16) uint32 {
	if p.Stash == nil {
		return 0
	}
	return p.Stash[itemID]
}

// GetStashSlotCount returns the number of distinct item entries in the stash.
func (p *Player) GetStashSlotCount() uint32 {
	if p.Stash == nil {
		return 0
	}
	return uint32(len(p.Stash))
}

// GetStashItems returns a copy of the stash map.
func (p *Player) GetStashItems() map[uint16]uint32 {
	out := make(map[uint16]uint32)
	for k, v := range p.Stash {
		out[k] = v
	}
	return out
}

// WithdrawFromStash removes count of itemID from stash. Returns false if insufficient.
func (p *Player) WithdrawFromStash(itemID uint16, count uint32) bool {
	return p.RemoveFromStash(itemID, count)
}

// IsStashMenuAvailable returns true if the stash context menu is active.
func (p *Player) IsStashMenuAvailable() bool {
	return p.stashMenuAvailable
}

// SetStashMenuAvailable enables or disables the stash context menu.
func (p *Player) SetStashMenuAvailable(v bool) {
	p.stashMenuAvailable = v
}

// StashContainerEntry maps an item instance to the count to stow.
type StashContainerEntry struct {
	Item  *Item
	Count uint32
}

// StashContainerList is a list of items to stow with their counts.
type StashContainerList []StashContainerEntry

// isItemStorable checks if a specific item instance can be stowed.
// C++: Item::isItemStorable (item.cpp:905)
func (p *Player) isItemStorable(item *Item) bool {
	if item == nil {
		return false
	}
	if item.Attr != nil {
		if item.Attr.StoreTimestamp != nil {
			return false
		}
		if item.Attr.Owner != nil && *item.Attr.Owner != p.DBID {
			return false
		}
	}
	return true
}

// isInsideDepot checks if the item is inside a depot locker (IDs 3497-3500).
func (p *Player) isInsideDepot(item *Item) bool {
	for parent := item.Parent; parent != nil; parent = parent.Parent {
		if parent.ID >= 3497 && parent.ID <= 3500 {
			return true
		}
	}
	return false
}

// FindItemInOpenContainers searches open containers for an item at the given
// position and stackpos index. Only matches containers whose Position matches.
func (p *Player) FindItemInOpenContainers(pos Position, stackpos int, itemID uint16) *Item {
	for _, oc := range p.openContainers {
		if oc.Container == nil {
			continue
		}
		if oc.Position == pos {
			if stackpos >= 0 && stackpos < len(oc.Container.Contents) {
				candidate := oc.Container.Contents[stackpos]
				if candidate != nil && candidate.ID == itemID {
					return candidate
				}
			}
		}
	}
	return nil
}

// StowItem moves items from inventory/containers to stash, ported 1:1 from
// C++ Player::stowItem (player.cpp:10101). The item parameter is the actual
// Item* the player clicked on (resolved from protocol position).
func (p *Player) StowItem(item *Item, count uint32, allItems bool) uint32 {
	if p.Stash == nil {
		p.Stash = make(map[uint16]uint32)
	}

	// C++: !isItemStorable && !goldPouch → cancel
	if !p.isItemStorable(item) {
		return 0
	}

	maxItemsToStow := uint32(1000) // STASH_MANAGE_AMOUNT
	itemDict := make(StashContainerList, 0)
	var totalItemsToStow uint32

	if allItems {
		// C++: containers cannot be stowed via allItems
		if len(item.Contents) > 0 {
			return 0
		}

		if p.isInsideDepot(item) {
			// C++: scan depot locker contents
			var depotContainer *Item
			for parent := item.Parent; parent != nil; parent = parent.Parent {
				if parent.ID >= 3497 && parent.ID <= 3500 {
					depotContainer = parent
					break
				}
			}
			if depotContainer != nil {
				totalItemsToStow = p.collectItemsSameType(item, depotContainer, &itemDict, totalItemsToStow, maxItemsToStow)
			}
		} else {
			// C++: scan backpack
			if bp := p.Inventory[ConstSlotBackpack]; bp != nil {
				totalItemsToStow = p.collectItemsSameType(item, bp, &itemDict, totalItemsToStow, maxItemsToStow)
			}
		}
	} else if len(item.Contents) > 0 {
		// C++: allItems=false, item is a container → scan its stowable items
		for _, child := range item.Contents {
			if totalItemsToStow >= maxItemsToStow {
				break
			}
			if child == nil || !p.isItemStorable(child) {
				continue
			}
			childCount := uint32(child.Count)
			if childCount == 0 {
				childCount = 1
			}
			if remaining := maxItemsToStow - totalItemsToStow; childCount > remaining {
				childCount = remaining
			}
			itemDict = append(itemDict, StashContainerEntry{Item: child, Count: childCount})
			totalItemsToStow += childCount
		}
	} else {
		// C++: allItems=false, single item
		stowCount := count
		if stowCount == 0 {
			stowCount = uint32(item.Count)
			if stowCount == 0 {
				stowCount = 1
			}
		}
		if remaining := maxItemsToStow - totalItemsToStow; stowCount > remaining {
			stowCount = remaining
		}
		itemDict = append(itemDict, StashContainerEntry{Item: item, Count: stowCount})
		totalItemsToStow += stowCount
	}

	if len(itemDict) == 0 {
		return 0
	}

	p.stashContainer(itemDict)
	return totalItemsToStow
}

// collectItemsSameType scans a container tree recursively for all items matching
// the target item's ID. C++: sendStowItems helper (player.cpp:10073) scans a
// single container's getStowableItems — we recurse so nested containers (bags
// in backpacks, items in nested depot chests) are found correctly.
// NOTE: itemDict deve ser um ponteiro para a slice original (Go passa slice
// header por valor; sem o ponteiro as modificações vão pra cópia).
func (p *Player) collectItemsSameType(target *Item, container *Item, itemDict *StashContainerList, totalSoFar, maxCount uint32) uint32 {
	var added uint32
	p.collectRecursive(target, container, itemDict, &added, totalSoFar, maxCount)
	return added
}

// collectRecursive is the recursive helper for collectItemsSameType.
func (p *Player) collectRecursive(target *Item, current *Item, itemDict *StashContainerList, added *uint32, totalSoFar, maxCount uint32) {
	if current == nil || totalSoFar+*added >= maxCount {
		return
	}

	if current.ID == target.ID && p.isItemStorable(current) {
		itemCount := uint32(current.Count)
		if itemCount == 0 {
			itemCount = 1
		}
		stowableToAdd := itemCount
		if remaining := maxCount - (totalSoFar + *added); stowableToAdd > remaining {
			stowableToAdd = remaining
		}
		if stowableToAdd > 0 {
			*itemDict = append(*itemDict, StashContainerEntry{Item: current, Count: stowableToAdd})
			*added += stowableToAdd
		}
	}

	for _, child := range current.Contents {
		if child != nil {
			p.collectRecursive(target, child, itemDict, added, totalSoFar, maxCount)
		}
	}
}

// stashContainer processes the item dict, removing items and adding to stash.
// C++: Player::stashContainer (player.cpp:5132)
func (p *Player) stashContainer(itemDict StashContainerList) {
	for _, entry := range itemDict {
		if entry.Item == nil || !p.isItemStorable(entry.Item) {
			continue
		}
		if p.removeItemFromContainer(entry.Item, uint16(entry.Count)) {
			p.AddToStash(entry.Item.ID, entry.Count)
		}
	}
}

// removeItemFromContainer removes a specific item instance from inventory or
// from any container tree (including open containers like depot chests).
// C++: handles both player-held items (removeItem) and external containers
// (parent->removeThing). Returns true on success.
func (p *Player) removeItemFromContainer(item *Item, itemCount uint16) bool {
	if item == nil {
		return false
	}

	// Check direct inventory slots
	for i := ConstSlotFirst; i <= ConstSlotLast; i++ {
		if p.Inventory[i] == item {
			if itemCount >= item.Count {
				p.Inventory[i] = nil
			} else {
				item.Count -= itemCount
			}
			return true
		}
	}

	// Check inventory container tree (backpack bags, etc.)
	for _, slot := range p.Inventory {
		if slot != nil && p.removeFromContentsRecursive(slot, item, itemCount) {
			return true
		}
	}

	// Check item's explicit parent
	if item.Parent != nil {
		p.removeFromContents(item.Parent, item, itemCount)
		return true
	}

	// Check open containers tree (depot chests, etc. — items might not have
	// Parent set but are referenced from openContainers)
	for _, oc := range p.openContainers {
		if oc.Container != nil && p.removeFromContentsRecursive(oc.Container, item, itemCount) {
			return true
		}
	}

	return false
}

// removeFromContents removes a child item from a parent's Contents slice.
func (p *Player) removeFromContents(parent *Item, item *Item, count uint16) {
	for i, child := range parent.Contents {
		if child == item {
			if count >= child.Count {
				parent.Contents = append(parent.Contents[:i], parent.Contents[i+1:]...)
			} else {
				child.Count -= count
			}
			return
		}
	}
}

// removeFromContentsRecursive recursively searches for an item in a container
// tree and removes it when found.
func (p *Player) removeFromContentsRecursive(container *Item, target *Item, count uint16) bool {
	if container == nil {
		return false
	}
	for i, child := range container.Contents {
		if child == target {
			if count >= child.Count {
				container.Contents = append(container.Contents[:i], container.Contents[i+1:]...)
			} else {
				child.Count -= count
			}
			return true
		}
		if len(child.Contents) > 0 {
			if p.removeFromContentsRecursive(child, target, count) {
				return true
			}
		}
	}
	return false
}

// countItemInTree recursively counts items matching the given ID in a container tree.
func (p *Player) countItemInTree(item *Item, id uint16) uint32 {
	if item == nil {
		return 0
	}
	var count uint32
	if item.ID == id {
		if item.Count > 0 {
			count += uint32(item.Count)
		} else {
			count++
		}
	}
	for _, child := range item.Contents {
		count += p.countItemInTree(child, id)
	}
	return count
}

func (p *Player) GetOpenContainers() map[uint8]OpenContainer {
	return p.openContainers
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

// VIPEntry represents a single player added to the VIP list.
type VIPEntry struct {
	PlayerID    uint32 // DBID of the target player
	PlayerName  string // Name of the target player
	Description string
	Icon        uint8
	Notify      bool
	Groups      []uint32 // IDs of the VIP groups this entry belongs to
}

// VIPGroup represents a custom or default group in the VIP list.
type VIPGroup struct {
	ID           uint32
	Name         string
	Customizable bool
}

func (p *Player) IsPremium() bool              { return true }
func (p *Player) GetPossessivePronoun() string { return "his" }

// ReportViolationEntry stores a single rule violation report submitted by a
// player against another character. Reports are typically routed to moderators
// via the violation channel system.
type ReportViolationEntry struct {
	ReporterID uint32
	Timestamp  time.Time
	Reason     byte
	Character  string
	Comment    string
}

func (p *Player) AddModalWindow(id uint32)      {}
func (p *Player) HasModalWindow(id uint32) bool { return false }
func (p *Player) RemoveModalWindow(id uint32)   {}
func (p *Player) ClearModalWindows()            {}

// GetStepHeight returns the step height for movement.
func (p *Player) GetStepHeight() uint8 { return 0 }

// GetStepDuration returns the walk animation duration.
func (p *Player) GetStepDuration(direction Direction) uint32 { return 500 }

// GetSleepTicks returns remaining sleep/afk ticks.
func (p *Player) GetSleepTicks() uint32 { return 0 }

// GetLastLoginSaved returns the last login timestamp.
func (p *Player) GetLastLoginSaved() uint64 { return p.LastLogin }

// GetLastLogoutSaved returns the last logout timestamp.
func (p *Player) GetLastLogoutSaved() uint64 { return p.LastLogout }

// IsAccessPlayer returns true for GM accounts.
func (p *Player) IsAccessPlayer() bool { return p.AccountType >= 4 }

// IsPlayerGroup checks if the player is in a specific group.
func (p *Player) IsPlayerGroup(group uint16) bool { return p.GroupID == group }

// GetGroupID returns the group ID.
func (p *Player) GetGroupID() uint16 { return p.GroupID }

// SetGroupID sets the group ID.
func (p *Player) SetGroupID(id uint16) { p.GroupID = id }

// GetDepotLocker returns the player's depot locker (autoCreate).
func (p *Player) GetDepotLocker(depotId uint16, autoCreate bool) *Item {
	if p.DepotManager != nil {
		return p.DepotManager.GetDepotLocker(depotId)
	}
	return nil
}

// GetDepotChest returns a depot chest (autoCreate).
func (p *Player) GetDepotChest(depotId uint16, autoCreate bool) *Item {
	if p.DepotManager != nil {
		return p.DepotManager.GetDepotChest(depotId, autoCreate)
	}
	return nil
}

// IsNearDepotBox checks if the player is near a depot.
func (p *Player) IsNearDepotBox() bool { return true }

// OnReceiveMail is called when mail arrives.
func (p *Player) OnReceiveMail() {
	if p.Session != nil {
		p.Session.SendTextMessage(0x15, "New mail has arrived in your inbox.")
	}
}

// GetMaxDepotItems returns the max items per depot.
func (p *Player) GetMaxDepotItems() uint32 { return 2000 }

// HasFlag checks if the player has a specific group flag.
func (p *Player) HasFlag(flag uint64) bool { return false }

// SendCancelMessage sends a cancel/error message.
func (p *Player) SendCancelMessage(msg string) {
	if p.Session != nil {
		p.Session.SendTextMessage(0x12, msg)
	}
}

// GetBaseMaxHealth returns base max health without equipment.
func (p *Player) GetBaseMaxHealth() uint32 { return p.MaxHealth }

// GetBaseMaxMana returns base max mana without equipment.
func (p *Player) GetBaseMaxMana() uint32 { return p.MaxMana }

// SetSpeed sets the current speed.
func (p *Player) SetSpeed(speed uint16) { p.Speed = speed }

// GetLight returns the current light level and color.
func (p *Player) GetLight() (uint8, uint8) { return p.LightLevel, p.LightColor }

// SetLight sets the current light.
func (p *Player) SetLight(level, color uint8) { p.LightLevel = level; p.LightColor = color }

// SetOutfit sets the current outfit.
func (p *Player) SetOutfit(outfit Outfit) { p.Outfit = outfit }

// GetBankBalance returns the bank balance.
func (p *Player) GetBankBalance() uint64 { return p.BankBalance }

// SetBankBalance sets the bank balance.
func (p *Player) SetBankBalance(balance uint64) { p.BankBalance = balance }

// GetCoinBalance returns Tibia Coins balance.
func (p *Player) GetCoinBalance() uint32 { return p.CoinBalance }

// SetTown sets the player's home town.
func (p *Player) SetTown(id uint16) { p.TownID = id }

// GetTown returns the player's town ID.
func (p *Player) GetTown() uint16 { return p.TownID }

// Kill is one entry of the unjustified-kill list (struct Kill,
// src/creatures/creatures_definitions.hpp:1613). Target is the victim's guid.
type Kill struct {
	Target    uint32
	Time      int64
	Unavenged bool
}
