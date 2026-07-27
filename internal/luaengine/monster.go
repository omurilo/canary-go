package luaengine

import (
	lua "github.com/yuin/gopher-lua"
	"github.com/opentibiabr/canary-go/internal/game"
)

func checkMonster(L *lua.LState) *game.Monster {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*game.Monster); ok {
		return v
	}
	L.ArgError(1, "Monster expected")
	return nil
}

// registerMonster registers the Monster userdata type.
func (e *Engine) registerMonster() {
	mt := e.L.NewTypeMetatable("Monster")
	// Monster IS-A Creature: expose all creature methods, monster-specific win.
	// Methods directly on the metatable (see registerCreatureType) for
	// revscriptsys CreatureIndex compatibility.
	e.L.SetFuncs(mt, creatureMethods)
	e.L.SetFuncs(mt, monsterMethods)
	e.L.SetField(mt, "teleportTo", e.L.NewFunction(e.creatureTeleportto))
	e.L.SetField(mt, "changeSpeed", e.L.NewFunction(e.creatureChangespeed))
	e.L.SetField(mt, "setSpeed", e.L.NewFunction(e.creatureSetspeed))
	e.L.SetField(mt, "getParent", e.L.NewFunction(e.creatureGetparent))
	e.L.SetField(mt, "getTile", e.L.NewFunction(e.creatureGettile))
	e.L.SetField(mt, "remove", e.L.NewFunction(e.creatureRemove))
	e.L.SetField(mt, "__index", mt)
}

var monsterMethods = map[string]lua.LGFunction{
	"isMonster": monsterIsmonster,
	"getType": monsterGettype,
	"setType": monsterSettype,
	"getSpawnPosition": monsterGetspawnposition,
	"isInSpawnRange": monsterIsinspawnrange,
	"isIdle": monsterIsidle,
	"setIdle": monsterSetidle,
	"isTarget": monsterIstarget,
	"isOpponent": monsterIsopponent,
	"isFriend": monsterIsfriend,
	"addFriend": monsterAddfriend,
	"removeFriend": monsterRemovefriend,
	"getFriendList": monsterGetfriendlist,
	"getFriendCount": monsterGetfriendcount,
	"addTarget": monsterAddtarget,
	"removeTarget": monsterRemovetarget,
	"getTargetList": monsterGettargetlist,
	"getTargetCount": monsterGettargetcount,
	"changeTargetDistance": monsterChangetargetdistance,
	"isChallenged": monsterIschallenged,
	"selectTarget": monsterSelecttarget,
	"searchTarget": monsterSearchtarget,
	"setSpawnPosition": monsterSetspawnposition,
	"getRespawnType": monsterGetrespawntype,
	"getTimeToChangeFiendish": monsterGettimetochangefiendish,
	"setTimeToChangeFiendish": monsterSettimetochangefiendish,
	"getMonsterForgeClassification": monsterGetmonsterforgeclassification,
	"setMonsterForgeClassification": monsterSetmonsterforgeclassification,
	"getForgeStack": monsterGetforgestack,
	"setForgeStack": monsterSetforgestack,
	"configureForgeSystem": monsterConfigureforgesystem,
	"clearFiendishStatus": monsterClearfiendishstatus,
	"isForgeable": monsterIsforgeable,
	"getName": monsterGetname,
	"setName": monsterSetname,
	"hazard": monsterHazard,
	"hazardCrit": monsterHazardcrit,
	"hazardDodge": monsterHazarddodge,
	"hazardDamageBoost": monsterHazarddamageboost,
	"hazardDefenseBoost": monsterHazarddefenseboost,
	"soulPit": monsterSoulpit,
	"addReflectElement": monsterAddreflectelement,
	"addDefense": monsterAdddefense,
	"getDefense": monsterGetdefense,
	"isDead": monsterIsdead,
	"immune": monsterImmune,
	"criticalChance": monsterCriticalchance,
	"criticalDamage": monsterCriticaldamage,
	"addAttackSpell": monsterAddattackspell,
	"addDefenseSpell": monsterAdddefensespell,
}

func monsterAddattackspell(L *lua.LState) int {
	return 0
}

func monsterAdddefense(L *lua.LState) int {
	return 0
}

func monsterAdddefensespell(L *lua.LState) int {
	return 0
}

func monsterAddfriend(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	other := checkCreature(L)
	if other == nil {
		return 0
	}
	if m.Friends == nil {
		m.Friends = make(map[uint32]game.Creature)
	}
	m.Friends[other.GetID()] = other
	return 0
}

func monsterAddreflectelement(L *lua.LState) int {
	return 0
}

func monsterAddtarget(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	other := checkCreature(L)
	if other == nil {
		return 0
	}
	if m.Targets == nil {
		m.Targets = make(map[uint32]game.Creature)
	}
	m.Targets[other.GetID()] = other
	return 0
}

func monsterChangetargetdistance(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	distance := int32(L.CheckNumber(2))
	game.GlobalDispatcher.AddEvent(0, func() {
		m.ChangeTargetDistance(distance)
	})
	return 0
}

func monsterClearfiendishstatus(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	m.ClearFiendishStatus()
	return 0
}

func monsterConfigureforgesystem(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	stack := uint16(L.OptInt(2, 0))
	m.ConfigureForgeSystem(stack)
	return 0
}

func monsterCriticalchance(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func monsterCriticaldamage(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func monsterGetdefense(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func monsterGetforgestack(L *lua.LState) int {
	if m := checkMonster(L); m != nil {
		L.Push(lua.LNumber(m.ForgeStack))
		return 1
	}
	return 0
}

func monsterGetfriendcount(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(len(m.Friends)))
	return 1
}

func monsterGetfriendlist(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func monsterGetmonsterforgeclassification(L *lua.LState) int {
	if m := checkMonster(L); m != nil {
		L.Push(lua.LNumber(m.ForgeClassification))
		return 1
	}
	return 0
}

func monsterGetname(L *lua.LState) int {
	if m := checkMonster(L); m != nil {
		L.Push(lua.LString(m.GetName()))
		return 1
	}
	return 0
}

func monsterGetrespawntype(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func monsterGetspawnposition(L *lua.LState) int {
	c := checkMonster(L)
	if c == nil {
		L.Push(lua.LNil)
		return 1
	}
	pushPosition(L, c.SpawnPosition)
	return 1
}

func monsterGettargetcount(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(len(m.Targets)))
	return 1
}

func monsterGettargetlist(L *lua.LState) int {
	L.Push(L.NewTable())
	return 1
}

func monsterGettimetochangefiendish(L *lua.LState) int {
	if m := checkMonster(L); m != nil {
		L.Push(lua.LNumber(m.TimeToChangeFiendish))
		return 1
	}
	return 0
}

func monsterGettype(L *lua.LState) int {
	L.Push(lua.LNil)
	return 1
}

func monsterHazard(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func monsterHazardcrit(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func monsterHazarddamageboost(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func monsterHazarddefenseboost(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func monsterHazarddodge(L *lua.LState) int {
	L.Push(lua.LNumber(0))
	return 1
}

func monsterImmune(L *lua.LState) int {
	L.Push(lua.LFalse)
	return 1
}

func monsterIschallenged(L *lua.LState) int {
	// TODO: implement isChallenged
	return 0
}

func monsterIsdead(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LBool(m.GetHealth() == 0))
	return 1
}

func monsterIsforgeable(L *lua.LState) int {
	if m := checkMonster(L); m != nil {
		L.Push(lua.LBool(m.CanBeForgeMonster()))
		return 1
	}
	return 0
}

func monsterIsfriend(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		L.Push(lua.LNil)
		return 1
	}
	other := checkCreature(L)
	if other == nil {
		L.Push(lua.LFalse)
		return 1
	}
	_, ok := m.Friends[other.GetID()]
	L.Push(lua.LBool(ok))
	return 1
}

func monsterIsidle(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LBool(m.Idle))
	return 1
}

func monsterIsinspawnrange(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		L.Push(lua.LNil)
		return 1
	}
	dist := m.GetPosition().MaxDistance(m.SpawnPosition)
	L.Push(lua.LBool(dist < 30))
	return 1
}

func monsterIsmonster(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func monsterIsopponent(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LTrue)
	return 1
}

func monsterIstarget(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		L.Push(lua.LNil)
		return 1
	}
	other := checkCreature(L)
	if other == nil {
		L.Push(lua.LFalse)
		return 1
	}
	_, ok := m.Targets[other.GetID()]
	L.Push(lua.LBool(ok))
	return 1
}

func monsterRemovefriend(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	other := checkCreature(L)
	if other == nil {
		return 0
	}
	delete(m.Friends, other.GetID())
	return 0
}

func monsterRemovetarget(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	other := checkCreature(L)
	if other == nil {
		return 0
	}
	delete(m.Targets, other.GetID())
	return 0
}

func monsterSearchtarget(L *lua.LState) int {
	L.Push(lua.LNil)
	return 1
}

func monsterSelecttarget(L *lua.LState) int {
	// Stub selection logic
	return 0
}

func monsterSetforgestack(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	m.ForgeStack = uint16(L.CheckInt(2))
	m.ApplyStacks()
	return 0
}

func monsterSetidle(L *lua.LState) int {
	// TODO: implement setIdle
	return 0
}

func monsterSetmonsterforgeclassification(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	m.ForgeClassification = game.ForgeClassification(L.CheckInt(2))
	return 0
}

func monsterSetname(L *lua.LState) int {
	// TODO: implement setName
	return 0
}

func monsterSetspawnposition(L *lua.LState) int {
	// TODO: implement setSpawnPosition
	return 0
}

func monsterSettimetochangefiendish(L *lua.LState) int {
	m := checkMonster(L)
	if m == nil {
		return 0
	}
	m.TimeToChangeFiendish = int64(L.CheckInt(2))
	return 0
}

func monsterSettype(L *lua.LState) int {
	// TODO: implement setType
	return 0
}

func monsterSoulpit(L *lua.LState) int {
	// TODO: implement soulPit
	return 0
}

