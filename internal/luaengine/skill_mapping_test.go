package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// skills_t is the same numbering on both sides, so the Lua value and the Go
// constant have to be the same number. They were off by one in the mapper, which
// is how training with a bow raised the axe skill.
func TestLuaSkillEnumsMatchGoConstants(t *testing.T) {
	want := map[string]game.Skill{
		"SKILL_FIST":                game.SkillFist,
		"SKILL_CLUB":                game.SkillClub,
		"SKILL_SWORD":               game.SkillSword,
		"SKILL_AXE":                 game.SkillAxe,
		"SKILL_DISTANCE":            game.SkillDistance,
		"SKILL_SHIELD":              game.SkillShielding,
		"SKILL_FISHING":             game.SkillFishing,
		"SKILL_CRITICAL_HIT_CHANCE": game.SkillCriticalHitChance,
		"SKILL_CRITICAL_HIT_DAMAGE": game.SkillCriticalHitDamage,
		"SKILL_LIFE_LEECH_CHANCE":   game.SkillLifeLeechChance,
		"SKILL_LIFE_LEECH_AMOUNT":   game.SkillLifeLeechAmount,
		"SKILL_MANA_LEECH_CHANCE":   game.SkillManaLeechChance,
		"SKILL_MANA_LEECH_AMOUNT":   game.SkillManaLeechAmount,
	}

	for name, goSkill := range want {
		luaValue, ok := luaEnumValue(name)
		if !ok {
			t.Errorf("%s is not registered as a Lua enum", name)
			continue
		}
		if luaValue != int(goSkill) {
			t.Errorf("%s = %d in Lua but %d in Go", name, luaValue, goSkill)
			continue
		}
		got, isMagic, ok := mapLuaSkillToGo(luaValue)
		if !ok {
			t.Errorf("%s (%d) was rejected by mapLuaSkillToGo", name, luaValue)
			continue
		}
		if isMagic {
			t.Errorf("%s must not route to the magic-level branch", name)
			continue
		}
		if got != goSkill {
			t.Errorf("%s (%d) mapped to %d, want %d — this is the bow-trains-axe bug",
				name, luaValue, got, goSkill)
		}
	}
}

// SKILL_MAGLEVEL is 13 and advances on mana spent, not skill tries. SKILL_LEVEL
// (14) is not a trainable skill at all; the mapper tested for it by mistake.
func TestMagicLevelAndPlayerLevelRoute(t *testing.T) {
	mag, ok := luaEnumValue("SKILL_MAGLEVEL")
	if !ok {
		t.Fatal("SKILL_MAGLEVEL is not registered")
	}
	if _, isMagic, ok := mapLuaSkillToGo(mag); !ok || !isMagic {
		t.Errorf("SKILL_MAGLEVEL (%d) must route to the mana-spent branch, got ok=%v magic=%v",
			mag, ok, isMagic)
	}

	lvl, _ := luaEnumValue("SKILL_LEVEL")
	if _, _, ok := mapLuaSkillToGo(lvl); ok {
		t.Errorf("SKILL_LEVEL (%d) is not a trainable skill and must be rejected", lvl)
	}
	if _, _, ok := mapLuaSkillToGo(-1); ok {
		t.Errorf("SKILL_NONE must be rejected")
	}
}

// luaEnumValue reads a registered global back out of a live engine, so the tests
// check what scripts actually see rather than a second copy of the table.
var skillEnumEngine *Engine

func luaEnumValue(name string) (int, bool) {
	if skillEnumEngine == nil {
		skillEnumEngine = New(game.NewWorld(), nil)
	}
	n, ok := skillEnumEngine.L.GetGlobal(name).(lua.LNumber)
	if !ok {
		return 0, false
	}
	return int(n), true
}
