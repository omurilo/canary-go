package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/combat"
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
	switch arg := L.Get(2); arg.Type() {
	case lua.LTUserData:
		if v, ok := arg.(*lua.LUserData).Value.(game.Creature); ok {
			c = v
		}
	case lua.LTNumber:
		if e.world != nil {
			c = e.world.CreatureByID(uint32(lua.LVAsNumber(arg)))
		}
	}
	if !e.pushCreatureAs(L, c, kind) {
		L.Push(lua.LNil)
	}
	return 1
}

// pushCreatureAs pushes c as a userdata bound to the metatable for kind
// ("Player"/"Npc"/"Monster"/"Creature"). It returns false (pushing nothing)
// when c is nil or doesn't match the requested concrete kind, so the caller
// can push nil — mirroring the C++ Player(cid)/Npc(cid) casts that yield nil
// on a type mismatch.
func (e *Engine) pushCreatureAs(L *lua.LState, c game.Creature, kind string) bool {
	if c == nil {
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
func (e *Engine) registerCreatureType() {
	mt := e.L.NewTypeMetatable("Creature")
	idx := e.L.SetFuncs(e.L.NewTable(), creatureMethods)
	// teleportTo needs the world (to broadcast the jump), so it's engine-bound.
	e.L.SetField(idx, "teleportTo", e.L.NewFunction(e.creatureTeleportto))
	e.L.SetField(mt, "__index", idx)
}

var creatureMethods = map[string]lua.LGFunction{
	"getEvents": creatureGetevents,
	"registerEvent": creatureRegisterevent,
	"unregisterEvent": creatureUnregisterevent,
	"isRemoved": creatureIsremoved,
	"isCreature": creatureIscreature,
	"isInGhostMode": creatureIsinghostmode,
	"isHealthHidden": creatureIshealthhidden,
	"isImmune": creatureIsimmune,
	"canSee": creatureCansee,
	"canSeeCreature": creatureCanseecreature,
	"getParent": creatureGetparent,
	"getId": creatureGetid,
	"getName": creatureGetname,
	"getTypeName": creatureGettypename,
	"getTarget": creatureGettarget,
	"setTarget": creatureSettarget,
	"getFollowCreature": creatureGetfollowcreature,
	"setFollowCreature": creatureSetfollowcreature,
	"reload": creatureReload,
	"getMaster": creatureGetmaster,
	"setMaster": creatureSetmaster,
	"getLight": creatureGetlight,
	"setLight": creatureSetlight,
	"getSpeed": creatureGetspeed,
	"setSpeed": creatureSetspeed,
	"getBaseSpeed": creatureGetbasespeed,
	"changeSpeed": creatureChangespeed,
	"setDropLoot": creatureSetdroploot,
	"setSkillLoss": creatureSetskillloss,
	"getPosition": creatureGetposition,
	"getTile": creatureGettile,
	"getDirection": creatureGetdirection,
	"setDirection": creatureSetdirection,
	"getHealth": creatureGethealth,
	"setHealth": creatureSethealth,
	"addHealth": creatureAddhealth,
	"getMaxHealth": creatureGetmaxhealth,
	"setMaxHealth": creatureSetmaxhealth,
	"setHiddenHealth": creatureSethiddenhealth,
	"isMoveLocked": creatureIsmovelocked,
	"isDirectionLocked": creatureIsdirectionlocked,
	"setMoveLocked": creatureSetmovelocked,
	"setDirectionLocked": creatureSetdirectionlocked,
	"getSkull": creatureGetskull,
	"setSkull": creatureSetskull,
	"getOutfit": creatureGetoutfit,
	"setOutfit": creatureSetoutfit,
	"getCondition": creatureGetcondition,
	"addCondition": creatureAddcondition,
	"removeCondition": creatureRemovecondition,
	"hasCondition": creatureHascondition,
	"remove": creatureRemove,
	"say": creatureSay,
	"getDamageMap": creatureGetdamagemap,
	"getSummons": creatureGetsummons,
	"hasBeenSummoned": creatureHasbeensummoned,
	"getDescription": creatureGetdescription,
	"getPathTo": creatureGetpathto,
	"move": creatureMove,
	"getZoneType": creatureGetzonetype,
	"getZones": creatureGetzones,
	"setIcon": creatureSeticon,
	"getIcon": creatureGeticon,
	"getIcons": creatureGeticons,
	"removeIcon": creatureRemoveicon,
	"clearIcons": creatureClearicons,
	"attachEffectById": creatureAttacheffectbyid,
	"detachEffectById": creatureDetacheffectbyid,
	"getAttachedEffects": creatureGetattachedeffects,
	"getShader": creatureGetshader,
	"setShader": creatureSetshader,
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
	if !ok || cond.condType == combat.ConditionNone {
		return 0
	}
	holder.AddCondition(&combat.ConditionGeneric{Type: cond.condType, Ticks: cond.ticks})
	L.Push(lua.LTrue)
	return 1
}

func creatureAddhealth(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		return 0
	}
	amount := int32(L.CheckNumber(2))
	game.GlobalDispatcher.AddEvent(0, func() {
		c.AddHealth(amount)
	})
	return 0
}

func creatureAttacheffectbyid(L *lua.LState) int { return 0 }

func creatureCansee(L *lua.LState) int {
	// No line-of-sight model yet; assume visibility so scripts proceed.
	L.Push(lua.LTrue)
	return 1
}

func creatureCanseecreature(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func creatureChangespeed(L *lua.LState) int { return 0 }

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
	// Returns the condition object; the store keeps them but doesn't expose a
	// lookup-by-type yet, so report absence (nil) rather than a wrong object.
	L.Push(lua.LNil)
	return 1
}

func creatureGetdamagemap(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func creatureGetdescription(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LString(c.GetName()))
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

func creatureGetparent(L *lua.LState) int {
	// Parent is the tile/container holding the creature; not modelled yet.
	L.Push(lua.LNil)
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
	// SKULL_NONE (0): skull system not modelled yet.
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

func creatureGettile(L *lua.LState) int {
	// Tile userdata for the creature's position isn't wired yet.
	L.Push(lua.LNil)
	return 1
}

func creatureGettypename(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LString(c.GetName()))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func creatureGetzonetype(L *lua.LState) int {
	L.Push(lua.LNumber(0))
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

func creatureRemove(L *lua.LState) int {
	// Removal lifecycle isn't exposed through the Creature interface; accept the
	// call (scripts use it to despawn temporary creatures) without erroring.
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

func creatureSetspeed(L *lua.LState) int { return 0 }

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
		game.GlobalDispatcher.AddEvent(0, func() { e.world.TeleportCreature(c, pos) })
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

