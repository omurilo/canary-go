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

	"github.com/opentibiabr/canary-go/internal/appproto"
)

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
	Article     string
	Description string
	Group       Group

	Stackable             bool
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
	FloorChange  string
	ForceUse     bool
	IsLadder     bool
	IsDoor       bool
	IsQuiver     bool

	Weight       uint32
	Armor        int32
	Attack       int32
	Defense      int32
	ExtraDefense int32
	DecayTo      uint16
	Duration     uint32
	ShowDuration bool
	Charges      uint32
	ShowCharges  bool
	Capacity     uint32

	MaxHitChance int32
	HitChance    int32
	Range        int32
	ShootType    string
	AmmoType     string

	TransformEquipTo   uint16
	TransformDeEquipTo uint16

	// Stats stores values like "skillsword", "absorbpercentfire", "elementice", "magiclevelpoints", etc.
	Stats map[string]int32
}

// AlwaysOnTop reports whether the item stacks below creatures on a tile.
func (t *ItemType) AlwaysOnTop() bool { return t.AlwaysOnTopOrder > 0 }

func (t *ItemType) IsContainer() bool      { return t.Group == GroupContainer }
func (t *ItemType) IsGround() bool         { return t.Group == GroupGround }
func (t *ItemType) IsFluidContainer() bool { return t.Group == GroupFluid }
func (t *ItemType) IsSplash() bool         { return t.Group == GroupSplash }

// Catalog maps client item ids to their metadata.
type Catalog struct {
	byID   map[uint16]*ItemType
	byName map[string]uint16 // lower-cased name -> first id seen
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
