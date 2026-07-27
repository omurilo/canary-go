package game

import (
	"sync"
)

// House represents a player house or guild hall. Houses are loaded from
// the houses.xml and persisted in the houses table. Mirrors
// C++ src/map/house/house.{hpp,cpp}.
type House struct {
	mu         sync.RWMutex
	ID         uint32
	Name       string
	OwnerID    uint32 // 0 = unowned / guild hall
	Rent       uint32
	Beds       uint8
	Size       uint32 // square meters
	RentPeriod string // "monthly", "weekly", "daily"
	TownID     uint16
	ClientID   uint32 // door item ID from XML (clientid attr)
	Position   Position // entrance

	GuestList    []string
	SubOwnerList []string
	DoorList     []HouseDoor
	AccessList   AccessList
	HouseTiles   []Position // all tiles that belong to this house

	// Auction/bid fields
	BidderName    string
	HighestBid    uint64
	InternalBid   uint64
	BidHolderLimit uint64
	BidEndDate    uint32
	Bidder        uint32 // player GUID who bid
}

// HouseDoor represents a lockable door in a house.
type HouseDoor struct {
	ID     uint8
	Locked bool
	Level  uint8 // minimum level required
}

// AccessList controls who can enter a house.
type AccessList struct {
	Players []string
	Guilds  []string
}

// --- House methods ---

// IsOwner reports whether the given player owns this house.
func (h *House) IsOwner(playerID uint32) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.OwnerID == playerID
}

// SetOwner assigns ownership to a player. Pass 0 to unown.
func (h *House) SetOwner(playerID uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.OwnerID = playerID
}

// GetHouseByDoorID finds the house that contains a door with the given door ID.
func (w *World) GetHouseByDoorID(doorID uint8) *House {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, h := range w.Houses {
		for _, d := range h.DoorList {
			if d.ID == doorID {
				return h
			}
		}
	}
	return nil
}

// CanPlayerUseDoor checks if a player can open a house door.
// The player must be the owner, a sub-owner, or on the guest list.
func (h *House) CanPlayerUseDoor(p *Player) bool {
	if p == nil {
		return false
	}
	if h.IsOwner(p.DBID) {
		return true
	}
	if h.IsSubOwner(p.Name) {
		return true
	}
	if h.IsGuest(p.Name) {
		return true
	}
	return false
}

// IsSubOwner checks if a player name is on the sub-owner list.
func (h *House) IsSubOwner(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, s := range h.SubOwnerList {
		if s == name {
			return true
		}
	}
	return false
}

// AddSubOwner adds a player to the sub-owner list.
func (h *House) AddSubOwner(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.SubOwnerList {
		if s == name {
			return false
		}
	}
	h.SubOwnerList = append(h.SubOwnerList, name)
	return true
}

// RemoveSubOwner removes a player from the sub-owner list.
func (h *House) RemoveSubOwner(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, s := range h.SubOwnerList {
		if s == name {
			h.SubOwnerList = append(h.SubOwnerList[:i], h.SubOwnerList[i+1:]...)
			return true
		}
	}
	return false
}

// IsGuest checks if a player name is on the guest list.
func (h *House) IsGuest(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, g := range h.GuestList {
		if g == name {
			return true
		}
	}
	return false
}

// AddGuest adds a player to the guest list.
func (h *House) AddGuest(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, g := range h.GuestList {
		if g == name {
			return false
		}
	}
	h.GuestList = append(h.GuestList, name)
	return true
}

// RemoveGuest removes a player from the guest list.
func (h *House) RemoveGuest(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, g := range h.GuestList {
		if g == name {
			h.GuestList = append(h.GuestList[:i], h.GuestList[i+1:]...)
			return true
		}
	}
	return false
}

// CanAccess checks if a player can enter the house (owner, guest, or guild).
func (h *House) CanAccess(playerName string, guildName string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, g := range h.GuestList {
		if g == playerName {
			return true
		}
	}
	if guildName != "" {
		for _, gn := range h.AccessList.Guilds {
			if gn == guildName {
				return true
			}
		}
	}
	return false
}

// --- World house methods ---

// GetHouse returns a house by ID.
func (w *World) GetHouse(houseID uint32) *House {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Houses[houseID]
}

// GetHouseByPlayerID returns the house owned by a player, or nil.
func (w *World) GetHouseByPlayerID(playerID uint32) *House {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, h := range w.Houses {
		if h.OwnerID == playerID {
			return h
		}
	}
	return nil
}

// GetHouseByClientID looks up a house by its door item client ID.
func (w *World) GetHouseByClientID(clientID uint32) *House {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, h := range w.Houses {
		if h != nil && h.ClientID == clientID {
			return h
		}
	}
	return nil
}

// RegisterHouse adds a house to the world.
func (w *World) RegisterHouse(h *House) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.Houses == nil {
		w.Houses = make(map[uint32]*House)
	}
	w.Houses[h.ID] = h
}

// AllHouses returns all registered houses.
func (w *World) AllHouses() []*House {
	w.mu.RLock()
	defer w.mu.RUnlock()
	list := make([]*House, 0, len(w.Houses))
	for _, h := range w.Houses {
		list = append(list, h)
	}
	return list
}

// GetHouseByPosition returns the house that owns the tile at pos, or nil.
func (w *World) GetHouseByPosition(pos Position) *House {
	w.mu.RLock()
	defer w.mu.RUnlock()
	tile := w.Map.GetTile(pos)
	if tile == nil || tile.HouseID == 0 {
		return nil
	}
	return w.Houses[tile.HouseID]
}

// RegisterHouseTiles iterates every map tile and populates each house's HouseTiles.
func (w *World) RegisterHouseTiles() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Map.Range(func(pos Position, t *Tile) bool {
		if t.HouseID == 0 {
			return true
		}
		h := w.Houses[t.HouseID]
		if h == nil {
			return true
		}
		h.HouseTiles = append(h.HouseTiles, pos)
		return true
	})
}

// HouseCountByAccount returns the number of houses owned by players of a given account.
func (w *World) HouseCountByAccount(accountID uint32, players map[uint32]*Player) int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	count := 0
	for _, p := range players {
		if p.AccountID == accountID {
			for _, h := range w.Houses {
				if h.OwnerID == p.DBID {
					count++
				}
			}
		}
	}
	return count
}
