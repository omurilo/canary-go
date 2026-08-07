package game

import (
	"testing"
	"time"

	"github.com/omurilo/canary-go/internal/config"
	lua "github.com/yuin/gopher-lua"
)

// Player.AttackSpeed divides the base vocation interval by config.lua's
// rateAttackSpeed (default 1.0): 2.0 halves the cadence, 0.5 doubles it. The
// datapack's vocations.xml keeps attackspeed=2000, so the knob is what lets an
// operator make combat snappier without editing the datapack.
func TestPlayerAttackSpeedRate(t *testing.T) {
	old := config.Active
	t.Cleanup(func() { config.Active = old })

	// Vocation 0 resolves to the base 2000 whether or not the registry is loaded.
	p := &Player{Vocation: 0}
	setRate := func(rate float64) {
		config.Active = &config.Config{Custom: map[string]lua.LValue{"rateattackspeed": lua.LNumber(rate)}}
	}

	// No rateAttackSpeed key → the datapack default cadence.
	config.Active = config.Default()
	if p.AttackSpeed() != 2000*time.Millisecond {
		t.Errorf("default AttackSpeed = %v, want 2000ms", p.AttackSpeed())
	}

	// rate 2.0 halves the interval.
	setRate(2.0)
	if p.AttackSpeed() != 1000*time.Millisecond {
		t.Errorf("rate 2.0 AttackSpeed = %v, want 1000ms", p.AttackSpeed())
	}

	// rate 0.5 doubles the interval.
	setRate(0.5)
	if p.AttackSpeed() != 4000*time.Millisecond {
		t.Errorf("rate 0.5 AttackSpeed = %v, want 4000ms", p.AttackSpeed())
	}

	// A non-positive rate falls back to 1.0 rather than dividing by zero.
	setRate(0)
	if p.AttackSpeed() != 2000*time.Millisecond {
		t.Errorf("rate 0 AttackSpeed = %v, want 2000ms", p.AttackSpeed())
	}

	// A runaway rate is floored so it cannot yield a zero-interval attack loop.
	setRate(1000)
	if p.AttackSpeed() != 100*time.Millisecond {
		t.Errorf("rate 1000 AttackSpeed = %v, want 100ms floor", p.AttackSpeed())
	}
}