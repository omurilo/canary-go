package luaengine

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/items"
)

// weapon:shootType did not exist, and a missing method is not a no-op in Lua: the
// call raises "attempt to call a non-function object" and aborts the script at that
// line. burst_arrow.lua:31, diamond_arrow.lua:38, poison_arrow.lua:24 and
// viper_star.lua:34 all died there, so their maxHitChance and register() never ran.
func TestWeaponShootTypeSetsTheItemType(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	e.world.Items = items.NewCatalog(&items.ItemType{ID: 3449, Name: "burst arrow"})

	// The tail of data/scripts/weapons/scripts/burst_arrow.lua, which used to abort
	// on the shootType line and never reach register().
	err := e.L.DoString(`
		local burstArrow = Weapon(WEAPON_AMMO)
		burstArrow:id(3449)
		burstArrow:attack(27)
		burstArrow:action("removecount")
		burstArrow:ammoType("arrow")
		burstArrow:shootType(CONST_ANI_BURSTARROW)
		burstArrow:maxHitChance(100)
		burstArrow:register()
	`)
	if err != nil {
		t.Fatalf("burst_arrow tail failed: %v", err)
	}

	// C++ stores it on the ITEM TYPE, not the weapon (weapon_functions.cpp:535).
	it := e.world.Items.Get(3449)
	if it == nil {
		t.Fatal("item 3449 vanished from the catalog")
	}
	if want := items.ShootTypes(7); it.ShootType != want { // CONST_ANI_BURSTARROW
		t.Errorf("ItemType(3449).ShootType = %d, want %d", it.ShootType, want)
	}
}

// Every weapon script in the datapack installs its callback by ASSIGNING the field
// rather than calling the method, in both spellings. A userdata with no __newindex
// cannot be assigned to at all, so this aborted the script on its first callback
// line — before it ever reached shootType.
func TestWeaponCallbackFieldAssignment(t *testing.T) {
	for _, form := range []string{
		`w.onUseWeapon = function(player, variant) return true end`,
		`function w.onUseWeapon(player, variant) return true end`,
	} {
		e := newTestEngine()
		e.world.Items = items.NewCatalog(&items.ItemType{ID: 3448, Name: "poison arrow"})
		err := e.L.DoString(`
			w = Weapon(WEAPON_AMMO)
			w:id(3448)
			` + form + `
			w:shootType(CONST_ANI_POISONARROW)
			w:register()
		`)
		if err != nil {
			t.Errorf("%s: %v", form, err)
		}
		e.Close()
	}
}

// A field that is not a known method must survive a round trip, the way it would on
// a plain table — __newindex must not be a black hole.
func TestWeaponArbitraryFieldRoundTrip(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	e.world.Items = items.NewCatalog(&items.ItemType{ID: 3448, Name: "poison arrow"})
	if err := e.L.DoString(`
		local w = Weapon(WEAPON_AMMO)
		w.myOwnState = 42
		assert(w.myOwnState == 42, "field did not round trip")
		assert(w.neverSet == nil, "an unset field must read as nil")
	`); err != nil {
		t.Fatalf("%v", err)
	}
}

// An id the catalog does not know must not panic; C++ would hand back a blank
// ItemType, and there is nothing useful to attach the animation to.
func TestWeaponShootTypeOnUnknownItem(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	e.world.Items = items.NewCatalog()

	err := e.L.DoString(`
		local w = Weapon(WEAPON_AMMO)
		w:id(65000)
		local ok = w:shootType(CONST_ANI_ARROW)
		assert(ok == nil, "expected nil for an unknown item id")
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// The items.xml name table must be complete. It replaced a 16-case switch whose
// default was CONST_ANI_ARROW, so every unlisted animation silently drew an arrow.
func TestShootTypeByNameCoversTheUpstreamTable(t *testing.T) {
	// Names that the old switch did not have, with their real CONST_ANI_ values.
	cases := map[string]items.ShootTypes{
		"envenomedarrow":   51,
		"prismaticbolt":    48,
		"leafstar":         56,
		"whirlwindsword":   25,
		"crystallinearrow": 49,
		"royalstar":        59,
		"simplearrow":      54,
		"cake":             42,
		// And a few the old switch did have, to pin the values it got right.
		"arrow":        3,
		"burstarrow":   7,
		"diamondarrow": 57,
		"holy":         31,
	}
	for name, want := range cases {
		got, ok := items.ShootTypeByName(name)
		if !ok {
			t.Errorf("ShootTypeByName(%q) not found", name)
			continue
		}
		if got != want {
			t.Errorf("ShootTypeByName(%q) = %d, want %d", name, got, want)
		}
	}

	// An unknown name must report the miss rather than defaulting to an arrow: the
	// C++ parser warns and leaves the field alone.
	if got, ok := items.ShootTypeByName("nosuchanimation"); ok {
		t.Errorf("ShootTypeByName(unknown) = %d, ok=true; want a miss", got)
	}
	// Case and whitespace are normalised, like the lowercase upstream keys.
	if got, _ := items.ShootTypeByName("  BurstArrow "); got != 7 {
		t.Errorf("ShootTypeByName is not normalising case/space: got %d", got)
	}
}
