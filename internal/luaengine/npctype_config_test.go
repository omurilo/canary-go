package luaengine

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
)

// Every one of the 1033 npc scripts in data-otservbr-global builds a npcConfig
// table in this shape and calls npcType:register(npcConfig). Before these fields
// were parsed, register() silently dropped walkInterval, walkRadius, description,
// flags, speechBubble and voices.
const sweatyCyclopsConfig = `
local npcType = Game.createNpcType("A Sweaty Cyclops")
local npcConfig = {}

npcConfig.name = "A Sweaty Cyclops"
npcConfig.description = "a sweaty cyclops"
npcConfig.health = 100
npcConfig.maxHealth = npcConfig.health
npcConfig.walkInterval = 2000
npcConfig.walkRadius = 2

npcConfig.outfit = { lookType = 22 }

npcConfig.flags = {
	floorchange = false,
}
npcConfig.speechBubble = SPEECHBUBBLE_TRADE

npcConfig.voices = {
	interval = 15000,
	chance = 50,
	{ text = "Hum hum, huhum" },
	{ text = "Silly lil' human", yell = true },
}

npcType:register(npcConfig)
`

func TestNpcConfigRegisterParsesDatapackFields(t *testing.T) {
	e := shimEngine(t)

	if err := e.L.DoString(sweatyCyclopsConfig); err != nil {
		t.Fatalf("register npcConfig: %v", err)
	}

	nt := e.world.TypeRegistry.Npcs["a sweaty cyclops"]
	if nt == nil {
		t.Fatal("npc type was not registered")
	}

	if nt.Description != "a sweaty cyclops" {
		t.Errorf("description: %q", nt.Description)
	}
	if nt.WalkInterval != 2000 {
		t.Errorf("walkInterval: %d want 2000", nt.WalkInterval)
	}
	if nt.WalkRadius != 2 {
		t.Errorf("walkRadius: %d want 2", nt.WalkRadius)
	}
	if nt.SpeechBubble != creatures.SpeechBubbleTrade {
		t.Errorf("speechBubble: %d want %d", nt.SpeechBubble, creatures.SpeechBubbleTrade)
	}
	if nt.FloorChange {
		t.Error("floorchange should be false")
	}
	if nt.CurrencyID != creatures.DefaultNpcCurrency {
		t.Errorf("currency should default to gold, got %d", nt.CurrencyID)
	}

	if nt.YellInterval != 15000 || nt.YellChance != 50 {
		t.Errorf("voices interval/chance: %d/%d want 15000/50", nt.YellInterval, nt.YellChance)
	}
	if len(nt.Voices) != 2 {
		t.Fatalf("expected 2 voices, got %d", len(nt.Voices))
	}
	if nt.Voices[0].Text != "Hum hum, huhum" || nt.Voices[0].Yell {
		t.Errorf("voice 0: %+v", nt.Voices[0])
	}
	if nt.Voices[1].Text != "Silly lil' human" || !nt.Voices[1].Yell {
		t.Errorf("voice 1: %+v", nt.Voices[1])
	}
}

// SPEECHBUBBLE_* is assigned in 1032 places across the datapack. An undefined Lua
// global is nil rather than an error, so a missing constant is silent data loss.
func TestSpeechBubbleEnumsDefined(t *testing.T) {
	w := game.NewWorld()
	e := New(w, nil)
	defer e.Close()

	// SPEECHBUBBLE_BANKER deliberately aliases TRADE upstream.
	script := `
		assert(SPEECHBUBBLE_NONE == 0, "NONE")
		assert(SPEECHBUBBLE_NORMAL == 1, "NORMAL")
		assert(SPEECHBUBBLE_TRADE == 2, "TRADE")
		assert(SPEECHBUBBLE_QUEST == 3, "QUEST")
		assert(SPEECHBUBBLE_QUESTTRADER == 4, "QUESTTRADER")
		assert(SPEECHBUBBLE_SAILOR == 5, "SAILOR")
		assert(SPEECHBUBBLE_BANKER == 2, "BANKER aliases TRADE")
		assert(SPEECHBUBBLE_HIRELING == 7, "HIRELING")
	`
	if err := e.L.DoString(script); err != nil {
		t.Fatalf("speech bubble enums: %v", err)
	}
}

// A config with no speechBubble must fall back to NORMAL, as NpcInfo does.
func TestNpcConfigSpeechBubbleDefault(t *testing.T) {
	w := game.NewWorld()
	e := New(w, nil)
	defer e.Close()
	loadNpcRegisterShim(t, e)

	script := `
		local npcType = Game.createNpcType("Plain Guy")
		npcType:register({ name = "Plain Guy", health = 100, maxHealth = 100 })
	`
	if err := e.L.DoString(script); err != nil {
		t.Fatalf("register: %v", err)
	}
	nt := w.TypeRegistry.Npcs["plain guy"]
	if nt == nil {
		t.Fatal("npc type not registered")
	}
	if nt.SpeechBubble != creatures.SpeechBubbleNormal {
		t.Errorf("speechBubble: %d want %d", nt.SpeechBubble, creatures.SpeechBubbleNormal)
	}
}
