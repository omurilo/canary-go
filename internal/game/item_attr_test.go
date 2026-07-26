package game

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/io/propstream"
)

func u16p(v uint16) *uint16 { return &v }
func u32p(v uint32) *uint32 { return &v }
func u8p(v uint8) *uint8    { return &v }
func i32p(v int32) *int32   { return &v }
func i64p(v int64) *int64   { return &v }
func i8p(v int8) *int8      { return &v }
func strp(v string) *string { return &v }

// buildBlob writes a synthetic OTBR attribute blob using propstream, mirroring
// the C++ serialize wire widths so the decoder can be exercised directly.
func buildBlob() []byte {
	w := propstream.NewPropWriteStream()
	w.WriteUint8(attrCount)
	w.WriteUint8(42) // stack count / subtype
	w.WriteUint8(attrCharges)
	w.WriteUint16(7)
	w.WriteUint8(attrActionID)
	w.WriteUint16(1000)
	w.WriteUint8(attrUniqueID)
	w.WriteUint16(2000)
	w.WriteUint8(attrText)
	w.WriteString("scroll text")
	w.WriteUint8(attrName)
	w.WriteString("custom name")
	w.WriteUint8(attrArticle)
	w.WriteString("a")
	w.WriteUint8(attrPluralName)
	w.WriteString("names")
	w.WriteUint8(attrDuration)
	w.WriteInt32(60000)
	w.WriteUint8(attrDecayingState)
	w.WriteUint8(1)
	w.WriteUint8(attrTier)
	w.WriteUint8(3)
	w.WriteUint8(attrOwner)
	w.WriteUint32(123456)
	return w.GetStream()
}

func TestDecodeItemAttributes_RoundTrip(t *testing.T) {
	blob := buildBlob()

	attr, subType, err := DecodeItemAttributes(blob, 1)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if subType != 42 {
		t.Errorf("expected subtype 42 (ATTR_COUNT overrides column), got %d", subType)
	}
	if !attr.HasCount {
		t.Error("expected HasCount")
	}
	if attr.Charges == nil || *attr.Charges != 7 {
		t.Errorf("charges = %v, want 7", attr.Charges)
	}
	if attr.ActionID == nil || *attr.ActionID != 1000 {
		t.Errorf("actionID = %v, want 1000", attr.ActionID)
	}
	if attr.UniqueID == nil || *attr.UniqueID != 2000 {
		t.Errorf("uniqueID = %v, want 2000", attr.UniqueID)
	}
	if attr.Text == nil || *attr.Text != "scroll text" {
		t.Errorf("text = %v", attr.Text)
	}
	if attr.Name == nil || *attr.Name != "custom name" {
		t.Errorf("name = %v", attr.Name)
	}
	if attr.Duration == nil || *attr.Duration != 60000 {
		t.Errorf("duration = %v, want 60000", attr.Duration)
	}
	if attr.DecayState == nil || *attr.DecayState != 1 {
		t.Errorf("decayState = %v, want 1", attr.DecayState)
	}
	if attr.Tier == nil || *attr.Tier != 3 {
		t.Errorf("tier = %v, want 3", attr.Tier)
	}
	if attr.Owner == nil || *attr.Owner != 123456 {
		t.Errorf("owner = %v, want 123456", attr.Owner)
	}

	// Encode -> decode must reproduce the same attribute set and subtype.
	encoded := attr.Encode(subType)
	attr2, subType2, err := DecodeItemAttributes(encoded, 1)
	if err != nil {
		t.Fatalf("re-decode failed: %v", err)
	}
	if subType2 != subType {
		t.Errorf("subtype changed on round-trip: %d != %d", subType2, subType)
	}
	if !itemAttrsEqual(attr, attr2) {
		t.Errorf("round-trip mismatch:\n first: %+v\nsecond: %+v", attr, attr2)
	}
}

// TestDecodeItemAttributes_AllScalars round-trips every modelled scalar tag.
func TestDecodeItemAttributes_AllScalars(t *testing.T) {
	orig := &ItemAttributes{
		StoreTimestamp: i64p(1700000000),
		HasCount:       true,
		Charges:        u16p(5),
		ActionID:       u16p(11),
		UniqueID:       u16p(22),
		Text:           strp("hi"),
		WrittenDate:    &[]uint64{1234567890}[0],
		WrittenBy:      strp("gm"),
		Description:    strp("desc"),
		Duration:       i32p(999),
		DecayState:     u8p(2),
		Name:           strp("n"),
		Article:        strp("an"),
		PluralName:     strp("ns"),
		Weight:         u32p(3500),
		Attack:         i32p(40),
		Defense:        i32p(30),
		ExtraDefense:   i32p(-2),
		Armor:          i32p(12),
		HitChance:      i8p(-10),
		ShootRange:     u8p(6),
		Tier:           u8p(4),
		Amount:         u16p(64),
		Owner:          u32p(777),
	}

	const subType = 200
	encoded := orig.Encode(subType)
	decoded, sub, err := DecodeItemAttributes(encoded, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if sub != subType {
		t.Errorf("subtype = %d, want %d", sub, subType)
	}
	if !itemAttrsEqual(orig, decoded) {
		t.Errorf("mismatch:\n orig: %+v\ndecoded: %+v", orig, decoded)
	}
}

func TestDecodeItemAttributes_Empty(t *testing.T) {
	attr, sub, err := DecodeItemAttributes(nil, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attr != nil {
		t.Errorf("expected nil attrs for empty blob, got %+v", attr)
	}
	if sub != 3 {
		t.Errorf("subtype should pass through unchanged: got %d", sub)
	}
}

func TestDecodeItemAttributes_UnsupportedFallsBack(t *testing.T) {
	// ATTR_CUSTOM is now handled (skipped) so DecodeItemAttributes should succeed.
	w := propstream.NewPropWriteStream()
	w.WriteUint8(attrCustom)
	w.WriteUint64(0)
	if attrs, _, err := DecodeItemAttributes(w.GetStream(), 0); err != nil {
		t.Errorf("ATTR_CUSTOM should be skippable, got err=%v", err)
	} else if attrs != nil {
		t.Error("expected nil attributes for empty ATTR_CUSTOM-only blob")
	}

	// ATTR_CUSTOM_ATTRIBUTES (deprecated nested structure) should still error.
	w2 := propstream.NewPropWriteStream()
	w2.WriteUint8(attrCustomAttributes)
	w2.WriteUint64(0)
	if _, _, err := DecodeItemAttributes(w2.GetStream(), 0); err == nil {
		t.Error("expected error for unsupported attrCustomAttributes so caller preserves raw blob")
	}
}

func itemAttrsEqual(a, b *ItemAttributes) bool {
	if a == nil || b == nil {
		return a == b
	}
	eqU8 := func(x, y *uint8) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	eqU16 := func(x, y *uint16) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	eqU32 := func(x, y *uint32) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	eqU64 := func(x, y *uint64) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	eqI8 := func(x, y *int8) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	eqI32 := func(x, y *int32) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	eqI64 := func(x, y *int64) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	eqStr := func(x, y *string) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }

	return a.HasCount == b.HasCount &&
		eqI64(a.StoreTimestamp, b.StoreTimestamp) &&
		eqU16(a.Charges, b.Charges) &&
		eqU16(a.ActionID, b.ActionID) &&
		eqU16(a.UniqueID, b.UniqueID) &&
		eqStr(a.Text, b.Text) &&
		eqU64(a.WrittenDate, b.WrittenDate) &&
		eqStr(a.WrittenBy, b.WrittenBy) &&
		eqStr(a.Description, b.Description) &&
		eqI32(a.Duration, b.Duration) &&
		eqU8(a.DecayState, b.DecayState) &&
		eqStr(a.Name, b.Name) &&
		eqStr(a.Article, b.Article) &&
		eqStr(a.PluralName, b.PluralName) &&
		eqU32(a.Weight, b.Weight) &&
		eqI32(a.Attack, b.Attack) &&
		eqI32(a.Defense, b.Defense) &&
		eqI32(a.ExtraDefense, b.ExtraDefense) &&
		eqI32(a.Armor, b.Armor) &&
		eqI8(a.HitChance, b.HitChance) &&
		eqU8(a.ShootRange, b.ShootRange) &&
		eqU8(a.Tier, b.Tier) &&
		eqU16(a.Amount, b.Amount) &&
		eqU32(a.Owner, b.Owner)
}
