package game

import (
	"sync"
	"time"
)

// MarketAction identifies whether a market offer is a buy or sell.
type MarketAction uint8

const (
	MarketActionBuy  MarketAction = 0
	MarketActionSell MarketAction = 1
)

// MarketOffer represents a single buy or sell offer in the market.
// Mirrors C++ IOMarket / MarketOffer.
type MarketOffer struct {
	ID         uint32
	PlayerID   uint32
	PlayerName string
	ItemID     uint16
	Amount     uint16
	Price      uint64
	Tier       uint8
	Timestamp  int64 // creation time (unix)
	Counter    uint16
	Anonymous  bool
}

// IsExpired reports whether the offer has exceeded the market duration.
func (o *MarketOffer) IsExpired(durationSecs int32) bool {
	return time.Now().Unix() > o.Timestamp+int64(durationSecs)
}

// Market manages the in-memory state of active market offers.
// The authoritative state lives in the market_offers DB table; this
// provides fast lookups for the protocol layer.
type Market struct {
	mu      sync.RWMutex
	byID    map[uint32]*MarketOffer
	byItem  map[uint16][]*MarketOffer // itemID → offers (buy or sell)
	nextID  uint32
}

// NewMarket creates an empty market.
func NewMarket() *Market {
	return &Market{
		byID:   make(map[uint32]*MarketOffer),
		byItem: make(map[uint16][]*MarketOffer),
		nextID: 1,
	}
}

// AddOffer registers a new offer.
func (m *Market) AddOffer(offer *MarketOffer) uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if offer.ID == 0 {
		offer.ID = m.nextID
		m.nextID++
	}
	m.byID[offer.ID] = offer
	m.byItem[offer.ItemID] = append(m.byItem[offer.ItemID], offer)
	return offer.ID
}

// RemoveOffer removes an offer by ID. Returns true if found.
func (m *Market) RemoveOffer(id uint32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	offer, ok := m.byID[id]
	if !ok {
		return false
	}
	delete(m.byID, id)
	offers := m.byItem[offer.ItemID]
	for i, o := range offers {
		if o.ID == id {
			m.byItem[offer.ItemID] = append(offers[:i], offers[i+1:]...)
			break
		}
	}
	return true
}

// GetOffer returns an offer by ID.
func (m *Market) GetOffer(id uint32) *MarketOffer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byID[id]
}

// GetOffersByItem returns all offers for a given item ID.
func (m *Market) GetOffersByItem(itemID uint16) []*MarketOffer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return a copy
	original := m.byItem[itemID]
	result := make([]*MarketOffer, len(original))
	copy(result, original)
	return result
}

// GetPlayerOffers returns all offers by a specific player.
func (m *Market) GetPlayerOffers(playerID uint32) []*MarketOffer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*MarketOffer
	for _, o := range m.byID {
		if o.PlayerID == playerID {
			result = append(result, o)
		}
	}
	return result
}

// PurgeExpired removes all offers older than the given duration.
func (m *Market) PurgeExpired(durationSecs int32) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().Unix()
	var removed int
	for id, offer := range m.byID {
		if now > offer.Timestamp+int64(durationSecs) {
			delete(m.byID, id)
			offers := m.byItem[offer.ItemID]
			for i, o := range offers {
				if o.ID == id {
					m.byItem[offer.ItemID] = append(offers[:i], offers[i+1:]...)
					break
				}
			}
			removed++
		}
	}
	return removed
}
