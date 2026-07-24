package charms

import "testing"

func TestBitHelpers(t *testing.T) {
	var bits int32
	bits = SetBit(bits, Wound)
	bits = SetBit(bits, Fatal) // id 20
	if !HasBit(bits, Wound) || !HasBit(bits, Fatal) {
		t.Fatal("set bits not reported as set")
	}
	if HasBit(bits, Enflame) {
		t.Fatal("unset bit reported as set")
	}
	used := UsedRunes(bits)
	if len(used) != 2 || used[0] != Wound || used[1] != Fatal {
		t.Fatalf("UsedRunes = %v, want [0 20] ascending", used)
	}
	bits = ClearBit(bits, Wound)
	if HasBit(bits, Wound) {
		t.Fatal("cleared bit still set")
	}
	if !HasBit(bits, Fatal) {
		t.Fatal("clearing one bit cleared another")
	}
}

func TestRegistryOrdering(t *testing.T) {
	r := NewRegistry()
	// add out of order; List must stay ascending by id
	r.Add(&Charm{ID: Poison})
	r.Add(&Charm{ID: Wound})
	r.Add(&Charm{ID: Enflame})
	if r.Len() != 3 {
		t.Fatalf("len = %d, want 3", r.Len())
	}
	if r.List[0].ID != Wound || r.List[1].ID != Enflame || r.List[2].ID != Poison {
		t.Fatalf("order = %d,%d,%d, want 0,1,2", r.List[0].ID, r.List[1].ID, r.List[2].ID)
	}
	// Add sets the binary bitmask = 1<<id
	if r.Get(Poison).Binary != 1<<Poison {
		t.Fatalf("binary = %d, want %d", r.Get(Poison).Binary, 1<<Poison)
	}
	// re-adding the same id mutates in place, keeps length
	r.Add(&Charm{ID: Wound, Name: "Wound"})
	if r.Len() != 3 || r.Get(Wound).Name != "Wound" {
		t.Fatalf("re-add changed len or lost update: len=%d name=%q", r.Len(), r.Get(Wound).Name)
	}
}

func TestOffensiveDamage(t *testing.T) {
	c := &Charm{ID: Wound, Percent: 5}
	// low level: 2*level dominates when 5% of maxHealth is larger
	if got := c.OffensiveDamage(10, 100000); got != 20 {
		t.Fatalf("dmg(level 10, hp 100000) = %d, want 20 (2*level)", got)
	}
	// low-HP target: 5% of maxHealth dominates when smaller than 2*level
	if got := c.OffensiveDamage(1000, 100); got != 5 {
		t.Fatalf("dmg(level 1000, hp 100) = %d, want 5 (5%% of 100)", got)
	}
}

func TestMinorEchoesGain(t *testing.T) {
	// 25*t^2 + 25*t + 50
	cases := map[uint8]uint32{0: 50, 1: 100, 2: 200}
	for tier, want := range cases {
		if got := MinorEchoesGain(tier); got != want {
			t.Errorf("MinorEchoesGain(%d) = %d, want %d", tier, got, want)
		}
	}
}
