package items

// ItemProperty is the ItemProperty enum (src/items/items_definitions.hpp:15), the
// value Lua's hasProperty takes.
type ItemProperty int

const (
	PropBlockSolid ItemProperty = iota
	PropHasHeight
	PropBlockProjectile
	PropBlockPath
	PropIsVertical
	PropIsHorizontal
	PropMovable
	PropImmovableBlockSolid
	PropImmovableBlockPath
	PropImmovableNoFieldBlockPath
	PropNoFieldBlockPath
	PropSupportHangable
)

// HasProperty ports Item::hasProperty (src/items/item.cpp). The "immovable"
// variants are a conjunction with not-movable, and the "nofield" variants also
// exclude magic fields — which is why this cannot be a flag lookup.
func (t *ItemType) HasProperty(prop ItemProperty) bool {
	if t == nil {
		return false
	}
	switch prop {
	case PropBlockSolid:
		return t.BlockSolid
	case PropMovable:
		return t.Movable
	case PropHasHeight:
		return t.HasHeight
	case PropBlockProjectile:
		return t.BlockProjectile
	case PropBlockPath:
		return t.BlockPathFind
	case PropIsVertical:
		return t.IsVertical
	case PropIsHorizontal:
		return t.IsHorizontal
	case PropImmovableBlockSolid:
		return t.BlockSolid && !t.Movable
	case PropImmovableBlockPath:
		return t.BlockPathFind && !t.Movable
	case PropImmovableNoFieldBlockPath:
		return !t.IsMagicField() && t.BlockPathFind && !t.Movable
	case PropNoFieldBlockPath:
		return !t.IsMagicField() && t.BlockPathFind
	case PropSupportHangable:
		return t.IsHorizontal || t.IsVertical
	default:
		return false
	}
}

// IsMagicField mirrors ItemType::isMagicField.
func (t *ItemType) IsMagicField() bool {
	return t != nil && t.Type == ItemTypeMagicField
}
