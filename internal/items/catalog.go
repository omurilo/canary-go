// Package items loads client item metadata from appearances.dat (the Tibia
// protobuf appearance catalog) and exposes the flags the protocol and gameplay
// need. Item ids here are CLIENT ids, which is what the OTBM map and the
// AddItem wire encoding use in Canary 13.x.
package items

import (
	"fmt"
	"os"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/omurilo/canary-go/internal/appproto"
)

// AugmentType mirrors the C++ enum Augment_t (creatures_definitions.hpp).
type AugmentType uint8

const (
	AugmentNone               AugmentType = 0
	AugmentSpellDamage        AugmentType = 1
	AugmentSpellCooldown      AugmentType = 2
	AugmentSpellGroupCooldown AugmentType = 3
	AugmentSkill              AugmentType = 4
	AugmentManaLeech          AugmentType = 5
	AugmentLifeLeech          AugmentType = 6
	AugmentCriticalChance     AugmentType = 7
	AugmentCriticalDamage     AugmentType = 8
	AugmentMagicLevel         AugmentType = 9
)

// AugmentInfo mirrors C++ struct AugmentInfo (items.hpp).
type AugmentInfo struct {
	SpellName string
	Type      AugmentType
	Value     int32
}

// Group is the single-valued item group derived from the appearance flags,
// following the C++ precedence: container > ground > fluid-container > splash.
type Group uint8

const (
	GroupNone Group = iota
	GroupContainer
	GroupGround
	GroupFluid
	GroupSplash
)

// ItemType holds the metadata needed to encode AddItem and drive basic gameplay.
type ItemType struct {
	ID          uint16
	Name        string
	PluralName  string
	Article     string
	Description string
	Group       Group

	Stackable bool
	// StackSize is the maximum count a single stack of this type may hold
	// (C++ ItemType::stackSize). Stackable/cumulative types default to 100;
	// non-stackable types are 1. Consumed by inventory add splitting, shop
	// delivery, and coin conversion.
	StackSize             uint16
	Podium                bool
	UpgradeClassification uint8
	Expire                bool
	ExpireStop            bool
	ClockExpire           bool
	WearOut               bool
	WrapKit               bool

	GroundSpeed uint16
	BlockSolid  bool // unpass: blocks walking
	Pickupable  bool
	// HasHeight marks items with an elevation (appearances height flag). A tile
	// with 3+ stacked HasHeight items is a "step" the player can climb up from,
	// and one below an empty tile is a step to descend onto (Tibia stair logic,
	// Tile::hasHeight / Game::internalMoveCreature).
	HasHeight bool

	// AlwaysOnTopOrder mirrors ItemType::alwaysOnTopOrder (items.cpp): clip=1,
	// bottom=2, top=3, else 0. Items with order > 0 stack BELOW creatures on a
	// tile (the "top items"); order 0 items stack above creatures ("down items").
	AlwaysOnTopOrder uint8

	SlotPosition string
	SlotType     string
	WeaponType   string
	// TypeName is the items.xml `type` attribute (e.g. "rune", "container",
	// "teleport"). Used by getObjectCategory for exact type-based loot routing.
	TypeName string
	// Type is TypeName resolved to its ItemTypes_t value, which is what
	// ItemType:getType reports.
	Type        ItemTypes
	FloorChange string
	ForceUse    bool
	IsLadder    bool
	IsDoor      bool
	IsQuiver    bool

	Weight       uint32
	Armor        int32
	Attack       int32
	Defense      int32
	FluidSource  uint16
	ExtraDefense int32
	DecayTo      uint16
	Duration     uint32
	ShowDuration bool
	Charges      uint32
	ShowCharges  bool
	Capacity     uint32

	ElementType      uint8
	ElementDamage    uint16
	ImbuementSlot    uint8
	MinReqLevel      uint16
	MinReqMagicLevel uint16
	DecayTime        uint32

	MaxHitChance int32
	HitChance    int32
	Range        int32
	ShootRange   uint8
	// IsCorpse comes from the appearance flags, not items.xml (items.cpp:233).
	IsCorpse bool
	// The remaining blocking/orientation flags Item::hasProperty answers with
	// (items.cpp:237-251). BlockSolid above is unpass; these complete the set.
	BlockProjectile bool // unsight
	BlockPathFind   bool // avoid
	Movable         bool // !unmove
	IsVertical      bool // hook direction south
	IsHorizontal    bool // hook direction east
	// ShootType is the numeric ShootType_t, as C++ stores it. Weapon:shootType sets
	// it from Lua, so it cannot be the items.xml name string it used to be.
	ShootType ShootTypes
	AmmoType  string

	TransformEquipTo   uint16
	TransformDeEquipTo uint16
	WrapableTo         uint16
	DestroyID          uint16

	// Stats stores values like "skillsword", "absorbpercentfire", "elementice", "magiclevelpoints", etc.
	Stats map[string]int32

	// Augments stores structured spell-modifier data from items.xml.
	Augments []AugmentInfo

	Speed          int32
	BaseSpeed      int32
	VocationString string
}

// AlwaysOnTop reports whether the item stacks below creatures on a tile.
func (t *ItemType) AlwaysOnTop() bool { return t.AlwaysOnTopOrder > 0 }

// ItemTypes is ItemTypes_t (items_definitions.hpp:140). It is the value behind
// ItemType:getType, which the datapack compares against the ITEM_TYPE_ globals.
type ItemTypes int

const (
	ItemTypeNone            ItemTypes = 0
	ItemTypeArmor           ItemTypes = 1
	ItemTypeAmulet          ItemTypes = 2
	ItemTypeBoots           ItemTypes = 3
	ItemTypeContainer       ItemTypes = 4
	ItemTypeDecoration      ItemTypes = 5
	ItemTypeFood            ItemTypes = 6
	ItemTypeHelmet          ItemTypes = 7
	ItemTypeLegs            ItemTypes = 8
	ItemTypeOther           ItemTypes = 9
	ItemTypePotion          ItemTypes = 10
	ItemTypeRing            ItemTypes = 11
	ItemTypeRune            ItemTypes = 12
	ItemTypeShield          ItemTypes = 13
	ItemTypeTools           ItemTypes = 14
	ItemTypeValuable        ItemTypes = 15
	ItemTypeAmmo            ItemTypes = 16
	ItemTypeAxe             ItemTypes = 17
	ItemTypeClub            ItemTypes = 18
	ItemTypeDistance        ItemTypes = 19
	ItemTypeSword           ItemTypes = 20
	ItemTypeWand            ItemTypes = 21
	ItemTypePremiumScroll   ItemTypes = 22
	ItemTypeTibiaCoin       ItemTypes = 23
	ItemTypeCreatureProduct ItemTypes = 24
	ItemTypeQuiver          ItemTypes = 25
	ItemTypeSoulCores       ItemTypes = 26
	ItemTypeFist            ItemTypes = 27
	ItemTypeDepot           ItemTypes = 28
	ItemTypeMailbox         ItemTypes = 29
	ItemTypeTrashHolder     ItemTypes = 30
	ItemTypeDoor            ItemTypes = 31
	ItemTypeMagicField      ItemTypes = 32
	ItemTypeTeleport        ItemTypes = 33
	ItemTypeBed             ItemTypes = 34
	ItemTypeKey             ItemTypes = 35
	ItemTypeSupply          ItemTypes = 36
	ItemTypeRewardChest     ItemTypes = 37
	ItemTypeCarpet          ItemTypes = 38
	ItemTypeRetrieve        ItemTypes = 39
	ItemTypeGold            ItemTypes = 40
	ItemTypeUnassigned      ItemTypes = 41
	ItemTypeLadder          ItemTypes = 42
	ItemTypeDummy           ItemTypes = 43
)

// itemTypesByName is item_parse.hpp:168 ItemTypesMap: the items.xml `type`
// attribute values C++ recognises. Anything else leaves the type as NONE.
var itemTypesByName = map[string]ItemTypes{
	"key":             ItemTypeKey,
	"magicfield":      ItemTypeMagicField,
	"container":       ItemTypeContainer,
	"depot":           ItemTypeDepot,
	"rewardchest":     ItemTypeRewardChest,
	"carpet":          ItemTypeCarpet,
	"mailbox":         ItemTypeMailbox,
	"trashholder":     ItemTypeTrashHolder,
	"teleport":        ItemTypeTeleport,
	"door":            ItemTypeDoor,
	"bed":             ItemTypeBed,
	"rune":            ItemTypeRune,
	"supply":          ItemTypeSupply,
	"creatureproduct": ItemTypeCreatureProduct,
	"food":            ItemTypeFood,
	"valuable":        ItemTypeValuable,
	"potion":          ItemTypePotion,
	"soulcore":        ItemTypeSoulCores,
	"ladder":          ItemTypeLadder,
	"dummy":           ItemTypeDummy,
}

// ItemTypeByName resolves an items.xml `type` attribute to its ItemTypes_t value.
func ItemTypeByName(name string) ItemTypes { return itemTypesByName[strings.ToLower(name)] }

func (t *ItemType) IsContainer() bool      { return t.Group == GroupContainer }
func (t *ItemType) IsGround() bool         { return t.Group == GroupGround }
func (t *ItemType) IsFluidContainer() bool { return t.ID > 1 && t.Group == GroupFluid }
func (t *ItemType) IsSplash() bool         { return t.Group == GroupSplash }
func (t *ItemType) IsMailbox() bool        { return t.TypeName == "mailbox" }

// HasSubType reports whether the item's count field carries a subtype rather
// than a stack size (C++ ItemType::hasSubType, items.hpp:188).
func (t *ItemType) HasSubType() bool {
	return t.IsFluidContainer() || t.IsSplash() || t.Stackable || t.Charges != 0
}

func (t *ItemType) GetWeight() uint32 {
	if t == nil {
		return 0
	}
	return t.Weight
}

// Catalog maps client item ids to their metadata.
type Catalog struct {
	byID       map[uint16]*ItemType
	byName     map[string]uint16 // lower-cased name -> first id seen
	byClientID map[uint16]uint16 // client ID -> server item ID
}

// NewCatalog builds a catalog from an explicit set of item types, indexing them
// by id and (first-seen) name. Primarily for tests and synthetic setups; the
// production path is Load.
func NewCatalog(types ...*ItemType) *Catalog {
	c := &Catalog{byID: make(map[uint16]*ItemType), byName: make(map[string]uint16)}
	for _, t := range types {
		if t == nil {
			continue
		}
		c.byID[t.ID] = t
		if t.Name != "" {
			if lname := strings.ToLower(t.Name); c.byName[lname] == 0 {
				c.byName[lname] = t.ID
			}
		}
	}
	return c
}

// IDByName resolves an item id from its (case-insensitive) name, mirroring
// Item::items.getItemIdByName used by the C++ loot loader. Returns (0, false)
// when unknown.
func (c *Catalog) IDByName(name string) (uint16, bool) {
	if c == nil || c.byName == nil {
		return 0, false
	}
	id, ok := c.byName[strings.ToLower(name)]
	return id, ok
}

// Get returns the item type or nil if unknown.
func (c *Catalog) Get(id uint16) *ItemType {
	if c == nil {
		return nil
	}
	return c.byID[id]
}

// Len returns the number of loaded item types.
func (c *Catalog) Len() int { return len(c.byID) }

// Load parses an appearances.dat protobuf file into a Catalog.
func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("items: read %s: %w", path, err)
	}
	var app appproto.Appearances
	if err := proto.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("items: unmarshal appearances: %w", err)
	}

	cat := &Catalog{
		byID:   make(map[uint16]*ItemType, len(app.GetObject())),
		byName: make(map[string]uint16, len(app.GetObject())),
	}
	for _, obj := range app.GetObject() {
		if obj.GetId() == 0 || obj.GetId() > 0xFFFF {
			continue
		}
		it := &ItemType{ID: uint16(obj.GetId()), Name: string(obj.GetName())}
		if f := obj.GetFlags(); f != nil {
			switch {
			case f.GetContainer():
				it.Group = GroupContainer
			case f.GetBank() != nil:
				it.Group = GroupGround
				it.GroundSpeed = uint16(f.GetBank().GetWaypoints())
			case f.GetLiquidcontainer():
				it.Group = GroupFluid
			case f.GetLiquidpool():
				it.Group = GroupSplash
			}
			it.Stackable = f.GetCumulative()
			if it.Stackable {
				it.StackSize = 100
			} else {
				it.StackSize = 1
			}
			it.Podium = f.GetShowOffSocket()
			it.Expire = f.GetExpire()
			it.ExpireStop = f.GetExpirestop()
			it.ClockExpire = f.GetClockexpire()
			it.WearOut = f.GetWearout()
			it.WrapKit = f.GetWrapkit()
			// ItemType::isCorpse (items.cpp:233): either flag counts.
			it.IsCorpse = f.GetCorpse() || f.GetPlayerCorpse()
			it.BlockProjectile = f.GetUnsight()
			it.BlockPathFind = f.GetAvoid()
			it.Movable = !f.GetUnmove()
			if h := f.GetHook(); h != nil {
				it.IsVertical = h.GetDirection() == appproto.HOOK_TYPE_HOOK_TYPE_SOUTH
				it.IsHorizontal = h.GetDirection() == appproto.HOOK_TYPE_HOOK_TYPE_EAST
			}
			it.BlockSolid = f.GetUnpass()
			it.Pickupable = f.GetTake()
			if h := f.GetHeight(); h != nil && h.GetElevation() > 0 {
				it.HasHeight = true
			}
			it.ForceUse = f.GetForceuse()
			switch {
			case f.GetClip():
				it.AlwaysOnTopOrder = 1
			case f.GetTop():
				it.AlwaysOnTopOrder = 3
			case f.GetBottom():
				it.AlwaysOnTopOrder = 2
			}
			if uc := f.GetUpgradeclassification(); uc != nil {
				it.UpgradeClassification = uint8(uc.GetUpgradeClassification())
			}
		}
		cat.byID[it.ID] = it
		if it.Name != "" {
			if lname := strings.ToLower(it.Name); cat.byName[lname] == 0 {
				cat.byName[lname] = it.ID
			}
		}
	}
	return cat, nil
}
