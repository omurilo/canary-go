package creatures

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	Name       string
	Speed      uint32
	MaxHealth  uint32
	Experience uint64 // exp awarded to the killer (MonsterType::info.experience)
	Corpse     uint16 // corpse item id dropped on death (0 = unknown)
	RaceID     uint16 // bestiary race id (monster.raceId)
	Outfit     Outfit

	// Attacks holds the monster's attack blocks. Only melee is applied by the
	// combat engine today; spell/distance attacks are captured verbatim as data
	// so the future spells agent can execute them. Mirrors
	// MonsterType::info.attackSpells (src/creatures/monsters/monsters.hpp).
	Attacks []MonsterAttack

	// Loot is the monster's loot table, rolled into the corpse on death. Mirrors
	// MonsterType::info.lootItems (src/creatures/monsters/monsters.hpp).
	Loot []LootBlock

	Flags MonsterFlags
}

// MonsterAttack mirrors one entry of monster.attacks. For "melee" the damage is
// MinDamage..MaxDamage (raw combat values, typically <= 0). Non-melee attacks
// keep Name (spell name / combat type) plus Range for the spells agent.
// See spellBlock_t / Monsters::deserializeSpell (src/creatures/monsters/monsters.cpp).
type MonsterAttack struct {
	Name      string // "melee" or a spell/combat-type name
	Interval  int    // ms between attempts (default 2000)
	Chance    int    // 0..100 chance per attempt
	MinDamage int    // raw min combat value (melee: usually 0)
	MaxDamage int    // raw max combat value (melee: usually negative)
	Range     int    // shoot range for distance/spell attacks; 0 = melee/adjacent
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

type xmlMonster struct {
	Name   string    `xml:"name,attr"`
	Speed  uint32    `xml:"speed,attr"`
	Health xmlHealth `xml:"health"`
	Look   xmlLook   `xml:"look"`
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
				Name:      mon.Name,
				Speed:     mon.Speed,
				MaxHealth: mon.Health.Max,
				Corpse:    mon.Look.Corpse,
				Outfit: Outfit{
					LookType:  mon.Look.Type,
					Head:      mon.Look.Head,
					Body:      mon.Look.Body,
					Legs:      mon.Look.Legs,
					Feet:      mon.Look.Feet,
					Addons:    mon.Look.Addons,
					LookMount: mon.Look.Mount,
				},
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
