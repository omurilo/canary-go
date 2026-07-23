package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	lua "github.com/yuin/gopher-lua"
)

// registerGame registers the global Game table and its methods.
func (e *Engine) registerGame() {
	gameMethods := map[string]lua.LGFunction{
		"getMonsterTypeByName":     e.gameGetMonsterTypeByName,
		"getSpectators":            e.gameGetSpectators,
		"getBoostedCreature":       e.gameGetBoostedCreature,
		"getBestiaryList":          e.gameGetBestiaryList,
		"getPlayers":               e.gameGetPlayers,
		"loadMap":                  e.gameLoadMap,
		"loadMapChunk":             e.gameLoadMapChunk,
		"getExperienceForLevel":    e.gameGetExperienceForLevel,
		"getMonsterCount":          e.gameGetMonsterCount,
		"getPlayerCount":           e.gameGetPlayerCount,
		"getNpcCount":              e.gameGetNpcCount,
		"getMonsterTypes":          e.gameGetMonsterTypes,
		"getTowns":                 e.gameGetTowns,
		"getHouses":                e.gameGetHouses,
		"getGameState":             e.gameGetGameState,
		"setGameState":             e.gameSetGameState,
		"getWorldType":             e.gameGetWorldType,
		"setWorldType":             e.gameSetWorldType,
		"getReturnMessage":         e.gameGetReturnMessage,
		"createItem":               e.gameCreateItem,
		"createContainer":          e.gameCreateContainer,
		"createMonster":            e.gameCreateMonster,
		"createSoulPitMonster":     e.gameCreateSoulPitMonster,
		"createNpc":                e.gameCreateNpc,
		"generateNpc":              e.gameGenerateNpc,
		"createTile":               e.gameCreateTile,
		"createBestiaryCharm":      e.gameCreateBestiaryCharm,
		"createItemClassification": e.gameCreateItemClassification,
		"getBestiaryCharm":         e.gameGetBestiaryCharm,
		"startRaid":                e.gameStartRaid,
		"getClientVersion":         e.gameGetClientVersion,
		"reload":                   e.gameReload,
		"hasDistanceEffect":        e.gameHasDistanceEffect,
		"hasEffect":                e.gameHasEffect,
		"getOfflinePlayer":         e.gameGetOfflinePlayer,
		"getNormalizedPlayerName":  e.gameGetNormalizedPlayerName,
		"getNormalizedGuildName":   e.gameGetNormalizedGuildName,
		"addInfluencedMonster":     e.gameAddInfluencedMonster,
		"removeInfluencedMonster":  e.gameRemoveInfluencedMonster,
		"getInfluencedMonsters":    e.gameGetInfluencedMonsters,
		"makeFiendishMonster":      e.gameMakeFiendishMonster,
		"removeFiendishMonster":    e.gameRemoveFiendishMonster,
		"getFiendishMonsters":      e.gameGetFiendishMonsters,
		"getBoostedBoss":           e.gameGetBoostedBoss,
		"getLadderIds":             e.gameGetLadderIds,
		"getDummies":               e.gameGetDummies,
		"getTalkActions":           e.gameGetTalkActions,
		"getEventCallbacks":        e.gameGetEventCallbacks,
		"registerAchievement":      e.gameRegisterAchievement,
		"getAchievementInfoById":   e.gameGetAchievementInfoById,
		"getAchievementInfoByName": e.gameGetAchievementInfoByName,
		"getSecretAchievements":    e.gameGetSecretAchievements,
		"getPublicAchievements":    e.gameGetPublicAchievements,
		"getAchievements":          e.gameGetAchievements,
		"getSoulCoreItems":         e.gameGetSoulCoreItems,
		"getMonstersByRace":        e.gameGetMonstersByRace,
		"getMonstersByBestiaryStars": e.gameGetMonstersByBestiaryStars,
		"broadcastMessage":         e.gameBroadcastMessage,
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	game, ok := e.L.GetGlobal("Game").(*lua.LTable)
	if !ok {
		game = e.L.NewTable()
		e.L.SetGlobal("Game", game)
	}

	for name, fn := range gameMethods {
		e.L.SetField(game, name, e.L.NewFunction(fn))
	}

	// Global world-time helpers used by NPC greetings / day-night scripts
	// (data/libs/functions/functions.lua getFormattedWorldTime → getWorldTime).
	// No day/night clock is modelled yet, so report a stable midday: enough for
	// the "day"/"night" branch and the |TIME| dialog tag to resolve.
	e.L.SetGlobal("getWorldTime", e.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(720)) // minutes since midnight → 12:00
		return 1
	}))
	e.L.SetGlobal("getWorldLight", e.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(250)) // full daylight level
		L.Push(lua.LNumber(215)) // default light color
		return 2
	}))
}


func (e *Engine) gameGetMonsterTypeByName(L *lua.LState) int { return 0 }
func (e *Engine) gameGetSpectators(L *lua.LState) int { return 0 }
func (e *Engine) gameGetBoostedCreature(L *lua.LState) int { return 0 }
func (e *Engine) gameGetBestiaryList(L *lua.LState) int { return 0 }
func (e *Engine) gameGetPlayers(L *lua.LState) int {
	var players []*game.Player
	if e.world != nil {
		players = e.world.Players()
	}

	table := L.NewTable()
	for _, p := range players {
		ud := L.NewUserData()
		ud.Value = p
		L.SetMetatable(ud, L.GetTypeMetatable("Player"))
		table.Append(ud)
	}
	L.Push(table)
	return 1
}
func (e *Engine) gameLoadMap(L *lua.LState) int { return 0 }
func (e *Engine) gameLoadMapChunk(L *lua.LState) int { return 0 }
func (e *Engine) gameGetExperienceForLevel(L *lua.LState) int { return 0 }
func (e *Engine) gameGetMonsterCount(L *lua.LState) int { return 0 }
func (e *Engine) gameGetPlayerCount(L *lua.LState) int { return 0 }
func (e *Engine) gameGetNpcCount(L *lua.LState) int { return 0 }
func (e *Engine) gameGetMonsterTypes(L *lua.LState) int { return 0 }
func (e *Engine) gameGetTowns(L *lua.LState) int { return 0 }
func (e *Engine) gameGetHouses(L *lua.LState) int { return 0 }
func (e *Engine) gameGetGameState(L *lua.LState) int { return 0 }
func (e *Engine) gameSetGameState(L *lua.LState) int { return 0 }
func (e *Engine) gameGetWorldType(L *lua.LState) int { return 0 }
func (e *Engine) gameSetWorldType(L *lua.LState) int { return 0 }
func (e *Engine) gameGetReturnMessage(L *lua.LState) int { return 0 }
func (e *Engine) gameCreateItem(L *lua.LState) int {
	id := L.CheckInt(1)
	count := L.OptInt(2, 1)

	cat := e.itemCatalog()
	if it := cat.Get(uint16(id)); it != nil && it.Stackable && count > 100 {
		count = 100
	}

	item := &game.Item{
		ID:    uint16(id),
		Count: uint16(count),
	}
	e.pushItem(L, item)
	return 1
}
func (e *Engine) gameCreateContainer(L *lua.LState) int { return 0 }

func (e *Engine) gameCreateMonster(L *lua.LState) int {
	name := L.CheckString(1)
	pos, ok := parsePosition(L, 2)
	if !ok {
		L.Push(lua.LNil)
		return 1
	}

	if e.world == nil || e.world.Map == nil {
		L.Push(lua.LNil)
		return 1
	}

	tile := e.world.Map.GetTile(pos)
	if tile == nil {
		L.Push(lua.LNil)
		return 1
	}

	id := e.world.GenerateCreatureID()
	var mType *creatures.MonsterType
	if e.world.Monsters != nil {
		mType = e.world.Monsters.Monsters[strings.ToLower(name)]
		if mType == nil {
			mType = e.world.Monsters.Monsters[name]
		}
	}

	m := game.NewMonster(id, name, mType)
	m.SetPosition(pos)
	e.world.AddCreature(m)

	if !e.pushCreatureAs(L, m, "Monster") {
		L.Push(lua.LNil)
	}
	return 1
}

func (e *Engine) gameCreateSoulPitMonster(L *lua.LState) int { return 0 }

func (e *Engine) gameCreateNpc(L *lua.LState) int {
	name := L.CheckString(1)
	pos, ok := parsePosition(L, 2)
	if !ok {
		L.Push(lua.LNil)
		return 1
	}

	if e.world == nil || e.world.Map == nil {
		L.Push(lua.LNil)
		return 1
	}

	tile := e.world.Map.GetTile(pos)
	if tile == nil {
		L.Push(lua.LNil)
		return 1
	}

	id := e.world.GenerateCreatureID()
	var nType *creatures.NpcType
	if e.world.Monsters != nil {
		nType = e.world.Monsters.Npcs[strings.ToLower(name)]
		if nType == nil {
			nType = e.world.Monsters.Npcs[name]
		}
	}

	npc := game.NewNpc(id, name, nType)
	npc.SetPosition(pos)
	e.world.AddCreature(npc)

	if !e.pushCreatureAs(L, npc, "Npc") {
		L.Push(lua.LNil)
	}
	return 1
}

func (e *Engine) gameGenerateNpc(L *lua.LState) int { return 0 }
func (e *Engine) gameCreateTile(L *lua.LState) int { return 0 }
func (e *Engine) gameCreateBestiaryCharm(L *lua.LState) int {
	mt := L.GetTypeMetatable("BestiaryCharm")
	ud := L.NewUserData()
	ud.Value = "BestiaryCharm"
	L.SetMetatable(ud, mt)
	L.Push(ud)
	return 1
}
func (e *Engine) gameCreateItemClassification(L *lua.LState) int {
	mt := L.GetTypeMetatable("ItemClassification")
	ud := L.NewUserData()
	ud.Value = "ItemClassification"
	L.SetMetatable(ud, mt)
	L.Push(ud)
	return 1
}
func (e *Engine) gameGetBestiaryCharm(L *lua.LState) int { return 0 }
func (e *Engine) gameStartRaid(L *lua.LState) int { return 0 }
func (e *Engine) gameGetClientVersion(L *lua.LState) int { return 0 }
func (e *Engine) gameReload(L *lua.LState) int { return 0 }
func (e *Engine) gameHasDistanceEffect(L *lua.LState) int { return 0 }
func (e *Engine) gameHasEffect(L *lua.LState) int { return 0 }
func (e *Engine) gameGetOfflinePlayer(L *lua.LState) int { return 0 }
func (e *Engine) gameGetNormalizedPlayerName(L *lua.LState) int {
	name := L.CheckString(1)
	if name == "" {
		L.Push(lua.LNil)
		return 1
	}

	if e.world != nil {
		if p := e.world.PlayerByName(name); p != nil {
			L.Push(lua.LString(p.Name))
			return 1
		}
	}

	if e.database != nil && e.database.SQL != nil {
		var dbName string
		err := e.database.SQL.QueryRow("SELECT name FROM players WHERE LOWER(name) = LOWER(?) LIMIT 1", name).Scan(&dbName)
		if err == nil && dbName != "" {
			L.Push(lua.LString(dbName))
			return 1
		}
	}

	L.Push(lua.LNil)
	return 1
}
func (e *Engine) gameGetNormalizedGuildName(L *lua.LState) int { return 0 }
func (e *Engine) gameAddInfluencedMonster(L *lua.LState) int { return 0 }
func (e *Engine) gameRemoveInfluencedMonster(L *lua.LState) int { return 0 }
func (e *Engine) gameGetInfluencedMonsters(L *lua.LState) int { return 0 }
func (e *Engine) gameMakeFiendishMonster(L *lua.LState) int { return 0 }
func (e *Engine) gameRemoveFiendishMonster(L *lua.LState) int { return 0 }
func (e *Engine) gameGetFiendishMonsters(L *lua.LState) int { return 0 }
func (e *Engine) gameGetBoostedBoss(L *lua.LState) int { return 0 }
func (e *Engine) gameGetLadderIds(L *lua.LState) int { return 0 }
func (e *Engine) gameGetDummies(L *lua.LState) int {
	dummies := map[uint16]uint16{
		28558: 100,
		28559: 110,
		28560: 110,
		28561: 110,
		28562: 110,
		28563: 110,
		28564: 110,
		28565: 100,
	}
	tbl := L.NewTable()
	for k, v := range dummies {
		tbl.RawSetInt(int(k), lua.LNumber(v))
	}
	L.Push(tbl)
	return 1
}
func (e *Engine) gameGetTalkActions(L *lua.LState) int { return 0 }
func (e *Engine) gameGetEventCallbacks(L *lua.LState) int { return 0 }
func (e *Engine) gameRegisterAchievement(L *lua.LState) int { return 0 }
func (e *Engine) gameGetAchievementInfoById(L *lua.LState) int { return 0 }
func (e *Engine) gameGetAchievementInfoByName(L *lua.LState) int { return 0 }
func (e *Engine) gameGetSecretAchievements(L *lua.LState) int { return 0 }
func (e *Engine) gameGetPublicAchievements(L *lua.LState) int { return 0 }
func (e *Engine) gameGetAchievements(L *lua.LState) int { return 0 }
func (e *Engine) gameGetSoulCoreItems(L *lua.LState) int { return 0 }
func (e *Engine) gameGetMonstersByRace(L *lua.LState) int { return 0 }
func (e *Engine) gameGetMonstersByBestiaryStars(L *lua.LState) int { return 0 }
func (e *Engine) gameBroadcastMessage(L *lua.LState) int {
	message := L.CheckString(1)
	messageType := L.OptInt(2, 0xB4) // opTextMessage (e.g. MESSAGE_STATUS_WARNING)

	if e.world != nil {
		for _, p := range e.world.Players() {
			if p.Session != nil {
				w := netmsg.NewWriter()
				w.AddByte(0xB4) // opTextMessage
				w.AddByte(byte(messageType))
				w.AddString(message)
				p.Session.SendToClient(w)
			}
		}
	}
	return 0
}
