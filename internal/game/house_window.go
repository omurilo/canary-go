package game

import "github.com/omurilo/canary-go/internal/netmsg"

// The house access-list editing window, ported from Player::getEditHouse /
// setEditHouse / sendHouseWindow (src/creatures/players/player.cpp:3057, :3063,
// :2154) and Game::playerUpdateHouseWindow (src/game/game.cpp:5765).
//
// None of this existed. player:sendHouseWindow and player:setEditHouse both
// returned true without doing anything, and the 0x8A handler read the packet
// and discarded it — so House::setAccessList, getAccessList and
// canEditAccessList were all ported and all unreachable. In play that means the
// "edit list" option on every house door silently did nothing: the window never
// opened, and if it had, the text would never have been saved.

// SetEditHouse is Player::setEditHouse (player.cpp:3063).
//
// The window id is incremented on every open, and that counter is the whole
// anti-replay mechanism: the update handler refuses a reply whose id does not
// match the one currently outstanding, so a stale window cannot overwrite a
// list the player edited since.
func (p *Player) SetEditHouse(house *House, listID uint32) {
	p.windowTextID++
	p.editHouse = house
	p.editListID = listID
}

// GetEditHouse is Player::getEditHouse (player.cpp:3057): the house being
// edited plus the window id and list id that go with it.
func (p *Player) GetEditHouse() (house *House, windowTextID, listID uint32) {
	return p.editHouse, p.windowTextID, p.editListID
}

// SendHouseWindow is Player::sendHouseWindow (player.cpp:2154).
//
// A list the house does not have is not sent at all — getAccessList's bool is
// load-bearing, because opening an empty window for a door id that does not
// exist would let the reply create one.
func (p *Player) SendHouseWindow(house *House, listID uint32) bool {
	if house == nil || p.Session == nil {
		return false
	}
	text, ok := house.GetAccessList(listID)
	if !ok {
		return false
	}
	w := netmsg.NewWriter()
	w.AddByte(0x97)
	w.AddByte(0x00)
	w.AddU32(p.windowTextID)
	w.AddString(text)
	p.Session.SendToClient(w)
	return true
}

// UpdateHouseWindow is Game::playerUpdateHouseWindow (game.cpp:5765): the
// player closed the window, so save the text if they are still allowed to.
//
// The permission is re-checked here rather than trusted from when the window
// opened, because ownership can change while it is open. listID from the packet
// must be 0 — upstream compares the packet's byte against zero and uses its own
// stored list id for the write, so a crafted packet cannot redirect the text at
// a different list.
func (p *Player) UpdateHouseWindow(w *World, listID uint8, windowTextID uint32, text string) {
	house, internalWindowTextID, internalListID := p.GetEditHouse()
	if house != nil && house.CanEditAccessList(internalListID, p) &&
		internalWindowTextID == windowTextID && listID == 0 {
		house.SetAccessList(w, internalListID, text)
	}
	p.SetEditHouse(nil, 0)
}
