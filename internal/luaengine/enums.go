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

		// Account types
		"ACCOUNT_TYPE_NORMAL":           1,
		"ACCOUNT_TYPE_TUTOR":            2,
		"ACCOUNT_TYPE_SENIORTUTOR":      3,
		"ACCOUNT_TYPE_GAMEMASTER":       4,
		"ACCOUNT_TYPE_COMMUNITYMANAGER": 5,
		"ACCOUNT_TYPE_GOD":              6,

		// Speak classes (talk types). NPC dialogue gates keyword processing on
		// TALKTYPE_PRIVATE_PN (player→NPC), so these MUST exist as globals — a
		// nil TALKTYPE_PRIVATE_PN silently kills all post-greeting interaction.
		"TALKTYPE_SAY":              1,
		"TALKTYPE_WHISPER":          2,
		"TALKTYPE_YELL":             3,
		"TALKTYPE_PRIVATE_FROM":     4,
		"TALKTYPE_PRIVATE_TO":       5,
		"TALKTYPE_CHANNEL_MANAGEMENT": 6,
		"TALKTYPE_CHANNEL_Y":        7,
		"TALKTYPE_CHANNEL_O":        8,
		"TALKTYPE_SPELL_USE":        9,
		"TALKTYPE_PRIVATE_NP":       10,
		"TALKTYPE_NPC_UNKNOWN":      11,
		"TALKTYPE_PRIVATE_PN":       12,
		"TALKTYPE_BROADCAST":        13,
		"TALKTYPE_CHANNEL_R1":       14,
		"TALKTYPE_PRIVATE_RED_FROM": 15,
		"TALKTYPE_PRIVATE_RED_TO":   16,
		"TALKTYPE_MONSTER_SAY":      36,
		"TALKTYPE_MONSTER_YELL":     37,
		"TALKTYPE_CHANNEL_R2":       0xFF,

		// Messages
		"MESSAGE_GAMEMASTER_CONSOLE":  13,
		"MESSAGE_LOGIN":               17,

		// Groups
		"GROUP_TYPE_NONE":             0,
		"GROUP_TYPE_NORMAL":           1,
		"GROUP_TYPE_TUTOR":            2,
		"GROUP_TYPE_SENIORTUTOR":      3,
		"GROUP_TYPE_GAMEMASTER":       4,
		"GROUP_TYPE_COMMUNITYMANAGER": 5,
		"GROUP_TYPE_GOD":              6,
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

		// Item Attributes
		"ITEM_ATTRIBUTE_NONE":                 0,
		"ITEM_ATTRIBUTE_ACTIONID":             1,
		"ITEM_ATTRIBUTE_UNIQUEID":             2,
		"ITEM_ATTRIBUTE_DESCRIPTION":          3,
		"ITEM_ATTRIBUTE_TEXT":                 4,
		"ITEM_ATTRIBUTE_DATE":                 5,
		"ITEM_ATTRIBUTE_WRITER":               6,
		"ITEM_ATTRIBUTE_NAME":                 7,
		"ITEM_ATTRIBUTE_ARTICLE":              8,
		"ITEM_ATTRIBUTE_PLURALNAME":           9,
		"ITEM_ATTRIBUTE_WEIGHT":               10,
		"ITEM_ATTRIBUTE_ATTACK":               11,
		"ITEM_ATTRIBUTE_DEFENSE":              12,
		"ITEM_ATTRIBUTE_EXTRADEFENSE":         13,
		"ITEM_ATTRIBUTE_ARMOR":                14,
		"ITEM_ATTRIBUTE_HITCHANCE":            15,
		"ITEM_ATTRIBUTE_SHOOTRANGE":           16,
		"ITEM_ATTRIBUTE_OWNER":                17,
		"ITEM_ATTRIBUTE_DURATION":             18,
		"ITEM_ATTRIBUTE_DECAYSTATE":           19,
		"ITEM_ATTRIBUTE_CORPSEOWNER":          20,
		"ITEM_ATTRIBUTE_CHARGES":              21,
		"ITEM_ATTRIBUTE_FLUIDTYPE":            22,
		"ITEM_ATTRIBUTE_DOORID":               23,
		"ITEM_ATTRIBUTE_SPECIAL":              24,
		"ITEM_ATTRIBUTE_IMBUEMENT_SLOT":       25,
		"ITEM_ATTRIBUTE_OPENCONTAINER":        26,
		"ITEM_ATTRIBUTE_QUICKLOOTCONTAINER":   27,
		"ITEM_ATTRIBUTE_DURATION_TIMESTAMP":   28,
		"ITEM_ATTRIBUTE_AMOUNT":               29,
		"ITEM_ATTRIBUTE_TIER":                 30,
		"ITEM_ATTRIBUTE_STORE":                31,
		"ITEM_ATTRIBUTE_CUSTOM":               32,
		"ITEM_ATTRIBUTE_LOOTMESSAGE_SUFFIX":   33,
		"ITEM_ATTRIBUTE_STORE_INBOX_CATEGORY": 34,
		"ITEM_ATTRIBUTE_OBTAINCONTAINER":      35,
		"ITEM_ATTRIBUTE_AUGMENTS":             36,
		"ITEM_ATTRIBUTE_MANTRA":               37,

		// Item Coin IDs
		"ITEM_GOLD_COIN":        3031,
		"ITEM_PLATINUM_COIN":    3035,
		"ITEM_CRYSTAL_COIN":     3043,

		// Skills
		"SKILL_NONE":                -1,
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

		// Player Pronouns
		"PLAYERPRONOUN_UNSET": 0,
		"PLAYERPRONOUN_THEY":  1,
		"PLAYERPRONOUN_SHE":   2,
		"PLAYERPRONOUN_HE":    3,
		"PLAYERPRONOUN_ZE":    4,
		"PLAYERPRONOUN_NAME":  5,

		// Concoction IDs
		"Concoction_KooldownAid":           1,
		"Concoction_StaminaExtension":      2,
		"Concoction_StrikeEnhancement":     3,
		"Concoction_CharmUpgrade":          4,
		"Concoction_WealthDuplex":          5,
		"Concoction_BestiaryBetterment":     6,
		"Concoction_FireResilience":        7,
		"Concoction_IceResilience":         8,
		"Concoction_EarthResilience":       9,
		"Concoction_EnergyResilience":      10,
		"Concoction_HolyResilience":        11,
		"Concoction_DeathResilience":       12,
		"Concoction_PhysicalResilience":    13,
		"Concoction_FireAmplification":     14,
		"Concoction_IceAmplification":      15,
		"Concoction_EarthAmplification":    16,
		"Concoction_EnergyAmplification":   17,
		"Concoction_HolyAmplification":     18,
		"Concoction_DeathAmplification":    19,
		"Concoction_PhysicalAmplification": 20,

		// World Types
		"WORLD_TYPE_NO_PVP":       0,
		"WORLD_TYPE_PVP":          1,
		"WORLD_TYPE_PVP_ENFORCED": 2,

		// Tile states. Values match the OTBM tile-flag bits actually stored in
		// Tile.Flags by the map loader (io_definitions.hpp OTBM_TILEFLAG_*):
		// PZ=1<<0, NOPVP=1<<2, NOLOGOUT=1<<3, PVP=1<<4. The remaining TILESTATE_*
		// constants are runtime-only in C++ and are not populated by the loader
		// (they read back as absent); they are defined here so datapack scripts
		// that reference them get a number rather than nil.
		"TILESTATE_NONE":           0,
		"TILESTATE_PROTECTIONZONE": 1 << 0,
		"TILESTATE_NOPVPZONE":      1 << 2,
		"TILESTATE_NOPVP":          1 << 2,
		"TILESTATE_NOLOGOUT":       1 << 3,
		"TILESTATE_PVPZONE":        1 << 4,
		"TILESTATE_TELEPORT":       1 << 11,
		"TILESTATE_MAGICFIELD":     1 << 12,
		"TILESTATE_MAILBOX":        1 << 13,
		"TILESTATE_TRASHHOLDER":    1 << 14,
		"TILESTATE_BED":            1 << 15,
		"TILESTATE_DEPOT":          1 << 16,
		// House is not a distinct OTBM bit (house tiles are stored as protection
		// zones); a high, never-set bit keeps hasFlag(TILESTATE_HOUSE) false.
		"TILESTATE_HOUSE": 1 << 30,

		// Item / Tile Properties (CONST_PROP_*)
		"CONST_PROP_BLOCKSOLID":                0,
		"CONST_PROP_HASHEIGHT":                 1,
		"CONST_PROP_BLOCKPROJECTILE":           2,
		"CONST_PROP_BLOCKPATH":                 3,
		"CONST_PROP_ISVERTICAL":                4,
		"CONST_PROP_ISHORIZONTAL":              5,
		"CONST_PROP_MOVABLE":                   6,
		"CONST_PROP_IMMOVABLEBLOCKSOLID":       7,
		"CONST_PROP_IMMOVABLEBLOCKPATH":        8,
		"CONST_PROP_IMMOVABLENOFIELDBLOCKPATH": 9,
		"CONST_PROP_NOFIELDBLOCKPATH":          10,
		"CONST_PROP_SUPPORTHANGABLE":           11,

		// Zones
		"ZONE_PROTECTION": 0,
		"ZONE_NOPVP":      1,
		"ZONE_PVP":        2,
		"ZONE_NOLOGOUT":   3,
		"ZONE_NORMAL":     4,

		// Bestiary Races (creatures_definitions.hpp BestiaryType_t). The client
		// groups the cyclopedia by these ids, so they must match C++ exactly.
		"BESTY_RACE_NONE":              0,
		"BESTY_RACE_AMPHIBIC":          1,
		"BESTY_RACE_AQUATIC":           2,
		"BESTY_RACE_BIRD":              3,
		"BESTY_RACE_CONSTRUCT":         4,
		"BESTY_RACE_DEMON":             5,
		"BESTY_RACE_DRAGON":            6,
		"BESTY_RACE_ELEMENTAL":         7,
		"BESTY_RACE_FEY":               8,
		"BESTY_RACE_GIANT":             9,
		"BESTY_RACE_HUMAN":             10,
		"BESTY_RACE_HUMANOID":          11,
		"BESTY_RACE_LYCANTHROPE":       12,
		"BESTY_RACE_MAGICAL":           13,
		"BESTY_RACE_MAMMAL":            14,
		"BESTY_RACE_PLANT":             15,
		"BESTY_RACE_REPTILE":           16,
		"BESTY_RACE_SLIME":             17,
		"BESTY_RACE_UNDEAD":            18,
		"BESTY_RACE_VERMIN":            19,
		"BESTY_RACE_EXTRA_DIMENSIONAL": 20,
		"BESTY_RACE_INKBORN":           21,

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
		// Condition slot ids (ConditionId_t). Food scripts pass CONDITIONID_DEFAULT.
		"CONDITIONID_DEFAULT": -1,
		"CONDITIONID_COMBAT":  0,
		// Sound effect used by the food action; sound playback is a no-op stub
		// for now, but the constant must exist so the script doesn't pass nil.
		"SOUND_EFFECT_TYPE_ACTION_EAT": 82,
		"CONDITION_SOUL":              15,
		"CONDITION_DROWN":             16,
		"CONDITION_MUTED":             17,
		"CONDITION_CHANNELMUTEDTICKS": 18,
		"CONDITION_YELLTICKS":         19,
		"CONDITION_ATTRIBUTES":        20,
		"CONDITION_FREEZING":          21,
		"CONDITION_DAZZLED":           22,
		"CONDITION_CURSED":            23,

		"FORGE_NORMAL_MONSTER":     0,
		"FORGE_INFLUENCED_MONSTER": 1,
		"FORGE_FIENDISH_MONSTER":   2,

		// Bosstiary rarities (monster.bosstiary.bossRace). Match bosstiary.Rarity.
		"RARITY_BANE":    0,
		"RARITY_ARCHFOE": 1,
		"RARITY_NEMESIS": 2,

		// Cylinder index / flags (src/items/cylinder.hpp, items_definitions.hpp).
		// Used by addItemEx / internalAddItem (e.g. Player:addItemStoreInbox).
		"INDEX_WHEREEVER":          -1,
		"FLAG_NOLIMIT":             1,
		"FLAG_IGNOREBLOCKITEM":     2,
		"FLAG_IGNOREBLOCKCREATURE": 4,
		"FLAG_CHILDISOWNER":        8,
		"FLAG_PATHFINDING":         16,
		"FLAG_IGNOREFIELDDAMAGE":   32,
		"FLAG_IGNORENOTMOVABLE":    64,
		"FLAG_IGNOREAUTOSTACK":     128,
		"FLAG_DROPONMAP":           256,
		"FLAG_LOOTPOUCH":           512,
	}

	for k, v := range enums {
		L.SetGlobal(k, v)
	}

	registerSpellEnums(L)
}
