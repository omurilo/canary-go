package luaengine

import (
	"fmt"
	"reflect"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/game/combat"
	lua "github.com/yuin/gopher-lua"
)

// conditionHolder is satisfied by any creature that embeds the game
// conditionStore (Player directly, Monster/Npc via BaseCreature).
type conditionHolder interface {
	AddCondition(c combat.Condition)
	RemoveCondition(t combat.ConditionType)
	HasCondition(t combat.ConditionType) bool
}

func creatureConditions(c game.Creature) (conditionHolder, bool) {
	h, ok := c.(conditionHolder)
	return h, ok
}

func getCreature(L *lua.LState, index int) game.Creature {
	ud := L.CheckUserData(index)
	if v, ok := ud.Value.(game.Creature); ok {
		return v
	}
	L.ArgError(index, "Creature expected")
	return nil
}

func (e *Engine) pushCreature(L *lua.LState, c game.Creature) {
	if c == nil {
		L.Push(lua.LNil)
		return
	}
	ud := L.NewUserData()
	ud.Value = c
	if _, ok := c.(*game.Player); ok {
		L.SetMetatable(ud, L.GetTypeMetatable("Player"))
	} else {
		L.SetMetatable(ud, L.GetTypeMetatable("Creature"))
	}
	L.Push(ud)
}

func checkCreature(L *lua.LState) game.Creature {
	return getCreature(L, 1)
}

// creatureConstructorCall implements the Player(x)/Creature(x)/Npc(x)/Monster(x)
// global constructors. __call passes the class table as arg #1, so the real
// argument is #2: either a creature userdata (returned as the matching kind) or
// a numeric creature id (looked up in the world). Pushes nil on miss/mismatch.
func (e *Engine) creatureConstructorCall(L *lua.LState, kind string) int {
	var c game.Creature
	arg := L.Get(2)
	// fmt.Printf("ctor %s: arg1=%v (%s) arg2=%v (%s)\n", kind, L.Get(1), L.Get(1).Type(), arg, arg.Type())
	switch arg.Type() {
	case lua.LTUserData:
		if v, ok := arg.(*lua.LUserData).Value.(game.Creature); ok {
			c = v
		}
	case lua.LTNumber:
		if e.world != nil {
			if cr := e.world.CreatureByID(uint32(lua.LVAsNumber(arg))); cr != nil {
				c = cr
			}
		}
	case lua.LTString:
		if e.world != nil {
			if cr := e.world.CreatureByName(lua.LVAsString(arg)); cr != nil {
				c = cr
			}
		}
	}
	if !e.pushCreatureAs(L, c, kind) {
		L.Push(lua.LNil)
		return 1
	}
	return 1
}

// pushCreatureAs pushes c as a userdata bound to the metatable for kind
// ("Player"/"Npc"/"Monster"/"Creature"). It returns false (pushing nothing)
// when c is nil or doesn't match the requested concrete kind, so the caller
// can push nil — mirroring the C++ Player(cid)/Npc(cid) casts that yield nil
// on a type mismatch.
// creatureGetplayer returns the creature userdata itself when it wraps a
// *game.Player, else nil (mirrors Creature::getPlayer).
func creatureGetplayer(L *lua.LState) int {
	ud := L.CheckUserData(1)
	if _, ok := ud.Value.(*game.Player); ok {
		L.Push(ud)
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func creatureGetmonster(L *lua.LState) int {
	ud := L.CheckUserData(1)
	if _, ok := ud.Value.(*game.Monster); ok {
		L.Push(ud)
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func creatureGetnpc(L *lua.LState) int {
	ud := L.CheckUserData(1)
	if _, ok := ud.Value.(*game.Npc); ok {
		L.Push(ud)
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

// metatableForCreature returns the most specific Lua metatable name for a
// creature so type predicates (isPlayer/isMonster/isNpc) resolve correctly.
func metatableForCreature(c game.Creature) string {
	switch c.(type) {
	case *game.Player:
		return "Player"
	case *game.Npc:
		return "Npc"
	case *game.Monster:
		return "Monster"
	default:
		return "Creature"
	}
}

func (e *Engine) pushCreatureAs(L *lua.LState, c game.Creature, kind string) bool {
	if c == nil || (reflect.ValueOf(c).Kind() == reflect.Ptr && reflect.ValueOf(c).IsNil()) {
		return false
	}
	metatable := kind
	switch kind {
	case "Player":
		if _, ok := c.(*game.Player); !ok {
			return false
		}
	case "Npc":
		if _, ok := c.(*game.Npc); !ok {
			return false
		}
	case "Monster":
		if _, ok := c.(*game.Monster); !ok {
			return false
		}
	default: // "Creature": accept any, but bind the most specific metatable.
		switch c.(type) {
		case *game.Player:
			metatable = "Player"
		case *game.Npc:
			metatable = "Npc"
		case *game.Monster:
			metatable = "Monster"
		default:
			metatable = "Creature"
		}
	}
	ud := L.NewUserData()
	ud.Value = c
	L.SetMetatable(ud, L.GetTypeMetatable(metatable))
	L.Push(ud)
	return true
}

// outfitToTable converts a game.Outfit into the { lookType, lookHead, ... }
// table shape the Lua API uses (Creature:getOutfit / setOutfit).
func outfitToTable(L *lua.LState, o game.Outfit) *lua.LTable {
	t := L.NewTable()
	L.SetField(t, "lookType", lua.LNumber(o.LookType))
	L.SetField(t, "lookTypeEx", lua.LNumber(o.LookTypeEx))
	L.SetField(t, "lookHead", lua.LNumber(o.Head))
	L.SetField(t, "lookBody", lua.LNumber(o.Body))
	L.SetField(t, "lookLegs", lua.LNumber(o.Legs))
	L.SetField(t, "lookFeet", lua.LNumber(o.Feet))
	L.SetField(t, "lookAddons", lua.LNumber(o.Addons))
	L.SetField(t, "lookMount", lua.LNumber(o.LookMount))
	L.SetField(t, "lookMountHead", lua.LNumber(o.MountHead))
	L.SetField(t, "lookMountBody", lua.LNumber(o.MountBody))
	L.SetField(t, "lookMountLegs", lua.LNumber(o.MountLegs))
	L.SetField(t, "lookMountFeet", lua.LNumber(o.MountFeet))
	return t
}

// registerCreatureType registers the Creature userdata type.
//
// Methods are stored DIRECTLY on the metatable (not a separate __index table)
// and __index points at the metatable itself. This is required because the
// datapack's revscriptsys.lua overwrites __index with a `CreatureIndex(self,key)`
// function that resolves methods via `getmetatable(self)[key]` — i.e. it reads
// them off the metatable. A separate __index table would be bypassed and every
// method (getId, getPosition, ...) would read back nil.
func (e *Engine) registerCreatureType() {
	mt := e.L.NewTypeMetatable("Creature")
	e.L.SetFuncs(mt, creatureMethods)
	// teleportTo, changeSpeed, setSpeed, getParent, getTile need the world/engine references.
	e.L.SetField(mt, "teleportTo", e.L.NewFunction(e.creatureTeleportto))
	e.L.SetField(mt, "changeSpeed", e.L.NewFunction(e.creatureChangespeed))
	e.L.SetField(mt, "setSpeed", e.L.NewFunction(e.creatureSetspeed))
	e.L.SetField(mt, "getParent", e.L.NewFunction(e.creatureGetparent))
	e.L.SetField(mt, "getTile", e.L.NewFunction(e.creatureGettile))
	e.L.SetField(mt, "remove", e.L.NewFunction(e.creatureRemove))
	e.L.SetField(mt, "getZoneType", e.L.NewFunction(e.creatureGetzonetype))
	e.L.SetField(mt, "__index", mt)
}

var creatureMethods = map[string]lua.LGFunction{
	"getEvents":       creatureGetevents,
	"registerEvent":   creatureRegisterevent,
	"unregisterEvent": creatureUnregisterevent,
	"isRemoved":       creatureIsremoved,
	"isCreature":      creatureIscreature,
	"isPlayer": func(L *lua.LState) int {
		c := checkCreature(L)
		_, ok := c.(*game.Player)
		L.Push(lua.LBool(ok))
		return 1
	},
	"isMonster": func(L *lua.LState) int {
		c := checkCreature(L)
		_, ok := c.(*game.Monster)
		L.Push(lua.LBool(ok))
		return 1
	},
	"isNpc": func(L *lua.LState) int {
		c := checkCreature(L)
		_, ok := c.(*game.Npc)
		L.Push(lua.LBool(ok))
		return 1
	},
	// Native down-casts: `creature:getPlayer()` etc. return self when the
	// underlying Go type matches, else nil. Many scripts start with
	// `local player = creature:getPlayer()` (e.g. the temple/citizen movement),
	// so these must be real methods on the metatable, not just the Lua-lib
	// versions on the global Creature table (which revscriptsys never resolves).
	"getPlayer":          creatureGetplayer,
	"getMonster":         creatureGetmonster,
	"getNpc":             creatureGetnpc,
	"getCreature":        func(L *lua.LState) int { L.Push(L.Get(1)); return 1 },
	"isInGhostMode":      creatureIsinghostmode,
	"isHealthHidden":     creatureIshealthhidden,
	"isImmune":           creatureIsimmune,
	"canSee":             creatureCansee,
	"canSeeCreature":     creatureCanseecreature,
	"getId":              creatureGetid,
	"getName":            creatureGetname,
	"getTypeName":        creatureGettypename,
	"getTarget":          creatureGettarget,
	"setTarget":          creatureSettarget,
	"getFollowCreature":  creatureGetfollowcreature,
	"setFollowCreature":  creatureSetfollowcreature,
	"reload":             creatureReload,
	"getMaster":          creatureGetmaster,
	"setMaster":          creatureSetmaster,
	"getLight":           creatureGetlight,
	"setLight":           creatureSetlight,
	"getSpeed":           creatureGetspeed,
	"getBaseSpeed":       creatureGetbasespeed,
	"setDropLoot":        creatureSetdroploot,
	"setSkillLoss":       creatureSetskillloss,
	"getPosition":        creatureGetposition,
	"getDirection":       creatureGetdirection,
	"setDirection":       creatureSetdirection,
	"getHealth":          creatureGethealth,
	"setHealth":          creatureSethealth,
	"addHealth":          creatureAddhealth,
	"getMaxHealth":       creatureGetmaxhealth,
	"setMaxHealth":       creatureSetmaxhealth,
	"setHiddenHealth":    creatureSethiddenhealth,
	"isMoveLocked":       creatureIsmovelocked,
	"isDirectionLocked":  creatureIsdirectionlocked,
	"setMoveLocked":      creatureSetmovelocked,
	"setDirectionLocked": creatureSetdirectionlocked,
	"getSkull":           creatureGetskull,
	"setSkull":           creatureSetskull,
	"getOutfit":          creatureGetoutfit,
	"setOutfit":          creatureSetoutfit,
	"getCondition":       creatureGetcondition,
	"addCondition":       creatureAddcondition,
	"removeCondition":    creatureRemovecondition,
	"hasCondition":       creatureHascondition,
	"say":                creatureSay,
	"getDamageMap":       creatureGetdamagemap,
	"getSummons":         creatureGetsummons,
	"hasBeenSummoned":    creatureHasbeensummoned,
	"getDescription":     creatureGetdescription,
	"getPathTo":          creatureGetpathto,
	"move":               creatureMove,
	// getZoneType is registered as an engine method below in registerCreatureType()
	"getZones":           creatureGetzones,
	"setIcon":            creatureSeticon,
	"getIcon":            creatureGeticon,
	"getIcons":           creatureGeticons,
	"removeIcon":         creatureRemoveicon,
	"clearIcons":         creatureClearicons,
	"attachEffectById":   creatureAttacheffectbyid,
	"detachEffectById":   creatureDetacheffectbyid,
	"getAttachedEffects": creatureGetattachedeffects,
	"getShader":          creatureGetshader,
	"setShader":          creatureSetshader,
}

func creatureAddcondition(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		return 0
	}
	holder, ok := creatureConditions(c)
	if !ok {
		return 0
	}
	ud, ok := L.Get(2).(*lua.LUserData)
	if !ok {
		return 0
	}
	cond, ok := ud.Value.(*luaCondition)
	if !ok {
		return 0
	}
	// Food/regeneration is tracked as RegenTicks on the player, not in the
	// generic combat condition store.
	if cond.rawType == conditionRegeneration {
		if pud, ok := L.Get(1).(*lua.LUserData); ok {
			if p, ok := pud.Value.(*game.Player); ok {
				p.RegenTicks = cond.getTicks()
				if p.Session != nil {
					p.Session.SendStats() // refresh the client's food timer
				}
				L.Push(lua.LTrue)
				return 1
			}
		}
		L.Push(lua.LTrue)
		return 1
	}
	if cond.cond == nil {
		return 0
	}
	holder.AddCondition(cond.cond.Clone())
	L.Push(lua.LTrue)
	return 1
}

func creatureAddhealth(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		return 0
	}
	amount := int32(L.CheckNumber(2))
	c.AddHealth(amount)
	return 0
}

func creatureAttacheffectbyid(L *lua.LState) int { return 0 }

func creatureCansee(L *lua.LState) int {
	// No line-of-sight model yet; assume visibility so scripts proceed.
	L.Push(lua.LTrue)
	return 1
}

// creatureCanseecreature is creature:canSeeCreature(other)
// (creature_functions.cpp:227). It answered a flat true, so a script asking
// whether a monster could spot an invisible player was always told yes.
func creatureCanseecreature(L *lua.LState) int {
	c := checkCreature(L)
	other, _ := L.CheckUserData(2).Value.(game.Creature)
	L.Push(lua.LBool(game.CanSeeCreature(c, other)))
	return 1
}

func (e *Engine) creatureChangespeed(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		delta := L.CheckInt(2)
		e.world.ChangeSpeed(c, int32(delta))
	}
	return 0
}

func creatureClearicons(L *lua.LState) int { return 0 }

func creatureDetacheffectbyid(L *lua.LState) int { return 0 }

func creatureGetattachedeffects(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func creatureGetbasespeed(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LNumber(c.GetSpeed()))
		return 1
	}
	L.Push(lua.LNumber(0))
	return 1
}

func creatureGetcondition(L *lua.LState) int {
	c := checkCreature(L)
	condType := luaOptInt(L, 2)
	// Food/regeneration: return a condition bound to the player's RegenTicks so
	// the food script can read getTicks() and accumulate via setTicks(). Only
	// report presence when there is active food, matching C++ getCondition.
	if condType == conditionRegeneration {
		if ud, ok := L.Get(1).(*lua.LUserData); ok {
			if p, ok := ud.Value.(*game.Player); ok && p.RegenTicks > 0 {
				lc := &luaCondition{rawType: conditionRegeneration, boundPlayer: p}
				cud := L.NewUserData()
				cud.Value = lc
				L.SetMetatable(cud, L.GetTypeMetatable(luaConditionTypeName))
				L.Push(cud)
				return 1
			}
		}
	}
	_ = c
	L.Push(lua.LNil)
	return 1
}

func creatureGetdamagemap(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func creatureGetdescription(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		desc := c.GetName()
		health := c.GetHealth()
		maxHealth := c.GetMaxHealth()
		if maxHealth > 0 {
			desc = fmt.Sprintf("%s (Health: %d/%d)", desc, health, maxHealth)
		}
		L.Push(lua.LString(desc))
		return 1
	}
	L.Push(lua.LString(""))
	return 1
}

func creatureGetdirection(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LNumber(c.GetDirection()))
		return 1
	}
	L.Push(lua.LNumber(0))
	return 1
}

func creatureGetevents(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func creatureGetfollowcreature(L *lua.LState) int {
	L.Push(lua.LNil)
	return 1
}

func creatureGethealth(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LNumber(c.GetHealth()))
		return 1
	}
	return 0
}

func creatureGeticon(L *lua.LState) int {
	L.Push(lua.LNil)
	return 1
}

func creatureGeticons(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func creatureGetid(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(c.GetID()))
	return 1
}

func creatureGetlight(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LNumber(c.GetLightLevel()))
		L.Push(lua.LNumber(c.GetLightColor()))
		return 2
	}
	L.Push(lua.LNumber(0))
	L.Push(lua.LNumber(0))
	return 2
}

func creatureGetmaster(L *lua.LState) int {
	// No summon/master relationship modelled yet.
	L.Push(lua.LNil)
	return 1
}

func creatureGetmaxhealth(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LNumber(c.GetMaxHealth()))
		return 1
	}
	return 0
}

func creatureGetname(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LString(c.GetName()))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func creatureGetoutfit(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(outfitToTable(L, c.GetOutfit()))
	return 1
}

func (e *Engine) creatureGetparent(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		L.Push(lua.LNil)
		return 1
	}
	if e.world == nil {
		L.Push(lua.LNil)
		return 1
	}
	pos := c.GetPosition()
	tile := e.world.Map.GetTile(pos)
	if tile == nil {
		L.Push(lua.LNil)
		return 1
	}
	pushTile(L, tile, pos)
	return 1
}

func creatureGetpathto(L *lua.LState) int {
	// Pathfinding-to-position result table; not modelled for Lua yet.
	L.Push(lua.LNil)
	return 1
}

func creatureGetposition(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		L.Push(lua.LNil)
		return 1
	}
	pushPosition(L, c.GetPosition())
	return 1
}

func creatureGetshader(L *lua.LState) int {
	L.Push(lua.LString(""))
	return 1
}

func creatureGetskull(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func creatureGetspeed(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LNumber(c.GetSpeed()))
		return 1
	}
	L.Push(lua.LNumber(0))
	return 1
}

func creatureGetsummons(L *lua.LState) int {
	// No summon system yet: an empty list keeps #getSummons() == 0 working.
	L.Push(L.NewTable())
	return 1
}

func creatureGettarget(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		target := c.GetTarget()
		if target != nil {
			ud := L.NewUserData()
			ud.Value = target
			L.Push(ud)
			return 1
		}
	}
	return 0
}

func (e *Engine) creatureGettile(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		L.Push(lua.LNil)
		return 1
	}
	if e.world == nil {
		L.Push(lua.LNil)
		return 1
	}
	pos := c.GetPosition()
	tile := e.world.Map.GetTile(pos)
	if tile == nil {
		L.Push(lua.LNil)
		return 1
	}
	pushTile(L, tile, pos)
	return 1
}

func creatureGettypename(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		switch c.GetCreatureType() {
		case 0:
			L.Push(lua.LString("Player"))
		default:
			L.Push(lua.LString(c.GetName()))
		}
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func (e *Engine) creatureGetzonetype(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil || e.world == nil || e.world.Map == nil {
		L.Push(lua.LNumber(4)) // ZONE_NORMAL
		return 1
	}
	pos := c.GetPosition()
	tile := e.world.Map.GetTile(pos)
	if tile == nil {
		L.Push(lua.LNumber(4)) // ZONE_NORMAL
		return 1
	}
	if tile.IsProtectionZone() {
		L.Push(lua.LNumber(0)) // ZONE_PROTECTION
		return 1
	}
	if (tile.Flags & game.TileFlagPvpZone) != 0 {
		L.Push(lua.LNumber(2)) // ZONE_PVP
		return 1
	}
	if (tile.Flags & game.TileFlagNoPvpZone) != 0 {
		L.Push(lua.LNumber(1)) // ZONE_NOPVP
		return 1
	}
	if (tile.Flags & game.TileFlagNoLogout) != 0 {
		L.Push(lua.LNumber(3)) // ZONE_NOLOGOUT
		return 1
	}
	L.Push(lua.LNumber(4)) // ZONE_NORMAL
	return 1
}

func creatureGetzones(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func creatureHasbeensummoned(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func creatureHascondition(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		L.Push(lua.LFalse)
		return 1
	}
	holder, ok := creatureConditions(c)
	if !ok {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LBool(holder.HasCondition(luaToConditionType(luaOptInt(L, 2)))))
	return 1
}

func creatureIscreature(L *lua.LState) int {
	L.Push(lua.LBool(checkCreature(L) != nil))
	return 1
}

func creatureIsdirectionlocked(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func creatureIshealthhidden(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func creatureIsimmune(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func creatureIsinghostmode(L *lua.LState) int {
	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if p, ok := ud.Value.(*game.Player); ok {
			L.Push(lua.LBool(p.Ghost))
			return 1
		}
	}
	L.Push(lua.LFalse)
	return 1
}

func creatureIsmovelocked(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func creatureIsremoved(L *lua.LState) int {
	// No removal lifecycle flag on the model yet; a live userdata is present.
	L.Push(lua.LFalse)
	return 1
}

func creatureMove(L *lua.LState) int {
	// Directional walk step; the movement engine drives creatures directly.
	L.Push(lua.LNumber(0))
	return 1
}

func creatureRegisterevent(L *lua.LState) int {
	// Event scripting isn't wired to creatures yet; accept so scripts continue.
	L.Push(lua.LTrue)
	return 1
}

func creatureReload(L *lua.LState) int { return 0 }

func (e *Engine) creatureRemove(L *lua.LState) int {
	c := checkCreature(L)
	if c != nil {
		if p, ok := c.(*game.Player); ok {
			if p.Session != nil {
				go p.Session.Disconnect()
			}
		} else {
			if e.world != nil {
				id := c.GetID()
				game.GlobalDispatcher.AddEvent(0, func() {
					e.world.RemoveCreature(id)
				})
			}
		}
	}
	L.Push(lua.LTrue)
	return 1
}

func creatureRemovecondition(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		return 0
	}
	if holder, ok := creatureConditions(c); ok {
		holder.RemoveCondition(luaToConditionType(luaOptInt(L, 2)))
	}
	return 0
}

func creatureRemoveicon(L *lua.LState) int { return 0 }

func creatureSay(L *lua.LState) int {
	// Creature speech broadcast isn't routed from the free function (no engine
	// handle here); accept the call so scripted yells don't error.
	L.Push(lua.LTrue)
	return 1
}

func creatureSetdirection(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		return 0
	}
	c.SetDirection(game.Direction(luaOptInt(L, 2)))
	L.Push(lua.LTrue)
	return 1
}

func creatureSetdirectionlocked(L *lua.LState) int { return 0 }

func creatureSetdroploot(L *lua.LState) int { return 0 }

func creatureSetfollowcreature(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func creatureSethealth(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		return 0
	}
	health := uint32(L.CheckNumber(2))
	game.GlobalDispatcher.AddEvent(0, func() {
		c.SetHealth(health)
	})
	return 0
}

func creatureSethiddenhealth(L *lua.LState) int { return 0 }

func creatureSeticon(L *lua.LState) int { return 0 }

func creatureSetlight(L *lua.LState) int { return 0 }

func creatureSetmaster(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func creatureSetmaxhealth(L *lua.LState) int {
	// No max-health setter on the interface yet; accept so boss-buff scripts run.
	L.Push(lua.LTrue)
	return 1
}

func creatureSetmovelocked(L *lua.LState) int { return 0 }

func creatureSetoutfit(L *lua.LState) int {
	// Outfit mutation isn't exposed through the interface yet; accept the call.
	L.Push(lua.LTrue)
	return 1
}

func creatureSetshader(L *lua.LState) int { return 0 }

func creatureSetskillloss(L *lua.LState) int { return 0 }

func creatureSetskull(L *lua.LState) int { return 0 }

func (e *Engine) creatureSetspeed(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		speed := L.CheckInt(2)
		// C++ setCreatureSpeed changes base speed, then broadcasts
		// We'll mimic this by finding the difference from current base speed
		// Wait, Creature doesn't have SetBaseSpeed. We can just add SetBaseSpeed,
		// but since we only use ChangeSpeed, let's just adjust the SpeedBonus
		// so that GetSpeed() == speed.
		// Actually, in Canary setCreatureSpeed does: creature->setBaseSpeed(speed); then broadcasts.
		// For now we'll just set speed delta to match requested speed.
		delta := int32(speed) - int32(c.GetBaseSpeed())
		e.world.ChangeSpeed(c, delta-int32(c.GetSpeed())+int32(c.GetBaseSpeed()))
		// Wait, ChangeSpeed adds to SpeedBonus.
		// Current Speed = BaseSpeed + SpeedBonus.
		// We want New Speed = speed.
		// So New SpeedBonus = speed - BaseSpeed.
		// Delta to apply = New SpeedBonus - Current SpeedBonus
		//                = (speed - BaseSpeed) - (Speed - BaseSpeed) // Wait, SpeedBonus = Speed - BaseSpeed
		//                = speed - Speed
		deltaToApply := int32(speed) - int32(c.GetSpeed())
		e.world.ChangeSpeed(c, deltaToApply)
	}
	return 0
}

func creatureSettarget(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		return 0
	}
	target := getCreature(L, 2)
	game.GlobalDispatcher.AddEvent(0, func() {
		c.SetTarget(target)
	})
	return 0
}

// creatureTeleportto relocates a creature to a Position (userdata) or {x,y,z}
// table and broadcasts the jump so the client actually moves (scripted travel).
func (e *Engine) creatureTeleportto(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		return 0
	}
	var pos game.Position
	switch v := L.Get(2); v.Type() {
	case lua.LTUserData:
		if p, ok := v.(*lua.LUserData).Value.(game.Position); ok {
			pos = p
		}
	case lua.LTTable:
		t := v.(*lua.LTable)
		pos = game.Position{
			X: uint16(lua.LVAsNumber(L.GetField(t, "x"))),
			Y: uint16(lua.LVAsNumber(L.GetField(t, "y"))),
			Z: uint8(lua.LVAsNumber(L.GetField(t, "z"))),
		}
	default:
		L.Push(lua.LFalse)
		return 1
	}

	if e.world != nil {
		e.world.TeleportCreature(c, pos)
	} else {
		c.SetPosition(pos)
	}
	L.Push(lua.LTrue)
	return 1
}

func creatureUnregisterevent(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}
