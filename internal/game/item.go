package game

import (
	"fmt"
	"strings"
	"sync"

	"github.com/omurilo/canary-go/internal/io/propstream"
	"github.com/omurilo/canary-go/internal/items"
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
	// no attributes (the common case: the vast majority of map items carry
	// none). Decoded fields are authoritative over the raw blob, which is kept
	// inside ItemAttributes.Raw for undecodable blobs.
	Attr *ItemAttributes

	// Container points to container data if this item is a container.
	Container *Container


	// Imbuements maps slot index to the applied imbuement on this item instance.
	// The map is nil when no imbuements have ever been set on this item.
	Imbuements map[uint8]ImbuementInfo
	imbueMu    sync.Mutex
}

// ImbuementInfo holds an applied imbuement's identity and remaining duration.
type ImbuementInfo struct {
	ID       uint16
	Duration uint32
}

// RawAttributes returns the raw OTBR blob kept for round-tripping, or nil.
// The blob moved from Item into ItemAttributes so that items without any
// attributes do not carry a 24-byte slice header each.
func (i *Item) RawAttributes() []byte {
	if i == nil || i.Attr == nil {
		return nil
	}
	return i.Attr.Raw
}

// HasPagination returns whether this container supports paginated browsing
// (scroll offset). Mirrors C++ Container::hasPagination.
func (i *Item) HasPagination() bool { return i != nil && i.Container != nil && i.Container.Pagination }

// ContainerCapacity returns the container's slot capacity, preferring the
// stored MaxSize and falling back to the catalog's ItemType.Capacity. catalog
// may be nil (then only the stored MaxSize is used). Mirrors Container::capacity.
// DefaultContainerCapacity is the slot count used for containers whose type
// carries no explicit containersize (mirrors the C++ default). A standard
// backpack overrides this via items.xml (e.g. 20).
const DefaultContainerCapacity = 8

func (i *Item) ContainerCapacity(catalog *items.Catalog) uint16 {
	if i == nil {
		return 0
	}
	if catalog != nil {
		if t := catalog.Get(i.ID); t != nil && t.Capacity > 0 {
			return uint16(t.Capacity)
		}
	}
	if i.Container != nil && i.Container.MaxSize > 0 {
		return i.Container.MaxSize
	}
	if catalog != nil {
		if t := catalog.Get(i.ID); t != nil && t.IsContainer() {
			return DefaultContainerCapacity
		}
	}
	return 0
}

// IsContainer reports whether this item is a container per the catalog.
func (i *Item) IsContainer(catalog *items.Catalog) bool {
	if catalog == nil {
		return i.Container != nil
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
	if i.Container == nil {
		return 0
	}
	return i.Container.HoldingCount()
}

// GetTier returns the forge tier of the item (0..10).
func (i *Item) GetTier() uint8 {
	if i == nil || i.Attr == nil || i.Attr.Tier == nil {
		return 0
	}
	return *i.Attr.Tier
}

func (i *Item) GetImbuementInfo(slot uint8) (ImbuementInfo, bool) {
	if i == nil || i.Imbuements == nil {
		return ImbuementInfo{}, false
	}
	i.imbueMu.Lock()
	info, ok := i.Imbuements[slot]
	i.imbueMu.Unlock()
	if !ok || info.Duration == 0 || info.ID == 0 {
		return ImbuementInfo{}, false
	}
	return info, true
}

func (i *Item) SetImbuement(slot uint8, id uint16, duration uint32) {
	if i == nil {
		return
	}
	i.imbueMu.Lock()
	defer i.imbueMu.Unlock()
	if i.Imbuements == nil {
		i.Imbuements = make(map[uint8]ImbuementInfo)
	}
	i.Imbuements[slot] = ImbuementInfo{ID: id, Duration: duration}
}

func (i *Item) ClearImbuement(slot uint8) {
	if i == nil || i.Imbuements == nil {
		return
	}
	i.imbueMu.Lock()
	delete(i.Imbuements, slot)
	i.imbueMu.Unlock()
}

func (i *Item) HasImbuements() bool {
	if i == nil || i.Imbuements == nil {
		return false
	}
	i.imbueMu.Lock()
	defer i.imbueMu.Unlock()
	for _, info := range i.Imbuements {
		if info.Duration > 0 && info.ID > 0 {
			return true
		}
	}
	return false
}

const (
	attrCustomTag          = 41  // ATTR_CUSTOM
	imbuementCustomKeyBase = 500 // ITEM_IMBUEMENT_SLOT
)

// EncodeImbuementBlob encodes the item's imbuements into an ATTR_CUSTOM
// attribute blob suitable for appending to the item's attribute stream.
// Returns nil when imbuements is empty.
func EncodeImbuementBlob(imbMap map[uint8]ImbuementInfo) []byte {
	if len(imbMap) == 0 {
		return nil
	}
	w := propstream.NewPropWriteStream()
	w.WriteUint8(attrCustomTag)
	w.WriteUint64(uint64(len(imbMap)))
	for slot, info := range imbMap {
		key := fmt.Sprintf("%d", imbuementCustomKeyBase+int(slot))
		w.WriteString(key)
		w.WriteUint8(2) // type: int64
		packed := int64(info.Duration)<<8 | int64(info.ID)
		w.WriteInt64(packed)
	}
	return w.GetStream()
}

// DecodeImbuementBlob scans a raw item attribute blob for ATTR_CUSTOM sections
// containing imbuement entries (key == "500"+slot). Returns the decoded map.
// Uses a simple byte scan to find ATTR_CUSTOM (0x29) markers.
func DecodeImbuementBlob(blob []byte) map[uint8]ImbuementInfo {
	result := make(map[uint8]ImbuementInfo)
	if len(blob) == 0 {
		return nil
	}
	r := propstream.NewPropStream(blob)
	for r.Size() > 0 {
		tag, err := r.ReadUint8()
		if err != nil {
			break
		}
		if tag == 0 {
			break
		}
		if tag != attrCustomTag {
			if !skipAttrTag(r, tag) {
				break
			}
			continue
		}
		count, err := r.ReadUint64()
		if err != nil {
			break
		}
		for i := uint64(0); i < count; i++ {
			key, err := r.ReadString()
			if err != nil {
				break
			}
			valType, err := r.ReadUint8()
			if err != nil {
				break
			}
			switch valType {
			case 1: // string
				if _, err := r.ReadString(); err != nil {
					return nil
				}
			case 2: // int64
				packed, err := r.ReadInt64()
				if err != nil {
					return nil
				}
				slot, ok := isImbuementKey(key)
				if !ok {
					continue
				}
				id := uint16(uint64(packed) & 0xFF)
				duration := uint32(uint64(packed) >> 8)
				if id > 0 && duration > 0 {
					result[slot] = ImbuementInfo{ID: id, Duration: duration}
				}
			default:
				r.Skip(8)
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func skipAttrTag(r *propstream.PropStream, tag uint8) bool {
	switch tag {
	case attrStore: // int64
		_, err := r.ReadInt64()
		return err == nil
	case attrTileFlags: // uint64
		_, err := r.ReadUint64()
		return err == nil
	case attrActionID: // uint16
		_, err := r.ReadUint16()
		return err == nil
	case attrUniqueID: // uint16
		_, err := r.ReadUint16()
		return err == nil
	case attrTeleDest: // position (x: u16, y: u16, z: u8)
		if err := r.Skip(5); err != nil {
			return false
		}
		return true
	case attrItem: // uint16
		_, err := r.ReadUint16()
		return err == nil
	case attrDepotID: // uint16
		_, err := r.ReadUint16()
		return err == nil
	case attrHouseDoorID: // uint8
		_, err := r.ReadUint8()
		return err == nil
	case attrRuneCharges, attrCount: // uint8
		_, err := r.ReadUint8()
		return err == nil
	case attrDuration: // int32
		_, err := r.ReadInt32()
		return err == nil
	case attrDecayingState: // uint8
		_, err := r.ReadUint8()
		return err == nil
	case attrWrittenDate: // uint64
		_, err := r.ReadUint64()
		return err == nil
	case attrWrittenBy: // string
		_, err := r.ReadString()
		return err == nil
	case attrSleeperGUID: // uint32
		_, err := r.ReadUint32()
		return err == nil
	case attrSleepStart: // uint64
		_, err := r.ReadUint64()
		return err == nil
	case attrCharges: // uint16
		_, err := r.ReadUint16()
		return err == nil
	case attrText, attrDesc, attrSpecial, attrStoreInboxCategory: // string
		_, err := r.ReadString()
		return err == nil
	case attrName, attrArticle, attrPluralName: // string
		_, err := r.ReadString()
		return err == nil
	case attrWeight: // uint32
		_, err := r.ReadUint32()
		return err == nil
	case attrAttack, attrDefense, attrExtraDefense, attrArmor: // int32
		_, err := r.ReadInt32()
		return err == nil
	case attrHitChance: // int8
		_, err := r.ReadInt8()
		return err == nil
	case attrShootRange: // uint8
		_, err := r.ReadUint8()
		return err == nil
	case attrImbuementSlot, attrMantra: // int32
		_, err := r.ReadInt32()
		return err == nil
	case attrOpenContainer: // uint8
		_, err := r.ReadUint8()
		return err == nil
	case attrAmount: // uint16
		_, err := r.ReadUint16()
		return err == nil
	case attrTier: // uint8
		_, err := r.ReadUint8()
		return err == nil
	case attrOwner: // uint32
		_, err := r.ReadUint32()
		return err == nil
	case attrQuickLootContainer: // uint32
		_, err := r.ReadUint32()
		return err == nil
	case attrObtainContainer: // uint32
		_, err := r.ReadUint32()
		return err == nil
	default:
		return false
	}
}

func isImbuementKey(key string) (uint8, bool) {
	if len(key) < 3 {
		return 0, false
	}
	// key should be numeric, base 500 + slot
	n := 0
	for _, c := range key {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	if n < imbuementCustomKeyBase {
		return 0, false
	}
	return uint8(n - imbuementCustomKeyBase), true
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
	StoreTimestamp *int64    // ATTR_STORE (1)      int64
	HasCount       bool      // ATTR_COUNT (15) / ATTR_RUNE_CHARGES (12) -> Item.Count (u8)
	Charges        *uint16   // ATTR_CHARGES (22)   uint16
	ActionID       *uint16   // ATTR_ACTION_ID (4)  uint16
	UniqueID       *uint16   // ATTR_UNIQUE_ID (5)  uint16
	TeleDest       *Position // ATTR_TELE_DEST (8)
	Text           *string   // ATTR_TEXT (6)
	WrittenDate    *uint64   // ATTR_WRITTENDATE (18) uint64
	WrittenBy      *string   // ATTR_WRITTENBY (19)
	Description    *string   // ATTR_DESC (7)
	Duration       *int32    // ATTR_DURATION (16)  int32
	DecayState     *uint8    // ATTR_DECAYING_STATE (17) uint8
	// DurationTimestamp is ItemAttribute_t::DURATION_TIMESTAMP: the absolute
	// millisecond at which this item is due to decay. Decay::stopDecay finds the
	// item in the decay map by it, so a decaying item must carry one.
	DurationTimestamp *int64
	Name              *string // ATTR_NAME (24)
	Article           *string // ATTR_ARTICLE (25)
	PluralName        *string // ATTR_PLURALNAME (26)
	Weight            *uint32 // ATTR_WEIGHT (27)    uint32
	Attack            *int32  // ATTR_ATTACK (28)    int32
	Defense           *int32  // ATTR_DEFENSE (29)   int32
	ExtraDefense      *int32  // ATTR_EXTRADEFENSE (30) int32
	Armor             *int32  // ATTR_ARMOR (31)     int32
	HitChance         *int8   // ATTR_HITCHANCE (32) int8
	ShootRange        *uint8  // ATTR_SHOOTRANGE (33) uint8
	Tier              *uint8
	HouseDoorID       *uint8  // ATTR_TIER (40)      uint8
	Amount            *uint16 // ATTR_AMOUNT (39)    uint16
	Owner             *uint32 // ATTR_OWNER (43)     uint32
	OpenContainer     *uint8  // ATTR_OPENCONTAINER (36) uint8
	// QuickLootContainer / ObtainContainer are bitmasks of the ObjectCategory
	// values this container is the managed loot / obtain container for
	// (ATTR_QUICKLOOTCONTAINER 38 / ATTR_OBTAINCONTAINER 44, u32). They persist
	// the quick-loot assignment per container instance, like C++.
	QuickLootContainer *uint32
	ObtainContainer    *uint32

	// Raw is the raw OTBR attribute blob, kept as a round-trip fallback for
	// blobs that DecodeItemAttributes could not fully model (e.g. nested custom
	// attributes). It is written back verbatim on save when Encode produces no
	// bytes (i.e. the item has no decoded attributes). Keeping it here rather
	// than on Item saves 24 bytes of slice header on every item that has no
	// attributes at all — the common case on the map.
	Raw []byte

	// Custom holds ATTR_CUSTOM: arbitrary script-defined values keyed by name, the
	// Go side of Item::setCustomAttribute. C++ stores int64, double, string or bool
	// in a variant; the datapack uses it for podium looks, unwrap ids, hireling
	// state and quest bookkeeping. Keys are always strings — a numeric key is
	// stringified, as luaItemSetCustomAttribute does.
	Custom map[string]any
}

// SetCustomAttribute stores a script-defined value. Mirrors
// Item::setCustomAttribute; the caller is responsible for having normalised the key.
func (i *Item) SetCustomAttribute(key string, value any) {
	if i.Attr == nil {
		i.Attr = &ItemAttributes{}
	}
	if i.Attr.Custom == nil {
		i.Attr.Custom = map[string]any{}
	}
	i.Attr.Custom[key] = value
}

// GetCustomAttribute returns a script-defined value and whether it was set.
func (i *Item) GetCustomAttribute(key string) (any, bool) {
	if i.Attr == nil || i.Attr.Custom == nil {
		return nil, false
	}
	v, ok := i.Attr.Custom[key]
	return v, ok
}

// RemoveCustomAttribute drops a script-defined value, reporting whether it existed.
func (i *Item) RemoveCustomAttribute(key string) bool {
	if i.Attr == nil || i.Attr.Custom == nil {
		return false
	}
	if _, ok := i.Attr.Custom[key]; !ok {
		return false
	}
	delete(i.Attr.Custom, key)
	return true
}

// Outfit describes a creature's appearance.
type Outfit struct {
	LookType      uint16
	Head          uint8
	Body          uint8
	Legs          uint8
	Feet          uint8
	Addons        uint8
	LookTypeEx    uint16
	LookMount     uint16
	MountHead     uint8
	MountBody     uint8
	MountLegs     uint8
	MountFeet     uint8
	FamiliarsType uint16
	// OTCR extension fields for wings, auras, effects, and shaders.
	LookWing   uint16
	LookAura   uint16
	LookEffect uint16
	LookShader uint16
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

	if i.Container != nil {
		for _, child := range i.Container.Contents {
			total += child.GetWeight(catalog)
		}
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

// ShootType returns the projectile animation this item flies with, or
// CONST_ANI_NONE when it has none.
func (i *Item) ShootType(catalog *items.Catalog) items.ShootTypes {
	if catalog == nil {
		return items.ShootTypeNone
	}
	if t := catalog.Get(i.ID); t != nil {
		return t.ShootType
	}
	return items.ShootTypeNone
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
