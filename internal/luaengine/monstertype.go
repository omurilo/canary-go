package luaengine

import (
	"strings"

	"github.com/opentibiabr/canary-go/internal/creatures"
	lua "github.com/yuin/gopher-lua"
)

const luaMonsterTypeName = "MonsterType"

func (e *Engine) registerMonsterType() {
	mt := e.L.NewTypeMetatable(luaMonsterTypeName)

	monsterTypeMethods := map[string]lua.LGFunction{
		"name": func(L *lua.LState) int {
			m := checkMonsterType(L)
			L.Push(lua.LString(m.Name))
			return 1
		},
		"register": func(L *lua.LState) int {
			m := checkMonsterType(L)
			table := L.CheckTable(2)
			
			if val := table.RawGetString("health"); val.Type() == lua.LTNumber {
				m.MaxHealth = uint32(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("maxHealth"); val.Type() == lua.LTNumber && m.MaxHealth == 0 {
				m.MaxHealth = uint32(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("speed"); val.Type() == lua.LTNumber {
				m.Speed = uint32(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("experience"); val.Type() == lua.LTNumber {
				m.Experience = uint64(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("corpse"); val.Type() == lua.LTNumber {
				m.Corpse = uint16(lua.LVAsNumber(val))
			}
			if val := table.RawGetString("raceId"); val.Type() == lua.LTNumber {
				m.RaceID = uint16(lua.LVAsNumber(val))
			}
			parseMonsterAttacks(m, table)
			parseMonsterLoot(m, table)
			parseMonsterFlags(m, table)
			if outfitTable := table.RawGetString("outfit"); outfitTable.Type() == lua.LTTable {
				tb := outfitTable.(*lua.LTable)
				if val := tb.RawGetString("lookType"); val.Type() == lua.LTNumber {
					m.Outfit.LookType = uint16(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookHead"); val.Type() == lua.LTNumber {
					m.Outfit.Head = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookBody"); val.Type() == lua.LTNumber {
					m.Outfit.Body = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookLegs"); val.Type() == lua.LTNumber {
					m.Outfit.Legs = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookFeet"); val.Type() == lua.LTNumber {
					m.Outfit.Feet = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookAddons"); val.Type() == lua.LTNumber {
					m.Outfit.Addons = uint8(lua.LVAsNumber(val))
				}
				if val := tb.RawGetString("lookMount"); val.Type() == lua.LTNumber {
					m.Outfit.LookMount = uint16(lua.LVAsNumber(val))
				}
			}

			if e != nil && e.world != nil && e.world.TypeRegistry != nil {
				e.world.TypeRegistry.Monsters[strings.ToLower(m.Name)] = m
			}
			
			L.Push(lua.LTrue)
			return 1
		},
		"addLoot": func(L *lua.LState) int {
			m := checkMonsterType(L)
			ud := L.CheckUserData(2)
			if l, ok := ud.Value.(*luaLoot); ok {
				m.Loot = append(m.Loot, l.Block)
			}
			return 0
		},
	}
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), monsterTypeMethods))

	// Game.createMonsterType
	gameTable := e.L.GetGlobal("Game")
	if gameTable.Type() == lua.LTTable {
		e.L.SetField(gameTable, "createMonsterType", e.L.NewFunction(func(L *lua.LState) int {
			name := L.CheckString(1)
			mType := &creatures.MonsterType{
				Name:      name,
				Speed:     200,
				MaxHealth: 100,
				// Loot drops by default; MonsterType::info.lootDrop is only cleared
				// via the `lootDrop = false` flag (src/creatures/monsters/monsters.hpp).
				Flags: creatures.MonsterFlags{LootDrop: true},
			}
			ud := L.NewUserData()
			ud.Value = mType
			L.SetMetatable(ud, mt)
			L.Push(ud)
			return 1
		}))
	}
}

// parseMonsterAttacks reads monster.attacks. Each entry is
// { name = "melee"|<spell>, interval, chance, minDamage, maxDamage, range=... }.
// Mirrors the attack-block loading in Monsters::deserializeSpell
// (src/creatures/monsters/monsters.cpp:57).
func parseMonsterAttacks(m *creatures.MonsterType, table *lua.LTable) {
	attacks, ok := table.RawGetString("attacks").(*lua.LTable)
	if !ok {
		return
	}
	attacks.ForEach(func(_, v lua.LValue) {
		at, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		atk := creatures.MonsterAttack{Interval: 2000, Chance: 100}
		if val := at.RawGetString("name"); val.Type() == lua.LTString {
			atk.Name = strings.ToLower(val.String())
		}
		if val := at.RawGetString("interval"); val.Type() == lua.LTNumber {
			atk.Interval = int(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("chance"); val.Type() == lua.LTNumber {
			atk.Chance = int(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("minDamage"); val.Type() == lua.LTNumber {
			atk.MinDamage = int(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("maxDamage"); val.Type() == lua.LTNumber {
			atk.MaxDamage = int(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("range"); val.Type() == lua.LTNumber {
			atk.Range = int(lua.LVAsNumber(val))
		}
		m.Attacks = append(m.Attacks, atk)
	})
}

// parseMonsterLoot reads monster.loot into LootBlocks (with nested child loot).
// Mirrors MonsterType::loadLoot (src/creatures/monsters/monsters.cpp:21).
func parseMonsterLoot(m *creatures.MonsterType, table *lua.LTable) {
	loot, ok := table.RawGetString("loot").(*lua.LTable)
	if !ok {
		return
	}
	m.Loot = parseLootList(loot)
}

func parseLootList(list *lua.LTable) []creatures.LootBlock {
	var out []creatures.LootBlock
	list.ForEach(func(_, v lua.LValue) {
		lt, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		out = append(out, parseLootBlock(lt))
	})
	return out
}

func parseLootBlock(lt *lua.LTable) creatures.LootBlock {
	lb := creatures.LootBlock{CountMin: 1, CountMax: 1}
	if val := lt.RawGetString("id"); val.Type() == lua.LTNumber {
		lb.ID = uint16(lua.LVAsNumber(val))
	}
	if val := lt.RawGetString("name"); val.Type() == lua.LTString {
		lb.Name = strings.ToLower(val.String())
	}
	if val := lt.RawGetString("chance"); val.Type() == lua.LTNumber {
		lb.Chance = uint32(lua.LVAsNumber(val))
	}
	// Canary uses maxCount/minCount; older data uses countmax/countmin.
	if val := lt.RawGetString("maxCount"); val.Type() == lua.LTNumber {
		lb.CountMax = uint32(lua.LVAsNumber(val))
	} else if val := lt.RawGetString("countmax"); val.Type() == lua.LTNumber {
		lb.CountMax = uint32(lua.LVAsNumber(val))
	}
	if val := lt.RawGetString("minCount"); val.Type() == lua.LTNumber {
		lb.CountMin = uint32(lua.LVAsNumber(val))
	} else if val := lt.RawGetString("countmin"); val.Type() == lua.LTNumber {
		lb.CountMin = uint32(lua.LVAsNumber(val))
	}
	if val := lt.RawGetString("subType"); val.Type() == lua.LTNumber {
		lb.SubType = int32(lua.LVAsNumber(val))
	}
	if child, ok := lt.RawGetString("child").(*lua.LTable); ok {
		lb.ChildLoot = parseLootList(child)
	}
	return lb
}

// parseMonsterFlags reads monster.flags. Mirrors the flag loading in
// MonsterType::info (src/creatures/monsters/monsters.hpp).
func parseMonsterFlags(m *creatures.MonsterType, table *lua.LTable) {
	flags, ok := table.RawGetString("flags").(*lua.LTable)
	if !ok {
		return
	}
	boolFlag := func(key string, dst *bool) {
		if val := flags.RawGetString(key); val.Type() == lua.LTBool {
			*dst = lua.LVAsBool(val)
		}
	}
	intFlag := func(key string, dst *int) {
		if val := flags.RawGetString(key); val.Type() == lua.LTNumber {
			*dst = int(lua.LVAsNumber(val))
		}
	}
	boolFlag("summonable", &m.Flags.Summonable)
	boolFlag("attackable", &m.Flags.Attackable)
	boolFlag("hostile", &m.Flags.Hostile)
	boolFlag("convinceable", &m.Flags.Convinceable)
	boolFlag("pushable", &m.Flags.Pushable)
	boolFlag("rewardBoss", &m.Flags.RewardBoss)
	boolFlag("illusionable", &m.Flags.Illusionable)
	boolFlag("canPushItems", &m.Flags.CanPushItems)
	boolFlag("canPushCreatures", &m.Flags.CanPushCreatures)
	boolFlag("healthHidden", &m.Flags.HealthHidden)
	boolFlag("canWalkOnEnergy", &m.Flags.CanWalkOnEnergy)
	boolFlag("canWalkOnFire", &m.Flags.CanWalkOnFire)
	boolFlag("canWalkOnPoison", &m.Flags.CanWalkOnPoison)
	boolFlag("lootDrop", &m.Flags.LootDrop)
	intFlag("staticAttackChance", &m.Flags.StaticAttackChance)
	intFlag("targetDistance", &m.Flags.TargetDistance)
	intFlag("runHealth", &m.Flags.RunHealth)
}

func checkMonsterType(L *lua.LState) *creatures.MonsterType {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*creatures.MonsterType); ok {
		return v
	}
	L.ArgError(1, "MonsterType expected")
	return nil
}
