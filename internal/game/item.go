package game

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/items"
)

// Item is an item instance: a client item id, a stack count/subtype, and the
// decoded OTBR attribute blob (see ItemAttributes).
//
// Count is the item's subtype (stack count for stackables, fluid subtype for
// splash/fluid containers). It mirrors C++ Item::getSubType(): the DB `count`
// column seeds it and, when present, the blob's ATTR_COUNT overrides it. The
// client encoder (addItem) reads Count directly.
type Item struct {
	ID    uint16
	Count uint16

	// Attr holds the decoded OTBR attribute TLV stream. Nil when the item has
	// no attributes. Decoded fields are authoritative over the raw blob.
	Attr *ItemAttributes

	// Attributes is the raw OTBR blob, kept as a round-trip fallback for blobs
	// that DecodeItemAttributes could not fully model (e.g. nested custom
	// attributes). It is written back verbatim on save only when Attr is nil;
	// otherwise Attr is authoritative.
	Attributes []byte

	// Contents holds the items inside a container item (chest, bag, ...), in
	// stack order. Empty for non-containers.
	Contents []*Item

	// Container metadata, mirroring C++ Container. MaxSize/MaxItems are the
	// per-slot capacities (Container::capacity / m_maxItems); they default to 0
	// and callers should fall back to ItemType.Capacity via ContainerCapacity.
	// Unlocked/Pagination drive the 0x6E open-container packet bytes. Parent is
	// the holding container/cylinder, needed for hasParent() and auto-close.
	MaxSize    uint16
	MaxItems   uint16
	Unlocked   bool
	Pagination bool
	Parent     *Item
	Actor      bool
}

// ContainerCapacity returns the container's slot capacity, preferring the
// stored MaxSize and falling back to the catalog's ItemType.Capacity. catalog
// may be nil (then only the stored MaxSize is used). Mirrors Container::capacity.
// DefaultContainerCapacity is the slot count used for containers whose type
// carries no explicit containersize (mirrors the C++ default). A standard
// backpack overrides this via items.xml (e.g. 20).
const DefaultContainerCapacity = 8

func (i *Item) ContainerCapacity(catalog *items.Catalog) uint16 {
	if i.MaxSize > 0 {
		return i.MaxSize
	}
	if catalog != nil {
		if t := catalog.Get(i.ID); t != nil {
			if t.Capacity > 0 {
				return uint16(t.Capacity)
			}
			// A container with no explicit size still has the default capacity;
			// never report 0 or the client shows a full, unexpandable window.
			if t.IsContainer() {
				return DefaultContainerCapacity
			}
		}
	}
	return 0
}

// IsContainer reports whether this item is a container per the catalog.
func (i *Item) IsContainer(catalog *items.Catalog) bool {
	if catalog == nil {
		return len(i.Contents) > 0
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.IsContainer()
	}
	return false
}

// IsQuiver reports whether this item is a quiver per the catalog.
func (i *Item) IsQuiver(catalog *items.Catalog) bool {
	if catalog == nil {
		return false
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.IsQuiver
	}
	return false
}

// HoldingCount returns the total number of items held recursively in this
// container (mirrors Container::getItemHoldingCount). Guards against cycles via
// a visited set is unnecessary for the acyclic inventory tree.
func (i *Item) HoldingCount() int {
	total := 0
	for _, child := range i.Contents {
		if child == nil {
			continue
		}
		total++
		if len(child.Contents) > 0 {
			total += child.HoldingCount()
		}
	}
	return total
}

// GetTier returns the forge tier of the item (0..10).
func (i *Item) GetTier() uint8 {
	if i == nil || i.Attr == nil || i.Attr.Tier == nil {
		return 0
	}
	return *i.Attr.Tier
}

// SetTier sets the forge tier of the item.
func (i *Item) SetTier(tier uint8) {
	if i == nil {
		return
	}
	if i.Attr == nil {
		i.Attr = &ItemAttributes{}
	}
	i.Attr.Tier = &tier
}

// ItemAttributes is the structured form of the OTBR ATTR_* TLV blob stored in
// the player_items.attributes column. Each optional field is a pointer whose
// non-nil value marks the attribute as present, so a decode→encode round-trip
// reproduces the same attribute set. Field order and wire widths mirror
// C++ Item::serializeAttr / Item::readAttr (src/items/item.cpp).
//
// The subtype (ATTR_COUNT / ATTR_RUNE_CHARGES) lives on Item.Count; HasCount
// records that the blob carried it so it is re-emitted on save.
type ItemAttributes struct {
	StoreTimestamp *int64  // ATTR_STORE (1)      int64
	HasCount       bool    // ATTR_COUNT (15) / ATTR_RUNE_CHARGES (12) -> Item.Count (u8)
	Charges        *uint16 // ATTR_CHARGES (22)   uint16
	ActionID       *uint16 // ATTR_ACTION_ID (4)  uint16
	UniqueID       *uint16 // ATTR_UNIQUE_ID (5)  uint16
	Text           *string // ATTR_TEXT (6)
	WrittenDate    *uint64 // ATTR_WRITTENDATE (18) uint64
	WrittenBy      *string // ATTR_WRITTENBY (19)
	Description    *string // ATTR_DESC (7)
	Duration       *int32  // ATTR_DURATION (16)  int32
	DecayState     *uint8  // ATTR_DECAYING_STATE (17) uint8
	Name           *string // ATTR_NAME (24)
	Article        *string // ATTR_ARTICLE (25)
	PluralName     *string // ATTR_PLURALNAME (26)
	Weight         *uint32 // ATTR_WEIGHT (27)    uint32
	Attack         *int32  // ATTR_ATTACK (28)    int32
	Defense        *int32  // ATTR_DEFENSE (29)   int32
	ExtraDefense   *int32  // ATTR_EXTRADEFENSE (30) int32
	Armor          *int32  // ATTR_ARMOR (31)     int32
	HitChance      *int8   // ATTR_HITCHANCE (32) int8
	ShootRange     *uint8  // ATTR_SHOOTRANGE (33) uint8
	Tier           *uint8  // ATTR_TIER (40)      uint8
	Amount         *uint16 // ATTR_AMOUNT (39)    uint16
	Owner          *uint32 // ATTR_OWNER (43)     uint32
	TeleDest       *Position // ATTR_TELE_DEST (8) x(u16) y(u16) z(u8)
	// QuickLootContainer / ObtainContainer are bitmasks of the ObjectCategory
	// values this container is the managed loot / obtain container for
	// (ATTR_QUICKLOOTCONTAINER 38 / ATTR_OBTAINCONTAINER 44, u32). They persist
	// the quick-loot assignment per container instance, like C++.
	QuickLootContainer *uint32
	ObtainContainer    *uint32
}

// Outfit describes a creature's appearance.
type Outfit struct {
	LookType   uint16
	Head       uint8
	Body       uint8
	Legs       uint8
	Feet       uint8
	Addons     uint8
	LookTypeEx uint16
	LookMount  uint16
	MountHead  uint8
	MountBody  uint8
	MountLegs  uint8
	MountFeet  uint8
	FamiliarsType uint16
}

// GetWeight returns the total weight of the item (and its contents if it's a container).
// Weight is in hundredths of an ounce (like capacity).
func (i *Item) GetWeight(catalog *items.Catalog) uint32 {
	weight := uint32(0)
	stackable := false
	
	if catalog != nil {
		if t := catalog.Get(i.ID); t != nil {
			weight = t.Weight
			stackable = t.Stackable
		}
	}
	
	if i.Attr != nil && i.Attr.Weight != nil {
		weight = *i.Attr.Weight
	}
	
	total := weight
	if stackable {
		count := uint32(i.Count)
		if count == 0 {
			count = 1
		}
		total = weight * count
	}

	for _, child := range i.Contents {
		total += child.GetWeight(catalog)
	}

	return total
}

const (
	ObjectCategoryNone             uint8 = 0
	ObjectCategoryArmors           uint8 = 1
	ObjectCategoryNecklaces        uint8 = 2
	ObjectCategoryBoots            uint8 = 3
	ObjectCategoryContainers       uint8 = 4
	ObjectCategoryDecoration       uint8 = 5
	ObjectCategoryFood             uint8 = 6
	ObjectCategoryHelmets          uint8 = 7
	ObjectCategoryLegs             uint8 = 8
	ObjectCategoryOthers           uint8 = 9
	ObjectCategoryPotions          uint8 = 10
	ObjectCategoryRings            uint8 = 11
	ObjectCategoryRunes            uint8 = 12
	ObjectCategoryShields          uint8 = 13
	ObjectCategoryTools            uint8 = 14
	ObjectCategoryValuables        uint8 = 15
	ObjectCategoryAmmo             uint8 = 16
	ObjectCategoryAxes             uint8 = 17
	ObjectCategoryClubs            uint8 = 18
	ObjectCategoryDistanceWeapons  uint8 = 19
	ObjectCategorySwords           uint8 = 20
	ObjectCategoryWands            uint8 = 21
	ObjectCategoryPremiumScrolls   uint8 = 22
	ObjectCategoryTibiaCoins       uint8 = 23
	ObjectCategoryCreatureProducts uint8 = 24
	ObjectCategoryQuivers          uint8 = 25
	ObjectCategoryFistWeapons      uint8 = 27
	ObjectCategoryGold             uint8 = 30
	ObjectCategoryDefault          uint8 = 31
)

// WeaponType returns the item's weapon type, e.g. "sword", "axe", "club", "distance", "wand", etc.
func (i *Item) WeaponType(catalog *items.Catalog) string {
	if catalog == nil {
		return ""
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.WeaponType
	}
	return ""
}

// GetObjectCategory categorizes an item for quick loot / stash routing.
func (i *Item) GetObjectCategory(catalog *items.Catalog) uint8 {
	if catalog == nil {
		return ObjectCategoryNone
	}

	// Example: Gold items have worth (we could hardcode IDs for gold, platinum, crystal coin)
	if i.ID == 2148 || i.ID == 2152 || i.ID == 2160 {
		return ObjectCategoryGold
	}

	it := catalog.Get(i.ID)
	if it == nil {
		return ObjectCategoryDefault
	}

	// Exact item-type routing (items.xml `type`) takes precedence over the name
	// heuristics below. Only a few categories are typed in items.xml (rune);
	// food/potion/valuables fall through to the heuristic.
	switch it.TypeName {
	case "rune":
		return ObjectCategoryRunes
	}

	// 1. Weapon checks
	if it.WeaponType != "" && it.WeaponType != "none" {
		switch it.WeaponType {
		case "fist":
			return ObjectCategoryFistWeapons
		case "sword":
			return ObjectCategorySwords
		case "club":
			return ObjectCategoryClubs
		case "axe":
			return ObjectCategoryAxes
		case "shield":
			return ObjectCategoryShields
		case "distance", "missile":
			return ObjectCategoryDistanceWeapons
		case "wand":
			return ObjectCategoryWands
		case "ammunition", "ammo":
			return ObjectCategoryAmmo
		}
	} else if it.SlotPosition != "" && it.SlotPosition != "hand" && it.SlotPosition != "two-handed" { // Check slots
		// We do a simple contains or match
		slot := it.SlotPosition
		if strings.Contains(slot, "head") {
			return ObjectCategoryHelmets
		} else if strings.Contains(slot, "necklace") {
			return ObjectCategoryNecklaces
		} else if strings.Contains(slot, "backpack") {
			return ObjectCategoryContainers
		} else if strings.Contains(slot, "armor") || strings.Contains(slot, "body") {
			return ObjectCategoryArmors
		} else if strings.Contains(slot, "legs") {
			return ObjectCategoryLegs
		} else if strings.Contains(slot, "feet") {
			return ObjectCategoryBoots
		} else if strings.Contains(slot, "ring") {
			return ObjectCategoryRings
		}
	} else {
		// Based on name or string tags as fallback (since we lack full item flags)
		name := strings.ToLower(it.Name)
		if strings.Contains(name, "rune") {
			return ObjectCategoryRunes
		} else if strings.Contains(name, "potion") {
			return ObjectCategoryPotions
		} else if strings.Contains(name, "food") || strings.Contains(name, "meat") || strings.Contains(name, "fish") || strings.Contains(name, "cheese") || strings.Contains(name, "apple") {
			return ObjectCategoryFood
		} else if strings.Contains(name, "creature product") {
			return ObjectCategoryCreatureProducts
		} else {
			// Some heuristics for missing types
			return ObjectCategoryOthers
		}
	}

	return ObjectCategoryDefault
}

// Attack returns the item's attack value, prioritizing custom attributes.
func (i *Item) Attack(catalog *items.Catalog) int32 {
	if i.Attr != nil && i.Attr.Attack != nil {
		return *i.Attr.Attack
	}
	if catalog == nil {
		return 0
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.Attack
	}
	return 0
}

// Defense returns the item's defense value, prioritizing custom attributes.
func (i *Item) Defense(catalog *items.Catalog) int32 {
	if i.Attr != nil && i.Attr.Defense != nil {
		return *i.Attr.Defense
	}
	if catalog == nil {
		return 0
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.Defense
	}
	return 0
}

// ExtraDefense returns the item's extra defense value, prioritizing custom attributes.
func (i *Item) ExtraDefense(catalog *items.Catalog) int32 {
	if i.Attr != nil && i.Attr.ExtraDefense != nil {
		return *i.Attr.ExtraDefense
	}
	if catalog == nil {
		return 0
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.ExtraDefense
	}
	return 0
}

// Armor returns the item's armor value, prioritizing custom attributes.
func (i *Item) Armor(catalog *items.Catalog) int32 {
	if i.Attr != nil && i.Attr.Armor != nil {
		return *i.Attr.Armor
	}
	if catalog == nil {
		return 0
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.Armor
	}
	return 0
}

// Range returns the weapon range, prioritizing custom attributes.
func (i *Item) Range(catalog *items.Catalog) int32 {
	if i.Attr != nil && i.Attr.ShootRange != nil {
		return int32(*i.Attr.ShootRange)
	}
	if catalog == nil {
		return 0
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.Range
	}
	return 0
}

// ShootType returns the projectile/missile animation shoot type of this item.
func (i *Item) ShootType(catalog *items.Catalog) string {
	if catalog == nil {
		return ""
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.ShootType
	}
	return ""
}

// AmmoType returns the ammunition type used or provided by this item.
func (i *Item) AmmoType(catalog *items.Catalog) string {
	if catalog == nil {
		return ""
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.AmmoType
	}
	return ""
}

// Worth returns the monetary value of an item based on its ID.
func (i *Item) Worth() uint64 {
	switch i.ID {
	case 2148: // gold coin
		return 1
	case 2152: // platinum coin
		return 100
	case 2160: // crystal coin
		return 10000
	}
	return 0
}

