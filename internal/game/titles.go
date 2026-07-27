package game

// TitleInfo represents a cyclopedia character title.
// Mirrors the C++ Title struct from player_title.hpp.
type TitleInfo struct {
	ID          uint8
	MaleName    string
	FemaleName  string
	Description string
	Permanent   bool
}

// DefaultTitles is the registry of all titles known to the server, matching the
// C++ Game::m_titles initialisation in game.cpp (lines 584-688).
//
// Reference: enum class CyclopediaTitle_t { NOTHING, GOLD, MOUNTS, OUTFITS, LEVEL,
// HIGHSCORES, BESTIARY, BOSSTIARY, DAILY_REWARD, TASK, MAP, OTHERS };
var DefaultTitles = []TitleInfo{
	// Gold hoarder titles (GOLD)
	{1, "Gold Hoarder", "", "Earned at least 1,000,000 gold.", false},
	{2, "Platinum Hoarder", "", "Earned at least 10,000,000 gold.", false},
	{3, "Crystal Hoarder", "", "Earned at least 100,000,000 gold.", false},

	// Mount titles (MOUNTS)
	{4, "Beaststrider (Grade 1)", "", "Unlocked 10 or more Mounts.", true},
	{5, "Beaststrider (Grade 2)", "", "Unlocked 20 or more Mounts.", true},
	{6, "Beaststrider (Grade 3)", "", "Unlocked 30 or more Mounts.", true},
	{7, "Beaststrider (Grade 4)", "", "Unlocked 40 or more Mounts.", true},
	{8, "Beaststrider (Grade 5)", "", "Unlocked 50 or more Mounts.", true},

	// Outfit titles (OUTFITS)
	{9, "Tibia's Topmodel (Grade 1)", "", "Unlocked 10 or more Outfits.", true},
	{10, "Tibia's Topmodel (Grade 2)", "", "Unlocked 20 or more Outfits.", true},
	{11, "Tibia's Topmodel (Grade 3)", "", "Unlocked 30 or more Outfits.", true},
	{12, "Tibia's Topmodel (Grade 4)", "", "Unlocked 40 or more Outfits.", true},
	{13, "Tibia's Topmodel (Grade 5)", "", "Unlocked 50 or more Outfits.", true},

	// Level titles (LEVEL)
	{14, "Trolltrasher", "", "Reached level 50.", false},
	{15, "Cyclopscamper", "", "Reached level 100.", false},
	{16, "Dragondouser", "", "Reached level 200.", false},
	{17, "Demondoom", "", "Reached level 300.", false},
	{18, "Drakenbane", "", "Reached level 400.", false},
	{19, "Silencer", "", "Reached level 500.", false},
	{20, "Exalted", "", "Reached level 1000.", false},

	// Highscore titles (HIGHSCORES)
	{21, "Apex Predator", "", "Highest Level on character's world.", false},
	{22, "Big Boss", "", "Highest score of accumulated boss points on character's world.", false},
	{23, "Jack of all Taints", "", "Highest score for killing Goshnar and his aspects on character's world.", false},
	{24, "Legend of Fishing", "", "Highest fishing level on character's world.", false},
	{25, "Legend of Magic", "", "Highest magic level on character's world.", false},
	{26, "Legend of Marksmanship", "", "Highest distance level on character's world.", false},
	{27, "Legend of the Axe", "", "Highest axe level on character's world.", false},
	{28, "Legend of the Club", "", "Highest club level on character's world.", false},
	{29, "Legend of the Fist", "", "Highest fist level on character's world.", false},
	{30, "Legend of the Shield", "", "Highest shielding level on character's world.", false},
	{31, "Legend of the Sword", "", "Highest sword level on character's world.", false},
	{32, "Prince Charming", "Princess Charming", "Highest score of accumulated charm points on character's world.", false},
	{33, "Reigning Drome Champion", "", "Finished most recent Tibiadrome rota ranked in the top 5.", false},

	// Bestiary titles (BESTIARY)
	{34, "Bipedantic", "", "Unlocked All Humanoid Bestiary entries.", false},
	{35, "Blood Moon Hunter", "Blood Moon Huntress", "Unlocked All Lycanthrope Bestiary entries.", false},
	{36, "Coldblooded", "", "Unlocked All Amphibic Bestiary entries.", false},
	{37, "Death from Below", "", "Unlocked all Bird Bestiary entries.", false},
	{38, "Demonator", "", "Unlocked all Demon Bestiary entries.", false},
	{39, "Dragonslayer", "", "Unlocked all Dragon Bestiary entries.", false},
	{40, "Elementalist", "", "Unlocked all Elemental Bestiary entries.", false},
	{41, "Exterminator", "", "Unlocked all Vermin Bestiary entries.", false},
	{42, "Fey Swatter", "", "Unlocked all Fey Bestiary entries.", false},
	{43, "Ghosthunter", "Ghosthuntress", "Unlocked all Undead Bestiary entries.", false},
	{44, "Handyman", "Handywoman", "Unlocked all Construct Bestiary entries.", false},
	{45, "Huntsman", "Huntress", "Unlocked all Mammal Bestiary entries.", false},
	{46, "Interdimensional Destroyer", "", "Unlocked all Extra Dimensional Bestiary entries.", false},
	{47, "Manhunter", "Manhuntress", "Unlocked all Human Bestiary entries.", false},
	{48, "Master of Illusion", "Mistress of Illusion", "Unlocked all Magical Bestiary entries.", false},
	{49, "Ooze Blues", "", "Unlocked all Slime Bestiary entries.", false},
	{50, "Sea Bane", "", "Unlocked all Aquatic Bestiary entries.", false},
	{51, "Snake Charmer", "", "Unlocked all Reptile Bestiary entries.", false},
	{52, "Tumbler", "", "Unlocked all Giant Bestiary entries.", false},
	{53, "Weedkiller", "", "Unlocked all Plant Bestiary entries.", false},
	{54, "Executioner", "", "Unlocked all Bestiary entries.", false},

	// Bosstiary titles (BOSSTIARY)
	{55, "Boss Annihilator", "", "Unlocked all Nemesis bosses.", false},
	{56, "Boss Destroyer", "", "Unlocked 10 or more Archfoe bosses.", true},
	{57, "Boss Devastator", "", "Unlocked 10 or more Nemesis bosses.", true},
	{58, "Boss Eraser", "", "Unlocked all Archfoe bosses.", false},
	{59, "Boss Executioner", "", "Unlocked all bosses.", false},
	{60, "Boss Hunter", "", "Unlocked 10 or more Bane bosses.", true},
	{61, "Boss Obliterator", "", "Unlocked 40 or more Nemesis bosses.", true},
	{62, "Boss Slayer", "", "Unlocked all Bane bosses.", false},
	{63, "Boss Smiter", "", "Unlocked 40 or more Archfoe bosses.", true},
	{64, "Boss Veteran", "", "Unlocked 40 or more Bane bosses.", true},

	// Daily reward titles (DAILY_REWARD)
	{65, "Creature of Habit (Grade 1)", "", "Reward Streak of at least 7 days of consecutive logins.", true},
	{66, "Creature of Habit (Grade 2)", "", "Reward Streak of at least 30 days of consecutive logins.", true},
	{67, "Creature of Habit (Grade 3)", "", "Reward Streak of at least 90 days of consecutive logins.", true},
	{68, "Creature of Habit (Grade 4)", "", "Reward Streak of at least 180 days of consecutive logins.", true},
	{69, "Creature of Habit (Grade 5)", "", "Reward Streak of at least 365 days of consecutive logins.", true},

	// Task titles (TASK)
	{70, "Aspiring Huntsman", "Aspiring Huntswoman", "Invested 160,000 tasks points.", true},
	{71, "Competent Beastslayer", "", "Invested 320,000 tasks points.", true},
	{72, "Feared Bountyhunter", "", "Invested 430,000 tasks points.", true},

	// Map exploration titles (MAP)
	{73, "Dedicated Entrepreneur", "", "Explored 50% of all the map areas.", false},
	{74, "Globetrotter", "", "Explored all map areas.", false},

	// Other titles (OTHERS)
	{75, "Guild Leader", "", "Leading a Guild.", false},
	{76, "Proconsul of Iksupan", "", "Only a true devotee to the cause of the ancient Iks and their lost legacy may step up to the rank of proconsul.", true},
	{77, "Admirer of the Crown", "", "Adjust your crown and handle it.", true},
	{78, "Big Spender", "", "Unlocked the full Golden Outfit.", true},
	{79, "Challenger of the Iks", "", "Challenged Ahau, guardian of Iksupan, in traditional Iks warrior attire.", true},
	{80, "Royal Bounacean Advisor", "", "Called to the court of Bounac by Kesar the Younger himself.", true},
	{81, "Aeternal", "", "Awarded exclusively to stalwart heroes keeping the faith under all circumstances.", true},
	{82, "Robinson Crusoe", "", "Some discoveries are reserved to only the most experienced adventurers. Until the next frontier opens on the horizon.", true},
	{83, "Chompmeister", "", "Awarded only to true connoisseurs undertaking even the most exotic culinary escapades.", true},
	{84, "Bringer of Rain", "", "Forging through battle after battle like a true gladiator.", true},
	{85, "Beastly", "", "Reached 2000 charm points. Quite beastly!", true},
	{86, "Midnight Hunter", "", "When the hunter becomes the hunted, perseverance decides the game.", true},
	{87, "Ratinator", "", "Killing some snarky cave rats is helpful, killing over ten thousand of them is a statement.", true},
	{88, "Doomsday Nemesis", "", "Awarded for great help in the battle against Gaz'haragoth.", true},
	{89, "Hero of Bounac", "", "You prevailed during the battle of Bounac and broke the siege that held Bounac's people in its firm grasp.", true},
	{90, "King of Demon", "Queen of Demon", "Defeat Morshabaal 5 times.", true},
	{91, "Planegazer", "", "Followed the trail of the Planestrider to the end.", true},
	{92, "Time Traveller", "", "Anywhere in time or space.", true},
	{93, "Truly Boss", "", "Reach 15,000 boss points.", true},
}

// PlayerTitles tracks which titles a player has unlocked.
// Mirrors C++ PlayerTitle (player_title.hpp / player_title.cpp).
type PlayerTitles struct {
	Unlocked  map[uint8]bool
	CurrentID uint8
}

// IsUnlocked returns true if the title with the given id has been unlocked.
func (pt *PlayerTitles) IsUnlocked(id uint8) bool {
	if pt == nil || pt.Unlocked == nil {
		return false
	}
	return pt.Unlocked[id]
}

// Unlock marks a title as unlocked for this player.
func (pt *PlayerTitles) Unlock(id uint8) {
	if pt.Unlocked == nil {
		pt.Unlocked = make(map[uint8]bool)
	}
	pt.Unlocked[id] = true
}
