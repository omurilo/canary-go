package creatures

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/opentibiabr/canary-go/internal/bosstiary"
)

type Outfit struct {
	LookType  uint16
	Head      uint8
	Body      uint8
	Legs      uint8
	Feet      uint8
	Addons    uint8
	LookMount uint16
}

type MonsterType struct {
	Name           string
	Speed          uint32
	MaxHealth      uint32
	TargetDistance int32
	Experience     uint64 // exp awarded to the killer (MonsterType::info.experience)
	Corpse     uint16 // corpse item id dropped on death (0 = unknown)
	RaceID     uint16 // bestiary race id (monster.raceId)
	// BestiaryStars is the monster's bestiary difficulty tier (1..5, from
	// monster.Bestiary.Stars). It drives task-hunting difficulty and prey grid
	// staging. 0 means "not in the bestiary".
	BestiaryStars uint8

	// Bestiary (Cyclopedia) entry data, from the monster.Bestiary block.
	// BestiaryRace is a BESTY_RACE_* class; the *Unlock/ToKill kill thresholds
	// stage the entry (getKillStatus); CharmsPoints is awarded on completion.
	BestiaryClass        string
	BestiaryRace         uint8
	BestiaryFirstUnlock  uint32
	BestiarySecondUnlock uint32
	BestiaryToKill       uint32
	BestiaryCharmsPoints uint16
	BestiaryOccurrence   uint8

	// Bosstiary (Boss Cyclopedia). BosstiaryRaceID is the boss's cyclopedia race
	// id (monster.bosstiary.bossRaceId), distinct from the bestiary RaceID.
	// BosstiaryRace is the rarity class (Bane/Archfoe/Nemesis) that determines
	// the kill thresholds and points. A zero BosstiaryRaceID means "not a boss".
	BosstiaryRaceID uint16
	BosstiaryRace   bosstiary.Rarity

	Outfit Outfit

	// Attacks holds the monster's attack blocks. Only melee is applied by the
	// combat engine today; spell/distance attacks are captured verbatim as data
	// so the future spells agent can execute them. Mirrors
	// MonsterType::info.attackSpells (src/creatures/monsters/monsters.hpp).
	Attacks []MonsterAttack

	// Loot is the monster's loot table, rolled into the corpse on death. Mirrors
	// MonsterType::info.lootItems (src/creatures/monsters/monsters.hpp).
	Loot []LootBlock

	Flags MonsterFlags
	Elements map[uint32]int16
}

type MonsterAttack struct {
	Name            string // "melee" or a spell/combat-type name
	Interval        int    // ms between attempts (default 2000)
	Chance          int    // 0..100 chance per attempt
	MinDamage       int    // raw min combat value (melee: usually 0)
	MaxDamage       int    // raw max combat value (melee: usually negative)
	Range           int    // shoot range for distance/spell attacks; 0 = melee/adjacent
	Effect          uint16 // visual magic effect on target (CONST_ME_*)
	ShootEffect     uint16 // distance missile effect (CONST_ANI_*)
	CastSound       string // audio cast identifier
	ImpactSound     string // audio impact identifier
	CombatType      string // damage type (physical, fire, ice, energy, etc.)
	ConditionType   string // condition type (poison, speed, paralyze)
	Duration        int    // condition duration in ms
	SpeedChange     int    // speed change for conditions
	NeedTarget      bool   // if true, requires target to cast
	Length          int    // wave length
	Spread          int    // wave spread
	Radius          int    // area radius
	OutfitMonster   string // monster name for transform
	OutfitItem      int    // item ID for transform
	ConditionDamage int    // damage over time
	TickInterval    int    // speed of ticks
	ScriptName      string // Lua file script name
}

// IsMelee reports whether this is the basic adjacent melee attack.
func (a MonsterAttack) IsMelee() bool { return a.Name == "melee" }

// LootBlock mirrors the C++ LootBlock (src/creatures/creatures_definitions.hpp:1763).
type LootBlock struct {
	ID        uint16 // resolved client/item id (0 until resolved by name)
	Name      string // item name when the Lua entry used `name` instead of `id`
	Chance    uint32 // out of MAX_LOOTCHANCE (100000)
	CountMin  uint32
	CountMax  uint32
	SubType   int32
	ChildLoot []LootBlock // nested loot for container items
}

// MonsterFlags captures the key monster.flags fields. Mirrors the flags parsed
// in MonsterType::info (src/creatures/monsters/monsters.hpp).
type MonsterFlags struct {
	Summonable         bool
	Attackable         bool
	Hostile            bool
	Convinceable       bool
	Pushable           bool
	RewardBoss         bool
	Illusionable       bool
	CanPushItems       bool
	CanPushCreatures   bool
	StaticAttackChance int
	TargetDistance     int
	RunHealth          int
	HealthHidden       bool
	CanWalkOnEnergy    bool
	CanWalkOnFire      bool
	CanWalkOnPoison    bool
	// LootDrop mirrors MonsterType::info.lootDrop; false disables loot entirely.
	LootDrop bool
}

type ShopItem struct {
	ID        uint16
	SubType   uint8
	Name      string
	BuyPrice  uint32
	SellPrice uint32
}

type NpcType struct {
	Name      string
	Speed     uint32
	MaxHealth uint32
	Outfit    Outfit
	ShopItems []ShopItem
}

type TypeRegistry struct {
	Monsters map[string]*MonsterType
	Npcs     map[string]*NpcType
}

func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		Monsters: make(map[string]*MonsterType),
		Npcs:     make(map[string]*NpcType),
	}
}

// IsBoss reports whether this monster type is a bosstiary boss.
func (m *MonsterType) IsBoss() bool {
	return m != nil && m.BosstiaryRaceID != 0 && bosstiary.IsBoss(m.BosstiaryRace)
}

// MonsterByBossRaceID returns the boss monster type with the given bosstiary
// race id (monster.bosstiary.bossRaceId), or nil. Mirrors
// IOBosstiary::getMonsterTypeByBossRaceId.
func (r *TypeRegistry) MonsterByBossRaceID(raceID uint16) *MonsterType {
	if r == nil || raceID == 0 {
		return nil
	}
	for _, m := range r.Monsters {
		if m.BosstiaryRaceID == raceID {
			return m
		}
	}
	return nil
}

// MonsterByRaceID returns the monster type with the given bestiary race id, or
// nil.
func (r *TypeRegistry) MonsterByRaceID(raceID uint16) *MonsterType {
	if r == nil || raceID == 0 {
		return nil
	}
	for _, m := range r.Monsters {
		if m.RaceID == raceID {
			return m
		}
	}
	return nil
}

// BosstiaryMonsters returns all boss monster types keyed by bosstiary race id
// (the boss list backing the Boss Cyclopedia).
func (r *TypeRegistry) BosstiaryMonsters() map[uint16]*MonsterType {
	out := make(map[uint16]*MonsterType)
	if r == nil {
		return out
	}
	for _, m := range r.Monsters {
		if m.IsBoss() {
			out[m.BosstiaryRaceID] = m
		}
	}
	return out
}

type xmlMonster struct {
	Name       string      `xml:"name,attr"`
	Speed      uint32      `xml:"speed,attr"`
	Experience uint64      `xml:"experience,attr"`
	Health     xmlHealth   `xml:"health"`
	Look       xmlLook     `xml:"look"`
	Elements   xmlElements `xml:"elements"`
}

type xmlElements struct {
	Element []xmlElement `xml:"element"`
}

type xmlElement struct {
	PhysicalPercent  *int16 `xml:"physicalPercent,attr"`
	FirePercent      *int16 `xml:"firePercent,attr"`
	IcePercent       *int16 `xml:"icePercent,attr"`
	EnergyPercent    *int16 `xml:"energyPercent,attr"`
	EarthPercent     *int16 `xml:"earthPercent,attr"`
	DeathPercent     *int16 `xml:"deathPercent,attr"`
	HolyPercent      *int16 `xml:"holyPercent,attr"`
	DrownPercent     *int16 `xml:"drownPercent,attr"`
	LifeDrainPercent *int16 `xml:"lifedrainPercent,attr"`
	ManaDrainPercent *int16 `xml:"manadrainPercent,attr"`
}

type xmlNpc struct {
	Name   string    `xml:"name,attr"`
	Speed  uint32    `xml:"speed,attr"`
	Health xmlHealth `xml:"health"`
	Look   xmlLook   `xml:"look"`
}

type xmlHealth struct {
	Max uint32 `xml:"max,attr"`
	Now uint32 `xml:"now,attr"`
}

type xmlLook struct {
	Type   uint16 `xml:"type,attr"`
	Head   uint8  `xml:"head,attr"`
	Body   uint8  `xml:"body,attr"`
	Legs   uint8  `xml:"legs,attr"`
	Feet   uint8  `xml:"feet,attr"`
	Addons uint8  `xml:"addons,attr"`
	Mount  uint16 `xml:"mount,attr"`
	Corpse uint16 `xml:"corpse,attr"`
}

func (r *TypeRegistry) LoadMonsters(dataDir string) error {
	return filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".xml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var mon xmlMonster
		if err := xml.Unmarshal(data, &mon); err == nil && mon.Name != "" {
			mType := &MonsterType{
				Name:       mon.Name,
				Speed:      mon.Speed,
				MaxHealth:  mon.Health.Max,
				Experience: mon.Experience,
				Corpse:     mon.Look.Corpse,
				Outfit: Outfit{
					LookType:  mon.Look.Type,
					Head:      mon.Look.Head,
					Body:      mon.Look.Body,
					Legs:      mon.Look.Legs,
					Feet:      mon.Look.Feet,
					Addons:    mon.Look.Addons,
					LookMount: mon.Look.Mount,
				},
				Elements: make(map[uint32]int16),
			}
			for _, el := range mon.Elements.Element {
				if el.PhysicalPercent != nil {
					mType.Elements[1] = *el.PhysicalPercent
				}
				if el.EnergyPercent != nil {
					mType.Elements[2] = *el.EnergyPercent
				}
				if el.EarthPercent != nil {
					mType.Elements[4] = *el.EarthPercent
				}
				if el.FirePercent != nil {
					mType.Elements[8] = *el.FirePercent
				}
				if el.DeathPercent != nil {
					mType.Elements[64] = *el.DeathPercent
				}
				if el.IcePercent != nil {
					mType.Elements[128] = *el.IcePercent
				}
				if el.HolyPercent != nil {
					mType.Elements[256] = *el.HolyPercent
				}
				if el.ManaDrainPercent != nil {
					mType.Elements[512] = *el.ManaDrainPercent
				}
				if el.LifeDrainPercent != nil {
					mType.Elements[1024] = *el.LifeDrainPercent
				}
			}
			if mType.MaxHealth == 0 {
				mType.MaxHealth = mon.Health.Now
			}
			if mType.MaxHealth == 0 {
				mType.MaxHealth = 100 // fallback
			}
			if mType.Speed == 0 {
				mType.Speed = 200 // default speed
			}
			r.Monsters[strings.ToLower(mon.Name)] = mType
		}

		return nil
	})
}

func (r *TypeRegistry) LoadNpcs(dataDir string) error {
	return filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".xml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var npc xmlNpc
		if err := xml.Unmarshal(data, &npc); err == nil && npc.Name != "" {
			nType := &NpcType{
				Name:      npc.Name,
				Speed:     npc.Speed,
				MaxHealth: npc.Health.Max,
				Outfit: Outfit{
					LookType:  npc.Look.Type,
					Head:      npc.Look.Head,
					Body:      npc.Look.Body,
					Legs:      npc.Look.Legs,
					Feet:      npc.Look.Feet,
					Addons:    npc.Look.Addons,
					LookMount: npc.Look.Mount,
				},
			}
			if nType.MaxHealth == 0 {
				nType.MaxHealth = npc.Health.Now
			}
			if nType.MaxHealth == 0 {
				nType.MaxHealth = 100
			}
			if nType.Speed == 0 {
				nType.Speed = 100
			}
			r.Npcs[strings.ToLower(npc.Name)] = nType
		}

		return nil
	})
}
