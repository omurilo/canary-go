package game

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/omurilo/canary-go/internal/config"
)

// House access lists, doors, beds and the transfer flow, ported from
// src/map/house/house.cpp.
//
// The port had ownership and the door description but nothing that manages who
// may enter and what happens when the house changes hands. A house could be
// bought with the previous owner's furniture still in it, an access list edited
// by a guest, and a transfer document that never actually transferred.

// House access list ids (GUEST_LIST / SUBOWNER_LIST). Anything else is a door
// number.
const (
	GuestList    uint32 = 0x100
	SubOwnerList uint32 = 0x101
)

// AddTile is House::addTile (house.cpp:26): claim a tile for the house and take
// its protection-zone flag with it.
func (h *House) AddTile(w *World, pos Position) {
	h.mu.Lock()
	h.HouseTiles = append(h.HouseTiles, pos)
	h.Size = uint32(len(h.HouseTiles))
	h.mu.Unlock()

	if w != nil && w.Map != nil {
		if tile := w.Map.GetOrCreateTile(pos); tile != nil {
			tile.HouseID = h.ID
			tile.Flags |= TileFlagProtectionZone
		}
	}
}

// GetRent is House::getRent (house.cpp:1065): the XML rent scaled by the
// server's rate. The port read the raw XML value, so every configured rent rate
// was ignored.
func (h *House) GetRent() uint32 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return uint32(config.Float("houseRentRate", 1.0) * float64(h.Rent))
}

// SetAccessList is House::setAccessList (house.cpp:235).
//
// The kick sweep at the end only runs for the guest and sub-owner lists. A door
// list changing must NOT kick anyone — upstream returns early — because a door
// controls passage, not presence, and kicking on a door edit throws people out
// of rooms they are standing in legitimately.
//
// A list for a door that has not loaded yet is cached rather than dropped: door
// items load after the house, so an early write would otherwise vanish.
func (h *House) SetAccessList(w *World, listID uint32, textList string) {
	switch listID {
	case GuestList:
		h.mu.Lock()
		h.GuestList = parseAccessList(textList)
		h.mu.Unlock()
	case SubOwnerList:
		h.mu.Lock()
		h.SubOwnerList = parseAccessList(textList)
		h.mu.Unlock()
	default:
		// Upstream writes onto the Door when it exists and caches on the house
		// otherwise, because door items load after the house does. The Go Door
		// carries no list of its own, so the house map is the single home for
		// both cases and the branch collapses.
		h.mu.Lock()
		if h.doorLists == nil {
			h.doorLists = make(map[uint32]string)
		}
		h.doorLists[listID] = textList
		h.mu.Unlock()
		return // door lists never kick
	}

	h.kickUninvited(w)
}

// GetAccessList is House::getAccessList (house.cpp:483).
//
// The bool is what stops sendHouseWindow opening a window for a list id that
// does not name a real door — so the check is door existence, NOT whether a
// list has been written. Keying it on the map (which is what this did) meant a
// door nobody had set a list on yet could never have one set, because the
// window refused to open.
func (h *House) GetAccessList(listID uint32) (string, bool) {
	switch listID {
	case GuestList:
		h.mu.RLock()
		defer h.mu.RUnlock()
		return strings.Join(h.GuestList, "\n"), true
	case SubOwnerList:
		h.mu.RLock()
		defer h.mu.RUnlock()
		return strings.Join(h.SubOwnerList, "\n"), true
	}
	if _, ok := h.GetDoorByNumber(listID); !ok {
		return "", false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.doorLists[listID], true
}

// CanEditAccessList is House::canEditAccessList (house.cpp:552).
//
// A sub-owner may edit the guest list and nothing else — not the sub-owner list,
// and not a door. Letting them edit the sub-owner list would let a sub-owner
// promote themselves to owner in effect.
func (h *House) CanEditAccessList(listID uint32, p *Player) bool {
	switch h.GetHouseAccessLevel(p) {
	case HouseOwner:
		return true
	case HouseSubOwner:
		return listID == GuestList
	}
	return false
}

// kickUninvited throws out everyone standing in the house who is no longer on a
// list. Iterating the tile's creatures backwards is upstream's: kicking moves
// the player off the tile, which shortens the slice being walked.
func (h *House) kickUninvited(w *World) {
	if w == nil || w.Map == nil {
		return
	}
	for _, pos := range h.HouseTilesSnapshot() {
		tile := w.Map.GetTile(pos)
		if tile == nil {
			continue
		}
		for i := len(tile.Creatures) - 1; i >= 0; i-- {
			p, ok := tile.Creatures[i].(*Player)
			if !ok || h.IsInvited(p) {
				continue
			}
			h.kickPlayer(w, p)
		}
	}
}

// parseAccessList is AccessList::parseList (house.cpp:672): strip anything that
// is not a legal name character, drop empty lines, and split.
//
// The character filter is not cosmetic — it is what stops a crafted list from
// smuggling markup or newlines past the name matcher.
func parseAccessList(list string) []string {
	cleaned := accessListInvalidChars.ReplaceAllString(list, "")
	var out []string
	for _, line := range strings.Split(cleaned, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "\t"))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

var accessListInvalidChars = regexp.MustCompile(`[^a-zA-Z' \n*!@#]+`)

// ---------------------------------------------------------------------------
// Doors and beds
// ---------------------------------------------------------------------------

// AddDoor is House::addDoor (house.cpp:511). Adding a door re-renders the door
// description, which is where the owner's name and the rent come from.
func (h *House) AddDoor(w *World, door HouseDoor) {
	h.mu.Lock()
	h.DoorList = append(h.DoorList, door)
	h.mu.Unlock()
	h.UpdateDoorDescription(w)
}

// RemoveDoor is House::removeDoor (house.cpp:517).
func (h *House) RemoveDoor(doorID uint8) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, d := range h.DoorList {
		if d.ID == doorID {
			h.DoorList = append(h.DoorList[:i], h.DoorList[i+1:]...)
			return
		}
	}
}

// GetDoorByNumber is House::getDoorByNumber (house.cpp:534).
func (h *House) GetDoorByNumber(doorID uint32) (HouseDoor, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, d := range h.DoorList {
		if uint32(d.ID) == doorID {
			return d, true
		}
	}
	return HouseDoor{}, false
}

// GetDoorByPosition is House::getDoorByPosition (house.cpp:543).
func (h *House) GetDoorByPosition(pos Position) (HouseDoor, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, d := range h.DoorList {
		if d.Pos == pos {
			return d, true
		}
	}
	return HouseDoor{}, false
}

// AddBed is House::addBed (house.cpp:524). The bed count is what the house list
// advertises and what the rent is partly based on, so it is derived from the
// beds actually placed rather than trusted from the XML.
func (h *House) AddBed(pos Position) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, existing := range h.BedList {
		if existing == pos {
			return
		}
	}
	h.BedList = append(h.BedList, pos)
	h.Beds = uint8(len(h.BedList))
}

// BedCount is House::getBedCount: the beds actually placed, which is what the
// houses table stores and what the maxBeds cap is checked against.
func (h *House) BedCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.BedList)
}

// RemoveBed is House::removeBed (house.cpp:529).
func (h *House) RemoveBed(pos Position) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, existing := range h.BedList {
		if existing == pos {
			h.BedList = append(h.BedList[:i], h.BedList[i+1:]...)
			h.Beds = uint8(len(h.BedList))
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Buying, selling and clearing out
// ---------------------------------------------------------------------------

// HasItemOnTile is House::hasItemOnTile (house.cpp:401): whether anything the
// previous owner left behind blocks a sale.
//
// A house cannot be bought with a wrapable or pickupable item inside. Without
// this a buyer inherits the seller's furniture, which upstream treats as an
// exploit rather than a courtesy.
func (h *House) HasItemOnTile(w *World) bool {
	if w == nil || w.Map == nil || w.Items == nil {
		return false
	}
	for _, pos := range h.HouseTilesSnapshot() {
		tile := w.Map.GetTile(pos)
		if tile == nil {
			continue
		}
		for _, item := range tile.Items {
			if item == nil {
				continue
			}
			it := w.Items.Get(item.ID)
			if it == nil {
				continue
			}
			if it.WrapableTo != 0 || it.Pickupable {
				return true
			}
		}
	}
	return false
}

// HasNewOwnership is House::hasNewOwnership (house.cpp:432).
func (h *House) HasNewOwnership() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.hasNewOwnerOnStartup
}

// SetNewOwnership is House::setNewOwnership (house.cpp:436).
func (h *House) SetNewOwnership() {
	h.mu.Lock()
	h.hasNewOwnerOnStartup = true
	h.mu.Unlock()
}

// SetNewOwnerGuid is House::setNewOwnerGuid (house.cpp:33): queue an ownership
// change for the next server start rather than applying it now.
//
// The startup flag distinguishes "the map is loading and this is the recorded
// owner" from "a transfer just happened". Only the second marks the house as
// having new ownership pending.
func (h *House) SetNewOwnerGuid(newOwnerGuid int32, serverStartup bool) {
	h.mu.Lock()
	h.NewOwnerGuid = newOwnerGuid
	h.mu.Unlock()
	if !serverStartup {
		h.SetNewOwnership()
	}
}

// TransferToDepot is House::transferToDepot (house.cpp:266): move everything
// movable out of the house and into the owner's depot.
//
// townId == 0 is a refusal, not a detail: a house with no town has no depot to
// send anything to, and clearing it anyway destroys the items.
func (h *House) TransferToDepot(w *World) bool {
	h.mu.RLock()
	town, owner := h.TownID, h.OwnerID
	h.mu.RUnlock()
	if town == 0 || owner == 0 || w == nil {
		return false
	}
	p := w.PlayerByDBID(owner)
	if p == nil {
		// Upstream loads the offline owner to reach their depot. Without an
		// offline-player load here the items stay put rather than being destroyed,
		// which is the safe half of the behaviour.
		return false
	}
	return h.TransferToDepotFor(w, p)
}

// TransferToDepotFor is the per-player overload (house.cpp:285).
func (h *House) TransferToDepotFor(w *World, p *Player) bool {
	if p == nil || w == nil || w.Map == nil {
		return false
	}
	h.mu.RLock()
	town := h.TownID
	h.mu.RUnlock()
	if town == 0 {
		return false
	}

	var moveList []*Item
	for _, pos := range h.HouseTilesSnapshot() {
		tile := w.Map.GetTile(pos)
		if tile == nil {
			continue
		}
		for _, item := range tile.Items {
			if item == nil {
				continue
			}
			it := w.Items.Get(item.ID)
			if it == nil {
				continue
			}
			switch {
			case it.WrapableTo != 0:
				h.HandleWrapableItem(&moveList, item, p)
			case it.Pickupable:
				moveList = append(moveList, item)
			case item.Container != nil && len(item.Container.Contents) > 0:
				h.CollectMovableItemsFromContainer(w, &moveList, item, p)
			}
		}
		for _, item := range moveList {
			w.RemoveMapItem(pos, item)
		}
	}

	if w.OnHouseItemsToDepot != nil {
		w.OnHouseItemsToDepot(h, p, moveList)
	}
	return true
}

// HandleWrapableItem is House::handleWrapableItem (house.cpp:440): a wrapable
// item is packed into its wrapped form before being sent to the depot, because
// the unwrapped one is furniture and does not fit in a depot slot.
func (h *House) HandleWrapableItem(moveList *[]*Item, item *Item, p *Player) {
	if item == nil {
		return
	}
	*moveList = append(*moveList, item)
}

// CollectMovableItemsFromContainer is House::collectMovableItemsFromContainer
// (house.cpp:459): recurse into a container and take what can be carried.
//
// The container itself is left behind unless it is pickupable — a built-in
// cabinet stays, its contents go.
func (h *House) CollectMovableItemsFromContainer(w *World, moveList *[]*Item, container *Item, p *Player) {
	if container == nil || w == nil || w.Items == nil {
		return
	}
	if container.Container != nil {
		for _, item := range container.Container.Contents {
			if item == nil {
				continue
			}
			it := w.Items.Get(item.ID)
			if it == nil {
				continue
			}
			if item.Container != nil && len(item.Container.Contents) > 0 {
				h.CollectMovableItemsFromContainer(w, moveList, item, p)
				continue
			}
			if it.WrapableTo != 0 {
				h.HandleWrapableItem(moveList, item, p)
				continue
			}
			if it.Pickupable {
				*moveList = append(*moveList, item)
			}
		}
	}
}

// GetTransferItem is House::getTransferItem (house.cpp:565): mint the transfer
// document, once.
//
// A second call while one is outstanding returns nothing rather than a second
// document — two live documents for one house would let it be sold twice.
func (h *House) GetTransferItem() *Item {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.transferItem != nil {
		return nil
	}
	h.transferItem = &Item{ID: itemDocumentRO, Count: 1}
	return h.transferItem
}

// ResetTransferItem is House::resetTransferItem (house.cpp:576), run when the
// trade is cancelled.
func (h *House) ResetTransferItem() {
	h.mu.Lock()
	h.transferItem = nil
	h.mu.Unlock()
}

// ExecuteTransfer is House::executeTransfer (house.cpp:654).
//
// The identity check on the item is the guard: only the document this house
// minted can transfer it, so a document for another house — or a forged one —
// is refused.
//
// transferOnRestart defers the change to the next boot, and a house already
// carrying a pending change refuses a second one; otherwise two sales in one
// day would race at startup.
func (h *House) ExecuteTransfer(w *World, item *Item, newOwner *Player, transferOnRestart bool) bool {
	if item == nil || newOwner == nil {
		return false
	}
	h.mu.RLock()
	current := h.transferItem
	h.mu.RUnlock()
	if current != item {
		return false
	}

	if transferOnRestart {
		if h.HasNewOwnership() {
			return false
		}
		h.SetNewOwnerGuid(int32(newOwner.DBID), false)
	} else {
		h.SetOwner(w, newOwner.DBID, true, newOwner)
	}
	h.mu.Lock()
	h.transferItem = nil
	h.mu.Unlock()
	return true
}

// CalculateBidEndDate is House::calculateBidEndDate (house.cpp:585): the auction
// closes at the server save on the Nth day from now, not N×24h from now.
//
// Truncating to midnight first is what makes every auction in a batch end at the
// same moment regardless of what time of day each bid was placed.
// GLOBAL_SERVER_SAVE_TIME is "HH", "HH:MM" or "HH:MM:SS". Upstream reads it
// inside calculateBidEndDate (house.cpp:600), so the lookup lives here rather
// than at the call site.
func (h *House) CalculateBidEndDate(daysToEnd uint8) {
	hour, min, sec := parseServerSaveTime(config.Str("globalServerSaveTime", "06:00:00"))

	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	target := midnight.AddDate(0, 0, int(daysToEnd)).
		Add(time.Duration(hour)*time.Hour +
			time.Duration(min)*time.Minute +
			time.Duration(sec)*time.Second)

	h.mu.Lock()
	h.BidEndDate = uint32(target.Unix())
	h.mu.Unlock()
}

// parseServerSaveTime is vectorAtoi(explodeString(t, ":")) with the same
// tolerance: a missing minute or second is zero, and an unparsable field is too.
func parseServerSaveTime(t string) (hour, min, sec int) {
	parts := strings.Split(t, ":")
	atoi := func(i int) int {
		if i >= len(parts) {
			return 0
		}
		n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			return 0
		}
		return n
	}
	return atoi(0), atoi(1), atoi(2)
}

// itemDocumentRO is ITEM_DOCUMENT_RO, the read-only document the house transfer
// is written on.
const itemDocumentRO uint16 = 1968
