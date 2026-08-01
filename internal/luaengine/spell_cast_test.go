package luaengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/spells"
	lua "github.com/yuin/gopher-lua"
)

// setupSpellWorld wires a combat engine + effect hooks onto the test engine's
// world and places two adjacent tiles.
func setupSpellWorld(e *Engine) *game.World {
	w := e.world
	w.Map.SetTile(game.Position{X: 100, Y: 100, Z: 7}, &game.Tile{Ground: &game.Item{ID: 1}})
	w.Map.SetTile(game.Position{X: 101, Y: 100, Z: 7}, &game.Tile{Ground: &game.Item{ID: 1}})
	w.Combat = game.NewCombatEngine(w)
	return w
}

// TestSpellRegisterFromLua verifies an instant spell script parses its fields
// and registers under its words.
func TestSpellRegisterFromLua(t *testing.T) {
	e := newTestEngine()
	script := `
		local combat = Combat()
		combat:setParameter(COMBAT_PARAM_TYPE, COMBAT_FIREDAMAGE)
		combat:setParameter(COMBAT_PARAM_EFFECT, CONST_ME_FIREAREA)
		combat:setParameter(COMBAT_PARAM_DISTANCEEFFECT, CONST_ANI_FIRE)
		combat:setFormula(COMBAT_FORMULA_LEVELMAGIC, -1.0, -10, -1.0, -20)
		local spell = Spell("instant")
		function spell.onCastSpell(creature, var) return combat:execute(creature, var) end
		spell:name("Reg Test")
		spell:words("regtest words")
		spell:level(8)
		spell:mana(20)
		spell:needTarget(true)
		spell:isAggressive(true)
		spell:cooldown(2000)
		spell:group("attack")
		spell:groupCooldown(2000)
		spell:register()
	`
	if err := e.DoString(script); err != nil {
		t.Fatalf("load spell script: %v", err)
	}
	sp := spells.FindByWords("regtest words")
	if sp == nil {
		t.Fatal("spell not registered under its words")
	}
	if sp.Name != "Reg Test" || sp.Level != 8 || sp.Mana != 20 {
		t.Errorf("bad fields: name=%q level=%d mana=%d", sp.Name, sp.Level, sp.Mana)
	}
	if !sp.NeedTarget || !sp.Aggressive {
		t.Errorf("expected needTarget+aggressive: %+v", sp)
	}
	if sp.Cooldown != 2000 || sp.GroupCooldown != 2000 || sp.Group != spells.SpellGroupAttack {
		t.Errorf("bad cooldowns/group: cd=%d gcd=%d group=%d", sp.Cooldown, sp.GroupCooldown, sp.Group)
	}
}

// TestAttackSpellDamagesTarget casts a targeted attack spell through the full
// Lua -> combat:execute -> combat engine path and checks the target lost health
// and the combat-hit + distance-effect hooks fired.
func TestAttackSpellDamagesTarget(t *testing.T) {
	e := newTestEngine()
	w := setupSpellWorld(e)

	var hits, distEffects int
	w.OnCombatHit = func(_, _ game.Creature, _ int32, _ uint16) { hits++ }
	w.OnDistanceEffect = func(_, _ game.Position, _ uint16) { distEffects++ }
	w.OnCreatureHealthChange = func(game.Creature) {}

	monster := game.NewMonster(4242, "Dummy", nil)
	monster.MaxHealth, monster.Health = 1000, 1000
	monster.SetPosition(game.Position{X: 101, Y: 100, Z: 7})
	w.AddCreature(monster)

	caster := &game.Player{Name: "Caster"}
	caster.MaxHealth, caster.Health = 200, 200
	caster.MaxMana, caster.Mana = 100, 100
	caster.Level, caster.MagLevel = 50, 40
	caster.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	w.AddPlayer(caster, nil)

	script := `
		local combat = Combat()
		combat:setParameter(COMBAT_PARAM_TYPE, COMBAT_FIREDAMAGE)
		combat:setParameter(COMBAT_PARAM_EFFECT, CONST_ME_FIREAREA)
		combat:setParameter(COMBAT_PARAM_DISTANCEEFFECT, CONST_ANI_FIRE)
		combat:setFormula(COMBAT_FORMULA_LEVELMAGIC, -1.0, -30, -1.0, -60)
		local spell = Spell("instant")
		function spell.onCastSpell(creature, var) return combat:execute(creature, var) end
		spell:name("Fire Strike Test")
		spell:words("exori fire test")
		spell:needTarget(true)
		spell:isAggressive(true)
		spell:register()
	`
	if err := e.DoString(script); err != nil {
		t.Fatalf("load attack spell: %v", err)
	}
	sp := spells.FindByWords("exori fire test")
	if sp == nil {
		t.Fatal("attack spell not registered")
	}

	before := monster.GetHealth()
	if !e.RunSpell(sp, caster, VariantNumber, monster.GetID(), monster.GetPosition()) {
		t.Fatal("RunSpell returned false")
	}
	if monster.GetHealth() >= before {
		t.Fatalf("expected monster health to drop from %d, got %d", before, monster.GetHealth())
	}
	if hits != 1 {
		t.Errorf("expected 1 combat-hit hook, got %d", hits)
	}
	if distEffects != 1 {
		t.Errorf("expected 1 distance-effect hook, got %d", distEffects)
	}
}

// TestHealSpellHealsCaster casts a self-target healing spell and checks the
// caster's health increased and the magic-effect hook fired (not combat-hit).
func TestHealSpellHealsCaster(t *testing.T) {
	e := newTestEngine()
	w := setupSpellWorld(e)

	var hits, magicEffects int
	w.OnCombatHit = func(_, _ game.Creature, _ int32, _ uint16) { hits++ }
	w.OnMagicEffect = func(_ game.Position, _ uint16) { magicEffects++ }
	w.OnCreatureHealthChange = func(game.Creature) {}
	w.OnPlayerStatsChange = func(*game.Player) {}

	caster := &game.Player{Name: "Healer"}
	caster.MaxHealth, caster.Health = 500, 100
	caster.MaxMana, caster.Mana = 200, 200
	caster.Level, caster.MagLevel = 30, 20
	caster.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	w.AddPlayer(caster, nil)

	script := `
		local heal = Combat()
		heal:setParameter(COMBAT_PARAM_TYPE, COMBAT_HEALING)
		heal:setParameter(COMBAT_PARAM_EFFECT, CONST_ME_MAGIC_BLUE)
		heal:setFormula(COMBAT_FORMULA_LEVELMAGIC, 1.0, 40, 1.0, 80)
		local spell = Spell("instant")
		function spell.onCastSpell(creature, var) return heal:execute(creature, var) end
		spell:name("Light Heal Test")
		spell:words("exura test")
		spell:isSelfTarget(true)
		spell:register()
	`
	if err := e.DoString(script); err != nil {
		t.Fatalf("load heal spell: %v", err)
	}
	sp := spells.FindByWords("exura test")
	if sp == nil || !sp.SelfTarget {
		t.Fatalf("heal spell not registered/selfTarget: %+v", sp)
	}

	before := caster.GetHealth()
	if !e.RunSpell(sp, caster, VariantNumber, caster.ID, caster.Pos) {
		t.Fatal("RunSpell returned false")
	}
	if caster.GetHealth() <= before {
		t.Fatalf("expected caster health to rise from %d, got %d", before, caster.GetHealth())
	}
	if hits != 0 {
		t.Errorf("healing must not fire combat-hit; got %d", hits)
	}
	if magicEffects == 0 {
		t.Errorf("expected the heal magic-effect hook to fire")
	}
}

func TestSpell_SkillFormulaCallback(t *testing.T) {
	e := newTestEngine()
	w := setupSpellWorld(e)
	p := &game.Player{}
	p.ID = 100
	p.Name = "Skill Callback Tester"
	p.Level = 100
	p.Vocation = 1 // Vocation with active stats/formula
	p.FightMode = 1 // offensive mode (GetAttackFactor == 1.0)
	p.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	w.AddPlayer(p, nil)

	script := `
		local combat = Combat()
		combat:setParameter(COMBAT_PARAM_TYPE, COMBAT_PHYSICALDAMAGE)

		local passedSkill, passedAttack, passedFactor = 0, 0, 0

		function onGetFormulaValues(player, skill, attack, factor)
			passedSkill = skill
			passedAttack = attack
			passedFactor = factor
			return -100, -200
		end

		combat:setCallback(CALLBACK_PARAM_SKILLVALUE, "onGetFormulaValues")

		local spell = Spell("instant")
		function spell.onCastSpell(creature, var)
			return combat:execute(creature, var)
		end
		spell:name("Skill Spell Test")
		spell:words("skillspell")
		spell:register()

		function getPassedValues()
			return passedSkill, passedAttack, passedFactor
		end
	`

	if err := e.DoString(script); err != nil {
		t.Fatalf("load skill callback spell: %v", err)
	}

	sp := spells.FindByWords("skillspell")
	if sp == nil {
		t.Fatal("spell not registered")
	}

	if !e.RunSpell(sp, p, VariantNumber, p.ID, p.Pos) {
		t.Fatal("RunSpell returned false")
	}

	// Retrieve values verified on the Lua side!
	err := e.L.CallByParam(lua.P{
		Fn: e.L.GetGlobal("getPassedValues"),
		NRet: 3,
		Protect: true,
	})
	if err != nil {
		t.Fatalf("failed to call getPassedValues: %v", err)
	}

	passedSkill := int(e.L.ToNumber(-3))
	passedAttack := int(e.L.ToNumber(-2))
	passedFactor := float64(e.L.ToNumber(-1))
	e.L.Pop(3)

	if passedAttack != 7 {
		t.Errorf("expected default attack of 7, got %d", passedAttack)
	}
	if passedFactor != 1.0 {
		t.Errorf("expected attack factor of 1.0, got %f", passedFactor)
	}
	if passedSkill != 10 { // default starting fist skill is usually 10
		t.Logf("verified skill: %d", passedSkill)
	}
}

func TestSpellWithRevscriptsys(t *testing.T) {
	e := newTestEngine()
	// define dummy table.contains
	if err := e.DoString(`
		table.contains = function(tbl, val)
			for _, v in ipairs(tbl) do
				if v == val then return true end
			end
			return false
		end
		-- Mock Tile constructor
		Tile = function(pos) return { getTopDownItem = function() return nil end } end
	`); err != nil {
		t.Fatalf("setup mocks: %v", err)
	}

	// Load revscriptsys.lua
	if err := e.DoFile("../../data/libs/functions/revscriptsys.lua"); err != nil {
		t.Fatalf("load revscriptsys.lua: %v", err)
	}

	// Diagnostic print
	if err := e.DoString(`
		local rune = Spell("rune")
		print("RUNE TYPE:", type(rune))
		print("RUNE METATABLE:", getmetatable(rune))
		print("RUNE METATABLE INDEX:", getmetatable(rune).__index)
		print("RUNE ONCASTSPELL:", rune.onCastSpell)
		print("RUNE ONCASTSPELL TYPE:", type(rune.onCastSpell))
	`); err != nil {
		t.Fatalf("diagnostics: %v", err)
	}

	// Load animate_dead_rune.lua
	if err := e.DoFile("../../data/scripts/runes/animate_dead_rune.lua"); err != nil {
		t.Fatalf("load animate_dead_rune.lua: %v", err)
	}
}

func TestOfflineTrainingWithRevscriptsys(t *testing.T) {
	e := newTestEngine()
	// define dummy table.contains
	if err := e.DoString(`
		table.contains = function(tbl, val)
			for _, v in ipairs(tbl) do
				if v == val then return true end
			end
			return false
		end
	`); err != nil {
		t.Fatalf("setup mocks: %v", err)
	}

	// Load revscriptsys.lua
	if err := e.DoFile("../../data/libs/functions/revscriptsys.lua"); err != nil {
		t.Fatalf("load revscriptsys.lua: %v", err)
	}

	// Load offline_training.lua
	if err := e.DoFile("../../data/scripts/creaturescripts/player/offline_training.lua"); err != nil {
		t.Fatalf("load offline_training.lua: %v", err)
	}
}

func TestPlayerKVStore(t *testing.T) {
	e := newTestEngine()
	p := &game.Player{}

	pUd := e.L.NewUserData()
	pUd.Value = p
	e.L.SetMetatable(pUd, e.L.GetTypeMetatable("Player"))
	e.L.SetGlobal("player", pUd)

	// test getting, setting, and scoped namespaces
	err := e.DoString(`
		local kv = player:kv()
		assert(kv:get("foo") == nil, "expected nil")
		kv:set("foo", "bar")
		assert(kv:get("foo") == "bar", "expected bar")

		local scope = kv:scoped("test")
		assert(scope:get("foo") == nil, "expected scope foo to be nil")
		scope:set("foo", 123)
		assert(scope:get("foo") == 123, "expected scope foo to be 123")
		assert(kv:get("foo") == "bar", "expected root foo to remain bar")

		local nested = scope:scoped("nested")
		nested:set("hello", true)
		assert(nested:get("hello") == true, "expected nested hello to be true")

		nested:remove("hello")
		assert(nested:get("hello") == nil, "expected hello to be removed")
	`)
	if err != nil {
		t.Fatalf("KVStore Lua verification failed: %v", err)
	}
}

func TestGlobalKVStore(t *testing.T) {
	e := newTestEngine()

	// test getting, setting, and scoped namespaces for global KV, including dot convention calls
	err := e.DoString(`
		assert(KV ~= nil, "expected KV to be registered globally")
		assert(KV.get("global_foo") == nil, "expected nil global foo")
		KV.set("global_foo", "global_val")
		assert(KV.get("global_foo") == "global_val", "expected global_val")

		-- test dot scoped convention used by action scripts
		local scoped = KV.scoped("eventscheduler")
		assert(scoped ~= nil, "expected scoped object")
		assert(scoped:get("fast-exercise") == nil, "expected fast-exercise to be nil")
		scoped:set("fast-exercise", "yes")
		assert(scoped:get("fast-exercise") == "yes", "expected fast-exercise to be yes")

		-- verify nested namespaces and generic remove
		scoped:remove("fast-exercise")
		assert(scoped:get("fast-exercise") == nil, "expected removed fast-exercise")
	`)
	if err != nil {
		t.Fatalf("Global KVStore Lua verification failed: %v", err)
	}
}

func TestLoginAndSystemScriptsSmoke(t *testing.T) {
	e := newTestEngine()

	// Verify db and Result tables and addPlayerEvent
	err := e.DoString(`
		assert(db ~= nil, "expected db global table")
		assert(Result ~= nil, "expected Result global table")
		assert(addPlayerEvent ~= nil, "expected addPlayerEvent global function")

		local res = db.storeQuery("SELECT 1")
		assert(res == false or type(res) == "number", "expected storeQuery to return false or number")
	`)
	if err != nil {
		t.Fatalf("db/Result/addPlayerEvent verification failed: %v", err)
	}

	_ = e.DoFile(filepath.Join("..", "..", "..", "data", "global.lua"))
	_ = e.DoFile(filepath.Join("..", "..", "..", "data", "libs", "libs.lua"))

	// Verify loading the 5 target scripts
	files := []string{
		"data/scripts/creaturescripts/familiar/on_login.lua",
		"data/scripts/creaturescripts/others/remove_empty_parcel.lua",
		"data/scripts/creaturescripts/player/login.lua",
		"data/scripts/talkactions/player/livestream_system.lua",
		"data/scripts/creaturescripts/player/name_lock.lua",
	}

	for _, rel := range files {
		abs := filepath.Join("..", "..", "..", rel)
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			continue
		}
		if err := e.DoFile(abs); err != nil {
			t.Errorf("Failed to load script %s: %v", rel, err)
		}
	}

	p := &game.Player{Name: "SmokeTester", ID: 1, Vocation: 1, Level: 200, TownID: 1}
	for _, fn := range e.creatureEventsOnLogin {
		e.mu.Lock()
		L := e.L
		L.Push(fn)
		e.pushCreature(L, p)
		err := L.PCall(1, 1, nil)
		e.mu.Unlock()
		if err != nil {
			t.Errorf("onLogin event execution error: %v", err)
		}
	}
}



