package game

// TierInfo holds the pricing data for upgrading an item from one tier to the
// next within a classification. Each field corresponds to a different upgrade
// path (regular fusion, core convergence, transfer convergence).
type TierInfo struct {
	CorePrice                uint8
	RegularPrice             uint64
	ConvergenceFusionPrice   uint64
	ConvergenceTransferPrice uint64
}

// ItemClassification groups items of the same upgrade tier and stores the price
// table for upgrading through each tier level. Loaded from
// data/scripts/systems/item_tiers.lua.
type ItemClassification struct {
	ID    uint8
	Tiers map[uint8]*TierInfo // tier → price info (1-based: cost to go from N-1 to N)
}

// NewItemClassification creates a new classification with the given ID.
func NewItemClassification(id uint8) *ItemClassification {
	return &ItemClassification{
		ID:    id,
		Tiers: make(map[uint8]*TierInfo),
	}
}

// AddTier registers price information for upgrading an item from (tier-1) to
// tier within this classification.
func (ic *ItemClassification) AddTier(tier uint8, corePrice uint8, regularPrice, convergenceFusionPrice, convergenceTransferPrice uint64) {
	ic.Tiers[tier] = &TierInfo{
		CorePrice:                corePrice,
		RegularPrice:             regularPrice,
		ConvergenceFusionPrice:   convergenceFusionPrice,
		ConvergenceTransferPrice: convergenceTransferPrice,
	}
}

// GetTierInfo returns the price information for the given tier upgrade, or nil
// if the tier is not defined for this classification.
func (ic *ItemClassification) GetTierInfo(tier uint8) *TierInfo {
	return ic.Tiers[tier]
}

// ForgeTierPriceOf resolves the price information for a given (classification,
// tier) pair from the World's ItemClassifications registry. Returns nil if the
// classification or tier is unknown.
func ForgeTierPriceOf(w *World, classification, tier uint8) *TierInfo {
	if w == nil || w.ItemClassifications == nil {
		return nil
	}
	cls, ok := w.ItemClassifications[classification]
	if !ok {
		return nil
	}
	return cls.GetTierInfo(tier)
}
