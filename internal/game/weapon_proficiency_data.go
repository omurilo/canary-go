package game

import (
	"github.com/opentibiabr/canary-go/internal/kv"
)

// ProficiencyPerk is one selected perk on a weapon, mirroring ProficiencyPerk
// (src/enums/weapon_proficiency.hpp:93).
type ProficiencyPerk struct {
	Level        uint8
	Index        uint8
	Value        float64
	SpellID      uint16
	Range        uint8
	BestiaryID   uint16
	BestiaryName string
	AugmentType  uint8
	SkillID      uint8
	Element      uint8
	Type         uint8
}

// WeaponProficiencyData is the per-weapon progression the C++ server persists,
// mirroring WeaponProficiencyData (src/enums/weapon_proficiency.hpp:125).
//
// This is the ONLY weapon-proficiency state C++ stores. The aggregated bonuses in
// WeaponProficiency are derived from these perks at runtime by
// WeaponProficiency::applyPerks (weapon_proficiency.cpp:441) and never written to
// the database. Go used to persist the derived cache instead, as a JSON blob in a
// `players.weapon_proficiency` column that does not exist in the C++ schema.
type WeaponProficiencyData struct {
	Experience uint32
	Perks      []ProficiencyPerk
	Mastered   bool
}

// Key names used in the KV map. They must match serialize/serializePerk in
// weapon_proficiency.cpp:408-430 exactly, or the C++ server reads zeros.
const (
	wpKeyExperience   = "experience"
	wpKeyMastered     = "mastered"
	wpKeyPerks        = "perks"
	wpKeyIndex        = "index"
	wpKeyType         = "type"
	wpKeyValue        = "value"
	wpKeyLevel        = "level"
	wpKeyAugmentType  = "augmentType"
	wpKeyBestiaryID   = "bestiaryId"
	wpKeyBestiaryName = "bestiaryName"
	wpKeyElement      = "element"
	wpKeyRange        = "range"
	wpKeySkillID      = "skillId"
	wpKeySpellID      = "spellId"
)

// ToKV encodes the perk as the map WeaponProficiency::serializePerk produces.
func (p ProficiencyPerk) ToKV() kv.Value {
	return kv.Map(map[string]kv.Value{
		wpKeyIndex:        kv.Int(int32(p.Index)),
		wpKeyType:         kv.Int(int32(p.Type)),
		wpKeyValue:        kv.Double(p.Value),
		wpKeyLevel:        kv.Int(int32(p.Level)),
		wpKeyAugmentType:  kv.Int(int32(p.AugmentType)),
		wpKeyBestiaryID:   kv.Int(int32(p.BestiaryID)),
		wpKeyBestiaryName: kv.String(p.BestiaryName),
		wpKeyElement:      kv.Int(int32(p.Element)),
		wpKeyRange:        kv.Int(int32(p.Range)),
		wpKeySkillID:      kv.Int(int32(p.SkillID)),
		wpKeySpellID:      kv.Int(int32(p.SpellID)),
	})
}

// ProficiencyPerkFromKV decodes a perk, tolerating missing keys the way
// deserializePerk's getInt lambda does (absent key reads as 0).
func ProficiencyPerkFromKV(v kv.Value) ProficiencyPerk {
	var p ProficiencyPerk
	if v.Kind != kv.KindMap {
		return p
	}
	getInt := func(key string) int32 {
		if got, ok := v.MapValue(key); ok {
			return got.GetInt()
		}
		return 0
	}

	p.Index = uint8(getInt(wpKeyIndex))
	p.Type = uint8(getInt(wpKeyType))
	if got, ok := v.MapValue(wpKeyValue); ok {
		p.Value = got.GetDouble()
	}
	p.Level = uint8(getInt(wpKeyLevel))
	p.AugmentType = uint8(getInt(wpKeyAugmentType))
	p.BestiaryID = uint16(getInt(wpKeyBestiaryID))
	if got, ok := v.MapValue(wpKeyBestiaryName); ok {
		p.BestiaryName = got.GetString()
	}
	p.Element = uint8(getInt(wpKeyElement))
	p.Range = uint8(getInt(wpKeyRange))
	p.SkillID = uint8(getInt(wpKeySkillID))
	p.SpellID = uint16(getInt(wpKeySpellID))
	return p
}

// ToKV encodes the weapon data as WeaponProficiency::serialize does.
func (d WeaponProficiencyData) ToKV() kv.Value {
	perks := make([]kv.Value, 0, len(d.Perks))
	for _, perk := range d.Perks {
		perks = append(perks, perk.ToKV())
	}
	return kv.Map(map[string]kv.Value{
		wpKeyExperience: kv.Int(int32(d.Experience)),
		wpKeyMastered:   kv.Bool(d.Mastered),
		wpKeyPerks:      kv.Array(perks...),
	})
}

// WeaponProficiencyDataFromKV decodes the weapon data, applying the same
// clamping WeaponProficiency::deserialize does: a stored experience <= 0 becomes 0.
func WeaponProficiencyDataFromKV(v kv.Value) WeaponProficiencyData {
	var d WeaponProficiencyData
	if v.Kind != kv.KindMap {
		return d
	}

	if got, ok := v.MapValue(wpKeyExperience); ok {
		if stored := got.GetInt(); stored > 0 {
			d.Experience = uint32(stored)
		}
	}
	if got, ok := v.MapValue(wpKeyMastered); ok {
		d.Mastered = got.GetBool()
	}
	if got, ok := v.MapValue(wpKeyPerks); ok && got.Kind == kv.KindArray {
		d.Perks = make([]ProficiencyPerk, 0, len(got.Array))
		for _, elem := range got.Array {
			d.Perks = append(d.Perks, ProficiencyPerkFromKV(elem))
		}
	}
	return d
}
