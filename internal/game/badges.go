package game

// BadgeInfo represents a cyclopedia badge (player title/achievement).
// Mirrors the C++ Badge struct from player_badge.hpp.
type BadgeInfo struct {
	ID   uint32
	Name string
}

// DefaultBadges is the registry of all badges known to the server, matching the
// C++ Game::m_badges initialisation in game.cpp (lines 555-582).  Badges are
// defined by their id, type, name and amount threshold; this simplified form
// keeps only the id and name used in the network protocol.
//
// Reference:
//
//	enum class CyclopediaBadge_t : uint8_t { ACCOUNT_AGE=1, LOYALTY,
//	ACCOUNT_ALL_LEVEL, ACCOUNT_ALL_VOCATIONS, TOURNAMENT_PARTICIPATION,
//	TOURNAMENT_POINTS };
var DefaultBadges = []BadgeInfo{
	// Account age badges (type ACCOUNT_AGE)
	{1, "Fledegeling Hero"},
	{2, "Veteran Hero"},
	{3, "Senior Hero"},
	{4, "Ancient Hero"},
	{5, "Exalted Hero"},

	// Loyalty badges (type LOYALTY)
	{6, "Tibia Loyalist (Grade 1)"},
	{7, "Tibia Loyalist (Grade 2)"},
	{8, "Tibia Loyalist (Grade 3)"},

	// Account all level badges (type ACCOUNT_ALL_LEVEL)
	{9, "Global Player (Grade 1)"},
	{10, "Global Player (Grade 2)"},
	{11, "Global Player (Grade 3)"},

	// Account all vocations badges (type ACCOUNT_ALL_VOCATIONS)
	{12, "Master Class (Grade 1)"},
	{13, "Master Class (Grade 2)"},
	{14, "Master Class (Grade 3)"},

	// Tournament participation badges
	{15, "Freshman of the Tournament"},
	{16, "Regular of the Tournament"},
	{17, "Hero of the Tournament"},

	// Tournament points badges
	{18, "Tournament Competitor"},
	{19, "Tournament Challenger"},
	{20, "Tournament Master"},
	{21, "Tournament Champion"},
}

// PlayerBadges tracks which badges a player has unlocked.
// Mirrors C++ PlayerBadge (player_badge.hpp / player_badge.cpp).
type PlayerBadges struct {
	Unlocked map[uint32]bool
}

// HasBadge returns true if the badge with the given id has been unlocked.
func (pb *PlayerBadges) HasBadge(id uint32) bool {
	if pb == nil || pb.Unlocked == nil {
		return false
	}
	return pb.Unlocked[id]
}

// UnlockBadge marks a badge as unlocked for this player.
func (pb *PlayerBadges) UnlockBadge(id uint32) {
	if pb.Unlocked == nil {
		pb.Unlocked = make(map[uint32]bool)
	}
	pb.Unlocked[id] = true
}
