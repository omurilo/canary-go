package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/otbm"
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


func (e *Engine) gameGetMonsterTypeByName(L *lua.LState) int {
	name := strings.ToLower(L.CheckString(1))
	if e == nil || e.world == nil || e.world.TypeRegistry == nil {
		return 0
	}
	mType := e.world.TypeRegistry.Monsters[name]
	if mType == nil {
		return 0
	}
	pushMonsterType(L, mType)
	return 1
}
func (e *Engine) gameGetSpectators(L *lua.LState) int {
	table := L.NewTable()
	if L.GetTop() == 0 {
		L.Push(table)
		return 1
	}

	centerPos := checkPosition(L, 1)
	multifloor := L.OptBool(2, false)
	onlyPlayers := L.OptBool(3, false)
	minRangeX := uint16(L.OptInt(4, 7))
	maxRangeX := uint16(L.OptInt(5, 7))
	minRangeY := uint16(L.OptInt(6, 5))
	maxRangeY := uint16(L.OptInt(7, 5))

	if e.world == nil || e.world.Map == nil {
		L.Push(table)
		return 1
	}

	var startZ, endZ uint8
	if multifloor {
		startZ = 0
		endZ = 15
	} else {
		startZ = centerPos.Z
		endZ = centerPos.Z
	}

	var minX, maxX, minY, maxY uint16
	if centerPos.X >= minRangeX {
		minX = centerPos.X - minRangeX
	} else {
		minX = 0
	}
	maxX = centerPos.X + maxRangeX

	if centerPos.Y >= minRangeY {
		minY = centerPos.Y - minRangeY
	} else {
		minY = 0
	}
	maxY = centerPos.Y + maxRangeY

	totalWidth := uint64(maxX - minX + 1)
	totalHeight := uint64(maxY - minY + 1)
	totalFloors := uint64(endZ - startZ + 1)
	totalTiles := totalWidth * totalHeight * totalFloors

	if totalTiles > 2500 {
		var candidates []game.Creature
		players := e.world.Players()
		if onlyPlayers {
			candidates = make([]game.Creature, 0, len(players))
			for _, p := range players {
				candidates = append(candidates, p)
			}
		} else {
			creatures := e.world.Creatures()
			candidates = make([]game.Creature, 0, len(players)+len(creatures))
			for _, p := range players {
				candidates = append(candidates, p)
			}
			for _, c := range creatures {
				candidates = append(candidates, c)
			}
		}

		for _, cr := range candidates {
			if cr == nil {
				continue
			}
			pos := cr.GetPosition()
			if pos.Z >= startZ && pos.Z <= endZ &&
				pos.X >= minX && pos.X <= maxX &&
				pos.Y >= minY && pos.Y <= maxY {
				e.pushCreature(L, cr)
				table.Append(L.Get(-1))
				L.Pop(1)
			}
		}

		L.Push(table)
		return 1
	}

	for z := startZ; z <= endZ; z++ {
		for x := minX; x <= maxX; x++ {
			for y := minY; y <= maxY; y++ {
				tile := e.world.Map.GetTile(game.Position{X: x, Y: y, Z: z})
				if tile == nil {
					continue
				}
				for _, cr := range tile.Creatures {
					if cr == nil {
						continue
					}
					if onlyPlayers {
						if _, ok := cr.(*game.Player); !ok {
							continue
						}
					}
					e.pushCreature(L, cr)
					table.Append(L.Get(-1))
					L.Pop(1)
				}
			}
		}
	}

	L.Push(table)
	return 1
}
func (e *Engine) gameGetBoostedCreature(L *lua.LState) int {
	if e == nil || e.world == nil {
		L.Push(lua.LString("Dragon"))
		return 1
	}
	L.Push(lua.LString(e.world.GetBoostedCreature()))
	return 1
}
func (e *Engine) gameGetBestiaryList(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
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
func (e *Engine) gameLoadMap(L *lua.LState) int {
	if e == nil || e.world == nil || e.world.Map == nil {
		L.Push(lua.LFalse)
		return 1
	}
	path := L.CheckString(1)
	_, err := otbm.Load(path, e.world.Items, e.world.Map)
	if err != nil {
		e.log.Warn("game.loadMap failed", "path", path, "err", err)
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LTrue)
	return 1
}

func (e *Engine) gameLoadMapChunk(L *lua.LState) int {
	return e.gameLoadMap(L)
}
func (e *Engine) gameGetExperienceForLevel(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }
func (e *Engine) gameGetMonsterCount(L *lua.LState) int {
	count := 0
	if e.world != nil {
		for _, c := range e.world.Creatures() {
			if c.GetCreatureType() == 1 { // CREATURETYPE_MONSTER
				count++
			}
		}
	}
	L.Push(lua.LNumber(count))
	return 1
}
func (e *Engine) gameGetPlayerCount(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }
func (e *Engine) gameGetNpcCount(L *lua.LState) int {
	count := 0
	if e.world != nil {
		for _, c := range e.world.Creatures() {
			if c.GetCreatureType() == 2 { // CREATURETYPE_NPC
				count++
			}
		}
	}
	L.Push(lua.LNumber(count))
	return 1
}
func (e *Engine) gameGetMonsterTypes(L *lua.LState) int {
	tbl := L.NewTable()
	if e != nil && e.world != nil && e.world.TypeRegistry != nil {
		for name, mType := range e.world.TypeRegistry.Monsters {
			ud := L.NewUserData()
			ud.Value = mType
			L.SetMetatable(ud, L.GetTypeMetatable(luaMonsterTypeName))
			L.SetField(tbl, name, ud)
		}
	}
	L.Push(tbl)
	return 1
}
func (e *Engine) gameGetTowns(L *lua.LState) int {
	tbl := L.NewTable()
	if e.world != nil {
		for id := range e.world.TownNames {
			pushTown(L, id)
			tbl.Append(L.Get(-1))
			L.Pop(1)
		}
	}
	L.Push(tbl)
	return 1
}
func (e *Engine) gameGetHouses(L *lua.LState) int {
	tbl := L.NewTable()
	if e.world != nil {
		houseMt := L.GetTypeMetatable("House")
		for _, h := range e.world.Houses {
			ud := L.NewUserData()
			ud.Value = h
			L.SetMetatable(ud, houseMt)
			tbl.Append(ud)
		}
	}
	L.Push(tbl)
	return 1
}
func (e *Engine) gameGetGameState(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }
func (e *Engine) gameSetGameState(L *lua.LState) int { return 0 }
func (e *Engine) gameGetWorldType(L *lua.LState) int { L.Push(lua.LNumber(0)); return 1 }
func (e *Engine) gameSetWorldType(L *lua.LState) int { return 0 }
func (e *Engine) gameGetReturnMessage(L *lua.LState) int {
	code := L.OptInt(1, 0)
	switch code {
	case 0:
		L.Push(lua.LString("No error."))
	case 1:
		L.Push(lua.LString("Sorry, not possible."))
	case 2:
		L.Push(lua.LString("There is not enough room."))
	case 3:
		L.Push(lua.LString("You can not enter a protection zone after attacking another player."))
	default:
		L.Push(lua.LString("Unknown error."))
	}
	return 1
}
func (e *Engine) gameCreateItem(L *lua.LState) int {
	id := L.CheckInt(1)
	count := L.OptInt(2, 1)

	// C++ splits count into a stack count and a subtype depending on the type
	// (game_functions.cpp:445-458). Without that split a fluid's subtype was
	// being written straight into Count and every stack ignored stackSize.
	itemCount, subType := 1, 1
	it := e.itemCatalog().Get(uint16(id))
	if it != nil && it.HasSubType() {
		if it.Stackable {
			stackSize := int(it.StackSize)
			if stackSize <= 0 {
				stackSize = 100
			}
			itemCount = (count + stackSize - 1) / stackSize // ceil
		}
		subType = count
	} else {
		itemCount = max(1, count)
	}

	// A single call may have to produce several stacks; C++ then returns a table.
	hasTable := itemCount > 1
	var tbl *lua.LTable
	if hasTable {
		tbl = L.NewTable()
	} else if itemCount == 0 {
		L.Push(lua.LNil)
		return 1
	}

	var pos game.Position
	var havePos bool
	if L.GetTop() >= 3 {
		pos, havePos = parsePosition(L, 3)
	}

	for i := 1; i <= itemCount; i++ {
		stackCount := subType
		if it != nil && it.Stackable {
			stackSize := int(it.StackSize)
			if stackSize <= 0 {
				stackSize = 100
			}
			stackCount = min(stackCount, stackSize)
			subType -= stackCount
		}

		item := &game.Item{ID: uint16(id), Count: uint16(stackCount)}

		// C++ Container constructor: ITEM_GOLD_POUCH -> pagination, maxSize=32
		if item.ID == game.ItemGoldPouch {
			item.Pagination = true
			item.Contents = make([]*game.Item, 0)
			item.MaxSize = 32
			// C++: m_maxItems = g_configManager().getNumber(LOOTPOUCH_MAXLIMIT)
			if v, ok := L.GetGlobal("lootPouchMaxLimit").(lua.LNumber); ok && v > 0 {
				item.MaxItems = uint16(v)
			} else {
				item.MaxItems = 2000
			}
		}

		if havePos {
			// C++ uses map.getTile, not getOrCreateTile: a position with no tile
			// yields nil and places nothing (game_functions.cpp:488-495).
			if e.world == nil || e.world.Map == nil || !e.world.AddItem(pos, item) {
				if !hasTable {
					L.Push(lua.LNil)
				}
				return 1
			}
		}

		if hasTable {
			L.RawSetInt(tbl, i, e.itemValue(L, item))
		} else {
			e.pushItem(L, item)
		}
	}

	if hasTable {
		L.Push(tbl)
	}
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
	if e.world.TypeRegistry != nil {
		mType = e.world.TypeRegistry.Monsters[strings.ToLower(name)]
		if mType == nil {
			mType = e.world.TypeRegistry.Monsters[name]
		}
	}
	if mType == nil && e.world.Monsters != nil {
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
	if e.world.TypeRegistry != nil {
		nType = e.world.TypeRegistry.Npcs[strings.ToLower(name)]
		if nType == nil {
			nType = e.world.TypeRegistry.Npcs[name]
		}
	}
	if nType == nil && e.world.Monsters != nil {
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
// gameCreateItemClassification is now defined in item_classification.go
func (e *Engine) gameGetBestiaryCharm(L *lua.LState) int { return 0 }
func (e *Engine) gameStartRaid(L *lua.LState) int {
	name := L.CheckString(1)
	if e.world == nil || e.world.Raids == nil {
		L.Push(lua.LNumber(1)) // RETURNVALUE_NOTPOSSIBLE
		return 1
	}
	if err := e.world.Raids.StartRaidWithWorld(name, e.world); err != nil {
		L.Push(lua.LNumber(1))
		return 1
	}
	L.Push(lua.LNumber(0)) // RETURNVALUE_NOERROR
	return 1
}
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
func (e *Engine) gameGetNormalizedGuildName(L *lua.LState) int { L.Push(lua.LString("")); return 1 }
func (e *Engine) gameAddInfluencedMonster(L *lua.LState) int { return 0 }
func (e *Engine) gameRemoveInfluencedMonster(L *lua.LState) int { return 0 }
func (e *Engine) gameGetInfluencedMonsters(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
func (e *Engine) gameMakeFiendishMonster(L *lua.LState) int { return 0 }
func (e *Engine) gameRemoveFiendishMonster(L *lua.LState) int { return 0 }
func (e *Engine) gameGetFiendishMonsters(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
func (e *Engine) gameGetBoostedBoss(L *lua.LState) int {
	if e == nil || e.world == nil {
		L.Push(lua.LString("None"))
		return 1
	}
	L.Push(lua.LString(e.world.GetBoostedBoss()))
	return 1
}
func (e *Engine) gameGetLadderIds(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
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
func (e *Engine) gameGetTalkActions(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
func (e *Engine) gameGetEventCallbacks(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
func (e *Engine) gameRegisterAchievement(L *lua.LState) int {
	// Lua scripts call with (name, description, secret, points) OR
	// (id, name, description, secret, grade, points). Auto-assigns the ID.
	var name, description string
	secret := false
	points := uint8(1)

	if L.GetTop() >= 6 {
		// Full format: (id, name, description, secret, grade, points) — skip id at 1
		name = L.CheckString(2)
		description = L.CheckString(3)
		secret = L.OptBool(4, false)
		points = uint8(L.OptInt(6, 1))
	} else if L.GetTop() >= 4 {
		// Full format: (id, name, description, secret, grade, points) — skip id at 1
		name = L.CheckString(2)
		description = L.CheckString(3)
		secret = L.OptBool(4, false)
		points = uint8(L.OptInt(6, 1))
	} else {
		return 0
	}
	if e.world == nil {
		return 0
	}
	reg := e.world.Achievements
	if reg == nil {
		return 0
	}
	id := reg.Register(name, description, secret, points)
	L.Push(lua.LNumber(id))
	return 1
}

func (e *Engine) gameGetAchievementInfoById(L *lua.LState) int {
	if e.world == nil {
		L.Push(lua.LNil)
		return 1
	}
	id := uint16(L.CheckInt(1))
	a := e.world.Achievements.GetByID(id)
	if a == nil {
		L.Push(lua.LNil)
		return 1
	}
	tbl := L.NewTable()
	tbl.RawSetString("id", lua.LNumber(a.ID))
	tbl.RawSetString("name", lua.LString(a.Name))
	tbl.RawSetString("description", lua.LString(a.Description))
	tbl.RawSetString("secret", lua.LBool(a.Secret))
	tbl.RawSetString("points", lua.LNumber(a.Points))
	L.Push(tbl)
	return 1
}

func (e *Engine) gameGetAchievementInfoByName(L *lua.LState) int {
	if e.world == nil {
		L.Push(lua.LNil)
		return 1
	}
	name := L.CheckString(1)
	a := e.world.Achievements.GetByName(name)
	if a == nil {
		L.Push(lua.LNil)
		return 1
	}
	tbl := L.NewTable()
	tbl.RawSetString("id", lua.LNumber(a.ID))
	tbl.RawSetString("name", lua.LString(a.Name))
	tbl.RawSetString("description", lua.LString(a.Description))
	tbl.RawSetString("secret", lua.LBool(a.Secret))
	tbl.RawSetString("points", lua.LNumber(a.Points))
	L.Push(tbl)
	return 1
}

func (e *Engine) gameGetSecretAchievements(L *lua.LState) int {
	tbl := L.NewTable()
	if e.world == nil || e.world.Achievements == nil {
		L.Push(tbl)
		return 1
	}
	for _, a := range e.world.Achievements.SecretAchievements() {
		item := L.NewTable()
		item.RawSetString("id", lua.LNumber(a.ID))
		item.RawSetString("name", lua.LString(a.Name))
		item.RawSetString("description", lua.LString(a.Description))
		item.RawSetString("points", lua.LNumber(a.Points))
		tbl.Append(item)
	}
	L.Push(tbl)
	return 1
}

func (e *Engine) gameGetPublicAchievements(L *lua.LState) int {
	tbl := L.NewTable()
	if e.world == nil || e.world.Achievements == nil {
		L.Push(tbl)
		return 1
	}
	for _, a := range e.world.Achievements.PublicAchievements() {
		item := L.NewTable()
		item.RawSetString("id", lua.LNumber(a.ID))
		item.RawSetString("name", lua.LString(a.Name))
		item.RawSetString("description", lua.LString(a.Description))
		item.RawSetString("points", lua.LNumber(a.Points))
		tbl.Append(item)
	}
	L.Push(tbl)
	return 1
}

func (e *Engine) gameGetAchievements(L *lua.LState) int {
	tbl := L.NewTable()
	if e.world == nil || e.world.Achievements == nil {
		L.Push(tbl)
		return 1
	}
	for _, a := range e.world.Achievements.AllAchievements() {
		item := L.NewTable()
		item.RawSetString("id", lua.LNumber(a.ID))
		item.RawSetString("name", lua.LString(a.Name))
		item.RawSetString("description", lua.LString(a.Description))
		item.RawSetString("secret", lua.LBool(a.Secret))
		item.RawSetString("points", lua.LNumber(a.Points))
		tbl.Append(item)
	}
	L.Push(tbl)
	return 1
}
func (e *Engine) gameGetSoulCoreItems(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
func (e *Engine) gameGetMonstersByRace(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
func (e *Engine) gameGetMonstersByBestiaryStars(L *lua.LState) int { L.Push(L.NewTable()); return 1 }
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
