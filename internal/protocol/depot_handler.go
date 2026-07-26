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
	depotLocker.Item.ID = worldLocker.ID

	// The depot locker is a special container that shows depot chests (boxes)
	// We'll open the depot locker itself, which contains the depot chests

	// Check if it's already open
	if cid := g.player.GetContainerID(depotLocker.Item); cid != -1 {
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

	// Ensure depot has at least one chest (depot chest 1)
	firstChest := depotLocker.GetOrCreateDepotChest(0)
	if firstChest != nil {
		// Add first chest to locker contents if not already there
		hasFirstChest := false
		for _, c := range depotLocker.Item.Contents {
			if c == firstChest {
				hasFirstChest = true
				break
			}
		}
		if !hasFirstChest {
			depotLocker.Item.Contents = append(depotLocker.Item.Contents, firstChest)
		}
	}

	// Open the depot locker container
	g.player.OpenContainerAtWithPos(index, depotLocker.Item, containerPos, isOnMap)
	g.sendDepotContainer(index, depotLocker, worldLocker, depotLocker.Item.Parent != nil)
}

// sendDepotContainer sends a depot locker container window with special depot handling.
func (g *GameProtocol) sendDepotContainer(cid uint8, depotLocker *game.DepotLocker, worldLocker *game.Item, hasParent bool) {
	item := depotLocker.Item
	// Use world locker for the icon (visual appearance)
	displayItem := worldLocker
	t := g.deps.Items.Get(displayItem.ID)
	name := "Depot Chest"
	if t != nil && t.Name != "" {
		name = t.Name
	}

	contents := item.Contents
	capacity := 17 // Depot lockers can hold up to 17 depot chests

	// Ensure we have at least the contents we're showing
	if capacity < len(contents) {
		capacity = len(contents)
	}
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
