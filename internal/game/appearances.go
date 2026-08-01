package game

// Cyclopedia outfit type constants (matching C++ CYCLOPEDIA_CHARACTERINFO_OUTFITTYPE_*).
const (
	OutfitTypeNone  uint8 = 0
	OutfitTypeQuest uint8 = 1
	OutfitTypeStore uint8 = 2
)

// OutfitInfo holds the display name and source type for an outfit look type.
type OutfitInfo struct {
	Name string
	From string // "none", "quest", or "store"
}

// FamiliarInfo holds the display name for a familiar.
type FamiliarInfo struct {
	Name string
	From string // "none" or "quest"
}

// outfitRegistry maps lookType -> display info.
var outfitRegistry = map[uint16]OutfitInfo{}

// femaleOutfitOrder is the ordered list of female outfit look types (matching
// the order in data/XML/outfits.xml so the client displays them consistently).
var femaleOutfitOrder = []uint16{}

// maleOutfitOrder is the ordered list of male outfit look types.
var maleOutfitOrder = []uint16{}

// familiarRegistry maps (vocationId, lookType) -> display info.
type familiarKey struct {
	VocationID uint16
	LookType   uint16
}

var familiarRegistry = map[familiarKey]FamiliarInfo{}

func init() {
	// ----- outfit registry -----
	outfitRegistry = map[uint16]OutfitInfo{
		136:  {Name: "Citizen", From: "none"},
		137:  {Name: "Hunter", From: "none"},
		138:  {Name: "Mage", From: "none"},
		139:  {Name: "Knight", From: "none"},
		140:  {Name: "Noblewoman", From: "none"},
		141:  {Name: "Summoner", From: "none"},
		142:  {Name: "Warrior", From: "none"},
		147:  {Name: "Barbarian", From: "none"},
		148:  {Name: "Druid", From: "none"},
		149:  {Name: "Wizard", From: "none"},
		150:  {Name: "Oriental", From: "none"},
		155:  {Name: "Pirate", From: "quest"},
		156:  {Name: "Assassin", From: "quest"},
		157:  {Name: "Beggar", From: "quest"},
		158:  {Name: "Shaman", From: "quest"},
		252:  {Name: "Norsewoman", From: "quest"},
		269:  {Name: "Nightmare", From: "quest"},
		270:  {Name: "Jester", From: "quest"},
		279:  {Name: "Brotherhood", From: "quest"},
		288:  {Name: "Demon Hunter", From: "quest"},
		324:  {Name: "Yalaharian", From: "quest"},
		329:  {Name: "Newly Wed", From: "quest"},
		336:  {Name: "Warmaster", From: "quest"},
		366:  {Name: "Wayfarer", From: "quest"},
		431:  {Name: "Afflicted", From: "quest"},
		433:  {Name: "Elementalist", From: "quest"},
		464:  {Name: "Deepling", From: "quest"},
		466:  {Name: "Insectoid", From: "quest"},
		471:  {Name: "Entrepreneur", From: "store"},
		513:  {Name: "Crystal Warlord", From: "quest"},
		514:  {Name: "Soil Guardian", From: "quest"},
		542:  {Name: "Demon", From: "quest"},
		575:  {Name: "Cave Explorer", From: "quest"},
		578:  {Name: "Dream Warden", From: "quest"},
		618:  {Name: "Glooth Engineer", From: "quest"},
		620:  {Name: "Jersey", From: "none"},
		632:  {Name: "Champion", From: "store"},
		635:  {Name: "Conjurer", From: "store"},
		636:  {Name: "Beastmaster", From: "store"},
		664:  {Name: "Chaos Acolyte", From: "store"},
		666:  {Name: "Death Herald", From: "store"},
		683:  {Name: "Ranger", From: "store"},
		694:  {Name: "Ceremonial Garb", From: "store"},
		696:  {Name: "Puppeteer", From: "store"},
		698:  {Name: "Spirit Caller", From: "store"},
		724:  {Name: "Evoker", From: "store"},
		732:  {Name: "Seaweaver", From: "store"},
		745:  {Name: "Recruiter", From: "none"},
		749:  {Name: "Sea Dog", From: "store"},
		759:  {Name: "Royal Pumpkin", From: "store"},
		845:  {Name: "Rift Warrior", From: "quest"},
		852:  {Name: "Winter Warden", From: "store"},
		874:  {Name: "Philosopher", From: "store"},
		885:  {Name: "Arena Champion", From: "store"},
		900:  {Name: "Lupine Warden", From: "store"},
		909:  {Name: "Grove Keeper", From: "store"},
		929:  {Name: "Festive", From: "quest"},
		956:  {Name: "Pharaoh", From: "store"},
		958:  {Name: "Trophy Hunter", From: "store"},
		963:  {Name: "Retro Warrior", From: "store"},
		965:  {Name: "Retro Summoner", From: "store"},
		967:  {Name: "Retro Noblewoman", From: "store"},
		969:  {Name: "Retro Mage", From: "store"},
		971:  {Name: "Retro Knight", From: "store"},
		973:  {Name: "Retro Hunter", From: "store"},
		975:  {Name: "Retro Citizen", From: "store"},
		1020: {Name: "Herbalist", From: "store"},
		1024: {Name: "Sun Priest", From: "store"},
		1043: {Name: "Makeshift Warrior", From: "quest"},
		1050: {Name: "Siege Master", From: "store"},
		1057: {Name: "Mercenary", From: "store"},
		1070: {Name: "Battle Mage", From: "quest"},
		1095: {Name: "Discoverer", From: "quest"},
		1103: {Name: "Sinister Archer", From: "store"},
		1128: {Name: "Pumpkin Mummy", From: "store"},
		1147: {Name: "Dream Warrior", From: "quest"},
		1162: {Name: "Percht Raider", From: "quest"},
		1174: {Name: "Owl Keeper", From: "store"},
		1187: {Name: "Guidon Bearer", From: "store"},
		1203: {Name: "Void Master", From: "store"},
		1205: {Name: "Veteran Paladin", From: "store"},
		1207: {Name: "Lion of War", From: "store"},
		1211: {Name: "Golden", From: "quest"},
		1244: {Name: "Hand of the Inquisition", From: "quest"},
		1246: {Name: "Breezy Garb", From: "store"},
		1252: {Name: "Orcsoberfest Garb", From: "quest"},
		1271: {Name: "Poltergeist", From: "quest"},
		1280: {Name: "Herder", From: "store"},
		1283: {Name: "Falconer", From: "quest"},
		1289: {Name: "Dragon Slayer", From: "store"},
		1293: {Name: "Trailblazer", From: "store"},
		1323: {Name: "Revenant", From: "quest"},
		1332: {Name: "Jouster", From: "store"},
		1339: {Name: "Moth Cape", From: "store"},
		1372: {Name: "Rascoohan", From: "quest"},
		1383: {Name: "Merry Garb", From: "store"},
		1385: {Name: "Rune Master", From: "store"},
		1387: {Name: "Citizen of Issavi", From: "quest"},
		1416: {Name: "Forest Warden", From: "store"},
		1437: {Name: "Royal Bounacean Advisor", From: "quest"},
		1445: {Name: "Dragon Knight", From: "store"},
		1450: {Name: "Arbalester", From: "store"},
		1456: {Name: "Royal Costume", From: "store"},
		1461: {Name: "Formal Dress", From: "store"},
		1490: {Name: "Ghost Blade", From: "store"},
		1501: {Name: "Nordic Chieftain", From: "store"},
		1569: {Name: "Fire-Fighter", From: "quest"},
		1576: {Name: "Fencer", From: "store"},
		1582: {Name: "Shadowlotus Disciple", From: "store"},
		1598: {Name: "Ancient Aucar", From: "quest"},
		1613: {Name: "Frost Tracer", From: "store"},
		1619: {Name: "Armoured Archer", From: "store"},
		1663: {Name: "Decaying Defender", From: "quest"},
		1676: {Name: "Darklight Evoker", From: "store"},
		1681: {Name: "Flamefury Mage", From: "store"},
		1714: {Name: "Doom Knight", From: "store"},
		1723: {Name: "Draccoon Herald", From: "quest"},
		1726: {Name: "Celestial Avenger", From: "store"},
		1746: {Name: "Blade Dancer", From: "store"},
		1775: {Name: "Rootwalker", From: "quest"},
		1777: {Name: "Beekeeper", From: "store"},
		1808: {Name: "Fiend Slayer", From: "quest"},
		1832: {Name: "Winged Druid", From: "store"},
		1825: {Name: "Monk", From: "none"},
		1838: {Name: "Martial Artist", From: "store"},
		1861: {Name: "Illuminator", From: "quest"},
		1860: {Name: "Illuminator", From: "quest"},
		// Male outfits
		128:  {Name: "Citizen", From: "none"},
		129:  {Name: "Hunter", From: "none"},
		130:  {Name: "Mage", From: "none"},
		131:  {Name: "Knight", From: "none"},
		132:  {Name: "Nobleman", From: "none"},
		133:  {Name: "Summoner", From: "none"},
		134:  {Name: "Warrior", From: "none"},
		143:  {Name: "Barbarian", From: "none"},
		144:  {Name: "Druid", From: "none"},
		145:  {Name: "Wizard", From: "none"},
		146:  {Name: "Oriental", From: "none"},
		151:  {Name: "Pirate", From: "quest"},
		152:  {Name: "Assassin", From: "quest"},
		153:  {Name: "Beggar", From: "quest"},
		154:  {Name: "Shaman", From: "quest"},
		251:  {Name: "Norseman", From: "quest"},
		268:  {Name: "Nightmare", From: "quest"},
		273:  {Name: "Jester", From: "quest"},
		278:  {Name: "Brotherhood", From: "quest"},
		289:  {Name: "Demon Hunter", From: "quest"},
		325:  {Name: "Yalaharian", From: "quest"},
		328:  {Name: "Newly Wed", From: "quest"},
		335:  {Name: "Warmaster", From: "quest"},
		367:  {Name: "Wayfarer", From: "quest"},
		430:  {Name: "Afflicted", From: "quest"},
		432:  {Name: "Elementalist", From: "quest"},
		463:  {Name: "Deepling", From: "quest"},
		465:  {Name: "Insectoid", From: "quest"},
		472:  {Name: "Entrepreneur", From: "store"},
		512:  {Name: "Crystal Warlord", From: "quest"},
		516:  {Name: "Soil Guardian", From: "quest"},
		541:  {Name: "Demon", From: "quest"},
		574:  {Name: "Cave Explorer", From: "quest"},
		577:  {Name: "Dream Warden", From: "quest"},
		610:  {Name: "Glooth Engineer", From: "quest"},
		619:  {Name: "Jersey", From: "none"},
		633:  {Name: "Champion", From: "store"},
		634:  {Name: "Conjurer", From: "store"},
		637:  {Name: "Beastmaster", From: "store"},
		665:  {Name: "Chaos Acolyte", From: "store"},
		667:  {Name: "Death Herald", From: "store"},
		684:  {Name: "Ranger", From: "store"},
		695:  {Name: "Ceremonial Garb", From: "store"},
		697:  {Name: "Puppeteer", From: "store"},
		699:  {Name: "Spirit Caller", From: "store"},
		725:  {Name: "Evoker", From: "store"},
		733:  {Name: "Seaweaver", From: "store"},
		746:  {Name: "Recruiter", From: "none"},
		750:  {Name: "Sea Dog", From: "store"},
		760:  {Name: "Royal Pumpkin", From: "store"},
		846:  {Name: "Rift Warrior", From: "quest"},
		853:  {Name: "Winter Warden", From: "store"},
		873:  {Name: "Philosopher", From: "store"},
		884:  {Name: "Arena Champion", From: "store"},
		899:  {Name: "Lupine Warden", From: "store"},
		908:  {Name: "Grove Keeper", From: "store"},
		931:  {Name: "Festive", From: "quest"},
		955:  {Name: "Pharaoh", From: "store"},
		957:  {Name: "Trophy Hunter", From: "store"},
		962:  {Name: "Retro Warrior", From: "store"},
		964:  {Name: "Retro Summoner", From: "store"},
		966:  {Name: "Retro Nobleman", From: "store"},
		968:  {Name: "Retro Mage", From: "store"},
		970:  {Name: "Retro Knight", From: "store"},
		972:  {Name: "Retro Hunter", From: "store"},
		974:  {Name: "Retro Citizen", From: "store"},
		1021: {Name: "Herbalist", From: "store"},
		1023: {Name: "Sun Priest", From: "store"},
		1042: {Name: "Makeshift Warrior", From: "quest"},
		1051: {Name: "Siege Master", From: "store"},
		1056: {Name: "Mercenary", From: "store"},
		1069: {Name: "Battle Mage", From: "quest"},
		1094: {Name: "Discoverer", From: "quest"},
		1102: {Name: "Sinister Archer", From: "store"},
		1127: {Name: "Pumpkin Mummy", From: "store"},
		1146: {Name: "Dream Warrior", From: "quest"},
		1161: {Name: "Percht Raider", From: "quest"},
		1173: {Name: "Owl Keeper", From: "store"},
		1186: {Name: "Guidon Bearer", From: "store"},
		1202: {Name: "Void Master", From: "store"},
		1204: {Name: "Veteran Paladin", From: "store"},
		1206: {Name: "Lion of War", From: "store"},
		1210: {Name: "Golden", From: "quest"},
		1243: {Name: "Hand of the Inquisition", From: "quest"},
		1245: {Name: "Breezy Garb", From: "store"},
		1251: {Name: "Orcsoberfest Garb", From: "quest"},
		1270: {Name: "Poltergeist", From: "quest"},
		1279: {Name: "Herder", From: "store"},
		1282: {Name: "Falconer", From: "quest"},
		1288: {Name: "Dragon Slayer", From: "store"},
		1292: {Name: "Trailblazer", From: "store"},
		1322: {Name: "Revenant", From: "quest"},
		1331: {Name: "Jouster", From: "store"},
		1338: {Name: "Moth Cape", From: "store"},
		1371: {Name: "Rascoohan", From: "quest"},
		1382: {Name: "Merry Garb", From: "store"},
		1384: {Name: "Rune Master", From: "store"},
		1386: {Name: "Citizen of Issavi", From: "quest"},
		1415: {Name: "Forest Warden", From: "store"},
		1436: {Name: "Royal Bounacean Advisor", From: "quest"},
		1444: {Name: "Dragon Knight", From: "store"},
		1449: {Name: "Arbalester", From: "store"},
		1457: {Name: "Royal Costume", From: "store"},
		1460: {Name: "Formal Dress", From: "store"},
		1489: {Name: "Ghost Blade", From: "store"},
		1500: {Name: "Nordic Chieftain", From: "store"},
		1568: {Name: "Fire-Fighter", From: "quest"},
		1575: {Name: "Fencer", From: "store"},
		1581: {Name: "Shadowlotus Disciple", From: "store"},
		1597: {Name: "Ancient Aucar", From: "quest"},
		1612: {Name: "Frost Tracer", From: "store"},
		1618: {Name: "Armoured Archer", From: "store"},
		1662: {Name: "Decaying Defender", From: "quest"},
		1675: {Name: "Darklight Evoker", From: "store"},
		1680: {Name: "Flamefury Mage", From: "store"},
		1713: {Name: "Doom Knight", From: "store"},
		1722: {Name: "Draccoon Herald", From: "quest"},
		1725: {Name: "Celestial Avenger", From: "store"},
		1745: {Name: "Blade Dancer", From: "store"},
		1774: {Name: "Rootwalker", From: "quest"},
		1776: {Name: "Beekeeper", From: "store"},
		1809: {Name: "Fiend Slayer", From: "quest"},
		1831: {Name: "Winged Druid", From: "store"},
		1824: {Name: "Monk", From: "none"},
		1837: {Name: "Martial Artist", From: "store"},
	}

	// ----- female outfit order (matching data/XML/outfits.xml) -----
	femaleOutfitOrder = []uint16{
		136, 137, 138, 139, 140, 141, 142, 147, 148, 149, 150,
		155, 156, 157, 158, 252, 269, 270, 279, 288, 324, 329,
		336, 366, 431, 433, 464, 466, 471, 513, 514, 542, 575,
		578, 618, 620, 632, 635, 636, 664, 666, 683, 694, 696,
		698, 724, 732, 745, 749, 759, 845, 852, 874, 885, 900,
		909, 929, 956, 958, 963, 965, 967, 969, 971, 973, 975,
		1020, 1024, 1043, 1050, 1057, 1070, 1095, 1103, 1128,
		1147, 1162, 1174, 1187, 1203, 1205, 1207, 1211, 1244,
		1246, 1252, 1271, 1280, 1283, 1289, 1293, 1323, 1332,
		1339, 1372, 1383, 1385, 1387, 1416, 1437, 1445, 1450,
		1456, 1461, 1490, 1501, 1569, 1576, 1582, 1598, 1613,
		1619, 1663, 1676, 1681, 1714, 1723, 1726, 1746, 1775,
		1777, 1808, 1832, 1825, 1838, 1861, 1860,
	}

	// ----- male outfit order (matching data/XML/outfits.xml) -----
	maleOutfitOrder = []uint16{
		128, 129, 130, 131, 132, 133, 134, 143, 144, 145, 146,
		151, 152, 153, 154, 251, 268, 273, 278, 289, 325, 328,
		335, 367, 430, 432, 463, 465, 472, 512, 516, 541, 574,
		577, 610, 619, 633, 634, 637, 665, 667, 684, 695, 697,
		699, 725, 733, 746, 750, 760, 846, 853, 873, 884, 899,
		908, 931, 955, 957, 962, 964, 966, 968, 970, 972, 974,
		1021, 1023, 1042, 1051, 1056, 1069, 1094, 1102, 1127,
		1146, 1161, 1173, 1186, 1202, 1204, 1206, 1210, 1243,
		1245, 1251, 1270, 1279, 1282, 1288, 1292, 1322, 1331,
		1338, 1371, 1382, 1384, 1386, 1415, 1436, 1444, 1449,
		1457, 1460, 1489, 1500, 1568, 1575, 1581, 1597, 1612,
		1618, 1662, 1675, 1680, 1713, 1722, 1725, 1745, 1774,
		1776, 1809, 1831, 1824, 1837,
	}

	// ----- familiar registry (by (vocationId, lookType)) -----
	// Vocation IDs match the canary-go vocations XML (data/XML/vocations.xml).
	// See: data/XML/familiars.xml
	familiarRegistry = map[familiarKey]FamiliarInfo{
		// Sorcerer (id=1) and Master Sorcerer (id=5)
		{VocationID: 1, LookType: 994}:  {Name: "Thundergiant", From: "none"},
		{VocationID: 5, LookType: 994}:  {Name: "Thundergiant", From: "none"},
		{VocationID: 1, LookType: 1367}: {Name: "Bladespark", From: "quest"},
		{VocationID: 5, LookType: 1367}: {Name: "Bladespark", From: "quest"},
		// Druid (id=2) and Elder Druid (id=6)
		{VocationID: 2, LookType: 993}:  {Name: "Grovebeast", From: "none"},
		{VocationID: 6, LookType: 993}:  {Name: "Grovebeast", From: "none"},
		{VocationID: 2, LookType: 1364}: {Name: "Mossmasher", From: "quest"},
		{VocationID: 6, LookType: 1364}: {Name: "Mossmasher", From: "quest"},
		// Paladin (id=3) and Royal Paladin (id=7)
		{VocationID: 3, LookType: 992}:  {Name: "Emberwing", From: "none"},
		{VocationID: 7, LookType: 992}:  {Name: "Emberwing", From: "none"},
		{VocationID: 3, LookType: 1366}: {Name: "Sandscourge", From: "quest"},
		{VocationID: 7, LookType: 1366}: {Name: "Sandscourge", From: "quest"},
		// Knight (id=4) and Elite Knight (id=8)
		{VocationID: 4, LookType: 991}:  {Name: "Skullfrost", From: "none"},
		{VocationID: 8, LookType: 991}:  {Name: "Skullfrost", From: "none"},
		{VocationID: 4, LookType: 1365}: {Name: "Snowbash", From: "quest"},
		{VocationID: 8, LookType: 1365}: {Name: "Snowbash", From: "quest"},
		// Monk (id=9) and Exalted Monk (id=10)
		{VocationID: 9, LookType: 1818}:  {Name: "Omniphant", From: "none"},
		{VocationID: 10, LookType: 1818}: {Name: "Omniphant", From: "none"},
		{VocationID: 9, LookType: 1819}:  {Name: "Moonhunter", From: "quest"},
		{VocationID: 10, LookType: 1819}: {Name: "Moonhunter", From: "quest"},
	}
}

// GetOutfitInfo returns the display info for a given outfit look type.
func GetOutfitInfo(lookType uint16) (OutfitInfo, bool) {
	info, ok := outfitRegistry[lookType]
	return info, ok
}

// GetOutfitsBySex returns the ordered list of outfit look types for the given
// sex (0=female, 1=male). Returns nil for unknown sex.
func GetOutfitsBySex(sex uint8) []uint16 {
	switch sex {
	case 0:
		return femaleOutfitOrder
	case 1:
		return maleOutfitOrder
	default:
		return nil
	}
}

// GetFamiliarInfo returns the display info for a familiar by vocation and
// look type. vocationID is the OTServBR vocation id (cf. data/XML/vocations.xml).
func GetFamiliarInfo(vocationID uint16, lookType uint16) (FamiliarInfo, bool) {
	info, ok := familiarRegistry[familiarKey{VocationID: vocationID, LookType: lookType}]
	return info, ok
}
