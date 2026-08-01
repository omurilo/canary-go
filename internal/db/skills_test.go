package db

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/omurilo/canary-go/internal/game"
)

// skillColumnOrder is the array IOLoginDataLoad::loadPlayerSkill iterates
// (iologindata_load_player.cpp:354). Position i is skills[i], so the Skill enum
// and this list have to agree or every skill is loaded into the wrong slot.
var skillColumnOrder = []struct {
	skill  game.Skill
	column string
}{
	{game.SkillFist, "skill_fist"},
	{game.SkillClub, "skill_club"},
	{game.SkillSword, "skill_sword"},
	{game.SkillAxe, "skill_axe"},
	{game.SkillDistance, "skill_dist"},
	{game.SkillShielding, "skill_shielding"},
	{game.SkillFishing, "skill_fishing"},
	{game.SkillCriticalHitChance, "skill_critical_hit_chance"},
	{game.SkillCriticalHitDamage, "skill_critical_hit_damage"},
	{game.SkillLifeLeechChance, "skill_life_leech_chance"},
	{game.SkillLifeLeechAmount, "skill_life_leech_amount"},
	{game.SkillManaLeechChance, "skill_mana_leech_chance"},
	{game.SkillManaLeechAmount, "skill_mana_leech_amount"},
}

func TestSkillEnumMatchesUpstreamOrder(t *testing.T) {
	if len(skillColumnOrder) != int(game.SkillCount) {
		t.Fatalf("SkillCount is %d but upstream has %d skills",
			game.SkillCount, len(skillColumnOrder))
	}
	for i, entry := range skillColumnOrder {
		if int(entry.skill) != i {
			t.Errorf("%s should be index %d, got %d", entry.column, i, entry.skill)
		}
	}
}

// Both statements must mention every skill column and its _tries companion,
// otherwise a skill silently stops persisting — which is what happened to the six
// special skills before this change.
func TestPlayerStatementsCoverEverySkillColumn(t *testing.T) {
	source := readPlayerSource(t)

	selectStmt := extractBetween(t, source, "SELECT p.id", "FROM players p JOIN accounts")
	updateStmt := extractBetween(t, source, "UPDATE players SET", "WHERE id=?")

	for _, entry := range skillColumnOrder {
		for _, col := range []string{entry.column, entry.column + "_tries"} {
			if !strings.Contains(selectStmt, "p."+col) {
				t.Errorf("SELECT is missing %s", col)
			}
			if !strings.Contains(updateStmt, col+"=?") {
				t.Errorf("UPDATE is missing %s", col)
			}
		}
	}
}

// A placeholder/argument mismatch is not a compile error, it is a runtime failure
// on every save, so it is worth asserting.
func TestUpdatePlaceholderCountMatchesArgs(t *testing.T) {
	source := readPlayerSource(t)
	updateStmt := extractBetween(t, source, "UPDATE players SET", "WHERE id=?")

	// +1 for the WHERE id=? placeholder.
	placeholders := strings.Count(updateStmt, "?") + 1

	callRe := regexp.MustCompile(`(?s)ExecContext\(ctx, q,(.*?)\n\t\)`)
	m := callRe.FindStringSubmatch(source[strings.Index(source, "UPDATE players SET"):])
	if m == nil {
		t.Fatal("could not find the SavePlayer ExecContext call")
	}
	if got := countTopLevelArgs(m[1]); got != placeholders {
		t.Errorf("UPDATE has %d placeholders but %d arguments", placeholders, got)
	}
}

func countTopLevelArgs(body string) int {
	depth, args := 0, 1
	for _, ch := range body {
		switch ch {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				args++
			}
		}
	}
	if strings.HasSuffix(strings.TrimSpace(body), ",") {
		args--
	}
	return args
}

func extractBetween(t *testing.T, s, start, end string) string {
	t.Helper()
	i := strings.Index(s, start)
	if i < 0 {
		t.Fatalf("could not find %q", start)
	}
	rest := s[i:]
	j := strings.Index(rest, end)
	if j < 0 {
		t.Fatalf("could not find %q after %q", end, start)
	}
	return rest[:j]
}

// readPlayerSource returns the source of player.go so the SQL text can be asserted.
func readPlayerSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("player.go")
	if err != nil {
		t.Fatalf("read player.go: %v", err)
	}
	return string(data)
}
