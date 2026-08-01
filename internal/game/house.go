package game

import (
	"strings"
	"sync"
	"time"

	"github.com/opentibiabr/canary-go/internal/config"
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

	// Ownership bookkeeping that House::setOwner maintains alongside `owner`.
	// Without OwnerName the door description cannot name the owner, and without
	// PaidUntil the rent cycle has no start date.
	OwnerName      string
	OwnerAccountID uint32
	PaidUntil      int64
	RentWarnings   uint32
	State          uint8

	// isLoaded gates the early return in setOwner: re-setting the SAME owner is a
	// no-op only after the house has been loaded once, so that loading a house
	// from the DB still runs the full initialisation.
	isLoaded bool

	// Auction/bid fields
	BidderName     string
	HighestBid     uint64
	InternalBid    uint64
	BidHolderLimit uint64
	BidEndDate     uint32
	Bidder         uint32 // player GUID who bid

	// Transfer fields
	TransferToName string
	TransferPrice  uint64
	TransferAccept uint32 // player GUID who accepted transfer
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

// CyclopediaHouseState values (src/enums/player_cyclopedia.hpp:64-69). setOwner
// writes Rented when a guid is given and Available when the house is unowned.
const (
	HouseStateAvailable uint8 = 0
	HouseStateRented    uint8 = 2
	HouseStateTransfer  uint8 = 3
	HouseStateMoveOut   uint8 = 4
)

// SetOwner assigns ownership to a player; pass 0 to unown. Port of
// House::setOwner (src/map/house/house.cpp:94-154).
//
// This used to assign h.OwnerID and nothing else, so `/owner` appeared to do
// nothing: the change lived only in memory, vanished on restart, and left the
// owner name, the rent clock and the bid columns describing the previous owner.
//
// w may be nil (tests, and the load path that has no world yet); the DB write and
// the owner lookup are World hooks, so a nil world simply means "in memory only",
// which is what updateDatabase=false expresses in C++.
func (h *House) SetOwner(w *World, guid uint32, updateDatabase bool, player *Player) {
	h.mu.Lock()
	current := h.OwnerID
	loaded := h.isLoaded
	h.mu.Unlock()

	// The row is written BEFORE the early return below, exactly as in C++: the
	// guard is on the in-memory state, not on the persistence.
	if updateDatabase && current != guid && w != nil && w.OnHouseOwnerChange != nil {
		w.OnHouseOwnerChange(h, guid)
	}

	if loaded && current == guid {
		return
	}

	h.mu.Lock()
	h.isLoaded = true
	h.mu.Unlock()

	if current != 0 {
		h.tryTransferOwnership(w, player, false)
	}

	h.mu.Lock()
	h.PaidUntil = housePaidUntil(guid)
	h.RentWarnings = 0
	h.mu.Unlock()

	if guid == 0 {
		return
	}

	// SELECT `name`, `account_id` FROM `players` WHERE `id` = guid. C++ abandons
	// the assignment entirely when the row is missing or the name is empty, so a
	// bad guid leaves the house unowned rather than owned by a ghost id.
	if w == nil || w.LookupPlayerAccount == nil {
		// No lookup available (tests, offline tools): take the guid on faith so the
		// in-memory ownership is still usable, but leave the name blank.
		h.mu.Lock()
		h.OwnerID = guid
		h.State = HouseStateRented
		h.mu.Unlock()
		return
	}
	name, accountID, ok := w.LookupPlayerAccount(guid)
	if !ok || name == "" {
		return
	}
	h.mu.Lock()
	h.OwnerID = guid
	h.OwnerName = name
	h.OwnerAccountID = accountID
	h.State = HouseStateRented
	h.mu.Unlock()
}

// housePaidUntil is the rent clock C++ starts in setOwner from the configured
// period; anything other than the four known periods means no rent at all.
func housePaidUntil(guid uint32) int64 {
	if guid == 0 {
		return 0
	}
	now := time.Now().Unix()
	switch strings.ToLower(config.Str("houseRentPeriod", "never")) {
	case "yearly":
		return now + 24*60*60*365
	case "monthly":
		return now + 24*60*60*30
	case "weekly":
		return now + 24*60*60*7
	case "daily":
		return now + 24*60*60
	default:
		return 0
	}
}

// tryTransferOwnership mirrors House::tryTransferOwnership (house.cpp:72-92) for
// the parts that exist here: kick everyone standing in the house and clear the
// access lists. The depot transfer of the furniture (transferToDepot) has no Go
// counterpart yet, so items are left where they are — see clearHouseInfo.
func (h *House) tryTransferOwnership(w *World, player *Player, serverStartup bool) {
	if w != nil {
		for _, pos := range h.HouseTilesSnapshot() {
			tile := w.Map.GetTile(pos)
			if tile == nil {
				continue
			}
			for _, c := range append([]Creature(nil), tile.Creatures...) {
				if p, ok := c.(*Player); ok && p != nil {
					h.kickPlayer(w, p)
				}
			}
		}
	}
	h.clearHouseInfo(serverStartup)
}

// clearHouseInfo is House::clearHouseInfo (house.cpp:50-70). preventOwnerDeletion
// keeps the owner while still wiping the access lists, which is what the
// server-startup path relies on.
func (h *House) clearHouseInfo(preventOwnerDeletion bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !preventOwnerDeletion {
		h.OwnerID = 0
		h.OwnerAccountID = 0
		h.OwnerName = ""
		h.State = HouseStateAvailable
	}
	h.SubOwnerList = nil
	h.GuestList = nil
	h.AccessList = AccessList{}
}

// kickPlayer sends a player standing in the house back to their town temple, the
// effect of House::kickPlayer once the access check has already been made.
func (h *House) kickPlayer(w *World, p *Player) {
	if w == nil || p == nil {
		return
	}
	dest, ok := w.TownsByID[p.TownID]
	if !ok {
		dest = w.DefaultSpawn
	}
	w.TeleportCreature(p, dest)
}

// HouseTilesSnapshot copies the tile list for iteration outside the lock.
func (h *House) HouseTilesSnapshot() []Position {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]Position(nil), h.HouseTiles...)
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

// GetHouseByClientID looks up a house by its door item client ID,
// falling back to the house ID if no door item match is found.
func (w *World) GetHouseByClientID(clientID uint32) *House {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// First try matching by door item client ID.
	for _, h := range w.Houses {
		if h != nil && h.ClientID == clientID {
			return h
		}
	}
	// Fallback: try matching by house ID (for houses with clientid="0" in XML).
	if h, ok := w.Houses[clientID]; ok && h != nil {
		return h
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
