package items

import "strings"

// ShootTypes is ShootType_t (src/utils/utils_definitions.hpp): the projectile
// animation the client draws between attacker and target.
type ShootTypes uint16

const ShootTypeNone ShootTypes = 0

// shootTypesByName is the full port of shootTypeNames (src/utils/tools.cpp:805),
// with the values taken from the CONST_ANI_ enum. Note the gaps — 43, 46, 47, 52,
// 55 have no name — so this cannot be a range check or an ordered list.
//
// It replaces a hand-written 16-case switch whose default was CONST_ANI_ARROW, so
// every unlisted name (envenomedarrow, prismaticbolt, leafstar, the whirlwind
// animations, all of them) silently drew an arrow.
var shootTypesByName = map[string]ShootTypes{
	"spear":            1,  // CONST_ANI_SPEAR
	"bolt":             2,  // CONST_ANI_BOLT
	"arrow":            3,  // CONST_ANI_ARROW
	"fire":             4,  // CONST_ANI_FIRE
	"energy":           5,  // CONST_ANI_ENERGY
	"poisonarrow":      6,  // CONST_ANI_POISONARROW
	"burstarrow":       7,  // CONST_ANI_BURSTARROW
	"throwingstar":     8,  // CONST_ANI_THROWINGSTAR
	"throwingknife":    9,  // CONST_ANI_THROWINGKNIFE
	"smallstone":       10, // CONST_ANI_SMALLSTONE
	"death":            11, // CONST_ANI_DEATH
	"largerock":        12, // CONST_ANI_LARGEROCK
	"snowball":         13, // CONST_ANI_SNOWBALL
	"powerbolt":        14, // CONST_ANI_POWERBOLT
	"poison":           15, // CONST_ANI_POISON
	"infernalbolt":     16, // CONST_ANI_INFERNALBOLT
	"huntingspear":     17, // CONST_ANI_HUNTINGSPEAR
	"enchantedspear":   18, // CONST_ANI_ENCHANTEDSPEAR
	"redstar":          19, // CONST_ANI_REDSTAR
	"greenstar":        20, // CONST_ANI_GREENSTAR
	"royalspear":       21, // CONST_ANI_ROYALSPEAR
	"sniperarrow":      22, // CONST_ANI_SNIPERARROW
	"onyxarrow":        23, // CONST_ANI_ONYXARROW
	"piercingbolt":     24, // CONST_ANI_PIERCINGBOLT
	"whirlwindsword":   25, // CONST_ANI_WHIRLWINDSWORD
	"whirlwindaxe":     26, // CONST_ANI_WHIRLWINDAXE
	"whirlwindclub":    27, // CONST_ANI_WHIRLWINDCLUB
	"etherealspear":    28, // CONST_ANI_ETHEREALSPEAR
	"ice":              29, // CONST_ANI_ICE
	"earth":            30, // CONST_ANI_EARTH
	"holy":             31, // CONST_ANI_HOLY
	"suddendeath":      32, // CONST_ANI_SUDDENDEATH
	"flasharrow":       33, // CONST_ANI_FLASHARROW
	"flammingarrow":    34, // CONST_ANI_FLAMMINGARROW
	"shiverarrow":      35, // CONST_ANI_SHIVERARROW
	"energyball":       36, // CONST_ANI_ENERGYBALL
	"smallice":         37, // CONST_ANI_SMALLICE
	"smallholy":        38, // CONST_ANI_SMALLHOLY
	"smallearth":       39, // CONST_ANI_SMALLEARTH
	"eartharrow":       40, // CONST_ANI_EARTHARROW
	"explosion":        41, // CONST_ANI_EXPLOSION
	"cake":             42, // CONST_ANI_CAKE
	"tarsalarrow":      44, // CONST_ANI_TARSALARROW
	"vortexbolt":       45, // CONST_ANI_VORTEXBOLT
	"prismaticbolt":    48, // CONST_ANI_PRISMATICBOLT
	"crystallinearrow": 49, // CONST_ANI_CRYSTALLINEARROW
	"drillbolt":        50, // CONST_ANI_DRILLBOLT
	"envenomedarrow":   51, // CONST_ANI_ENVENOMEDARROW
	"gloothspear":      53, // CONST_ANI_GLOOTHSPEAR
	"simplearrow":      54, // CONST_ANI_SIMPLEARROW
	"leafstar":         56, // CONST_ANI_LEAFSTAR
	"diamondarrow":     57, // CONST_ANI_DIAMONDARROW
	"spectralbolt":     58, // CONST_ANI_SPECTRALBOLT
	"royalstar":        59, // CONST_ANI_ROYALSTAR
}

// ShootTypeByName resolves an items.xml `shoottype` value, returning ok=false for
// an unknown name. The C++ parser warns and leaves the field alone in that case
// (item_parse.cpp), which is why this reports the miss instead of guessing.
func ShootTypeByName(name string) (ShootTypes, bool) {
	st, ok := shootTypesByName[strings.ToLower(strings.TrimSpace(name))]
	return st, ok
}

// ShootTypeName is the reverse lookup, for the ItemType:getShootType binding that
// predates this table and hands scripts a string.
func ShootTypeName(st ShootTypes) string {
	for name, v := range shootTypesByName {
		if v == st {
			return name
		}
	}
	return ""
}
