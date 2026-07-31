package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
	lua "github.com/yuin/gopher-lua"
)

const luaWeaponTypeName = "Weapon"

// LuaWeapon wraps a weapon definition for Lua.
type LuaWeapon struct {
	*game.Weapon
	onUseWeapon *lua.LFunction
	// fields holds arbitrary values a script assigns on the weapon object, so
	// `weapon.foo = x` behaves like it would on a table instead of vanishing.
	fields map[string]lua.LValue
}

// registeredWeapons holds weapons that have been registered via weapon:register()
var registeredWeapons []*LuaWeapon

// weaponMethods has all Weapon methods.
var weaponMethods = map[string]lua.LGFunction{
	"action":             weaponAction,
	"register":           weaponRegister,
	"id":                 weaponID,
	"level":              weaponLevel,
	"magicLevel":         weaponMagicLevel,
	"mana":               weaponMana,
	"manaPercent":        weaponManaPercent,
	"health":             weaponHealth,
	"healthPercent":      weaponHealthPercent,
	"soul":               weaponSoul,
	"breakChance":        weaponBreakChance,
	"premium":            weaponPremium,
	"wieldUnproperly":    weaponWieldUnproperly,
	"vocation":           weaponVocation,
	"onUseWeapon":        weaponOnUseWeapon,
	"element":            weaponElement,
	"attack":             weaponAttack,
	"defense":            weaponDefense,
	"range":              weaponRange,
	"charges":            weaponCharges,
	"duration":           weaponDuration,
	"decayTo":            weaponDecayTo,
	"transformEquipTo":   weaponTransformEquipTo,
	"transformDeEquipTo": weaponTransformDeEquipTo,
	"slotType":           weaponSlotType,
	"hitChance":          weaponHitChance,
	"extraElement":       weaponExtraElement,
	"ammoType":           weaponAmmoType,
	"maxHitChance":       weaponMaxHitChance,
	"damage":             weaponWandDamage,
}

// registerWeaponType registers the Weapon global constructor and metatable.
func (e *Engine) registerWeaponType() {
	mt := e.L.NewTypeMetatable(luaWeaponTypeName)
	// shootType writes to the item type rather than the weapon, so it needs the
	// catalog; the rest of weaponMethods are plain package-level functions.
	methods := make(map[string]lua.LGFunction, len(weaponMethods)+1)
	for name, fn := range weaponMethods {
		methods[name] = fn
	}
	methods["shootType"] = e.weaponShootType

	e.setClassConstructor("Weapon", weaponConstructor, methods)
	// __index resolves methods first, then anything a script stashed via __newindex,
	// so a stored field can be read back rather than disappearing.
	methodsTable := e.L.SetFuncs(e.L.NewTable(), methods)
	e.L.SetField(mt, "__index", e.L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(2)
		if fn := L.GetField(methodsTable, key); fn != lua.LNil {
			L.Push(fn)
			return 1
		}
		if ud, ok := L.Get(1).(*lua.LUserData); ok {
			if w, ok := ud.Value.(*LuaWeapon); ok && w.fields != nil {
				if v, ok := w.fields[key]; ok {
					L.Push(v)
					return 1
				}
			}
		}
		L.Push(lua.LNil)
		return 1
	}))
	// Every weapon script in the datapack installs its callback by ASSIGNING the
	// field, not by calling the method:
	//
	//	function poisonArrow.onUseWeapon(player, variant) ... end
	//	burstArrow.onUseWeapon = function(player, variant) ... end
	//
	// A userdata with no __newindex cannot be assigned to at all — Lua raises
	// "attempt to index a non-table object(userdata)" — so without this every one of
	// those scripts aborts on its first callback line.
	e.L.SetField(mt, "__newindex", e.L.NewFunction(weaponNewIndex))
}

// weaponNewIndex routes `weapon.field = value` to the same place the equivalent
// method writes. Only onUseWeapon is meaningful today; anything else is kept on a
// per-weapon table so a script can stash its own state on the object, which is what
// assigning to a plain table would have given it.
func weaponNewIndex(L *lua.LState) int {
	w := checkWeapon(L)
	if w == nil {
		return 0
	}
	key := L.CheckString(2)
	val := L.Get(3)
	if key == "onUseWeapon" {
		if fn, ok := val.(*lua.LFunction); ok {
			w.onUseWeapon = fn
			return 0
		}
	}
	if w.fields == nil {
		w.fields = map[string]lua.LValue{}
	}
	w.fields[key] = val
	return 0
}

// weaponShootType ports WeaponFunctions::luaWeaponShootType
// (src/lua/functions/items/weapon_functions.cpp:535): weapon:shootType(type)
// stores the projectile animation on the weapon's ITEM TYPE, not on the weapon.
//
// It was missing entirely, and a missing method is not a no-op in Lua: the call
// raised "attempt to call a non-function object" and aborted the whole script at
// that line, so burst_arrow, diamond_arrow, poison_arrow and viper_star never
// reached their maxHitChance or register() calls either.
func (e *Engine) weaponShootType(L *lua.LState) int {
	w := checkWeapon(L)
	if w == nil {
		L.Push(lua.LNil)
		return 1
	}
	if e.world == nil || e.world.Items == nil {
		L.Push(lua.LNil)
		return 1
	}
	it := e.world.Items.Get(w.ID)
	if it == nil {
		// C++ getItemType inserts a blank type for an unknown id; there is nothing
		// useful to attach the animation to, so say so instead of failing silently.
		e.log.Warn("weapon:shootType on an item id that is not in the catalog", "itemId", w.ID)
		L.Push(lua.LNil)
		return 1
	}
	it.ShootType = items.ShootTypes(L.CheckInt(2))
	L.Push(lua.LTrue)
	return 1
}

// weaponConstructor creates a new Weapon with the given item id.
func weaponConstructor(L *lua.LState) int {
	// __call puts the class table at index 1, so Weapon(WEAPON_AMMO) arrives as
	// (class, WEAPON_AMMO). Reading index 1 as a number made every weapon script
	// abort with "number expected, got table"; the table form Weapon{...} passes
	// no id at all, which is why the fallback is a zero id rather than an error.
	var id uint16
	for _, idx := range []int{2, 1} {
		if L.GetTop() >= idx && L.Get(idx).Type() == lua.LTNumber {
			id = uint16(L.CheckInt(idx))
			break
		}
	}
	w := &LuaWeapon{
		Weapon: &game.Weapon{ID: id},
	}
	ud := L.NewUserData()
	ud.Value = w
	L.SetMetatable(ud, L.GetTypeMetatable(luaWeaponTypeName))
	L.Push(ud)
	return 1
}

// checkWeapon extracts the LuaWeapon pointer from userdata.
func checkWeapon(L *lua.LState) *LuaWeapon {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*LuaWeapon); ok {
		return v
	}
	L.ArgError(1, "Weapon expected")
	return nil
}

// weaponSelf returns self (arg 1) for method chaining.
func weaponSelf(L *lua.LState) int {
	L.Push(L.Get(1))
	return 1
}

// --- Chainable setters ---

// weapon:id(id) sets the item id.
func weaponID(L *lua.LState) int {
	w := checkWeapon(L)
	w.ID = uint16(L.CheckInt(2))
	return weaponSelf(L)
}

// weapon:level(lvl) sets the required level.
func weaponLevel(L *lua.LState) int {
	w := checkWeapon(L)
	w.Level = uint16(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:magicLevel(lvl) sets the required magic level.
func weaponMagicLevel(L *lua.LState) int {
	w := checkWeapon(L)
	w.MagicLevel = uint16(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:mana(mana) sets the mana cost.
func weaponMana(L *lua.LState) int {
	w := checkWeapon(L)
	w.Mana = uint32(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:manaPercent(percent) sets the mana percent cost.
func weaponManaPercent(L *lua.LState) int {
	w := checkWeapon(L)
	w.ManaPercent = uint32(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:health(health) sets the health cost.
func weaponHealth(L *lua.LState) int {
	w := checkWeapon(L)
	w.Health = int32(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:healthPercent(percent) sets the health percent cost.
func weaponHealthPercent(L *lua.LState) int {
	w := checkWeapon(L)
	w.HealthPercent = uint32(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:soul(soul) sets the soul cost.
func weaponSoul(L *lua.LState) int {
	w := checkWeapon(L)
	w.Soul = uint8(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:breakChance(percent) sets the break chance.
func weaponBreakChance(L *lua.LState) int {
	w := checkWeapon(L)
	w.BreakChance = uint8(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:premium(bool) sets whether the weapon needs premium.
func weaponPremium(L *lua.LState) int {
	w := checkWeapon(L)
	w.Premium = luaOptBool(L, 2)
	return weaponSelf(L)
}

// weapon:wieldUnproperly(bool) sets whether the weapon can be wielded unproperly.
func weaponWieldUnproperly(L *lua.LState) int {
	w := checkWeapon(L)
	w.WieldUnproperly = luaOptBool(L, 2)
	return weaponSelf(L)
}

// weapon:vocation(vocName[, showInDescription, lastVoc]) adds a vocation requirement.
func weaponVocation(L *lua.LState) int {
	w := checkWeapon(L)
	// Collect all string arguments as vocations
	for i := 2; i <= L.GetTop(); i++ {
		if L.Get(i).Type() != lua.LTString {
			continue
		}
		// Handle semicolons (C++ also handles "name;showInDesc" format)
		name := L.Get(i).String()
		if idx := strings.IndexByte(name, ';'); idx >= 0 {
			name = name[:idx]
		}
		name = strings.TrimSpace(name)
		if name != "" {
			w.Vocations = append(w.Vocations, strings.ToLower(name))
		}
	}
	return weaponSelf(L)
}

// weapon:onUseWeapon(callback) stores the use-weapon function.
func weaponOnUseWeapon(L *lua.LState) int {
	w := checkWeapon(L)
	if L.GetTop() >= 2 {
		if fn, ok := L.Get(2).(*lua.LFunction); ok {
			w.onUseWeapon = fn
		}
	}
	return weaponSelf(L)
}

// resolveCombatType converts a Lua argument (number or string name) to a combat type value.
// Returns the combat type enum value (COMBAT_*), defaulting to COMBAT_NONE (255) if unknown.
func resolveCombatType(L *lua.LState, n int) uint8 {
	v := L.Get(n)
	if v.Type() == lua.LTNumber {
		return uint8(lua.LVAsNumber(v))
	}
	if v.Type() == lua.LTString {
		switch strings.ToLower(v.String()) {
		case "earth":
			return 2 // COMBAT_EARTHDAMAGE
		case "ice":
			return 9 // COMBAT_ICEDAMAGE
		case "energy":
			return 3 // COMBAT_ENERGYDAMAGE
		case "fire":
			return 1 // COMBAT_FIREDAMAGE
		case "death":
			return 11 // COMBAT_DEATHDAMAGE
		case "holy":
			return 10 // COMBAT_HOLYDAMAGE
		case "physical":
			return 0 // COMBAT_PHYSICALDAMAGE
		}
	}
	return 255 // COMBAT_NONE
}

// weapon:element(combatType) sets the weapon's element type.
func weaponElement(L *lua.LState) int {
	w := checkWeapon(L)
	if L.GetTop() >= 2 {
		w.ElementType = resolveCombatType(L, 2)
	}
	return weaponSelf(L)
}

// weapon:attack(atk) sets the attack value.
func weaponAttack(L *lua.LState) int {
	w := checkWeapon(L)
	w.Attack = int32(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:defense(def[, extraDef]) sets the defense and optional extra defense values.
func weaponDefense(L *lua.LState) int {
	w := checkWeapon(L)
	w.Defense = int32(luaOptInt(L, 2))
	if L.GetTop() >= 3 {
		w.ExtraDefense = int32(luaOptInt(L, 3))
	}
	return weaponSelf(L)
}

// weapon:range(range) sets the weapon range.
func weaponRange(L *lua.LState) int {
	w := checkWeapon(L)
	w.Range = uint8(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:charges(charges[, showCharges = true]) sets the charges and optional show flag.
func weaponCharges(L *lua.LState) int {
	w := checkWeapon(L)
	w.Charges = uint8(luaOptInt(L, 2))
	if L.GetTop() >= 3 {
		w.ShowCharges = luaOptBool(L, 3)
	} else {
		w.ShowCharges = true
	}
	return weaponSelf(L)
}

// weapon:duration(duration[, showDuration = true]) sets the duration and optional show flag.
func weaponDuration(L *lua.LState) int {
	w := checkWeapon(L)
	w.Duration = uint32(luaOptInt(L, 2))
	if L.GetTop() >= 3 {
		w.ShowDuration = luaOptBool(L, 3)
	} else {
		w.ShowDuration = true
	}
	return weaponSelf(L)
}

// weapon:decayTo([itemid = 0]) sets the decay item id.
func weaponDecayTo(L *lua.LState) int {
	w := checkWeapon(L)
	if L.GetTop() >= 2 {
		w.DecayTo = uint16(luaOptInt(L, 2))
	} else {
		w.DecayTo = 0
	}
	return weaponSelf(L)
}

// weapon:transformEquipTo(itemid) sets the transform-on-equip item id.
func weaponTransformEquipTo(L *lua.LState) int {
	w := checkWeapon(L)
	w.TransformEquipTo = uint16(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:transformDeEquipTo(itemid) sets the transform-on-deequip item id.
func weaponTransformDeEquipTo(L *lua.LState) int {
	w := checkWeapon(L)
	w.TransformDeEquipTo = uint16(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:slotType(slot) sets the slot type ("two-handed" or otherwise "hand").
func weaponSlotType(L *lua.LState) int {
	w := checkWeapon(L)
	if L.GetTop() >= 2 {
		slot := L.Get(2).String()
		if strings.ToLower(slot) == "two-handed" {
			w.SlotType = "two-handed"
		} else {
			w.SlotType = "hand"
		}
	}
	return weaponSelf(L)
}

// weapon:hitChance(chance) sets the hit chance.
func weaponHitChance(L *lua.LState) int {
	w := checkWeapon(L)
	w.HitChance = int8(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:extraElement(atk, combatType) sets extra element damage and type.
func weaponExtraElement(L *lua.LState) int {
	w := checkWeapon(L)
	if L.GetTop() >= 2 {
		w.ExtraElementDamage = uint16(luaOptInt(L, 2))
	}
	if L.GetTop() >= 3 {
		w.ExtraElement = resolveCombatType(L, 3)
	}
	return weaponSelf(L)
}

// weapon:ammoType(type) sets the ammo type ("arrow" or "bolt").
func weaponAmmoType(L *lua.LState) int {
	w := checkWeapon(L)
	if L.GetTop() >= 2 {
		ammo := strings.ToLower(L.Get(2).String())
		if ammo == "arrow" || ammo == "bolt" {
			w.AmmoType = ammo
		}
	}
	return weaponSelf(L)
}

// weapon:maxHitChance(max) sets the maximum hit chance.
func weaponMaxHitChance(L *lua.LState) int {
	w := checkWeapon(L)
	w.MaxHitChance = int8(luaOptInt(L, 2))
	return weaponSelf(L)
}

// weapon:damage(min[, max]) sets the wand damage range.
// If only one argument is given, both min and max are set to that value.
func weaponWandDamage(L *lua.LState) int {
	w := checkWeapon(L)
	w.WandMinDamage = int32(luaOptInt(L, 2))
	if L.GetTop() >= 3 {
		w.WandMaxDamage = int32(luaOptInt(L, 3))
	} else {
		w.WandMaxDamage = w.WandMinDamage
	}
	return weaponSelf(L)
}

// weapon:action(name) sets the weapon action type from a string name.
func weaponAction(L *lua.LState) int {
	w := checkWeapon(L)
	if L.GetTop() >= 2 {
		typeName := strings.ToLower(L.Get(2).String())
		switch typeName {
		case "removecount":
			w.Action = 1 // WEAPONACTION_REMOVECOUNT
		case "removecharge":
			w.Action = 2 // WEAPONACTION_REMOVECHARGE
		case "move":
			w.Action = 3 // WEAPONACTION_MOVE
		}
	}
	return weaponSelf(L)
}

// weapon:register() stores the weapon in the package-level registry and returns true.
func weaponRegister(L *lua.LState) int {
	w := checkWeapon(L)
	registeredWeapons = append(registeredWeapons, w)
	// Index by item id so the combat engine can find the script for the item a
	// player is actually shooting. Weapons::registerLuaEvent keys off getID() the
	// same way. Only the last registration for an id wins, as in C++.
	if w.ID != 0 {
		if weaponsByItemID == nil {
			weaponsByItemID = map[uint16]*LuaWeapon{}
		}
		weaponsByItemID[w.ID] = w
	}
	L.Push(lua.LTrue)
	return 1
}

// weaponsByItemID indexes registered weapons by the item id they apply to.
var weaponsByItemID map[uint16]*LuaWeapon

// UseWeapon runs the datapack onUseWeapon callback for the item a player is
// attacking with, and reports whether one existed. It is the port of the
// isLoadedScriptId branch of Weapon::internalUseWeapon (weapons.cpp): a weapon with
// a script attached applies its own damage through combat:execute, and the
// built-in damage is skipped entirely.
//
// The callback signature is onUseWeapon(player, variant) where the variant is a
// NUMBER carrying the target's creature id, matching Weapon::executeUseWeapon.
func (e *Engine) UseWeapon(p *game.Player, weaponItemID uint16, target game.Creature) bool {
	if p == nil || target == nil {
		return false
	}
	w := weaponsByItemID[weaponItemID]
	if w == nil || w.onUseWeapon == nil {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.pushPlayerUserdata(p)
	playerArg := e.L.Get(-1)
	e.L.Pop(1)

	v := &luaVariant{vtype: VariantNumber, number: target.GetID()}
	ud := e.L.NewUserData()
	ud.Value = v
	e.L.SetMetatable(ud, e.L.GetTypeMetatable(variantTypeName))

	if err := e.L.CallByParam(lua.P{Fn: w.onUseWeapon, NRet: 1, Protect: true}, playerArg, ud); err != nil {
		e.log.Warn("onUseWeapon error", "itemId", weaponItemID, "err", err)
		// C++ treats a failed script as handled: it does not fall back to the
		// built-in damage, and neither do we.
		return true
	}
	e.L.Pop(e.L.GetTop())
	return true
}
