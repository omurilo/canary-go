package game

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/omurilo/canary-go/internal/config"
	"github.com/omurilo/canary-go/internal/items"
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
	ClientID   uint32   // door item ID from XML (clientid attr)
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

	// doorLists caches an access list written for a door that has not loaded
	// yet. Door items load after the house, so an early write would be dropped.
	doorLists map[uint32]string
	// BedList is the beds actually placed in the house; Beds is derived from it
	// rather than trusted from the XML.
	BedList []Position
	// MaxBeds is House::maxBeds, the `beds` attribute of houses.xml
	// (house.cpp:940). It is the CAP, not the count — the count is Beds, and it
	// is what the houses table stores (iomapserialize.cpp:411 writes
	// getBedCount()). The loader used to put the cap into Beds, so a house
	// reported four beds the moment it loaded and then had that overwritten by
	// however many were actually placed.
	//
	// -1 means uncapped, matching the default when the attribute is absent.
	MaxBeds int32
	// hasNewOwnerOnStartup and NewOwnerGuid queue an ownership change for the
	// next boot, when house transfers are configured to apply on restart.
	hasNewOwnerOnStartup bool
	NewOwnerGuid         int32
	// transferItem is the single outstanding house transfer document.
	transferItem *Item

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
	Pos    Position
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
		h.UpdateDoorDescription(w)
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
		h.UpdateDoorDescription(w)
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

	// house.cpp:153 — the door text is refreshed as the last step of setOwner, so
	// looking at the door reports the new owner immediately.
	h.UpdateDoorDescription(w)
}

// GetPrice is House::getPrice (house.cpp:1069-1073).
func (h *House) GetPrice() uint32 {
	rent := h.GetRent()
	h.mu.RLock()
	size := h.Size
	h.mu.RUnlock()
	sqmPrice := uint32(config.Number("housePriceEachSQM", 1000)) * size
	// getPrice multiplies the RATED rent, not the raw XML value, so a server
	// running a rent rate sells houses at a price that matches what it charges.
	rentPrice := uint32(float64(rent) * config.Float("housePriceRentMultiplier", 1.0))
	return sqmPrice + rentPrice
}

// UpdateDoorDescription writes the house blurb onto every door of the house, the
// text a player reads when they look at it — and, with no /houseinfo command in
// the datapack, the only way in game to find out who owns a house.
//
// Port of House::updateDoorDescription (src/map/house/house.cpp:156-181). C++ can
// walk doorList because its entries are the door items themselves; game.HouseDoor
// carries only an id, so the doors are found by scanning the house tiles for
// items with a HouseDoorID.
func (h *House) UpdateDoorDescription(w *World) {
	if w == nil {
		return
	}
	desc := h.doorDescription()
	for _, pos := range h.HouseTilesSnapshot() {
		tile := w.Map.GetTile(pos)
		if tile == nil {
			continue
		}
		for _, it := range tile.Items {
			if it == nil || it.Attr == nil || it.Attr.HouseDoorID == nil {
				continue
			}
			d := desc
			it.Attr.Description = &d
		}
	}
}

func (h *House) doorDescription() string {
	h.mu.RLock()
	name, owner, ownerName, size := h.Name, h.OwnerID, h.OwnerName, h.Size
	h.mu.RUnlock()
	rent := h.GetRent()

	var b strings.Builder
	if owner != 0 {
		fmt.Fprintf(&b, "It belongs to house '%s'. %s owns this house.", name, ownerName)
	} else {
		fmt.Fprintf(&b, "It belongs to house '%s'. Nobody owns this house.", name)
	}

	// With the cyclopedia auction on — the default — C++ stops here and leaves the
	// size, price and rent to the auction window.
	if config.Bool("toggleCyclopediaHouseAuction", true) {
		return b.String()
	}

	fmt.Fprintf(&b, " It is %d square meters.", size)
	price := h.GetPrice()
	if config.Bool("housePurchasedShowPrice", false) || owner == 0 {
		fmt.Fprintf(&b, " It costs %s gold coins.", formatNumber(uint64(price)))
	}
	if period := strings.ToLower(config.Str("houseRentPeriod", "never")); period != "never" {
		fmt.Fprintf(&b, " The rent cost is %s gold coins and it is billed %s.",
			formatNumber(uint64(rent)), period)
	}
	return b.String()
}

// formatNumber groups digits in threes with commas, as the C++ helper of the same
// name does for prices in item and house descriptions.
func formatNumber(n uint64) string {
	s := strconv.FormatUint(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	lead := len(s) % 3
	if lead > 0 {
		b.WriteString(s[:lead])
	}
	for i := lead; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
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

// tryTransferOwnership is House::tryTransferOwnership (house.cpp:72).
//
// The furniture goes to the departing owner's depot FIRST, before anyone is
// kicked and before the access lists are cleared. Order matters: kicking first
// would move the owner off their own tiles, and clearing the lists first would
// make the sweep look at a house with no owner to send the items to.
func (h *House) tryTransferOwnership(w *World, player *Player, serverStartup bool) {
	if player != nil {
		h.TransferToDepotFor(w, player)
	} else {
		h.TransferToDepot(w)
	}
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
	// house.cpp:67 clears every door's list too. Leaving them behind hands the
	// next owner a set of doors the previous owner's friends can still open.
	h.doorLists = nil
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

// RemoveMapItem takes an item off a tile and runs the removal hooks that go
// with it. It is Game::internalRemoveItem's call into Item::onRemoved.
//
// Right now the only hook is the house door one: Door::onRemoved
// (house.cpp:848) drops the door from its house, and without it a house keeps
// a door in its list after the item is gone — so getDoorByNumber still finds
// it and getAccessList still opens an editing window for a door that no longer
// exists.
func (w *World) RemoveMapItem(pos Position, item *Item) bool {
	if w == nil || w.Map == nil || item == nil {
		return false
	}
	removed := w.Map.RemoveItemPtr(pos, item)
	if removed && item.Attr != nil && item.Attr.HouseDoorID != nil {
		if house := w.GetHouseByPosition(pos); house != nil {
			house.RemoveDoor(*item.Attr.HouseDoorID)
		}
	}
	return removed
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

// AccessHouseLevel_t (src/map/map_definitions.hpp:19-22).
type AccessHouseLevel uint8

const (
	HouseNotInvited AccessHouseLevel = 0
	HouseGuest      AccessHouseLevel = 1
	HouseSubOwner   AccessHouseLevel = 2
	HouseOwner      AccessHouseLevel = 3
)

// GetHouseAccessLevel is House::getHouseAccessLevel (src/map/house/house.cpp:
// 183-211). A nil player is HOUSE_OWNER, as in C++, because the callers that pass
// nil are the server acting on its own behalf.
func (h *House) GetHouseAccessLevel(p *Player) AccessHouseLevel {
	if p == nil {
		return HouseOwner
	}
	h.mu.RLock()
	owner, ownerAccount := h.OwnerID, h.OwnerAccountID
	h.mu.RUnlock()

	if config.Bool("houseOwnedByAccount", false) && ownerAccount != 0 && ownerAccount == p.AccountID {
		return HouseOwner
	}
	if p.CanEditHouses() {
		return HouseOwner
	}
	if p.DBID == owner {
		return HouseOwner
	}
	if h.IsSubOwner(p.Name) {
		return HouseSubOwner
	}
	if h.IsGuest(p.Name) {
		return HouseGuest
	}
	return HouseNotInvited
}

// IsInvited is House::isInvited (house.hpp:118).
func (h *House) IsInvited(p *Player) bool {
	return h.GetHouseAccessLevel(p) != HouseNotInvited
}

// CanPlayerUseDoor is Door::canUse (src/map/house/house.cpp:819-829): sub-owner
// and above always, otherwise the door's OWN access list.
//
// That last list is per door, not per house — House::setAccessList routes any
// list id that is neither GUEST_LIST nor SUBOWNER_LIST to the door with that
// number. So a plain house guest does not open a door unless that specific door
// names them, which is upstream behaviour and not an oversight here.
//
// game.HouseDoor carries an id, a lock flag and a level, with no list, so the
// fallback is always empty — the same answer C++ gives for a door nobody has
// called setAccessList on. Modelling per-door lists would change only the case
// where a datapack sets one.
func (h *House) CanPlayerUseDoor(p *Player) bool {
	if h == nil {
		return true // Door::canUse returns true for a door with no house.
	}
	return h.GetHouseAccessLevel(p) >= HouseSubOwner
}

// GetDoorHouse returns the house a door at pos belongs to. C++ keeps a back
// pointer on the Door because House::addDoor sets it from the house's own tiles;
// here the tile is the association.
//
// This replaces a lookup by door id, which could not work: house door ids are
// numbered per house, so scanning every house for id 1 answered with whichever of
// the 1086 houses Go's randomised map iteration reached first.
func (w *World) GetDoorHouse(pos Position) *House {
	tile := w.Map.GetTile(pos)
	if tile == nil || tile.HouseID == 0 {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Houses[uint32(tile.HouseID)]
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
// RegisterHouseTiles claims every OTBM tile carrying a house id, and picks up
// the doors and beds standing on it.
//
// It used to append to HouseTiles directly. Going through House::addTile also
// sets the tile.s protection-zone flag and keeps Size in step, and the door and
// bed sweeps are what House::addDoor and addBed exist for — without them a
// house had no doors to hang an access list on and reported the XML.s bed count
// rather than the beds actually in it.
func (w *World) RegisterHouseTiles() {
	type claim struct {
		h   *House
		pos Position
		t   *Tile
	}
	var claims []claim

	w.mu.Lock()
	w.Map.Range(func(pos Position, t *Tile) bool {
		if t.HouseID == 0 {
			return true
		}
		if h := w.Houses[t.HouseID]; h != nil {
			claims = append(claims, claim{h: h, pos: pos, t: t})
		}
		return true
	})
	w.mu.Unlock()

	// AddTile takes the house lock and touches the map, so it runs outside the
	// world lock rather than inside Range.
	for _, c := range claims {
		c.h.AddTile(w, c.pos)
		w.registerHouseFurniture(c.h, c.pos, c.t)
	}
}

// registerHouseFurniture finds the doors and beds on a house tile.
func (w *World) registerHouseFurniture(h *House, pos Position, t *Tile) {
	if w.Items == nil {
		return
	}
	for _, item := range t.Items {
		if item == nil {
			continue
		}
		it := w.Items.Get(item.ID)
		if it == nil {
			continue
		}
		if it.Type == items.ItemTypeBed {
			h.AddBed(pos)
			continue
		}
		if item.Attr != nil && item.Attr.HouseDoorID != nil {
			h.AddDoor(w, HouseDoor{ID: *item.Attr.HouseDoorID, Pos: pos})
		}
	}
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
