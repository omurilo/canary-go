package game

import (
	"testing"
)

func TestItemClassification_AddTier(t *testing.T) {
	ic := NewItemClassification(1)

	// Add a tier and verify.
	ic.AddTier(1, 1, 25000, 0, 0)
	info := ic.GetTierInfo(1)
	if info == nil {
		t.Fatal("expected tier 1 info, got nil")
	}
	if info.CorePrice != 1 || info.RegularPrice != 25000 {
		t.Errorf("unexpected tier 1 prices: core=%d regular=%d", info.CorePrice, info.RegularPrice)
	}

	// Add another tier.
	ic.AddTier(5, 15, 100000000, 875000000, 2000000000)
	info = ic.GetTierInfo(5)
	if info == nil {
		t.Fatal("expected tier 5 info, got nil")
	}
	if info.CorePrice != 15 || info.ConvergenceFusionPrice != 875000000 || info.ConvergenceTransferPrice != 2000000000 {
		t.Errorf("unexpected tier 5 prices: core=%d fusion=%d transfer=%d",
			info.CorePrice, info.ConvergenceFusionPrice, info.ConvergenceTransferPrice)
	}
}

func TestItemClassification_GetUnknownTier(t *testing.T) {
	ic := NewItemClassification(1)
	ic.AddTier(1, 1, 25000, 0, 0)
	info := ic.GetTierInfo(99)
	if info != nil {
		t.Error("expected nil for unknown tier, got non-nil")
	}
}

func TestItemClassification_WorldStorage(t *testing.T) {
	w := NewWorld()

	// Create and store a classification.
	ic := NewItemClassification(3)
	ic.AddTier(1, 1, 4000000, 0, 0)
	ic.AddTier(3, 5, 20000000, 0, 0)
	w.ItemClassifications[3] = ic

	// Retrieve via ForgeTierPriceOf.
	info := ForgeTierPriceOf(w, 3, 1)
	if info == nil {
		t.Fatal("expected tier 1 info for classification 3, got nil")
	}
	if info.RegularPrice != 4000000 {
		t.Errorf("expected regular price 4000000, got %d", info.RegularPrice)
	}

	info = ForgeTierPriceOf(w, 3, 3)
	if info == nil {
		t.Fatal("expected tier 3 info for classification 3, got nil")
	}
	if info.CorePrice != 5 {
		t.Errorf("expected core price 5, got %d", info.CorePrice)
	}
}

func TestForgeTierPriceOf_UnknownClassification(t *testing.T) {
	w := NewWorld()
	w.ItemClassifications[1] = NewItemClassification(1)

	info := ForgeTierPriceOf(w, 99, 1)
	if info != nil {
		t.Error("expected nil for unknown classification, got non-nil")
	}
}

func TestForgeTierPriceOf_NilWorld(t *testing.T) {
	info := ForgeTierPriceOf(nil, 1, 1)
	if info != nil {
		t.Error("expected nil for nil world, got non-nil")
	}
}

func TestNewItemClassification_InitialState(t *testing.T) {
	ic := NewItemClassification(5)
	if ic.ID != 5 {
		t.Errorf("expected ID 5, got %d", ic.ID)
	}
	if len(ic.Tiers) != 0 {
		t.Errorf("expected empty tiers map, got %d entries", len(ic.Tiers))
	}
}
