package game

// HighscoreEntry represents a single highscore entry.
type HighscoreEntry struct {
	Rank     uint16
	Name     string
	Level    uint16
	Vocation uint8
	Value    uint32 // experience or skill value depending on category
	TownID   uint32
}

// HighscoreCategory represents a highscore category.
type HighscoreCategory struct {
	Name string
	ID   uint8
}

// HighscoreType for the request.
type HighscoreType uint8

const (
	HighscoreGetEntries    HighscoreType = 0
	HighscoreGetCategories HighscoreType = 1
)

// DefaultHighscoreCategories are the standard ranking categories shown by the
// client. The order and IDs must match C++ protocolgame.cpp.
var DefaultHighscoreCategories = []HighscoreCategory{
	{"Experience", 0},
	{"Fist Fighting", 1},
	{"Club Fighting", 2},
	{"Sword Fighting", 3},
	{"Axe Fighting", 4},
	{"Distance Fighting", 5},
	{"Shielding", 6},
	{"Fishing", 7},
	{"Magic Level", 8},
	{"Loyalty", 9},
}
