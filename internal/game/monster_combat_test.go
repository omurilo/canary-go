package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
)

// ratType builds a rat-like MonsterType for combat/loot/xp tests.
func ratType() *creatures.MonsterType {
	return &creatures.MonsterType{
		Name:       "Rat",
		MaxHealth:  20,
		Speed:      67,
		Experience: 5,
		Corpse:     5964,
		Attacks: []creatures.MonsterAttack{
			{Name: "melee", Interval: 2000, Chance: 100, MinDamage: 0, MaxDamage: -8},
		},
		Loot: []creatures.LootBlock{
			{ID: 3031, Chance: 100000, CountMin: 1, CountMax: 4}, // gold coin, always
			{ID: 3607, Chance: 100000, CountMin: 1, CountMax: 1}, // cheese, forced for test
		},
		Flags: creatures.MonsterFlags{Hostile: true, Attackable: true, LootDrop: true},
	}
}

// TestMonsterMeleeDamage checks a monster's melee damage stays within its
// parsed min/max range (rat: 0..8).
func TestMonsterMeleeDamage(t *testing.T) {
	e := NewCombatEngine(NewWorld())
	m := NewMonster(1, "Rat", ratType())

	for i := 0; i < 500; i++ {
		dmg := e.meleeDamage(m)
		if dmg < 0 || dmg > 8 {
			t.Fatalf("melee damage %d out of range [0,8]", dmg)
		}
	}
}

// TestMonsterAttackInterval verifies the interval comes from the melee block.
func TestMonsterAttackInterval(t *testing.T) {
	m := NewMonster(1, "Rat", ratType())
	if got := m.AttackInterval().Milliseconds(); got != 2000 {
		t.Errorf("AttackInterval = %dms, want 2000", got)
	}
	// No type -> default.
	if got := NewMonster(2, "X", nil).AttackInterval(); got != defaultMonsterAttackSpeed {
		t.Errorf("default AttackInterval = %v, want %v", got, defaultMonsterAttackSpeed)
	}
}

// TestDeathAwardsExperienceAndLoot verifies handleDeath grants the monster's
// experience to the player killer and fills the corpse with loot.
func TestDeathAwardsExperienceAndLoot(t *testing.T) {
	w := NewWorld()
	pos := Position{X: 100, Y: 100, Z: 7}
	w.Map.SetTile(pos, &Tile{Ground: &Item{ID: 1}})

	var statsPushed int
	w.OnPlayerStatsChange = func(*Player) { statsPushed++ }

	mt := ratType()
	mt.Experience = 100 // enough for level 1 -> level 2 (ExpForLevel(2) == 100)
	monster := NewMonster(42, "Rat", mt)
	monster.SetPosition(pos)
	monster.SetHealth(0)
	w.AddCreature(monster)

	player := &Player{Level: 1, MaxHealth: 150, MaxMana: 30}
	player.ID = 0x10000001

	var droppedFor, postDroppedFor []*Item
	w.OnMonsterDropLoot = func(_ *Monster, corpse *Item) { droppedFor = append(droppedFor, corpse) }
	w.OnMonsterPostDropLoot = func(_ *Monster, corpse *Item) { postDroppedFor = append(postDroppedFor, corpse) }

	e := NewCombatEngine(w)
	e.handleDeath(monster, player)

	if player.Experience != 100 {
		t.Errorf("Experience = %d, want 100", player.Experience)
	}
	if player.Level != 2 {
		t.Errorf("Level = %d, want 2 after level-up", player.Level)
	}
	if statsPushed != 1 {
		t.Errorf("stats hook fired %d times, want 1", statsPushed)
	}

	tile := w.Map.GetTile(pos)
	if tile == nil || len(tile.Items) != 1 {
		t.Fatalf("expected corpse on tile, got %+v", tile)
	}
	corpse := tile.Items[0]
	if corpse.ID != 5964 {
		t.Errorf("corpse id = %d, want 5964", corpse.ID)
	}
	// The core no longer rolls loot: Monster::dropLoot delegates it to the
	// monsterOnDropLoot event, whose datapack script fills the corpse. What the core
	// owes is the corpse itself plus the two callbacks, in order.
	if len(droppedFor) != 1 || droppedFor[0] != corpse {
		t.Errorf("OnMonsterDropLoot should have been called once with the corpse, got %d call(s)", len(droppedFor))
	}
	if len(postDroppedFor) != 1 || postDroppedFor[0] != corpse {
		t.Errorf("OnMonsterPostDropLoot should have been called once with the corpse, got %d call(s)", len(postDroppedFor))
	}
	// Nothing is in the corpse here because no event engine is wired in this unit
	// test; internal/luaengine/loot_flow_test.go drives the real chain.
	if len(corpse.Contents) != 0 {
		t.Errorf("the core must not put loot in the corpse itself, got %+v", corpse.Contents)
	}
}

// TestNoLootWhenLootDropDisabled verifies the lootDrop flag gates loot.
func TestNoLootWhenLootDropDisabled(t *testing.T) {
	w := NewWorld()
	pos := Position{X: 100, Y: 100, Z: 7}
	w.Map.SetTile(pos, &Tile{Ground: &Item{ID: 1}})

	mt := ratType()
	mt.Flags.LootDrop = false
	monster := NewMonster(42, "Rat", mt)
	monster.SetPosition(pos)
	monster.SetHealth(0)
	w.AddCreature(monster)

	// With lootDrop off, Monster::dropLoot never reaches the callbacks at all, so
	// assert on that rather than only on an empty corpse.
	var dropCalls int
	w.OnMonsterDropLoot = func(*Monster, *Item) { dropCalls++ }

	e := NewCombatEngine(w)
	e.handleDeath(monster, nil)

	corpse := w.Map.GetTile(pos).Items[0]
	if len(corpse.Contents) != 0 {
		t.Errorf("expected empty corpse with lootDrop=false, got %+v", corpse.Contents)
	}
	if dropCalls != 0 {
		t.Errorf("OnMonsterDropLoot fired %d time(s) with lootDrop disabled, want 0", dropCalls)
	}
}

// TestDeathAwardsExperienceToSummonMaster verifies that killing a monster via a summon
// grants experience to the player master and triggers text messages.
func TestDeathAwardsExperienceToSummonMaster(t *testing.T) {
	w := NewWorld()
	pos := Position{X: 100, Y: 100, Z: 7}
	w.Map.SetTile(pos, &Tile{Ground: &Item{ID: 1}})

	var textMsgPushed int
	w.OnTextMessage = func(p *Player, class uint8, value uint64, text string) {
		if class == 26 && value == 100 {
			textMsgPushed++
		}
	}

	mt := ratType()
	mt.Experience = 100
	monster := NewMonster(42, "Rat", mt)
	monster.SetPosition(pos)
	monster.SetHealth(0)

	player := &Player{Level: 1, MaxHealth: 150, MaxMana: 30}
	player.ID = 0x10000001

	summon := NewMonster(43, "Summon", nil)
	summon.Master = player

	e := NewCombatEngine(w)
	e.handleDeath(monster, summon)

	if player.Experience != 100 {
		t.Errorf("Experience = %d, want 100", player.Experience)
	}
	if textMsgPushed != 1 {
		t.Errorf("OnTextMessage hook fired %d times, want 1", textMsgPushed)
	}
}

func TestGetLevelPercent(t *testing.T) {
	p := &Player{Level: 1, Experience: 50}
	// Level 1: 0 exp -> Level 2: 100 exp. 50 exp = 50% = 5000/10000
	if pct := p.GetLevelPercent(); pct != 5000 {
		t.Errorf("GetLevelPercent() = %d, want 5000", pct)
	}

	p.Level = 100
	p.Experience = ExpForLevel(100) // exact minimum for level 100
	if pct := p.GetLevelPercent(); pct != 0 {
		t.Errorf("GetLevelPercent() at exact min level exp = %d, want 0", pct)
	}

	// Mid-way to level 101
	minExp := ExpForLevel(100)
	nextExp := ExpForLevel(101)
	p.Experience = minExp + (nextExp-minExp)/2
	if pct := p.GetLevelPercent(); pct != 5000 {
		t.Errorf("GetLevelPercent() halfway = %d, want 5000", pct)
	}
}

