package items

import "testing"

// The bed transform attributes from items.xml must reach the catalog: the
// occupied variants per sex and the partner-half id drive the bed sleep visual.
func TestBedTransformAttributes(t *testing.T) {
	cat := NewCatalog(&ItemType{ID: 694})
	it := cat.Get(694)
	if it == nil {
		t.Fatal("item 694 not registered")
	}
	attr := func(key, value string) {
		processAttr(it, xmlAttribute{Key: key, Value: value})
	}
	attr("type", "bed")
	attr("bedpart", "pillow")
	attr("bedpartof", "695")
	attr("maletransformto", "2487")
	attr("femaletransformto", "2486")

	if it.Type != ItemTypeBed {
		t.Errorf("type = %v, want ItemTypeBed", it.Type)
	}
	if it.BedPartOf != 695 {
		t.Errorf("BedPartOf = %d, want 695", it.BedPartOf)
	}
	if it.TransformToOnUseMale != 2487 {
		t.Errorf("TransformToOnUseMale = %d, want 2487", it.TransformToOnUseMale)
	}
	if it.TransformToOnUseFemale != 2486 {
		t.Errorf("TransformToOnUseFemale = %d, want 2486", it.TransformToOnUseFemale)
	}
}
