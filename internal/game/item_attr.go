package game

import (
	"math"
	"sort"
	"fmt"

	"github.com/opentibiabr/canary-go/internal/io/propstream"
)

// OTBR item attribute tags (AttrTypes_t). Values mirror
// src/items/items_definitions.hpp:202-252.
const (
	attrStore              = 1
	attrTileFlags          = 3
	attrActionID           = 4
	attrUniqueID           = 5
	attrText               = 6
	attrDesc               = 7
	attrTeleDest           = 8
	attrItem               = 9
	attrDepotID            = 10
	attrRuneCharges        = 12
	attrHouseDoorID        = 14
	attrCount              = 15
	attrDuration           = 16
	attrDecayingState      = 17
	attrWrittenDate        = 18
	attrWrittenBy          = 19
	attrSleeperGUID        = 20
	attrSleepStart         = 21
	attrCharges            = 22
	attrContainerItems     = 23
	attrName               = 24
	attrArticle            = 25
	attrPluralName         = 26
	attrWeight             = 27
	attrAttack             = 28
	attrDefense            = 29
	attrExtraDefense       = 30
	attrArmor              = 31
	attrHitChance          = 32
	attrShootRange         = 33
	attrSpecial            = 34
	attrImbuementSlot      = 35
	attrOpenContainer      = 36
	attrCustomAttributes   = 37 // deprecated, replaced by attrCustom
	attrQuickLootContainer = 38
	attrAmount             = 39
	attrTier               = 40
	attrCustom             = 41
	attrStoreInboxCategory = 42
	attrOwner              = 43
	attrObtainContainer    = 44
	attrMantra             = 45
	attrNone               = 0
)

// decayingFalse mirrors DECAYING_FALSE (src/enums/item_attribute.hpp:54).
const decayingFalse = 0

// DecodeItemAttributes parses the OTBR ATTR_* TLV blob into structured fields,
// mirroring C++ Item::unserializeAttr / Item::readAttr (src/items/item.cpp:926,
// 1365). subType is the initial subtype seeded from the player_items.count
// column (mirroring Item::CreateItem(type, count)); ATTR_COUNT/ATTR_RUNE_CHARGES
// override it.
//
// It returns the decoded attributes (nil when the blob carries none) and the
// resulting subtype. An error is returned for tags whose payload cannot be
// faithfully modelled here (nested custom attributes / container items); callers
// should then fall back to preserving the raw blob verbatim.
func DecodeItemAttributes(blob []byte, subType uint16) (*ItemAttributes, uint16, error) {
	if len(blob) == 0 {
		return nil, subType, nil
	}
	a, sub, children, err := DecodeItemAttributesFrom(propstream.NewPropStream(blob), subType)
	if err != nil {
		return nil, subType, err
	}
	if children >= 0 {
		// A whole-blob caller (player_items) has no way to read the nested children:
		// those rows link containers by pid/sid instead, so the tag never appears
		// there. Reject it rather than silently dropping the contents.
		return nil, subType, fmt.Errorf("item attr %d not supported for decode", attrContainerItems)
	}
	return a, sub, nil
}

// DecodeItemAttributesFrom reads ONE item's attribute list from ps, stopping at the
// ATTR_NONE terminator, and reports how far it got. It is the streaming half of
// DecodeItemAttributes, needed because tile_store nests whole items inside an
// attribute list (IOMapSerialize::saveItem) and a whole-blob decoder cannot tell the
// caller where one item's attributes end.
//
// children is -1 when the list ended at the terminator, and >= 0 when it stopped on
// ATTR_CONTAINER_ITEMS — in which case it is the declared child count and the caller
// must read that many nested items before the terminator.
func DecodeItemAttributesFrom(ps *propstream.PropStream, subType uint16) (*ItemAttributes, uint16, int64, error) {
	a := &ItemAttributes{}
	children := int64(-1)

	for ps.Size() > 0 {
		attrType, err := ps.ReadUint8()
		if err != nil {
			break
		}
		if attrType == attrNone {
			break
		}

		switch int(attrType) {
		case attrStore: // int64
			v, err := ps.ReadInt64()
			if err != nil {
				return nil, subType, children, err
			}
			a.StoreTimestamp = &v

		case attrCount, attrRuneCharges: // uint8 -> subtype
			v, err := ps.ReadUint8()
			if err != nil {
				return nil, subType, children, err
			}
			subType = uint16(v)
			a.HasCount = true

		case attrActionID: // uint16
			v, err := ps.ReadUint16()
			if err != nil {
				return nil, subType, children, err
			}
			a.ActionID = &v

		case attrUniqueID: // uint16
			v, err := ps.ReadUint16()
			if err != nil {
				return nil, subType, children, err
			}
			a.UniqueID = &v

		case attrTeleDest:
			x, err := ps.ReadUint16()
			if err != nil {
				return nil, subType, children, err
			}
			y, err := ps.ReadUint16()
			if err != nil {
				return nil, subType, children, err
			}
			z, err := ps.ReadUint8()
			if err != nil {
				return nil, subType, children, err
			}
			a.TeleDest = &Position{X: x, Y: y, Z: z}

		case attrText:
			v, err := ps.ReadString()
			if err != nil {
				return nil, subType, children, err
			}
			a.Text = &v

		case attrWrittenDate: // uint64
			v, err := ps.ReadUint64()
			if err != nil {
				return nil, subType, children, err
			}
			a.WrittenDate = &v

		case attrWrittenBy:
			v, err := ps.ReadString()
			if err != nil {
				return nil, subType, children, err
			}
			a.WrittenBy = &v

		case attrDesc:
			v, err := ps.ReadString()
			if err != nil {
				return nil, subType, children, err
			}
			a.Description = &v

		case attrCharges: // uint16
			v, err := ps.ReadUint16()
			if err != nil {
				return nil, subType, children, err
			}
			a.Charges = &v

		case attrDuration: // int32
			v, err := ps.ReadInt32()
			if err != nil {
				return nil, subType, children, err
			}
			a.Duration = &v

		case attrDecayingState: // uint8
			v, err := ps.ReadUint8()
			if err != nil {
				return nil, subType, children, err
			}
			a.DecayState = &v

		case attrName:
			v, err := ps.ReadString()
			if err != nil {
				return nil, subType, children, err
			}
			a.Name = &v

		case attrArticle:
			v, err := ps.ReadString()
			if err != nil {
				return nil, subType, children, err
			}
			a.Article = &v

		case attrPluralName:
			v, err := ps.ReadString()
			if err != nil {
				return nil, subType, children, err
			}
			a.PluralName = &v

		case attrWeight: // uint32
			v, err := ps.ReadUint32()
			if err != nil {
				return nil, subType, children, err
			}
			a.Weight = &v

		case attrAttack: // int32
			v, err := ps.ReadInt32()
			if err != nil {
				return nil, subType, children, err
			}
			a.Attack = &v

		case attrDefense: // int32
			v, err := ps.ReadInt32()
			if err != nil {
				return nil, subType, children, err
			}
			a.Defense = &v

		case attrExtraDefense: // int32
			v, err := ps.ReadInt32()
			if err != nil {
				return nil, subType, children, err
			}
			a.ExtraDefense = &v

		case attrArmor: // int32
			v, err := ps.ReadInt32()
			if err != nil {
				return nil, subType, children, err
			}
			a.Armor = &v

		case attrHitChance: // int8
			v, err := ps.ReadInt8()
			if err != nil {
				return nil, subType, children, err
			}
			a.HitChance = &v

		case attrShootRange: // uint8
			v, err := ps.ReadUint8()
			if err != nil {
				return nil, subType, children, err
			}
			a.ShootRange = &v

		case attrTier: // uint8
			v, err := ps.ReadUint8()
			if err != nil {
				return nil, subType, children, err
			}
			a.Tier = &v

		case attrAmount: // uint16
			v, err := ps.ReadUint16()
			if err != nil {
				return nil, subType, children, err
			}
			a.Amount = &v

		case attrOwner: // uint32
			v, err := ps.ReadUint32()
			if err != nil {
				return nil, subType, children, err
			}
			a.Owner = &v

		// Attributes handled by C++ readAttr that we do not model as first-class
		// fields. We consume their exact payload so the stream stays aligned, but
		// they are not re-serialized (rare on player inventory items). These tags
		// still round-trip via the raw-blob fallback in db when they are the only
		// unmodelled content (see DecodeItemAttributes callers).
		case attrTileFlags: // uint32 (map tiles)
			if err := ps.Skip(4); err != nil {
				return nil, subType, children, err
			}
		case attrDepotID: // uint16
			if err := ps.Skip(2); err != nil {
				return nil, subType, children, err
			}
		case attrHouseDoorID: // uint8
			v, err := ps.ReadUint8()
			if err != nil {
				return nil, subType, children, err
			}
			a.HouseDoorID = &v
		case attrSleeperGUID: // uint32
			if err := ps.Skip(4); err != nil {
				return nil, subType, children, err
			}
		case attrSleepStart: // uint32
			if err := ps.Skip(4); err != nil {
				return nil, subType, children, err
			}

		case attrItem: // uint16
			if err := ps.Skip(2); err != nil {
				return nil, subType, children, err
			}
		case attrImbuementSlot, attrMantra: // int32
			if err := ps.Skip(4); err != nil {
				return nil, subType, children, err
			}
		case attrOpenContainer: // uint8
			v, err := ps.ReadUint8()
			if err != nil {
				return nil, subType, children, err
			}
			a.OpenContainer = &v
		case attrQuickLootContainer: // uint32 category bitmask
			v, err := ps.ReadUint32()
			if err != nil {
				return nil, subType, children, err
			}
			a.QuickLootContainer = &v
		case attrObtainContainer: // uint32 category bitmask
			v, err := ps.ReadUint32()
			if err != nil {
				return nil, subType, children, err
			}
			a.ObtainContainer = &v
		case attrSpecial, attrStoreInboxCategory: // string
			if _, err := ps.ReadString(); err != nil {
				return nil, subType, children, err
			}

		// Tags whose payload is a nested structure we cannot faithfully parse
		// yet: signal the caller to preserve the raw blob instead.
		case attrContainerItems:
			// Hand control back so the caller can read the nested items: tile_store
			// writes whole items inside the attribute list.
			n, err := ps.ReadUint32()
			if err != nil {
				return nil, subType, children, err
			}
			return a, subType, int64(n), nil

		case attrCustomAttributes:
			return nil, subType, children, fmt.Errorf("item attr %d not supported for decode", attrType)

		// ATTR_CUSTOM carries the script-defined key/value pairs (item.cpp:1300-1325).
		// These were READ AND DISCARDED, so every custom attribute died on the first
		// save/load round trip. The store stamps a decoration kit with "unWrapId" to
		// record what it unwraps into (data/libs/gamestore/purchases.lua:140); after a
		// restart the kit came back without it and unwrapping answered "Sorry, not
		// possible" with nothing to go on.
		//
		// u64 count, then per pair: string key, u8 type, value.
		// Types are CustomAttribute::serialize's (custom_attribute.cpp:88-103):
		// 1 = string, 2 = int64, 3 = double, 4 = bool.
		case attrCustom:
			count, err := ps.ReadUint64()
			if err != nil {
				return nil, subType, children, err
			}
			for i := uint64(0); i < count; i++ {
				key, err := ps.ReadString()
				if err != nil {
					return nil, subType, children, err
				}
				valType, err := ps.ReadUint8()
				if err != nil {
					return nil, subType, children, err
				}
				if a.Custom == nil {
					a.Custom = map[string]any{}
				}
				switch valType {
				case 1:
					v, err := ps.ReadString()
					if err != nil {
						return nil, subType, children, err
					}
					a.Custom[key] = v
				case 2:
					v, err := ps.ReadInt64()
					if err != nil {
						return nil, subType, children, err
					}
					a.Custom[key] = v
				case 3:
					bits, err := ps.ReadUint64()
					if err != nil {
						return nil, subType, children, err
					}
					a.Custom[key] = math.Float64frombits(bits)
				case 4:
					v, err := ps.ReadUint8()
					if err != nil {
						return nil, subType, children, err
					}
					a.Custom[key] = v != 0
				default:
					// An unknown type has no length, so the rest of the blob cannot be
					// trusted; stopping beats silently misreading the following items.
					return nil, subType, children, fmt.Errorf("unknown custom attribute type %d for key %q", valType, key)
				}
			}

		default:
			return nil, subType, children, fmt.Errorf("unknown item attr %d", attrType)
		}
	}

	return a, subType, children, nil
}

// Encode serialises the structured attributes back into an OTBR ATTR_* blob,
// mirroring the field order and wire widths of C++ Item::serializeAttr
// (src/items/item.cpp:1382). subType is the item's current subtype, written for
// ATTR_COUNT when the blob originally carried it (HasCount).
//
// Unlike C++ — which decides ATTR_COUNT/ATTR_ACTION_ID emission from the item
// type (stackable/fluid/splash, movable) via the item database — this port has
// no item type available at the db layer, so emission is driven purely by which
// attributes were present in the decoded blob. This preserves the attribute set
// across a load→save round-trip.
func (a *ItemAttributes) Encode(subType uint16) []byte {
	if a == nil {
		return nil
	}
	w := propstream.NewPropWriteStream()

	if a.StoreTimestamp != nil {
		w.WriteUint8(attrStore)
		w.WriteInt64(*a.StoreTimestamp)
	}
	if a.HasCount {
		w.WriteUint8(attrCount)
		w.WriteUint8(uint8(subType))
	}
	if a.Charges != nil {
		w.WriteUint8(attrCharges)
		w.WriteUint16(*a.Charges)
	}
	if a.ActionID != nil {
		w.WriteUint8(attrActionID)
		w.WriteUint16(*a.ActionID)
	}
	if a.UniqueID != nil {
		w.WriteUint8(attrUniqueID)
		w.WriteUint16(*a.UniqueID)
	}
	if a.TeleDest != nil {
		w.WriteUint8(attrTeleDest)
		w.WriteUint16(a.TeleDest.X)
		w.WriteUint16(a.TeleDest.Y)
		w.WriteUint8(a.TeleDest.Z)
	}
	if a.Text != nil {
		w.WriteUint8(attrText)
		w.WriteString(*a.Text)
	}
	if a.WrittenDate != nil {
		w.WriteUint8(attrWrittenDate)
		w.WriteUint64(*a.WrittenDate)
	}
	if a.WrittenBy != nil {
		w.WriteUint8(attrWrittenBy)
		w.WriteString(*a.WrittenBy)
	}
	if a.Description != nil {
		w.WriteUint8(attrDesc)
		w.WriteString(*a.Description)
	}
	if a.Duration != nil {
		w.WriteUint8(attrDuration)
		w.WriteInt32(*a.Duration)
	}
	if a.DecayState != nil {
		w.WriteUint8(attrDecayingState)
		w.WriteUint8(*a.DecayState)
	}
	if a.Name != nil {
		w.WriteUint8(attrName)
		w.WriteString(*a.Name)
	}
	if a.Article != nil {
		w.WriteUint8(attrArticle)
		w.WriteString(*a.Article)
	}
	if a.PluralName != nil {
		w.WriteUint8(attrPluralName)
		w.WriteString(*a.PluralName)
	}
	if a.Weight != nil {
		w.WriteUint8(attrWeight)
		w.WriteUint32(*a.Weight)
	}
	if a.Attack != nil {
		w.WriteUint8(attrAttack)
		w.WriteInt32(*a.Attack)
	}
	if a.Defense != nil {
		w.WriteUint8(attrDefense)
		w.WriteInt32(*a.Defense)
	}
	if a.ExtraDefense != nil {
		w.WriteUint8(attrExtraDefense)
		w.WriteInt32(*a.ExtraDefense)
	}
	if a.Armor != nil {
		w.WriteUint8(attrArmor)
		w.WriteInt32(*a.Armor)
	}
	if a.HitChance != nil {
		w.WriteUint8(attrHitChance)
		w.WriteInt8(*a.HitChance)
	}
	if a.ShootRange != nil {
		w.WriteUint8(attrShootRange)
		w.WriteUint8(*a.ShootRange)
	}
	if a.Tier != nil {
		w.WriteUint8(attrTier)
		w.WriteUint8(*a.Tier)
	}
	if a.OpenContainer != nil {
		w.WriteUint8(attrOpenContainer)
		w.WriteUint8(*a.OpenContainer)
	}
	if a.Amount != nil {
		w.WriteUint8(attrAmount)
		w.WriteUint16(*a.Amount)
	}
	if a.Owner != nil {
		w.WriteUint8(attrOwner)
		w.WriteUint32(*a.Owner)
	}
	if a.QuickLootContainer != nil {
		w.WriteUint8(attrQuickLootContainer)
		w.WriteUint32(*a.QuickLootContainer)
	}
	if a.ObtainContainer != nil {
		w.WriteUint8(attrObtainContainer)
		w.WriteUint32(*a.ObtainContainer)
	}

	// ATTR_CUSTOM (item.cpp:1537-1544). Decoding these without writing them back
	// would still lose them on the next save, so the pair has to land together.
	// Keys are sorted so the same attributes always produce the same bytes — Go map
	// order is randomised, and an item whose blob changes on every save churns the
	// tile_store rows for no reason.
	if len(a.Custom) > 0 {
		keys := make([]string, 0, len(a.Custom))
		for k := range a.Custom {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		w.WriteUint8(attrCustom)
		w.WriteUint64(uint64(len(keys)))
		for _, k := range keys {
			w.WriteString(k)
			switch v := a.Custom[k].(type) {
			case string:
				w.WriteUint8(1)
				w.WriteString(v)
			case int64:
				w.WriteUint8(2)
				w.WriteInt64(v)
			case float64:
				w.WriteUint8(3)
				w.WriteUint64(math.Float64bits(v))
			case bool:
				w.WriteUint8(4)
				if v {
					w.WriteUint8(1)
				} else {
					w.WriteUint8(0)
				}
			default:
				// CustomAttribute holds exactly these four; anything else came from a
				// binding that should be storing one of them instead.
				w.WriteUint8(2)
				w.WriteInt64(0)
			}
		}
	}

	return w.GetStream()
}
