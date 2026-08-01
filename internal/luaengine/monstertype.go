package luaengine

import (
	"strings"

	"github.com/omurilo/canary-go/internal/bosstiary"
	"github.com/omurilo/canary-go/internal/creatures"
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
		"getName": func(L *lua.LState) int {
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
			if bt := table.RawGetString("Bestiary"); bt.Type() == lua.LTTable {
				b := bt.(*lua.LTable)
				if val := b.RawGetString("Stars"); val.Type() == lua.LTNumber {
					m.BestiaryStars = uint8(lua.LVAsNumber(val))
				}
				if val := b.RawGetString("class"); val.Type() == lua.LTString {
					m.BestiaryClass = val.String()
				}
				if val := b.RawGetString("race"); val.Type() == lua.LTNumber {
					m.BestiaryRace = uint8(lua.LVAsNumber(val))
				}
				if val := b.RawGetString("FirstUnlock"); val.Type() == lua.LTNumber {
					m.BestiaryFirstUnlock = uint32(lua.LVAsNumber(val))
				}
				if val := b.RawGetString("SecondUnlock"); val.Type() == lua.LTNumber {
					m.BestiarySecondUnlock = uint32(lua.LVAsNumber(val))
				}
				if val := b.RawGetString("toKill"); val.Type() == lua.LTNumber {
					m.BestiaryToKill = uint32(lua.LVAsNumber(val))
				}
				if val := b.RawGetString("CharmsPoints"); val.Type() == lua.LTNumber {
					m.BestiaryCharmsPoints = uint16(lua.LVAsNumber(val))
				}
				if val := b.RawGetString("Occurrence"); val.Type() == lua.LTNumber {
					m.BestiaryOccurrence = uint8(lua.LVAsNumber(val))
				}
			}
			// Bosstiary (Boss Cyclopedia): monster.bosstiary = { bossRaceId, bossRace }.
			if bt := table.RawGetString("bosstiary"); bt.Type() == lua.LTTable {
				bTbl := bt.(*lua.LTable)
				if val := bTbl.RawGetString("bossRaceId"); val.Type() == lua.LTNumber {
					m.BosstiaryRaceID = uint16(lua.LVAsNumber(val))
				}
				if val := bTbl.RawGetString("bossRace"); val.Type() == lua.LTNumber {
					m.BosstiaryRace = bosstiary.Rarity(lua.LVAsNumber(val))
				}
			}
			parseMonsterAttacks(m, table)
			parseMonsterDefenses(m, table)
			parseMonsterLoot(m, table)
			parseMonsterFlags(m, table)
			parseMonsterStrategies(m, table)
			parseMonsterChangeTarget(m, table)
			parseMonsterVoices(m, table)
			parseMonsterSummons(m, table)
			parseMonsterMisc(m, table)
			parseMonsterElements(m, table)
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
		// getLoot returns the loot table in the shape MonsterType:generateLootRoll
		// expects (data/libs/functions/monstertype.lua:4): an array of entries keyed
		// by itemId, chance, minCount, maxCount, unique and childLoot.
		//
		// This is what the monsterOnDropLoot event needs; without it the base loot
		// script could not roll anything.
		"getLoot": func(L *lua.LState) int {
			m := checkMonsterType(L)
			if m == nil {
				L.Push(L.NewTable())
				return 1
			}
			L.Push(lootBlocksToLua(L, m.Loot))
			return 1
		},
		// These four are get/set in C++: called with no argument they report, with a
		// boolean they assign (monster_type_functions.cpp). The monster .lua files use
		// the setter form while loading a type.
		"isSummonable":   monsterTypeBoolFlag(func(m *creatures.MonsterType) *bool { return &m.Flags.Summonable }),
		"isIllusionable": monsterTypeBoolFlag(func(m *creatures.MonsterType) *bool { return &m.Flags.Illusionable }),
		"isConvinceable": monsterTypeBoolFlag(func(m *creatures.MonsterType) *bool { return &m.Flags.Convinceable }),
		// BestiaryStars / Bestiaryrace keep the upstream capitalisation.
		"BestiaryStars": func(L *lua.LState) int {
			m := checkMonsterType(L)
			if m == nil {
				L.Push(lua.LNil)
				return 1
			}
			if L.GetTop() >= 2 {
				m.BestiaryStars = uint8(L.CheckInt(2))
				L.Push(lua.LTrue)
				return 1
			}
			L.Push(lua.LNumber(m.BestiaryStars))
			return 1
		},
		"Bestiaryrace": func(L *lua.LState) int {
			m := checkMonsterType(L)
			if m == nil {
				L.Push(lua.LNil)
				return 1
			}
			if L.GetTop() >= 2 {
				m.BestiaryRace = uint8(L.CheckInt(2))
				L.Push(lua.LTrue)
				return 1
			}
			L.Push(lua.LNumber(m.BestiaryRace))
			return 1
		},
		"isRewardBoss": func(L *lua.LState) int {
			m := checkMonsterType(L)
			L.Push(lua.LBool(m != nil && m.Flags.RewardBoss))
			return 1
		},
		"addAttack": func(L *lua.LState) int {
			m := checkMonsterType(L)
			ud := L.CheckUserData(2)
			if s, ok := ud.Value.(*luaMonsterSpell); ok {
				m.Attacks = append(m.Attacks, s.Attack)
			}
			return 0
		},
		"addDefense": func(L *lua.LState) int {
			m := checkMonsterType(L)
			ud := L.CheckUserData(2)
			if s, ok := ud.Value.(*luaMonsterSpell); ok {
				m.Defenses = append(m.Defenses, s.Attack)
			}
			return 0
		},
		"targetDistance": func(L *lua.LState) int {
			m := checkMonsterType(L)
			if L.GetTop() >= 2 {
				m.TargetDistance = int32(L.CheckInt(2))
				return 0
			}
			val := m.TargetDistance
			if val == 0 {
				val = 1
			}
			L.Push(lua.LNumber(val))
			return 1
		},
		"raceId": func(L *lua.LState) int {
			m := checkMonsterType(L)
			L.Push(lua.LNumber(m.RaceID))
			return 1
		},
		"getRaceId": func(L *lua.LState) int {
			m := checkMonsterType(L)
			L.Push(lua.LNumber(m.RaceID))
			return 1
		},
		"isMonsterType": func(L *lua.LState) int {
			L.Push(lua.LTrue)
			return 1
		},

		// -- Methods added for 1:1 C++ API parity (returning safe defaults) --

		"isPreyExclusive": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LFalse)
			return 1
		},
		"isForgeCreature": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LFalse)
			return 1
		},
		"canSpawn": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LFalse)
			return 1
		},
		"critChance": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(0))
			return 1
		},
		"nameDescription": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LString(""))
			return 1
		},
		"runHealth": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(0))
			return 1
		},
		// faction / enemyFactions used to answer FACTION_DEFAULT and an empty table
		// no matter what the monster declared, so Monster::isEnemyFaction could
		// never be true and monsters never fought each other.
		"faction": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			if L.GetTop() >= 2 {
				mt.Faction = uint8(L.CheckInt(2))
				return 0
			}
			L.Push(lua.LNumber(mt.Faction))
			return 1
		},
		"enemyFactions": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			if L.GetTop() >= 2 {
				// monsterType:enemyFactions(FACTION_X) and a table of them are both
				// used by the datapack.
				switch v := L.Get(2).(type) {
				case lua.LNumber:
					mt.EnemyFactions = append(mt.EnemyFactions, uint8(v))
				case *lua.LTable:
					v.ForEach(func(_, item lua.LValue) {
						if n, ok := item.(lua.LNumber); ok {
							mt.EnemyFactions = append(mt.EnemyFactions, uint8(n))
						}
					})
				}
				return 0
			}
			out := L.NewTable()
			for _, f := range mt.EnemyFactions {
				out.Append(lua.LNumber(f))
			}
			L.Push(out)
			return 1
		},
		"targetPreferPlayer": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LFalse)
			return 1
		},
		"targetPreferMaster": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LFalse)
			return 1
		},
		"combatImmunities": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(L.NewTable())
			return 1
		},
		"conditionImmunities": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(L.NewTable())
			return 1
		},
		"familiar": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LFalse)
			return 1
		},
		"getCorpseId": func(L *lua.LState) int {
			mt := checkMonsterType(L)
			if mt == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(0))
			return 1
		},
	}

	// Resilient __index fallback: if a method on MonsterType doesn't exist,
	// return a safe no-op function that returns 'self' to support method chaining.
	mtIndex := e.L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(2)
		if fn, ok := monsterTypeMethods[key]; ok {
			L.Push(L.NewFunction(fn))
			return 1
		}
		// Then the global MonsterType class table, which is where the datapack
		// extends the class: data/libs/functions/monstertype.lua declares
		// `function MonsterType:generateLootRoll(...)`, and the loot chain is built
		// on it. Without this lookup that method resolved to the no-op below and
		// monsterOnDropLoot silently produced nothing.
		if tbl, ok := L.GetGlobal(luaMonsterTypeName).(*lua.LTable); ok {
			if v := L.GetField(tbl, key); v != lua.LNil {
				L.Push(v)
				return 1
			}
		}
		// TODO: this catch-all hides every genuinely missing MonsterType method (80
		// of the C++ 100 are still absent). It stays for now because the datapack
		// calls several of them during load, but it is why those gaps are invisible.
		noOpFn := L.NewFunction(func(L *lua.LState) int {
			L.Push(L.Get(1)) // Return self to allow method chaining (e.g. mtype:foo():bar())
			return 1
		})
		L.Push(noOpFn)
		return 1
	})
	e.L.SetField(mt, "__index", mtIndex)

	// Populate methods onto the global class table so they are discoverable via pairs()
	var tbl *lua.LTable
	classTable := e.L.GetGlobal(luaMonsterTypeName)
	if classTable.Type() == lua.LTTable {
		tbl = classTable.(*lua.LTable)
	} else {
		tbl = e.L.NewTable()
		e.L.SetGlobal(luaMonsterTypeName, tbl)
	}
	for k, v := range monsterTypeMethods {
		e.L.SetField(tbl, k, e.L.NewFunction(v))
	}

	// Set constructor __call metamethod on MonsterType global table
	ctorMt := e.L.NewTable()
	e.L.SetField(ctorMt, "__call", e.L.NewFunction(func(L *lua.LState) int {
		if L.GetTop() < 2 {
			return 0
		}
		var mType *creatures.MonsterType
		arg := L.Get(2)
		if arg.Type() == lua.LTNumber {
			raceId := uint16(lua.LVAsNumber(arg))
			if e != nil && e.world != nil && e.world.TypeRegistry != nil {
				for _, m := range e.world.TypeRegistry.Monsters {
					if m.RaceID == raceId {
						mType = m
						break
					}
				}
			}
		} else if arg.Type() == lua.LTString {
			name := strings.ToLower(arg.String())
			if e != nil && e.world != nil && e.world.TypeRegistry != nil {
				mType = e.world.TypeRegistry.Monsters[name]
			}
			if mType == nil {
				mType = &creatures.MonsterType{Name: arg.String()}
			}
		}
		if mType == nil {
			return 0
		}
		ud := L.NewUserData()
		ud.Value = mType
		L.SetMetatable(ud, mt)
		L.Push(ud)
		return 1
	}))
	e.L.SetMetatable(tbl, ctorMt)

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
	m.Attacks = append(m.Attacks, parseSpellBlocks(table, "attacks")...)
}

// parseMonsterDefenses reads monster.defenses, which doubles as a config block:
// `defense`, `armor` and `mitigation` are scalars sitting on the same table as
// the array of defensive spell blocks. Mirrors MonsterType loadDefenses
// (src/creatures/monsters/monsters.cpp).
//
// Every monster in the datapack has this block and none of it was read, so no
// monster ever healed itself and Monster::getDefense always saw zero.
func parseMonsterDefenses(m *creatures.MonsterType, table *lua.LTable) {
	def, ok := table.RawGetString("defenses").(*lua.LTable)
	if !ok {
		return
	}
	if val := def.RawGetString("defense"); val.Type() == lua.LTNumber {
		m.Defense = int(lua.LVAsNumber(val))
	}
	if val := def.RawGetString("armor"); val.Type() == lua.LTNumber {
		m.Armor = int(lua.LVAsNumber(val))
	}
	if val := def.RawGetString("mitigation"); val.Type() == lua.LTNumber {
		m.Mitigation = float64(lua.LVAsNumber(val))
	}
	m.Defenses = append(m.Defenses, parseSpellBlocks(table, "defenses")...)
}

// parseSpellBlocks parses the array part of monster.attacks or monster.defenses.
// Both carry the same spellBlock_t shape, so the two used to be parsed by one
// body that only ever ran on "attacks".
func parseSpellBlocks(table *lua.LTable, key string) []creatures.MonsterAttack {
	blocks, ok := table.RawGetString(key).(*lua.LTable)
	if !ok {
		return nil
	}
	var out []creatures.MonsterAttack
	blocks.ForEach(func(_, v lua.LValue) {
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
		if val := at.RawGetString("radius"); val.Type() == lua.LTNumber {
			atk.Radius = int(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("length"); val.Type() == lua.LTNumber {
			atk.Length = int(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("spread"); val.Type() == lua.LTNumber {
			atk.Spread = int(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("shootEffect"); val.Type() == lua.LTNumber {
			atk.ShootEffect = uint16(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("effect"); val.Type() == lua.LTNumber {
			atk.Effect = uint16(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("type"); val.Type() == lua.LTNumber {
			// Convert to internal CombatType string name to match logic
			cType := luaToCombatType(int(lua.LVAsNumber(val)))
			switch cType {
			case 1:
				atk.CombatType = "physical"
			case 8:
				atk.CombatType = "fire"
			case 4:
				atk.CombatType = "earth"
			case 2:
				atk.CombatType = "energy"
			case 128:
				atk.CombatType = "ice"
			case 64:
				atk.CombatType = "death"
			case 256:
				atk.CombatType = "holy"
			case 1024:
				atk.CombatType = "lifedrain"
			case 512:
				atk.CombatType = "manadrain"
			case 32:
				atk.CombatType = "healing"
			}
		}
		if val := at.RawGetString("condition"); val.Type() == lua.LTString {
			atk.ConditionType = val.String()
		}
		if val := at.RawGetString("speedChange"); val.Type() == lua.LTNumber {
			atk.SpeedChange = int(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("duration"); val.Type() == lua.LTNumber {
			atk.Duration = int(lua.LVAsNumber(val))
		}
		if val := at.RawGetString("target"); val.Type() == lua.LTBool {
			atk.NeedTarget = lua.LVAsBool(val)
		}
		out = append(out, atk)
	})
	return out
}

// parseMonsterStrategies reads monster.strategiesTarget = {nearest, health,
// damage, random}, the weights Monster::searchTargetImmediate rolls against
// (monster.cpp:906-924).
//
// These were previously looked for as `strategiesTargetNearest` inside
// monster.flags — a key no monster file has — so all 1633 monsters that declare
// a strategy stayed at zero, and zero weights fall all the way through to
// TARGETSEARCH_RANDOM. Every monster in the game picked its target at random.
func parseMonsterStrategies(m *creatures.MonsterType, table *lua.LTable) {
	st, ok := table.RawGetString("strategiesTarget").(*lua.LTable)
	if !ok {
		return
	}
	intKey := func(key string, dst *int) {
		if val := st.RawGetString(key); val.Type() == lua.LTNumber {
			*dst = int(lua.LVAsNumber(val))
		}
	}
	intKey("nearest", &m.Flags.StrategiesTargetNearest)
	intKey("health", &m.Flags.StrategiesTargetHealth)
	intKey("damage", &m.Flags.StrategiesTargetDamage)
	intKey("random", &m.Flags.StrategiesTargetRandom)
}

// parseMonsterChangeTarget reads monster.changeTarget = {interval, chance},
// which drives Monster::onThinkTarget (monster.cpp:2140). `interval` is
// changeTargetSpeed and `chance` is changeTargetChance.
func parseMonsterChangeTarget(m *creatures.MonsterType, table *lua.LTable) {
	ct, ok := table.RawGetString("changeTarget").(*lua.LTable)
	if !ok {
		return
	}
	if val := ct.RawGetString("interval"); val.Type() == lua.LTNumber {
		m.ChangeTargetInterval = int(lua.LVAsNumber(val))
	}
	if val := ct.RawGetString("chance"); val.Type() == lua.LTNumber {
		m.ChangeTargetChance = int(lua.LVAsNumber(val))
	}
}

// parseMonsterVoices reads monster.voices: two scalars plus an array of
// {text, yell} entries, consumed by Monster::onThinkYell (monster.cpp:2273).
func parseMonsterVoices(m *creatures.MonsterType, table *lua.LTable) {
	voices, ok := table.RawGetString("voices").(*lua.LTable)
	if !ok {
		return
	}
	if val := voices.RawGetString("interval"); val.Type() == lua.LTNumber {
		m.YellInterval = int(lua.LVAsNumber(val))
	}
	if val := voices.RawGetString("chance"); val.Type() == lua.LTNumber {
		m.YellChance = int(lua.LVAsNumber(val))
	}
	voices.ForEach(func(_, v lua.LValue) {
		vt, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		text := vt.RawGetString("text")
		if text.Type() != lua.LTString {
			return
		}
		m.Voices = append(m.Voices, creatures.MonsterVoice{
			Text: text.String(),
			Yell: lua.LVAsBool(vt.RawGetString("yell")),
		})
	})
}

// parseMonsterSummons reads monster.summon (or the rarer plural spelling) and
// monster.maxSummons. Mirrors the summon arm of Monster::onThinkDefense
// (monster.cpp:2223-2270); `interval` there is summonBlock_t::speed.
func parseMonsterSummons(m *creatures.MonsterType, table *lua.LTable) {
	if val := table.RawGetString("maxSummons"); val.Type() == lua.LTNumber {
		m.MaxSummons = int(lua.LVAsNumber(val))
	}
	summons, ok := table.RawGetString("summon").(*lua.LTable)
	if !ok {
		if summons, ok = table.RawGetString("summons").(*lua.LTable); !ok {
			return
		}
	}
	summons.ForEach(func(_, v lua.LValue) {
		st, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		name := st.RawGetString("name")
		if name.Type() != lua.LTString {
			return
		}
		s := creatures.MonsterSummon{Name: name.String(), Chance: 100, Interval: 1000, Count: 1}
		if val := st.RawGetString("chance"); val.Type() == lua.LTNumber {
			s.Chance = int(lua.LVAsNumber(val))
		}
		if val := st.RawGetString("interval"); val.Type() == lua.LTNumber {
			s.Interval = int(lua.LVAsNumber(val))
		}
		if val := st.RawGetString("count"); val.Type() == lua.LTNumber {
			s.Count = int(lua.LVAsNumber(val))
		}
		if val := st.RawGetString("max"); val.Type() == lua.LTNumber {
			s.Count = int(lua.LVAsNumber(val))
		}
		s.Force = lua.LVAsBool(st.RawGetString("force"))
		m.Summons = append(m.Summons, s)
	})
}

// parseMonsterMisc picks up the remaining top-level scalars register() skipped:
// blood race, mana cost and the light the monster emits.
func parseMonsterMisc(m *creatures.MonsterType, table *lua.LTable) {
	if val := table.RawGetString("race"); val.Type() == lua.LTString {
		m.BloodRace = val.String()
	}
	if val := table.RawGetString("manaCost"); val.Type() == lua.LTNumber {
		m.ManaCost = uint32(lua.LVAsNumber(val))
	}
	if lt, ok := table.RawGetString("light").(*lua.LTable); ok {
		if val := lt.RawGetString("level"); val.Type() == lua.LTNumber {
			m.LightLevel = uint8(lua.LVAsNumber(val))
		}
		if val := lt.RawGetString("color"); val.Type() == lua.LTNumber {
			m.LightColor = uint8(lua.LVAsNumber(val))
		}
	}
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
	if val := lt.RawGetString("unique"); val.Type() == lua.LTBool {
		lb.Unique = lua.LVAsBool(val)
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
	// Weighted target-search strategies (monster.cpp:906-924). No datapack monster
	// sets these today, so every one of them resolves to nearest — the plumbing is
	// here so a type that does set them is obeyed rather than silently ignored.
	intFlag("strategiesTargetNearest", &m.Flags.StrategiesTargetNearest)
	intFlag("strategiesTargetHealth", &m.Flags.StrategiesTargetHealth)
	intFlag("strategiesTargetDamage", &m.Flags.StrategiesTargetDamage)
	intFlag("strategiesTargetRandom", &m.Flags.StrategiesTargetRandom)
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

func pushMonsterType(L *lua.LState, m *creatures.MonsterType) {
	ud := L.NewUserData()
	ud.Value = m
	L.SetMetatable(ud, L.GetTypeMetatable(luaMonsterTypeName))
	L.Push(ud)
}

// parseMonsterElements reads monster.elements list.
// E.g. { type = COMBAT_EARTHDAMAGE, percent = 20 }
func parseMonsterElements(m *creatures.MonsterType, table *lua.LTable) {
	elements, ok := table.RawGetString("elements").(*lua.LTable)
	if !ok {
		return
	}
	if m.Elements == nil {
		m.Elements = make(map[uint32]int16)
	}
	elements.ForEach(func(_, v lua.LValue) {
		et, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		var hasType bool
		var cType uint32
		var percent int16
		if val := et.RawGetString("type"); val.Type() == lua.LTNumber {
			cType = uint32(lua.LVAsNumber(val))
			hasType = true
		}
		if val := et.RawGetString("percent"); val.Type() == lua.LTNumber {
			percent = int16(lua.LVAsNumber(val))
		}
		if hasType {
			actualType := uint32(luaToCombatType(int(cType)))
			m.Elements[actualType] = percent
		}
	})
}

// lootBlocksToLua converts Go loot blocks into the array of tables the datapack's
// MonsterType:generateLootRoll iterates. Key names are the contract with
// data/libs/functions/monstertype.lua — renaming one silently drops that field.
func lootBlocksToLua(L *lua.LState, blocks []creatures.LootBlock) *lua.LTable {
	out := L.NewTable()
	for i, lb := range blocks {
		entry := L.NewTable()
		L.SetField(entry, "itemId", lua.LNumber(lb.ID))
		L.SetField(entry, "chance", lua.LNumber(lb.Chance))
		L.SetField(entry, "minCount", lua.LNumber(max32(lb.CountMin, 1)))
		L.SetField(entry, "maxCount", lua.LNumber(max32(lb.CountMax, 1)))
		L.SetField(entry, "unique", lua.LBool(lb.Unique))
		// subType and actionId must ALWAYS be present and default to -1, not be
		// omitted: Container:addLoot guards them with `~= -1`
		// (data/libs/functions/container.lua:48,54), so a nil would be passed
		// straight into transform/setActionId and error out.
		subType := int32(-1)
		if lb.SubType > 0 {
			subType = lb.SubType
		}
		L.SetField(entry, "subType", lua.LNumber(subType))
		// Go's LootBlock carries no action id, so it is always "none".
		L.SetField(entry, "actionId", lua.LNumber(-1))
		if len(lb.ChildLoot) > 0 {
			L.SetField(entry, "childLoot", lootBlocksToLua(L, lb.ChildLoot))
		}
		out.RawSetInt(i+1, entry)
	}
	return out
}

func max32(v, min uint32) uint32 {
	if v < min {
		return min
	}
	return v
}

// monsterTypeBoolFlag builds a get/set accessor over a bool field of MonsterType,
// the shape C++ uses for isSummonable/isIllusionable/isConvinceable: no argument
// reads, a boolean argument assigns and returns true.
func monsterTypeBoolFlag(field func(*creatures.MonsterType) *bool) lua.LGFunction {
	return func(L *lua.LState) int {
		m := checkMonsterType(L)
		if m == nil {
			L.Push(lua.LNil)
			return 1
		}
		p := field(m)
		if L.GetTop() >= 2 {
			*p = L.ToBool(2)
			L.Push(lua.LTrue)
			return 1
		}
		L.Push(lua.LBool(*p))
		return 1
	}
}
