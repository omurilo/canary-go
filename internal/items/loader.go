package items

import (
	"fmt"
	"os"
	"strings"

	"github.com/opentibiabr/canary-go/internal/protobuf/appearances"
	"google.golang.org/protobuf/proto"
)

type ItemGroup int

const (
	ItemGroupNone ItemGroup = iota
	ItemGroupGround
	ItemGroupContainer
	ItemGroupFluid
	ItemGroupSplash
)

type ItemType struct {
	ID               uint16
	Name             string
	Description      string
	Group            ItemGroup
	AlwaysOnTopOrder int
	IsContainer      bool
	Stackable        bool
	WrapContainer    bool
	IsCorpse         bool
	HasHeight        bool
	BlockSolid       bool
	BlockProjectile  bool
	BlockPathFind    bool
	Pickupable       bool
	Rotatable        bool
	MultiUse         bool
	Movable          bool
	IsVertical       bool
	IsHorizontal     bool
	IsHangable       bool
	LookThrough      bool
	WearOut          bool
	ClockExpire      bool
	Expire           bool
	ExpireStop       bool
	IsWrapKit        bool
	IsDualWielding   bool
}

type Catalog struct {
	Items       []ItemType
	NameToItems map[string]uint16
}

func NewCatalog() *Catalog {
	return &Catalog{
		Items:       make([]ItemType, 0),
		NameToItems: make(map[string]uint16),
	}
}

func (c *Catalog) LoadFromProtobuf(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read appearances.dat: %w", err)
	}

	app := &appearances.Appearances{}
	if err := proto.Unmarshal(data, app); err != nil {
		return fmt.Errorf("failed to parse protobuf: %w", err)
	}

	for _, obj := range app.GetObject() {
		// Ignore invalid objects lacking flags
		if obj.GetFlags() == nil {
			continue
		}

		id := obj.GetId()
		// Resize the slice if necessary to accommodate the object's ID
		if int(id) >= len(c.Items) {
			newItems := make([]ItemType, id+1)
			copy(newItems, c.Items)
			c.Items = newItems
		}

		iType := &c.Items[id]
		iType.ID = uint16(id)
		iType.Name = string(obj.GetName())
		iType.Description = string(obj.GetDescription())

		flags := obj.GetFlags()
		if flags.GetContainer() {
			iType.IsContainer = true
			iType.Group = ItemGroupContainer
		} else if flags.GetBank() != nil {
			iType.Group = ItemGroupGround
		} else if flags.GetLiquidcontainer() {
			iType.Group = ItemGroupFluid
		} else if flags.GetLiquidpool() {
			iType.Group = ItemGroupSplash
		}

		if flags.GetClip() {
			iType.AlwaysOnTopOrder = 1
		} else if flags.GetTop() {
			iType.AlwaysOnTopOrder = 3
		} else if flags.GetBottom() {
			iType.AlwaysOnTopOrder = 2
		}

		iType.IsCorpse = flags.GetCorpse() || flags.GetPlayerCorpse()
		iType.HasHeight = flags.GetHeight() != nil
		iType.BlockSolid = flags.GetUnpass()
		iType.BlockProjectile = flags.GetUnsight()
		iType.BlockPathFind = flags.GetAvoid()
		iType.Pickupable = flags.GetTake()
		iType.Rotatable = flags.GetRotate()
		iType.WrapContainer = flags.GetWrap() || flags.GetUnwrap()
		iType.MultiUse = flags.GetMultiuse()
		iType.Movable = !flags.GetUnmove()

		if hook := flags.GetHook(); hook != nil {
			if hook.GetDirection() == appearances.HOOK_TYPE_HOOK_TYPE_SOUTH {
				iType.IsVertical = true
			} else if hook.GetDirection() == appearances.HOOK_TYPE_HOOK_TYPE_EAST {
				iType.IsHorizontal = true
			}
		}

		iType.IsHangable = flags.GetHang()
		iType.LookThrough = flags.GetIgnoreLook()
		iType.Stackable = flags.GetCumulative()
		iType.WearOut = flags.GetWearout()
		iType.ClockExpire = flags.GetClockexpire()
		iType.Expire = flags.GetExpire()
		iType.ExpireStop = flags.GetExpirestop()
		iType.IsWrapKit = flags.GetWrapkit()
		iType.IsDualWielding = flags.GetDualWielding()

		if iType.Name != "" {
			c.NameToItems[strings.ToLower(iType.Name)] = iType.ID
		}
	}

	return nil
}
