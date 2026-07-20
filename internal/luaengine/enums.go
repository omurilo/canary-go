package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

// RegisterEnums registers the major Tibia global enums in the given Lua state.
func RegisterEnums(L *lua.LState) {
	enums := map[string]lua.LNumber{
		// Slots
		"CONST_SLOT_FIRST":       0,
		"CONST_SLOT_HEAD":        1,
		"CONST_SLOT_NECKLACE":    2,
		"CONST_SLOT_BACKPACK":    3,
		"CONST_SLOT_ARMOR":       4,
		"CONST_SLOT_RIGHT":       5,
		"CONST_SLOT_LEFT":        6,
		"CONST_SLOT_LEGS":        7,
		"CONST_SLOT_FEET":        8,
		"CONST_SLOT_RING":        9,
		"CONST_SLOT_AMMO":        10,
		"CONST_SLOT_STORE_INBOX": 11,
		"CONST_SLOT_LAST":        12,

		// Messages
		"MESSAGE_GAMEMASTER_CONSOLE":  13,
		"MESSAGE_LOGIN":               17,
		"MESSAGE_ADMINISTRATOR":       18,
		"MESSAGE_EVENT_ADVANCE":       19,
		"MESSAGE_GAME_HIGHLIGHT":      20,
		"MESSAGE_FAILURE":             21,
		"MESSAGE_LOOK":                22,
		"MESSAGE_DAMAGE_DEALT":        23,
		"MESSAGE_DAMAGE_RECEIVED":     24,
		"MESSAGE_HEALED":              25,
		"MESSAGE_EXPERIENCE":          26,
		"MESSAGE_DAMAGE_OTHERS":       27,
		"MESSAGE_HEALED_OTHERS":       28,
		"MESSAGE_EXPERIENCE_OTHERS":   29,
		"MESSAGE_STATUS":              30,
		"MESSAGE_LOOT":                31,
		"MESSAGE_TRADE":               32,
		"MESSAGE_GUILD":               33,
		"MESSAGE_PARTY_MANAGEMENT":    34,
		"MESSAGE_PARTY":               35,
		"MESSAGE_LAST_OLDPROTOCOL":    37,
		"MESSAGE_REPORT":              38,
		"MESSAGE_HOTKEY_PRESSED":      39,
		"MESSAGE_TUTORIAL_HINT":       40,
		"MESSAGE_THANK_YOU":           41,
		"MESSAGE_MARKET":              42,
		"MESSAGE_MANA":                43,
		"MESSAGE_BEYOND_LAST":         44,
		"MESSAGE_ATTENTION":           48,
		"MESSAGE_BOOSTED_CREATURE":    49,
		"MESSAGE_OFFLINE_TRAINING":    50,
		"MESSAGE_TRANSACTION":         51,
		"MESSAGE_POTION":              52,
		"MESSAGE_STATUS_CONSOLE_BLUE": 30, // alias

		// Combat Types (CombatType_t, src/creatures/creatures_definitions.hpp).
		// These are sequential C++ enum values, not bitflags; the setParameter
		// binding maps them to the internal combat.CombatType flags.
		"COMBAT_PHYSICALDAMAGE":  0,
		"COMBAT_FIREDAMAGE":      1,
		"COMBAT_EARTHDAMAGE":     2,
		"COMBAT_ENERGYDAMAGE":    3,
		"COMBAT_UNDEFINEDDAMAGE": 4,
		"COMBAT_LIFEDRAIN":       5,
		"COMBAT_MANADRAIN":       6,
		"COMBAT_HEALING":         7,
		"COMBAT_DROWNDAMAGE":     8,
		"COMBAT_ICEDAMAGE":       9,
		"COMBAT_HOLYDAMAGE":      10,
		"COMBAT_DEATHDAMAGE":     11,
		"COMBAT_AGONYDAMAGE":     12,
		"COMBAT_NEUTRALDAMAGE":   13,
		"COMBAT_NONE":            255,

		// Item Types
		"ITEM_TYPE_DEPOT":       0,
		"ITEM_TYPE_REWARDCHEST": 1,
		"ITEM_TYPE_MAILBOX":     2,
		"ITEM_TYPE_TRASHHOLDER": 3,
		"ITEM_TYPE_DOOR":        4,
		"ITEM_TYPE_MAGICFIELD":  5,
		"ITEM_TYPE_TELEPORT":    6,
		"ITEM_TYPE_BED":         7,
		"ITEM_TYPE_KEY":         8,
		"ITEM_TYPE_SUPPLY":      9,

		// Skills
		"SKILL_NONE":                0,
		"SKILL_FIST":                1,
		"SKILL_CLUB":                2,
		"SKILL_SWORD":               3,
		"SKILL_AXE":                 4,
		"SKILL_DISTANCE":            5,
		"SKILL_SHIELD":              6,
		"SKILL_FISHING":             7,
		"SKILL_CRITICAL_HIT_CHANCE": 8,
		"SKILL_CRITICAL_HIT_DAMAGE": 9,
		"SKILL_LIFE_LEECH_CHANCE":   10,
		"SKILL_LIFE_LEECH_AMOUNT":   11,
		"SKILL_MANA_LEECH_CHANCE":   12,
		"SKILL_MANA_LEECH_AMOUNT":   13,
		"SKILL_MAGLEVEL":            14,
		"SKILL_LEVEL":               15,

		// World Types
		"WORLD_TYPE_NO_PVP":       0,
		"WORLD_TYPE_PVP":          1,
		"WORLD_TYPE_PVP_ENFORCED": 2,

		// Zones
		"ZONE_PROTECTION": 0,
		"ZONE_NOPVP":      1,
		"ZONE_PVP":        2,
		"ZONE_NOLOGOUT":   3,
		"ZONE_NORMAL":     4,

		// Bestiary Races
		"BESTY_RACE_AMPHIBIC": 1,
		"BESTY_RACE_DEMON":    2,
		"BESTY_RACE_DRAGON":   3,
		"BESTY_RACE_GIANT":    4,
		"BESTY_RACE_HUMAN":    5,
		"BESTY_RACE_HUMANOID": 6,
		"BESTY_RACE_MAGICAL":  7,
		"BESTY_RACE_MAMMAL":   8,
		"BESTY_RACE_REPTILE":  9,
		"BESTY_RACE_SLIME":    10,
		"BESTY_RACE_UNDEAD":   11,
		"BESTY_RACE_VERMIN":   12,

		// Creature Types
		"CREATURETYPE_PLAYER":   0,
		"CREATURETYPE_MONSTER":  1,
		"CREATURETYPE_NPC":      2,
		"CREATURETYPE_SUMMON_OWN":    3,
		"CREATURETYPE_SUMMON_OTHERS": 4,

		// Directions
		"DIRECTION_NORTH":     0,
		"DIRECTION_EAST":      1,
		"DIRECTION_SOUTH":     2,
		"DIRECTION_WEST":      3,
		"DIRECTION_DIAGONAL_MASK": 4,
		"DIRECTION_SOUTHWEST": 4,
		"DIRECTION_SOUTHEAST": 5,
		"DIRECTION_NORTHWEST": 6,
		"DIRECTION_NORTHEAST": 7,

		// Return values
		"RETURNVALUE_NOERROR":         0,
		"RETURNVALUE_NOTPOSSIBLE":     1,
		"RETURNVALUE_NOTENOUGHROOM":   2,
		"RETURNVALUE_PLAYERISPZLOCKED": 3,

		// Condition types
		"CONDITION_NONE":              0,
		"CONDITION_POISON":            1,
		"CONDITION_FIRE":              2,
		"CONDITION_ENERGY":            3,
		"CONDITION_BLEEDING":          4,
		"CONDITION_HASTE":             5,
		"CONDITION_PARALYZE":          6,
		"CONDITION_OUTFIT":            7,
		"CONDITION_INVISIBLE":         8,
		"CONDITION_LIGHT":             9,
		"CONDITION_MANASHIELD":        10,
		"CONDITION_INFIGHT":           11,
		"CONDITION_DRUNK":             12,
		"CONDITION_EXHAUST":           13,
		"CONDITION_REGENERATION":      14,
		"CONDITION_SOUL":              15,
		"CONDITION_DROWN":             16,
		"CONDITION_MUTED":             17,
		"CONDITION_CHANNELMUTEDTICKS": 18,
		"CONDITION_YELLTICKS":         19,
		"CONDITION_ATTRIBUTES":        20,
		"CONDITION_FREEZING":          21,
		"CONDITION_DAZZLED":           22,
		"CONDITION_CURSED":            23,
	}

	for k, v := range enums {
		L.SetGlobal(k, v)
	}

	registerSpellEnums(L)
}
