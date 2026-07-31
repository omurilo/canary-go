package items

import "testing"

// Item::hasProperty (src/items/item.cpp) is a switch, not a flag lookup: the
// "immovable" variants are a conjunction with not-movable and the "nofield" ones
// also exclude magic fields. The Lua binding used to discard the property argument
// and answer BlockSolid to every question.
func TestItemTypeHasProperty(t *testing.T) {
	// A solid, immovable wall.
	wall := &ItemType{BlockSolid: true, BlockProjectile: true, BlockPathFind: true, Movable: false}
	// A movable box that blocks pathfinding but nothing else.
	box := &ItemType{BlockPathFind: true, Movable: true}
	// A magic field: blocks pathfinding, but the nofield variants must exclude it.
	field := &ItemType{BlockPathFind: true, Movable: false, Type: ItemTypeMagicField}
	hook := &ItemType{IsVertical: true, Movable: true}

	tests := []struct {
		name string
		it   *ItemType
		prop ItemProperty
		want bool
	}{
		{"wall blocks solid", wall, PropBlockSolid, true},
		{"box does not block solid", box, PropBlockSolid, false},
		{"wall blocks projectiles", wall, PropBlockProjectile, true},
		{"box does not block projectiles", box, PropBlockProjectile, false},
		{"wall blocks path", wall, PropBlockPath, true},
		{"box blocks path", box, PropBlockPath, true},

		{"wall is immovable-block-solid", wall, PropImmovableBlockSolid, true},
		// The distinction the old binding could not express: it blocks solid OR it is
		// immovable, but not both.
		{"a movable solid is not immovable-block-solid", &ItemType{BlockSolid: true, Movable: true}, PropImmovableBlockSolid, false},
		{"box is not immovable-block-path", box, PropImmovableBlockPath, false},
		{"wall is immovable-block-path", wall, PropImmovableBlockPath, true},

		{"a magic field blocks path", field, PropBlockPath, true},
		{"but is excluded from nofield-block-path", field, PropNoFieldBlockPath, false},
		{"and from immovable-nofield-block-path", field, PropImmovableNoFieldBlockPath, false},
		{"wall is immovable-nofield-block-path", wall, PropImmovableNoFieldBlockPath, true},

		{"movable box is movable", box, PropMovable, true},
		{"wall is not movable", wall, PropMovable, false},
		{"a vertical hook supports hangables", hook, PropSupportHangable, true},
		{"a wall does not", wall, PropSupportHangable, false},
		{"vertical", hook, PropIsVertical, true},
		{"not horizontal", hook, PropIsHorizontal, false},

		{"a nil type has no properties", nil, PropBlockSolid, false},
		{"an unknown property is false", wall, ItemProperty(99), false},
	}
	for _, tc := range tests {
		if got := tc.it.HasProperty(tc.prop); got != tc.want {
			t.Errorf("%s: HasProperty(%d) = %v, want %v", tc.name, tc.prop, got, tc.want)
		}
	}
}
