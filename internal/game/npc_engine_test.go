package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
)

func newTestNpc(nt *creatures.NpcType, pos Position) *Npc {
	n := NewNpc(1, "Test Npc", nt)
	n.SetPosition(pos)
	n.MasterPos = pos
	return n
}

// walkRadius 0 means "does not walk" in Npc::canWalkTo, not "walks anywhere".
// Getting this backwards would send every radius-0 NPC wandering off.
func TestRandomStepRadiusZeroDoesNotWalk(t *testing.T) {
	e := &NpcEngine{world: NewWorld()}
	npc := newTestNpc(&creatures.NpcType{WalkRadius: 0, WalkInterval: 1000}, Position{X: 100, Y: 100, Z: 7})

	if _, ok := e.randomStep(npc); ok {
		t.Error("an NPC with walkRadius 0 must not step")
	}
}

// isInSpawnRange bounds the walk to a square of walkRadius around MasterPos.
func TestIsInSpawnRange(t *testing.T) {
	npc := newTestNpc(&creatures.NpcType{WalkRadius: 2}, Position{X: 100, Y: 100, Z: 7})

	cases := []struct {
		pos  Position
		want bool
	}{
		{Position{X: 100, Y: 100, Z: 7}, true},
		{Position{X: 102, Y: 102, Z: 7}, true},  // on the boundary
		{Position{X: 98, Y: 98, Z: 7}, true},    // boundary, other side
		{Position{X: 103, Y: 100, Z: 7}, false}, // one past
		{Position{X: 100, Y: 103, Z: 7}, false},
		{Position{X: 100, Y: 100, Z: 6}, false}, // different floor
	}
	for _, c := range cases {
		if got := npc.isInSpawnRange(c.pos); got != c.want {
			t.Errorf("%v: got %v want %v", c.pos, got, c.want)
		}
	}
}

// A radius of 0 disables the range restriction itself (used by NPCs that are
// teleported around by scripts), matching the early return upstream.
func TestIsInSpawnRangeUnrestricted(t *testing.T) {
	npc := newTestNpc(&creatures.NpcType{WalkRadius: 0}, Position{X: 100, Y: 100, Z: 7})
	if !npc.isInSpawnRange(Position{X: 500, Y: 500, Z: 7}) {
		t.Error("radius 0 must not restrict range")
	}
}

// onThinkWalk resets the timer while a player is being talked to, so the NPC does
// not step the moment the conversation ends.
func TestThinkWalkPausedWhileInteracting(t *testing.T) {
	e := &NpcEngine{world: NewWorld()}
	nt := &creatures.NpcType{WalkRadius: 2, WalkInterval: 1000}
	npc := newTestNpc(nt, Position{X: 100, Y: 100, Z: 7})

	npc.walkTicks = 900
	npc.SetPlayerInteraction(42, 0)
	e.thinkWalk(npc, 500)

	if npc.walkTicks != 0 {
		t.Errorf("walkTicks should reset while interacting, got %d", npc.walkTicks)
	}
}

// The timer only fires once the accumulated interval is reached.
func TestThinkWalkAccumulatesInterval(t *testing.T) {
	e := &NpcEngine{world: NewWorld()}
	nt := &creatures.NpcType{WalkRadius: 2, WalkInterval: 1000}
	npc := newTestNpc(nt, Position{X: 100, Y: 100, Z: 7})

	e.thinkWalk(npc, 400)
	if npc.walkTicks != 400 {
		t.Fatalf("expected 400 ticks, got %d", npc.walkTicks)
	}
	e.thinkWalk(npc, 400)
	if npc.walkTicks != 800 {
		t.Fatalf("expected 800 ticks, got %d", npc.walkTicks)
	}
	// Crossing the interval resets the counter. There is no map here so the step
	// itself fails, but the timer must still have rolled over.
	e.thinkWalk(npc, 400)
	if npc.walkTicks != 0 {
		t.Errorf("expected the counter to reset at the interval, got %d", npc.walkTicks)
	}
}

// walkInterval 0 disables walking outright (onThinkWalk's first guard).
func TestThinkWalkDisabledByZeroInterval(t *testing.T) {
	e := &NpcEngine{world: NewWorld()}
	npc := newTestNpc(&creatures.NpcType{WalkRadius: 5, WalkInterval: 0}, Position{X: 100, Y: 100, Z: 7})

	e.thinkWalk(npc, 5000)
	if npc.walkTicks != 0 {
		t.Errorf("walkTicks must stay 0 when walking is disabled, got %d", npc.walkTicks)
	}
}

func TestThinkYellFiresAtInterval(t *testing.T) {
	var said []string
	e := &NpcEngine{
		world: NewWorld(),
		Say:   func(_ *Npc, _ byte, text string) { said = append(said, text) },
	}
	nt := &creatures.NpcType{
		YellInterval: 1000,
		YellChance:   100, // always
		Voices:       []creatures.NpcVoice{{Text: "hello"}},
	}
	npc := newTestNpc(nt, Position{X: 100, Y: 100, Z: 7})

	e.thinkYell(npc, 500)
	if len(said) != 0 {
		t.Fatalf("should not yell before the interval, got %v", said)
	}
	e.thinkYell(npc, 500)
	if len(said) != 1 || said[0] != "hello" {
		t.Fatalf("expected one yell of %q, got %v", "hello", said)
	}
	if npc.yellTicks != 0 {
		t.Errorf("yellTicks should reset after firing, got %d", npc.yellTicks)
	}
}

// chance 0 must never fire, and the tick counter still resets so it does not
// accumulate forever.
func TestThinkYellZeroChanceNeverSpeaks(t *testing.T) {
	var said []string
	e := &NpcEngine{
		world: NewWorld(),
		Say:   func(_ *Npc, _ byte, text string) { said = append(said, text) },
	}
	nt := &creatures.NpcType{
		YellInterval: 1000,
		YellChance:   0,
		Voices:       []creatures.NpcVoice{{Text: "hello"}},
	}
	npc := newTestNpc(nt, Position{X: 100, Y: 100, Z: 7})

	for i := 0; i < 20; i++ {
		e.thinkYell(npc, 1000)
	}
	if len(said) != 0 {
		t.Errorf("chance 0 must never yell, got %v", said)
	}
}

// A voice marked yell goes out as TALKTYPE_YELL, otherwise TALKTYPE_SAY.
func TestThinkYellTalkType(t *testing.T) {
	var types []byte
	e := &NpcEngine{
		world: NewWorld(),
		Say:   func(_ *Npc, talkType byte, _ string) { types = append(types, talkType) },
	}
	nt := &creatures.NpcType{
		YellInterval: 1000,
		YellChance:   100,
		Voices:       []creatures.NpcVoice{{Text: "shout", Yell: true}},
	}
	npc := newTestNpc(nt, Position{X: 100, Y: 100, Z: 7})

	e.thinkYell(npc, 1000)
	if len(types) != 1 || types[0] != talkTypeYell {
		t.Errorf("expected TALKTYPE_YELL (%d), got %v", talkTypeYell, types)
	}
}

func TestThinkYellDisabledWithoutVoices(t *testing.T) {
	var said []string
	e := &NpcEngine{
		world: NewWorld(),
		Say:   func(_ *Npc, _ byte, text string) { said = append(said, text) },
	}
	npc := newTestNpc(&creatures.NpcType{YellInterval: 1000, YellChance: 100}, Position{X: 1, Y: 1, Z: 7})

	e.thinkYell(npc, 5000)
	if len(said) != 0 {
		t.Errorf("no voices means no yelling, got %v", said)
	}
}

// SpeechBubble and CurrencyID fall back to upstream's defaults.
func TestNpcDefaults(t *testing.T) {
	npc := NewNpc(1, "Nameless", nil)
	if got := npc.SpeechBubble(); got != creatures.SpeechBubbleNormal {
		t.Errorf("speech bubble: got %d want %d", got, creatures.SpeechBubbleNormal)
	}
	if got := npc.CurrencyID(); got != creatures.DefaultNpcCurrency {
		t.Errorf("currency: got %d want %d", got, creatures.DefaultNpcCurrency)
	}
}

// Teleporting home ends every conversation, as Npc::onThink does via
// resetPlayerInteractions.
func TestResetPlayerInteractions(t *testing.T) {
	npc := newTestNpc(&creatures.NpcType{}, Position{X: 1, Y: 1, Z: 7})
	npc.SetPlayerInteraction(7, 0)
	npc.SetPlayerInteraction(8, 1)
	if !npc.IsInteractingWithPlayer(7) {
		t.Fatal("expected an interaction to be registered")
	}
	npc.resetPlayerInteractions()
	if npc.IsInteractingWithPlayer(7) || npc.IsInteractingWithPlayer(8) {
		t.Error("interactions should be cleared")
	}
}
