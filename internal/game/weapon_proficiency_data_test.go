package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/kv"
)

// The key names are the contract with the C++ server: serialize/serializePerk in
// weapon_proficiency.cpp:408-430 write exactly these, and deserialize looks them
// up by name. A rename on either side silently reads zeros.
func TestWeaponProficiencyKeyNames(t *testing.T) {
	data := WeaponProficiencyData{
		Experience: 4200,
		Mastered:   true,
		Perks:      []ProficiencyPerk{{Index: 2}},
	}
	encoded := data.ToKV()
	for _, key := range []string{"experience", "mastered", "perks"} {
		if _, ok := encoded.MapValue(key); !ok {
			t.Errorf("WeaponProficiencyData is missing key %q", key)
		}
	}
	if got := len(encoded.Map); got != 3 {
		t.Errorf("expected exactly 3 keys, got %d", got)
	}

	perk := ProficiencyPerk{Index: 1}.ToKV()
	want := []string{
		"index", "type", "value", "level", "augmentType",
		"bestiaryId", "bestiaryName", "element", "range", "skillId", "spellId",
	}
	for _, key := range want {
		if _, ok := perk.MapValue(key); !ok {
			t.Errorf("ProficiencyPerk is missing key %q", key)
		}
	}
	if got := len(perk.Map); got != len(want) {
		t.Errorf("expected exactly %d perk keys, got %d", len(want), got)
	}
}

func TestWeaponProficiencyRoundTrip(t *testing.T) {
	in := WeaponProficiencyData{
		Experience: 123456,
		Mastered:   true,
		Perks: []ProficiencyPerk{
			{
				Level: 3, Index: 1, Value: 12.5, SpellID: 88, Range: 4,
				BestiaryID: 77, BestiaryName: "dragon lord",
				AugmentType: 2, SkillID: 5, Element: 6, Type: 7,
			},
			{Level: 1, Index: 0},
		},
	}

	// Through the full protobuf encode/decode, which is what actually lands in
	// the kv_store column.
	raw := in.ToKV().Marshal()
	decoded, err := kv.Unmarshal(raw, 0)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := WeaponProficiencyDataFromKV(decoded)

	if out.Experience != in.Experience || out.Mastered != in.Mastered {
		t.Errorf("header mismatch: got %+v want exp=%d mastered=%v",
			out, in.Experience, in.Mastered)
	}
	if len(out.Perks) != len(in.Perks) {
		t.Fatalf("perk count: got %d want %d", len(out.Perks), len(in.Perks))
	}
	for i := range in.Perks {
		if out.Perks[i] != in.Perks[i] {
			t.Errorf("perk %d:\n got %+v\nwant %+v", i, out.Perks[i], in.Perks[i])
		}
	}
}

// deserialize clamps a stored experience <= 0 to 0 (weapon_proficiency.cpp:341).
func TestWeaponProficiencyNegativeExperienceClamped(t *testing.T) {
	v := kv.Map(map[string]kv.Value{
		"experience": kv.Int(-5),
		"mastered":   kv.Bool(false),
	})
	if got := WeaponProficiencyDataFromKV(v).Experience; got != 0 {
		t.Errorf("expected clamp to 0, got %d", got)
	}
}

// A missing key must read as the zero value rather than panicking, matching the
// getInt lambda in deserializePerk.
func TestWeaponProficiencyMissingKeys(t *testing.T) {
	out := WeaponProficiencyDataFromKV(kv.Map(map[string]kv.Value{}))
	if out.Experience != 0 || out.Mastered || len(out.Perks) != 0 {
		t.Errorf("unexpected %+v", out)
	}
	perk := ProficiencyPerkFromKV(kv.Map(map[string]kv.Value{}))
	if perk != (ProficiencyPerk{}) {
		t.Errorf("unexpected %+v", perk)
	}
	// A non-map value must not blow up either.
	if got := WeaponProficiencyDataFromKV(kv.Int(3)); got.Experience != 0 {
		t.Errorf("unexpected %+v", got)
	}
}
