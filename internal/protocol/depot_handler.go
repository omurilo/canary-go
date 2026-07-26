package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// handleDepotLocker handles opening a depot locker. When a player clicks on a depot
// locker in the game world (item IDs 3497-3500), this opens the player's depot for
// their current town, creating depot chests as needed.
func (g *GameProtocol) handleDepotLocker(worldLocker *game.Item, pos netmsg.Position, index uint8) {
	if g.player == nil || g.player.TownID == 0 {
		return
	}

	// Initialize depot manager if needed
	if g.player.DepotManager == nil {
		g.player.DepotManager = game.NewPlayerDepotManager(g.player)
	}

	// Get or create the depot locker for the player's current town
	depotLocker := g.player.DepotManager.GetDepotLocker(g.player.TownID)
	if depotLocker == nil {
		return
	}

	// Use the world locker's ID for the visual (3497-3500)
	// but the depot contents come from the player's depot
	depotLocker.ID = worldLocker.ID

	// Check if it's already open
	if cid := g.player.GetContainerID(depotLocker); cid != -1 {
		// Already open, close it
		g.player.CloseContainer(uint8(cid))
		w := netmsg.NewWriter()
		w.AddByte(opContainerClose)
		w.AddByte(uint8(cid))
		g.SendToClient(w)
		return
	}

	// Open the depot locker
	var isOnMap bool
	var containerPos game.Position

	if pos.X != 0xFFFF {
		// Depot locker is on the map
		isOnMap = true
		containerPos = game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
	}

	// Open the depot locker container
	g.player.OpenContainerAtWithPos(index, depotLocker, containerPos, isOnMap)
	g.sendDepotContainer(index, depotLocker, worldLocker, depotLocker.Parent != nil)
}

// sendDepotContainer sends a depot locker container window with special depot handling.
func (g *GameProtocol) sendDepotContainer(cid uint8, depotLocker *game.Item, worldLocker *game.Item, hasParent bool) {
	// Use world locker for the icon (visual appearance)
	displayItem := worldLocker
	t := g.deps.Items.Get(displayItem.ID)
	name := "Depot Chest"
	if t != nil && t.Name != "" {
		name = t.Name
	}

	contents := depotLocker.Contents
	capacity := len(contents)
	if capacity < 4 {
		capacity = 4 // usually holds Market, Inbox, Stash, and the nested Depot Chest
	}

	// Ensure we have at least the contents we're showing
	if capacity < 1 {
		capacity = 1
	}
	if capacity > 0xFF {
		capacity = 0xFF
	}

	unlocked := byte(1)
	pagination := byte(0) // Depot lockers don't paginate
	firstIndex := uint16(0)
	page := len(contents)
	if page > 0xFF {
		page = 0xFF
	}

	w := netmsg.NewWriter()
	w.AddByte(opContainerOpen)
	w.AddByte(cid)
	g.addItem(w, displayItem) // the world locker for visual (3497-3500)
	w.AddString(name)
	w.AddByte(byte(capacity))
	w.AddByte(boolByte(hasParent))
	w.AddByte(1) // depot search available
	w.AddByte(unlocked)
	w.AddByte(pagination)
	w.AddU16(uint16(len(contents)))
	w.AddU16(firstIndex)
	w.AddByte(byte(page))
	for i := 0; i < page; i++ {
		g.addItem(w, contents[i])
	}
	// 13.21+ trailer for a depot container
	w.AddByte(0x00)
	w.AddByte(0x00)
	w.AddByte(0) // not movable
	w.AddByte(0) // not held by a player
	g.SendToClient(w)
}
