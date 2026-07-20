package luaengine

import (
	lua "github.com/yuin/gopher-lua"
	"github.com/opentibiabr/canary-go/internal/game"
)

func getCreature(L *lua.LState, index int) game.Creature {
	ud := L.CheckUserData(index)
	if v, ok := ud.Value.(game.Creature); ok {
		return v
	}
	L.ArgError(index, "Creature expected")
	return nil
}

func checkCreature(L *lua.LState) game.Creature {
	return getCreature(L, 1)
}

// registerCreatureType registers the Creature userdata type.
func (e *Engine) registerCreatureType() {
	mt := e.L.NewTypeMetatable("Creature")
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), creatureMethods))
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
	"teleportTo": creatureTeleportto,
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
	// TODO: implement addCondition
	return 0
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

func creatureAttacheffectbyid(L *lua.LState) int {
	// TODO: implement attachEffectById
	return 0
}

func creatureCansee(L *lua.LState) int {
	// TODO: implement canSee
	return 0
}

func creatureCanseecreature(L *lua.LState) int {
	// TODO: implement canSeeCreature
	return 0
}

func creatureChangespeed(L *lua.LState) int {
	// TODO: implement changeSpeed
	return 0
}

func creatureClearicons(L *lua.LState) int {
	// TODO: implement clearIcons
	return 0
}

func creatureDetacheffectbyid(L *lua.LState) int {
	// TODO: implement detachEffectById
	return 0
}

func creatureGetattachedeffects(L *lua.LState) int {
	// TODO: implement getAttachedEffects
	return 0
}

func creatureGetbasespeed(L *lua.LState) int {
	// TODO: implement getBaseSpeed
	return 0
}

func creatureGetcondition(L *lua.LState) int {
	// TODO: implement getCondition
	return 0
}

func creatureGetdamagemap(L *lua.LState) int {
	// TODO: implement getDamageMap
	return 0
}

func creatureGetdescription(L *lua.LState) int {
	// TODO: implement getDescription
	return 0
}

func creatureGetdirection(L *lua.LState) int {
	// TODO: implement getDirection
	return 0
}

func creatureGetevents(L *lua.LState) int {
	// TODO: implement getEvents
	return 0
}

func creatureGetfollowcreature(L *lua.LState) int {
	// TODO: implement getFollowCreature
	return 0
}

func creatureGethealth(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LNumber(c.GetHealth()))
		return 1
	}
	return 0
}

func creatureGeticon(L *lua.LState) int {
	// TODO: implement getIcon
	return 0
}

func creatureGeticons(L *lua.LState) int {
	// TODO: implement getIcons
	return 0
}

func creatureGetid(L *lua.LState) int {
	// TODO: implement getId
	return 0
}

func creatureGetlight(L *lua.LState) int {
	// TODO: implement getLight
	return 0
}

func creatureGetmaster(L *lua.LState) int {
	// TODO: implement getMaster
	return 0
}

func creatureGetmaxhealth(L *lua.LState) int {
	if c := checkCreature(L); c != nil {
		L.Push(lua.LNumber(c.GetMaxHealth()))
		return 1
	}
	return 0
}

func creatureGetname(L *lua.LState) int {
	// TODO: implement getName
	return 0
}

func creatureGetoutfit(L *lua.LState) int {
	// TODO: implement getOutfit
	return 0
}

func creatureGetparent(L *lua.LState) int {
	// TODO: implement getParent
	return 0
}

func creatureGetpathto(L *lua.LState) int {
	// TODO: implement getPathTo
	return 0
}

func creatureGetposition(L *lua.LState) int {
	// TODO: implement getPosition
	return 0
}

func creatureGetshader(L *lua.LState) int {
	// TODO: implement getShader
	return 0
}

func creatureGetskull(L *lua.LState) int {
	// TODO: implement getSkull
	return 0
}

func creatureGetspeed(L *lua.LState) int {
	// TODO: implement getSpeed
	return 0
}

func creatureGetsummons(L *lua.LState) int {
	// TODO: implement getSummons
	return 0
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
	// TODO: implement getTile
	return 0
}

func creatureGettypename(L *lua.LState) int {
	// TODO: implement getTypeName
	return 0
}

func creatureGetzonetype(L *lua.LState) int {
	// TODO: implement getZoneType
	return 0
}

func creatureGetzones(L *lua.LState) int {
	// TODO: implement getZones
	return 0
}

func creatureHasbeensummoned(L *lua.LState) int {
	// TODO: implement hasBeenSummoned
	return 0
}

func creatureHascondition(L *lua.LState) int {
	// TODO: implement hasCondition
	return 0
}

func creatureIscreature(L *lua.LState) int {
	// TODO: implement isCreature
	return 0
}

func creatureIsdirectionlocked(L *lua.LState) int {
	// TODO: implement isDirectionLocked
	return 0
}

func creatureIshealthhidden(L *lua.LState) int {
	// TODO: implement isHealthHidden
	return 0
}

func creatureIsimmune(L *lua.LState) int {
	// TODO: implement isImmune
	return 0
}

func creatureIsinghostmode(L *lua.LState) int {
	// TODO: implement isInGhostMode
	return 0
}

func creatureIsmovelocked(L *lua.LState) int {
	// TODO: implement isMoveLocked
	return 0
}

func creatureIsremoved(L *lua.LState) int {
	// TODO: implement isRemoved
	return 0
}

func creatureMove(L *lua.LState) int {
	// TODO: implement move
	return 0
}

func creatureRegisterevent(L *lua.LState) int {
	// TODO: implement registerEvent
	return 0
}

func creatureReload(L *lua.LState) int {
	// TODO: implement reload
	return 0
}

func creatureRemove(L *lua.LState) int {
	// TODO: implement remove
	return 0
}

func creatureRemovecondition(L *lua.LState) int {
	// TODO: implement removeCondition
	return 0
}

func creatureRemoveicon(L *lua.LState) int {
	// TODO: implement removeIcon
	return 0
}

func creatureSay(L *lua.LState) int {
	// TODO: implement say
	return 0
}

func creatureSetdirection(L *lua.LState) int {
	// TODO: implement setDirection
	return 0
}

func creatureSetdirectionlocked(L *lua.LState) int {
	// TODO: implement setDirectionLocked
	return 0
}

func creatureSetdroploot(L *lua.LState) int {
	// TODO: implement setDropLoot
	return 0
}

func creatureSetfollowcreature(L *lua.LState) int {
	// TODO: implement setFollowCreature
	return 0
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

func creatureSethiddenhealth(L *lua.LState) int {
	// TODO: implement setHiddenHealth
	return 0
}

func creatureSeticon(L *lua.LState) int {
	// TODO: implement setIcon
	return 0
}

func creatureSetlight(L *lua.LState) int {
	// TODO: implement setLight
	return 0
}

func creatureSetmaster(L *lua.LState) int {
	// TODO: implement setMaster
	return 0
}

func creatureSetmaxhealth(L *lua.LState) int {
	// TODO: implement setMaxHealth
	return 0
}

func creatureSetmovelocked(L *lua.LState) int {
	// TODO: implement setMoveLocked
	return 0
}

func creatureSetoutfit(L *lua.LState) int {
	// TODO: implement setOutfit
	return 0
}

func creatureSetshader(L *lua.LState) int {
	// TODO: implement setShader
	return 0
}

func creatureSetskillloss(L *lua.LState) int {
	// TODO: implement setSkillLoss
	return 0
}

func creatureSetskull(L *lua.LState) int {
	// TODO: implement setSkull
	return 0
}

func creatureSetspeed(L *lua.LState) int {
	// TODO: implement setSpeed
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

func creatureTeleportto(L *lua.LState) int {
	c := checkCreature(L)
	if c == nil {
		return 0
	}
	// pos is a table at index 2
	posTable := L.CheckTable(2)
	pos := game.Position{
		X: uint16(L.GetField(posTable, "x").(lua.LNumber)),
		Y: uint16(L.GetField(posTable, "y").(lua.LNumber)),
		Z: uint8(L.GetField(posTable, "z").(lua.LNumber)),
	}

	c.SetPosition(pos)

	L.Push(lua.LTrue)
	return 1
}

func creatureUnregisterevent(L *lua.LState) int {
	// TODO: implement unregisterEvent
	return 0
}

