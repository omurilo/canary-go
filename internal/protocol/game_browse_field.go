package protocol

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// sendBrowseField opens a temporary container showing all items on a tile.
// Mirrors C++ Container::createBrowseField + Game::playerBrowseField.
func (g *GameProtocol) sendBrowseField(pos game.Position) {
	if g.player == nil {
		return
	}

	tile := g.deps.World.Map.GetTile(pos)
	if tile == nil {
		return
	}

	// Collect items from the tile (C++ uses tile->getItemList() which excludes Ground).
	var items []*game.Item
	for i := len(tile.Items) - 1; i >= 0; i-- {
		it := tile.Items[i]
		if it == nil {
			continue
		}
		// Skip items with UniqueID (quest items)
		if it.Attr != nil && it.Attr.UniqueID != nil {
			continue
		}
		items = append(items, it)
	}

	// Create the browse field container (ID 470, like C++ ITEM_BROWSEFIELD)
	browseContainer := &game.Item{
		ID:        game.ItemBrowseField,
		Container: game.NewContainer(30),
	}
	browseContainer.Container.Contents = items
	browseContainer.Container.Pagination = false
	browseContainer.Container.MaxItems = 30

	// Calculate dummy container ID from position (C++ logic)
	dummyCID := uint8(0xF - ((pos.X % 3) * 3 + (pos.Y % 3)))

	// Close any existing container at this dummy CID
	if existing := g.player.GetContainerByID(dummyCID); existing != nil {
		g.player.CloseContainer(dummyCID)
		g.sendCloseContainer(dummyCID)
	}

	// Register in world browse field map (C++ browseFields[shared_ptr<Tile>] = weak_ptr<Container>)
	g.deps.World.BrowseFieldSet(pos, browseContainer)

	// Register and send the container
	g.player.OpenContainerAtWithPos(dummyCID, browseContainer, pos, true)
	g.sendContainer(dummyCID, browseContainer, false)
}

// sendCloseContainer sends the container close packet (0x6F).
func (g *GameProtocol) sendCloseContainer(cid uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0x6F)
	w.AddByte(cid)
	// In protocol 1525+, there is no content revision
	g.SendToClient(w)
}
