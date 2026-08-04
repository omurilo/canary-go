package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/items"

	"github.com/omurilo/canary-go/internal/io/propstream"
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
	if _, _, err := DecodeItemAttributes(w.GetStream(), 0); err != nil {
		t.Errorf("ATTR_CUSTOM should be skippable, got err=%v", err)
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

// The unwrap bug, at its root: ATTR_CUSTOM was read and thrown away, so every
// script-set attribute died on the first save/load round trip. The store stamps a
// decoration kit with "unWrapId" to record what it becomes
// (data/libs/gamestore/purchases.lua:140); after a restart the kit came back
// without it and unwrapping refused with nothing to go on.
func TestCustomAttributesSurviveTheRoundTrip(t *testing.T) {
	in := &ItemAttributes{Custom: map[string]any{
		"unWrapId": int64(20718),
		"label":    "a parquet floor",
		"ratio":    2.5,
		"bound":    true,
	}}

	blob := in.Encode(0)
	out, _, err := DecodeItemAttributes(blob, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out == nil || out.Custom == nil {
		t.Fatalf("custom attributes were dropped entirely")
	}
	if got := out.Custom["unWrapId"]; got != int64(20718) {
		t.Errorf("unWrapId = %v (%T), want int64(20718)", got, got)
	}
	if got := out.Custom["label"]; got != "a parquet floor" {
		t.Errorf("label = %v", got)
	}
	if got := out.Custom["ratio"]; got != 2.5 {
		t.Errorf("ratio = %v", got)
	}
	if got := out.Custom["bound"]; got != true {
		t.Errorf("bound = %v", got)
	}
}

// Encoding must be stable: a map iterated in random order would rewrite the blob
// on every save and churn the tile_store rows.
func TestCustomAttributeEncodingIsDeterministic(t *testing.T) {
	a := &ItemAttributes{Custom: map[string]any{
		"z": int64(1), "a": int64(2), "m": "x", "b": true,
	}}
	first := a.Encode(0)
	for i := 0; i < 20; i++ {
		if got := a.Encode(0); string(got) != string(first) {
			t.Fatalf("encoding differs between runs")
		}
	}
}

// item:remove on something held in a container or an inventory slot only zeroed
// the count and left the object in place. A mystic bag survived its own
// mysticBag.onUse — still in the backpack, still usable — so it handed out a
// prize on every click, forever.
func TestRemoveItemFromHolder(t *testing.T) {
	p := &Player{Name: "Holder"}
	bag := &Item{ID: 1987, Count: 1, Container: NewContainer(20)}
	mystic := &Item{ID: 6571, Count: 1, Container: NewContainer(0)}
	mystic.Container.Parent = bag
	bag.Container.Contents = []*Item{mystic}
	p.Inventory[3] = bag

	if !(&World{}).RemoveItemFromHolder(p, mystic, 1) {
		t.Fatalf("an item inside a backpack must be removable")
	}
	if len(bag.Container.Contents) != 0 {
		t.Errorf("the bag still holds %d items", len(bag.Container.Contents))
	}
	if mystic.Container != nil && mystic.Container.Parent != nil {
		t.Errorf("the removed item must not keep pointing at its old container")
	}

	// A stack loses only the requested amount and stays put.
	gold := &Item{ID: 3031, Count: 100, Container: NewContainer(0)}
	gold.Container.Parent = bag
	bag.Container.Contents = []*Item{gold}
	if !(&World{}).RemoveItemFromHolder(p, gold, 40) {
		t.Fatalf("a partial removal must report success")
	}
	if gold.Count != 60 {
		t.Errorf("gold count = %d, want 60", gold.Count)
	}
	if len(bag.Container.Contents) != 1 {
		t.Errorf("a partially removed stack must stay in the container")
	}

	// Directly in an inventory slot, with no container in between.
	loose := &Item{ID: 6571, Count: 1}
	p.Inventory[5] = loose
	if !(&World{}).RemoveItemFromHolder(p, loose, 1) {
		t.Fatalf("an item in an inventory slot must be removable")
	}
	if p.Inventory[5] != nil {
		t.Errorf("the inventory slot must be cleared")
	}

	// Nothing holds it.
	if (&World{}).RemoveItemFromHolder(p, &Item{ID: 1}, 1) {
		t.Errorf("an unheld item has no holder to remove it from")
	}
}

// The depot chest page size travels to the client in the 0x6E frame and is what
// it pages by; 36 was invented and disagrees with DepotChest::maxSize.
func TestDepotChestPageSize(t *testing.T) {
	if DepotChestPageSize != 32 {
		t.Fatalf("page size = %d, want 32 (depotchest.cpp:17)", DepotChestPageSize)
	}
	dm := NewPlayerDepotManager(&Player{Name: "Owner"})
	chest := dm.GetDepotChest(1, true)
	if chest.Container == nil || chest.Container.MaxSize != DepotChestPageSize {
		t.Errorf("chest MaxSize = %d, want %d", chest.Container.MaxSize, DepotChestPageSize)
	}
	if !chest.HasPagination() {
		t.Errorf("a depot chest must be paginated, or nothing past the first page is reachable")
	}

	// The page is not the limit: a chest holds up to maxDepotItems, and pagination
	// is how the rest is seen. Nothing here may cap contents at the page size.
	for i := 0; i < 100; i++ {
		chest.Container.Contents = append(chest.Container.Contents, &Item{ID: 3031, Count: 1})
	}
	if len(chest.Container.Contents) != 100 {
		t.Errorf("a depot chest must hold more than one page, got %d", len(chest.Container.Contents))
	}
}

// A depot box holds up to maxDepotItems (2000), not one page. MaxItems was left
// at 0, so AddItemToContainer fell back to the page size and refused everything
// past the 32nd item — and the move path had already taken the item off the
// floor, so it was destroyed rather than rejected.
func TestDepotChestHoldsMoreThanOnePage(t *testing.T) {
	dm := NewPlayerDepotManager(&Player{Name: "Owner"})
	chest := dm.GetDepotChest(1, true)

	if chest.Container == nil || chest.Container.MaxItems != MaxDepotItems {
		t.Fatalf("chest MaxItems = %d, want %d (depotchest.cpp:16)", chest.Container.MaxItems, MaxDepotItems)
	}
	if chest.Container.MaxSize != DepotChestPageSize {
		t.Errorf("the page size must stay %d", DepotChestPageSize)
	}

	cat := items.NewCatalog(&items.ItemType{ID: 1650, Name: "table"})
	for i := 0; i < DepotChestPageSize+10; i++ {
		if !AddItemToContainer(cat, chest, &Item{ID: 1650, Count: 1}) {
			t.Fatalf("the depot refused item %d, well short of its %d limit", i+1, MaxDepotItems)
		}
	}
	if len(chest.Container.Contents) != DepotChestPageSize+10 {
		t.Errorf("chest holds %d, want %d", len(chest.Container.Contents), DepotChestPageSize+10)
	}
}
