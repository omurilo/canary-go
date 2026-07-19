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
		"MESSAGE_GAMEMASTER_CONSOLE":  0,
		"MESSAGE_LOGIN":               1,
		"MESSAGE_ADMINISTRATOR":       2,
		"MESSAGE_EVENT_ADVANCE":       3,
		"MESSAGE_GAME_HIGHLIGHT":      4,
		"MESSAGE_FAILURE":             5,
		"MESSAGE_LOOK":                6,
		"MESSAGE_DAMAGE_DEALT":        7,
		"MESSAGE_DAMAGE_RECEIVED":     8,
		"MESSAGE_HEALED":              9,
		"MESSAGE_EXPERIENCE":          10,
		"MESSAGE_DAMAGE_OTHERS":       11,
		"MESSAGE_HEALED_OTHERS":       12,
		"MESSAGE_EXPERIENCE_OTHERS":   13,
		"MESSAGE_STATUS":              14,
		"MESSAGE_LOOT":                15,
		"MESSAGE_TRADE":               16,
		"MESSAGE_GUILD":               17,
		"MESSAGE_PARTY_MANAGEMENT":    18,
		"MESSAGE_PARTY":               19,
		"MESSAGE_REPORT":              20,
		"MESSAGE_HOTKEY_PRESSED":      21,
		"MESSAGE_TUTORIAL_HINT":       22,
		"MESSAGE_THANK_YOU":           23,
		"MESSAGE_MARKET":              24,
		"MESSAGE_MANA":                25,
		"MESSAGE_BEYOND_LAST":         26,
		"MESSAGE_ATTENTION":           27,
		"MESSAGE_BOOSTED_CREATURE":    28,
		"MESSAGE_OFFLINE_TRAINING":    29,
		"MESSAGE_TRANSACTION":         30,
		"MESSAGE_POTION":              31,
		"MESSAGE_STATUS_CONSOLE_BLUE": 32, // commonly used alias/value

		// Combat Types
		"COMBAT_NONE":            0,
		"COMBAT_PHYSICALDAMAGE":  1,
		"COMBAT_ENERGYDAMAGE":    2,
		"COMBAT_EARTHDAMAGE":     3,
		"COMBAT_FIREDAMAGE":      4,
		"COMBAT_UNDEFINEDDAMAGE": 5,
		"COMBAT_LIFEDRAIN":       6,
		"COMBAT_MANADRAIN":       7,
		"COMBAT_HEALING":         8,
		"COMBAT_DROWNDAMAGE":     9,
		"COMBAT_ICEDAMAGE":       10,
		"COMBAT_HOLYDAMAGE":      11,
		"COMBAT_DEATHDAMAGE":     12,
		"COMBAT_AGONYDAMAGE":     13,
		"COMBAT_NEUTRALDAMAGE":   14,

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
	}

	for k, v := range enums {
		L.SetGlobal(k, v)
	}
}
