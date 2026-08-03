package protocol

import (
	"log/slog"
	"strings"
	"time"

	"github.com/omurilo/canary-go/internal/actions"
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// Container-related opcodes.
const (
	opContainerOpen  = 0x6E
	opContainerClose = 0x6F
)

// parseUseItem handles a use-item request (0x82). For now only map positions and
// the "open a container" outcome are supported, mirroring the container branch of
// Game::playerUseItem. Layout: position, itemId u16, stackpos u8, index u8.
func (g *GameProtocol) parseUseItem(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	pos := r.GetPosition()
	itemID := r.GetU16()
	stackpos := r.GetByte() // stackpos
	index := r.GetByte()    // index

	g.useItem(pos, itemID, stackpos, index)
}

// useItem is the body of Game::playerUseItem, split from the packet read so an
// out-of-reach use can walk over and run again.
func (g *GameProtocol) useItem(pos netmsg.Position, itemID uint16, stackpos, index uint8) {
	if g.player == nil {
		return
	}
	item := g.getItemAt(pos, itemID, stackpos)
	if item == nil {
		return
	}
	gamePos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}

	// Actions::canUse (src/lua/creature/actions.cpp:179-191): a plain use has to be
	// within arm's reach, and out of reach means walk there — not refuse. The
	// square-8 guard this replaces allowed a use from across the screen and said
	// nothing at all beyond that.
	if ret := g.actionCanUse(gamePos); ret != retNoError {
		if ret == retTooFarAway && g.walkToThenRetry(gamePos, func() {
			g.useItem(pos, itemID, stackpos, index)
		}) {
			return
		}
		g.sendCancelMessage(ret.message())
		return
	}

	t := g.deps.Items.Get(item.ID)
	if t == nil {
		return
	}

	// Door access comes FIRST, before any action runs — Actions::internalUseItem
	// opens with it (src/lua/creature/actions.cpp:260-264). The check used to sit
	// far below, after the Lua-action branch had already returned, so it never ran
	// for a door: every door in the datapack has an action registered by
	// custom_door.lua, and locked house doors admitted anyone.
	if t.IsDoor && pos.X != 0xFFFF {
		house := g.deps.World.GetDoorHouse(gamePos)
		if house != nil && !house.CanPlayerUseDoor(g.player) {
			g.sendCancelMessage("You cannot use this object.")
			return
		}
	}

	// Execute Lua action first
	action := actions.FindAction(item, game.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	if action != nil {
		isEx := g.isExAction(item)
		if isEx {
			if !g.player.CanDoPotionAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		} else {
			if !g.player.CanDoAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		}

		gamePos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
		originalPos := g.player.Pos
		beforeCount := item.Count
		if g.deps.Lua.CallAction(action, g.player, item, gamePos, nil, gamePos, false) {
			if isEx {
				g.player.SetNextPotionAction(1000 * time.Millisecond)
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(1000)
			} else {
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(200)
			}

			// If the script consumed/changed the item (e.g. food, runes calling
			// item:remove), reflect it on the client.
			if item.Count != beforeCount {
				g.reconcileUsedItem(item, pos, stackpos)
			}
			if g.player.Pos != originalPos {
				teleportedTo := g.player.Pos
				g.player.Pos = originalPos
				g.broadcastRemove(g.player)
				g.player.Pos = teleportedTo

				w := netmsg.NewWriter()
				w.AddByte(opFullMap)
				w.AddPosition(netmsg.Position{X: g.player.Pos.X, Y: g.player.Pos.Y, Z: g.player.Pos.Z})
				g.addMapDescription(w, int(g.player.Pos.X)-viewportX, int(g.player.Pos.Y)-viewportY, g.player.Pos.Z, mapWidth, mapHeight)
				g.SendToClient(w)
				g.broadcastAppear(g.player)
			}
			return // Handled by Lua script
		}
	}

	// Beds: using a free bed makes the player lie down and log out, mirroring
	// BedItem::sleep (src/items/bed.cpp). There is no datapack action for beds —
	// the behaviour lives in the item type — so this runs after the Lua-action
	// branch.
	if t.Type == items.ItemTypeBed {
		g.useBed(item, gamePos)
		return
	}

	// Fallback to FloorChange if the item has it
	if t.FloorChange != "" {
		teleportPos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
		// Typically, using a ladder/sewer drops you at the same X/Y but different Z,
		// but let's apply the floor change shift if any.
		switch t.FloorChange {
		case "down":
			teleportPos.Z++
		case "north":
			teleportPos.Z--
			teleportPos.Y--
		case "south":
			teleportPos.Z--
			teleportPos.Y++
		case "east":
			teleportPos.Z--
			teleportPos.X++
		case "west":
			teleportPos.Z--
			teleportPos.X--
		}

		g.broadcastRemove(g.player)
		g.deps.World.SetPosition(g.player, teleportPos)

		w := netmsg.NewWriter()
		w.AddByte(opFullMap)
		w.AddPosition(netmsg.Position{X: g.player.Pos.X, Y: g.player.Pos.Y, Z: g.player.Pos.Z})
		g.addMapDescription(w, int(g.player.Pos.X)-viewportX, int(g.player.Pos.Y)-viewportY, g.player.Pos.Z, mapWidth, mapHeight)
		g.SendToClient(w)

		g.broadcastAppear(g.player)
		return
	}

	// Special handling for depot lockers (item IDs 3497-3500)
	if item.ID >= 3497 && item.ID <= 3500 {
		g.handleDepotLocker(item, pos, index)
		return
	}

	// (The door access check that used to sit here was unreachable — the Lua-action
	// branch above returns for every door — and resolved the house by scanning all
	// of them for a per-house door id. It now runs before the action, keyed on the
	// tile.)

	// Market (B11) — item ID 12903 in the depot locker.
	if item.ID == game.ItemMarket {
		g.SendOpenMarket()
		return
	}
	// Gold pouch (ITEM_GOLD_POUCH = 23721) pode nao ser marcado como container no protobuf
	// Tratar explicitamente como container (C++ Container subclass)
	if item.ID == game.ItemGoldPouch || t.IsContainer() {
		if cid := g.player.GetContainerID(item); cid != -1 {
			g.player.CloseContainer(uint8(cid))
			w := netmsg.NewWriter()
			w.AddByte(opContainerClose)
			w.AddByte(uint8(cid))
			g.SendToClient(w)
			return
		}

		if pos.X == 0xFFFF {
			var isOnMap bool
			var containerPos game.Position
			if pos.Y >= 0x40 {
				parentCid := uint8(pos.Y - 0x40)
				if parentOc, ok := g.player.OpenContainersSnapshot()[parentCid]; ok {
					if parentOc.IsOnMap {
						isOnMap = true
						containerPos = parentOc.Position
					}
				}
			}
			// No toggle here: the already-open case is handled above and returns, so a
			// second check would be unreachable. (One was added here and looked like a
			// fix for the duplicate-window report; it never ran.)
			g.player.OpenContainerAtWithPos(index, item, containerPos, isOnMap)
			g.sendContainer(index, item, item.Parent != nil)
		} else {
			g.openContainerWithPos(item, game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}, true)
		}
	} else if t.FloorChange != "" {
		teleportPos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
		switch t.FloorChange {
		case "down":
			teleportPos.Z++
		case "north":
			teleportPos.Z--
			teleportPos.Y--
		case "south":
			teleportPos.Z--
			teleportPos.Y++
		case "east":
			teleportPos.Z--
			teleportPos.X++
		case "west":
			teleportPos.Z--
			teleportPos.X--
		case "southalt":
			teleportPos.Z--
			teleportPos.Y += 2
		case "eastalt":
			teleportPos.Z--
			teleportPos.X += 2
		}

		g.broadcastRemove(g.player)
		g.deps.World.SetPosition(g.player, teleportPos)

		w := netmsg.NewWriter()
		w.AddByte(opFullMap)
		w.AddPosition(netmsg.Position{X: teleportPos.X, Y: teleportPos.Y, Z: teleportPos.Z})
		g.addMapDescription(w, int(teleportPos.X)-viewportX, int(teleportPos.Y)-viewportY, teleportPos.Z, mapWidth, mapHeight)
		g.SendToClient(w)

		g.broadcastAppear(g.player)
	} else if (t.ForceUse || t.IsLadder) && pos.X != 0xFFFF {
		teleportPos := game.Position{X: pos.X, Y: pos.Y + 1, Z: pos.Z - 1}
		p := g.player

		g.broadcastRemove(p)

		g.deps.World.SetPosition(p, teleportPos)

		w := netmsg.NewWriter()
		w.AddByte(opFullMap)
		w.AddPosition(netmsg.Position{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z})
		g.addMapDescription(w, int(p.Pos.X)-viewportX, int(p.Pos.Y)-viewportY, p.Pos.Z, mapWidth, mapHeight)
		g.SendToClient(w)

		g.broadcastAppear(p)
	}
}

// useBed handles using a bed item (ItemTypeBed): the player lies down on the
// bed and logs out, mirroring BedItem::sleep (src/items/bed.cpp:168). A bed
// already occupied by someone else refuses the attempt with a poof; using the
// bed you sleep in wakes you up. The player's position is set to the bed before
// the logout, so their next login places them there to wake up.
func (g *GameProtocol) useBed(item *game.Item, pos game.Position) {
	world := g.deps.World
	sleeper := world.BedSleeper(pos)
	if sleeper != 0 && sleeper != g.player.DBID {
		if world.OnMagicEffect != nil {
			world.OnMagicEffect(pos, 2) // CONST_ME_POFF
		}
		g.sendCancelMessage("This bed is already in use.")
		return
	}
	if sleeper == g.player.DBID {
		// Waking up — transform every occupied part of this bed back to its free
		// variant and free the sleeper entries.
		for _, part := range world.PlayerBedParts(g.player.DBID) {
			if tile := world.Map.GetTile(part.Pos); tile != nil {
				for _, it := range tile.Items {
					if it == nil {
						continue
					}
					if t := g.deps.Items.Get(it.ID); t != nil && t.Type == items.ItemTypeBed {
						world.TransformItem(part.Pos, it, part.FreeID)
						break
					}
				}
			}
			world.RemoveBedSleeper(part.Pos)
		}
		return
	}

	// Claim the bed by the player's DB id (their "GUID", as C++ setBedSleeper
	// uses g_game().getGUID) — the creature id changes every session, so keying
	// on it would never match on a later login. Transform the free bed (and, for a
	// two-tile bed, its partner half) into the occupied variant so the client
	// shows the sleeper lying there (C++ BedItem::updateAppearance), then move the
	// player onto it, show the Zzz effect and log the player out at the bed;
	// addPlayer frees the sleeper when they come back.
	freeID := item.ID
	occupiedID := g.bedOccupiedVariant(freeID)
	world.SetBedSleeper(pos, g.player.DBID, freeID)
	if occupiedID != 0 {
		world.TransformItem(pos, item, occupiedID)
	}

	// Two-tile bed: transform the partner half too, and mark both parts as the
	// sleeper so the occupied check and wake cover either half.
	if t := g.deps.Items.Get(freeID); t != nil && t.BedPartOf != 0 {
		if partItem, partPos := findBedPart(world, pos, t.BedPartOf); partItem != nil {
			if partOcc := g.bedOccupiedVariant(partItem.ID); partOcc != 0 {
				world.TransformItem(partPos, partItem, partOcc)
			}
			world.SetBedSleeper(partPos, g.player.DBID, partItem.ID)
		}
	}

	g.broadcastRemove(g.player)
	world.SetPosition(g.player, pos)
	g.broadcastAppear(g.player)
	if world.OnMagicEffect != nil {
		world.OnMagicEffect(pos, 22) // CONST_ME_SLEEP
	}
	if g.conn != nil {
		g.conn.Close()
	}
}

// bedOccupiedVariant returns the occupied-bed variant id for a given bed type id
// and the player's sex, or 0 when the type has no such transform.
func (g *GameProtocol) bedOccupiedVariant(id uint16) uint16 {
	t := g.deps.Items.Get(id)
	if t == nil {
		return 0
	}
	if g.player.Sex == 1 { // PLAYERSEX_MALE
		return t.TransformToOnUseMale
	}
	return t.TransformToOnUseFemale
}

// findBedPart locates the partner half of a two-tile bed (an item with id
// partnerID) on a tile adjacent to center, returning it and its position.
func findBedPart(w *game.World, center game.Position, partnerID uint16) (*game.Item, game.Position) {
	offsets := [8][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}, {1, -1}, {1, 1}, {-1, 1}, {-1, -1}}
	for _, off := range offsets {
		p := game.Position{X: uint16(int(center.X) + off[0]), Y: uint16(int(center.Y) + off[1]), Z: center.Z}
		tile := w.Map.GetTile(p)
		if tile == nil {
			continue
		}
		for _, it := range tile.Items {
			if it != nil && it.ID == partnerID {
				return it, p
			}
		}
	}
	return nil, game.Position{}
}

// reconcileUsedItem updates the client after a use-action mutated an item's
// stack count (e.g. eating food, using a rune). When the stack is emptied the
// item is removed from its container/inventory slot; otherwise the reduced
// stack is re-sent. `pos` is the item's source location as sent by the client
// and `stackpos` is the map stack index (only used for map items).
func (g *GameProtocol) reconcileUsedItem(item *game.Item, pos netmsg.Position, stackpos uint8) {
	consumed := item.Count == 0
	if pos.X == 0xFFFF {
		if pos.Y == 0 && g.player != nil {
			foundSlot := uint8(0)
			for slot := uint8(1); slot <= 10; slot++ {
				if g.player.Inventory[slot] == item {
					foundSlot = slot
					break
				}
			}
			if foundSlot > 0 {
				pos.Y = uint16(foundSlot)
			} else {
				foundCID := uint8(255)
				foundContSlot := uint8(0)
				for cid := uint8(0); cid < 16; cid++ {
					if cont, _, ok := g.openContainerByCID(cid); ok {
						for i, contItem := range cont.Contents {
							if contItem == item {
								foundCID = cid
								foundContSlot = uint8(i)
								break
							}
						}
						if foundCID != 255 {
							break
						}
					}
				}
				if foundCID != 255 {
					pos.Y = uint16(0x40 + foundCID)
					pos.Z = uint8(foundContSlot)
				}
			}
		}
		if pos.Y >= 0x40 { // inside a container
			cid := uint8(pos.Y - 0x40)
			cont, offset, ok := g.openContainerByCID(cid)
			if !ok {
				return
			}
			slot := uint8(int(pos.Z) + offset)
			if !ok {
				return
			}
			if consumed {
				if int(slot) < len(cont.Contents) {
					cont.Contents = append(cont.Contents[:slot], cont.Contents[slot+1:]...)
				}
				g.sendRemoveContainerItem(cid, slot, nil)
				g.refreshContainerIfOpen(cont)
			} else {
				g.sendUpdateContainerItem(cid, slot, item)
			}
			return
		}
		// equipment slot
		slot := uint8(pos.Y)
		if slot == 0 || slot > 10 {
			return
		}
		if consumed {
			g.player.Inventory[slot] = nil
			g.sendInventoryEmpty(slot)
		} else {
			g.sendInventoryItem(slot, item)
		}
		return
	}
	// On the map.
	gp := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
	if consumed {
		g.deps.World.RemoveMapItem(gp, item)
		g.broadcastRemoveTileThing(gp, stackpos)
	} else {
		g.broadcastUpdateTileThing(gp, stackpos, item)
	}
}

// findTileItem returns the first ground/stacked item on the tile with the id.
func findTileItem(tile *game.Tile, id uint16) *game.Item {
	if tile.Ground != nil && tile.Ground.ID == id {
		return tile.Ground
	}
	for _, it := range tile.Items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

// openContainer assigns a client container id and sends the container window.
func (g *GameProtocol) openContainer(item *game.Item) {
	g.openContainerWithPos(item, game.Position{}, false)
}

// openContainerWithPos assigns a client container id, setting explicit Position / IsOnMap metadata.
func (g *GameProtocol) openContainerWithPos(item *game.Item, pos game.Position, isOnMap bool) {
	if g.player == nil {
		return
	}
	cid := g.player.AddContainerWithPos(item, pos, isOnMap)
	if cid < 0 {
		return // all 16 container slots in use
	}
	g.sendContainer(uint8(cid), item, item.Parent != nil)
}

// sendContainer sends the container window (0x6E), mirroring the modern layout of
// the original protocol.
func (g *GameProtocol) sendContainer(cid uint8, item *game.Item, hasParent bool) {
	t := g.deps.Items.Get(item.ID)
	name := "Container"
	movable := byte(0)
	if t != nil {
		if t.Name != "" {
			name = t.Name
		}
		if t.Pickupable {
			movable = 1
		}
	}
	contents := item.Contents
	// Force pagination for items C++ marks in Container constructor
	if item.ID == game.ItemGoldPouch || item.ID == game.ItemStoreInbox {
		item.Pagination = true
		if item.MaxSize == 0 {
			item.MaxSize = 32
		}
		if item.ID == game.ItemGoldPouch && item.MaxItems == 0 {
			item.MaxItems = 2000
		}
	}
	capacity := int(item.ContainerCapacity(g.deps.Items))
	if capacity < 1 {
		capacity = 1
	}
	if capacity > 0xFF {
		capacity = 0xFF
	}
	unlocked := byte(1) // drag & drop allowed unless explicitly locked
	pagination := boolByte(item.Pagination)
	firstIndex := uint16(0)
	if g.player != nil {
		firstIndex = g.player.GetContainerIndex(cid)
	}
	page := len(contents)
	if page > 0xFF {
		page = 0xFF
	}
	// C++: if paginated, maxItems = min(capacity, size-firstIndex)
	// else: maxItems = capacity
	var maxItems int
	if item.Pagination && firstIndex > 0 {
		maxItems = int(firstIndex) + capacity
		if maxItems > len(contents) {
			maxItems = len(contents)
		}
	} else {
		maxItems = len(contents)
		if maxItems > capacity {
			maxItems = capacity
		}
	}
	if maxItems > 0xFF {
		maxItems = 0xFF
	}
	if maxItems < 0 {
		maxItems = 0
	}

	w := netmsg.NewWriter()
	w.AddByte(opContainerOpen)
	w.AddByte(cid)
	g.addItem(w, item)
	w.AddString(name)
	w.AddByte(byte(capacity))
	w.AddByte(boolByte(hasParent))
	w.AddByte(0)
	w.AddByte(unlocked)
	w.AddByte(pagination)
	w.AddU16(uint16(len(contents)))
	w.AddU16(firstIndex)
	// C++: if firstIndex >= containerSize, send 0 items
	var itemsToSend int
	if int(firstIndex) >= len(contents) {
		itemsToSend = 0
	} else {
		itemsToSend = maxItems - int(firstIndex)
		if itemsToSend < 0 {
			itemsToSend = 0
		}
	}
	w.AddByte(byte(itemsToSend))
	for i := int(firstIndex); i < int(firstIndex)+itemsToSend && i < len(contents); i++ {
		g.addItem(w, contents[i])
	}
	// 13.21+ trailer for a normal container.
	w.AddByte(0x00)
	w.AddByte(0x00)
	w.AddByte(movable) // is movable
	w.AddByte(0)       // held by a player
	g.SendToClient(w)
}

// parseCloseContainer handles a close-container request (0x87) and confirms it.
func (g *GameProtocol) parseCloseContainer(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	cid := r.GetByte()
	g.player.CloseContainer(cid)
	w := netmsg.NewWriter()
	w.AddByte(opContainerClose)
	w.AddByte(cid)
	g.SendToClient(w)
}

// parseSeekContainer handles inbound 0x6E (seek/scroll in paginated container).
// C++: ProtocolGame::parseSeekInContainer → Game::playerSeekInContainer.
func (g *GameProtocol) parseSeekContainer(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	cid := r.GetByte()
	index := r.GetU16()
	r.GetByte() // containerCategory (unused for now)

	container := g.player.GetContainerByID(cid)
	if container == nil {
		slog.Default().Info("parseSeekContainer: container nil", "cid", cid)
		return
	}
	if !container.HasPagination() {
		slog.Default().Info("parseSeekContainer: no pagination", "cid", cid, "id", container.ID)
		return
	}
	cap := int(container.ContainerCapacity(g.deps.Items))
	if cap <= 0 {
		cap = 1
	}
	if int(index)%cap != 0 || int(index) >= len(container.Contents) {
		slog.Default().Info("parseSeekContainer: validation failed",
			"cid", cid, "index", index, "cap", cap, "size", len(container.Contents))
		return
	}

	g.player.SetContainerIndex(cid, index)
	g.sendContainer(cid, container, container.Parent != nil)
}

// parseContainerUp handles a container up navigation request (0x88).
func (g *GameProtocol) parseContainerUp(r *netmsg.Reader) {
	cid := r.GetByte()
	if g.player == nil {
		return
	}
	c := g.player.GetContainerByID(cid)
	if c != nil && c.Parent != nil {
		// C++: reseta o scroll index ao subir (evita que o container pai
		// herde um firstIndex inválido do container filho paginado)
		g.player.SetContainerIndex(cid, 0)
		g.player.OpenContainerAt(cid, c.Parent)
		g.sendContainer(cid, c.Parent, c.Parent.Parent != nil)
	}
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func (g *GameProtocol) sendAddContainerItem(cid uint8, slot uint16, item *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x70) // opContainerAddItem
	w.AddByte(cid)
	w.AddU16(slot)
	g.addItem(w, item)
	g.SendToClient(w)
}

func (g *GameProtocol) sendUpdateContainerItem(cid uint8, slot uint8, item *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x71) // opContainerUpdateItem
	w.AddByte(cid)
	w.AddU16(uint16(slot))
	g.addItem(w, item)
	g.SendToClient(w)
}

func (g *GameProtocol) sendRemoveContainerItem(cid uint8, slot uint8, lastItem *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x72) // opContainerRemoveItem
	w.AddByte(cid)
	w.AddU16(uint16(slot))
	if lastItem != nil {
		g.addItem(w, lastItem)
	} else {
		w.AddU16(0x00) // Empty item indicating no more items paginated
	}
	g.SendToClient(w)
}

func (g *GameProtocol) sendInventoryItem(slot uint8, item *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x78) // opInventoryItem
	w.AddByte(slot)
	g.addItem(w, item)
	g.SendToClient(w)
}

func (g *GameProtocol) sendInventoryEmpty(slot uint8) {
	w := netmsg.NewWriter()
	w.AddByte(0x79) // opInventoryEmpty
	w.AddByte(slot)
	g.SendToClient(w)
}

// CheckMapContainersDistance automatically closes any open map/ground containers
// that have exceeded a distance of 2 steps (or any floor change) from the player.
func (g *GameProtocol) CheckMapContainersDistance() {
	if g.player == nil {
		return
	}
	for cid, oc := range g.player.OpenContainersSnapshot() {
		if oc.IsOnMap {
			dx := absDiff(g.player.Pos.X, oc.Position.X)
			dy := absDiff(g.player.Pos.Y, oc.Position.Y)
			dz := absDiffByte(g.player.Pos.Z, oc.Position.Z)
			if dx > 2 || dy > 2 || dz != 0 {
				g.player.CloseContainer(cid)
				w := netmsg.NewWriter()
				w.AddByte(opContainerClose)
				w.AddByte(cid)
				g.SendToClient(w)
			}
		}
	}
}

func absDiff(a, b uint16) uint16 {
	if a > b {
		return a - b
	}
	return b - a
}

func absDiffByte(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// parseUseItemWith handles a use-item-with request (0x83).
func (g *GameProtocol) parseUseItemWith(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	fromPos := r.GetPosition()
	fromItemID := r.GetU16()
	fromStackPos := r.GetByte()
	toPos := r.GetPosition()
	toItemID := r.GetU16()
	toStackPos := r.GetByte()

	g.deps.Log.Debug("parseUseItemWith", "player", g.player.Name, "fromPos", fromPos, "fromItemID", fromItemID, "toPos", toPos, "toItemID", toItemID)

	g.useItemWith(fromPos, fromItemID, fromStackPos, toPos, toItemID, toStackPos)
}

// useItemWith is the body of playerUseItemEx, split out from the packet read so
// that walkToThenRetry can run it again once the player has walked into reach.
func (g *GameProtocol) useItemWith(fromPos netmsg.Position, fromItemID uint16, fromStackPos uint8, toPos netmsg.Position, toItemID uint16, toStackPos uint8) {
	if g.player == nil {
		return
	}
	fromItem := g.getItemAt(fromPos, fromItemID, fromStackPos)
	if fromItem == nil {
		g.deps.Log.Debug("parseUseItemWith: fromItem is nil")
		return
	}
	fromGamePos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
	toGamePos := game.Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z}

	toItem := g.getItemAt(toPos, toItemID, toStackPos)

	// Execute Lua action
	action := actions.FindAction(fromItem, game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z})
	if action != nil {
		// Game::playerUseItemEx (src/game/game.cpp:4594-4601): reach the item
		// first, then ask the action whether the target is reachable — an action
		// with allowFarUse answers by the viewport rather than by arm's length.
		ret := g.actionCanUse(fromGamePos)
		if ret == retNoError {
			ret = g.actionCanExecute(action, toGamePos)
		}
		if ret == retTooFarAway {
			// Out of reach is not a refusal upstream: the player walks over and
			// the action runs again on arrival.
			walkTo := fromGamePos
			if g.actionCanUse(fromGamePos) == retNoError {
				walkTo = toGamePos
			}
			if isMapPosition(walkTo) && g.walkToThenRetry(walkTo, func() {
				g.useItemWith(fromPos, fromItemID, fromStackPos, toPos, toItemID, toStackPos)
			}) {
				return
			}
		}
		if ret != retNoError {
			g.sendCancelMessage(ret.message())
			return
		}

		isEx := g.isExAction(fromItem)
		if isEx {
			if !g.player.CanDoPotionAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		} else {
			if !g.player.CanDoAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		}

		fromGamePos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
		toGamePos := game.Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z}
		beforeCount := fromItem.Count
		if g.deps.Lua.CallAction(action, g.player, fromItem, fromGamePos, toItem, toGamePos, false) {
			if isEx {
				g.player.SetNextPotionAction(1000 * time.Millisecond)
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(1000)
			} else {
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(200)
			}
			if fromItem.Count != beforeCount {
				g.reconcileUsedItem(fromItem, fromPos, fromStackPos)
			}
			return
		}
	}
}

// parseUseWithCreature handles a use-item-with-creature request (0x84).
func (g *GameProtocol) parseUseWithCreature(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	fromPos := r.GetPosition()
	fromItemID := r.GetU16()
	fromStackPos := r.GetByte()
	creatureID := r.GetU32()

	g.deps.Log.Debug("parseUseWithCreature", "player", g.player.Name, "fromPos", fromPos, "fromItemID", fromItemID, "creatureID", creatureID)

	g.useWithCreature(fromPos, fromItemID, fromStackPos, creatureID)
}

// useWithCreature is the body of playerUseWithCreature, split from the packet
// read so an out-of-reach use can walk over and run again.
func (g *GameProtocol) useWithCreature(fromPos netmsg.Position, fromItemID uint16, fromStackPos uint8, creatureID uint32) {
	if g.player == nil {
		return
	}
	fromItem := g.getItemAt(fromPos, fromItemID, fromStackPos)
	if fromItem == nil {
		g.deps.Log.Debug("parseUseWithCreature: fromItem is nil")
		return
	}
	fromGamePos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}

	targetCreature := g.deps.World.CreatureByID(creatureID)
	if targetCreature == nil {
		g.deps.Log.Debug("parseUseWithCreature: targetCreature is nil", "creatureID", creatureID)
		return
	}

	// Execute Lua action
	action := actions.FindAction(fromItem, game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z})
	if action != nil {
		// Game::playerUseWithCreature (src/game/game.cpp:4911-4917), the same two
		// checks as playerUseItemEx. The square-8 guards this replaces both let a
		// rune fly further than the client can draw and refused a container item
		// outright, since a container slot's "z" is a slot index.
		creaturePos := targetCreature.GetPosition()
		ret := g.actionCanUse(fromGamePos)
		if ret == retNoError {
			ret = g.actionCanExecute(action, creaturePos)
		}
		if ret == retTooFarAway {
			walkTo := fromGamePos
			if g.actionCanUse(fromGamePos) == retNoError {
				walkTo = creaturePos
			}
			if isMapPosition(walkTo) && g.walkToThenRetry(walkTo, func() {
				g.useWithCreature(fromPos, fromItemID, fromStackPos, creatureID)
			}) {
				return
			}
		}
		if ret != retNoError {
			g.sendCancelMessage(ret.message())
			return
		}

		isEx := g.isExAction(fromItem)
		if isEx {
			if !g.player.CanDoPotionAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		} else {
			if !g.player.CanDoAction() {
				g.sendCancelMessage("You are exhausted.")
				return
			}
		}

		fromGamePos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
		toGamePos := targetCreature.GetPosition()
		beforeCount := fromItem.Count
		if g.deps.Lua.CallAction(action, g.player, fromItem, fromGamePos, targetCreature, toGamePos, false) {
			if isEx {
				g.player.SetNextPotionAction(1000 * time.Millisecond)
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(1000)
			} else {
				g.player.SetNextAction(200 * time.Millisecond)
				g.SendUseItemCooldown(200)
			}
			if fromItem.Count != beforeCount {
				g.reconcileUsedItem(fromItem, fromPos, fromStackPos)
			}
			return
		}
	}
}

// getItemAt returns an item from the given client netmsg.Position and stackpos.
func (g *GameProtocol) getItemAt(pos netmsg.Position, itemID uint16, stackpos uint8) *game.Item {
	var item *game.Item
	if pos.X == 0xFFFF {
		if pos.Y == 0 {
			if g.player != nil {
				item = g.player.FindItemOfType(g.deps.Items, itemID, true, -1)
			}
		} else if pos.Y >= 0x40 {
			cid := uint8(pos.Y - 0x40)
			if cont, offset, ok := g.openContainerByCID(cid); ok {
				fromSlot := int(pos.Z) + offset
				if fromSlot < len(cont.Contents) {
					item = cont.Contents[fromSlot]
				}
			}
		} else {
			slot := uint8(pos.Y)
			if slot > 0 && slot <= 10 {
				item = g.player.Inventory[slot]
			} else if slot == 11 { // CONST_SLOT_STORE_INBOX
				if g.player.StoreInbox == nil {
					g.player.StoreInbox = &game.Item{ID: 23396}
				}
				item = g.player.StoreInbox
			}
		}
	} else {
		tile := g.deps.World.Map.GetTile(game.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
		if tile != nil {
			item = g.findTileItemByStackPos(tile, itemID, stackpos)
		}
	}
	if item == nil && itemID != 0 && g.player != nil {
		item = g.player.FindItemOfType(g.deps.Items, itemID, true, -1)
	}
	return item
}

func (g *GameProtocol) sendCancelMessage(text string) {
	w := netmsg.NewWriter()
	w.AddByte(opTextMessage)
	w.AddByte(22) // MESSAGE_FAILURE / STATUS_SMALL
	w.AddString(text)
	g.SendToClient(w)
}

// SendUseItemCooldown sends an item use cooldown packet (0xA6) to the client.
func (g *GameProtocol) SendUseItemCooldown(ms uint32) {
	w := netmsg.NewWriter()
	w.AddByte(0xA6)
	w.AddU32(ms)
	g.SendToClient(w)
}

func (g *GameProtocol) isExAction(item *game.Item) bool {
	if item == nil {
		return false
	}
	if g.deps != nil && g.deps.Items != nil {
		if t := g.deps.Items.Get(item.ID); t != nil {
			if strings.EqualFold(t.TypeName, "potion") || strings.EqualFold(t.TypeName, "rune") || strings.Contains(strings.ToLower(t.Name), "potion") {
				return true
			}
		}
	}
	id := item.ID
	if (id >= 236 && id <= 239) || id == 266 || id == 7618 || id == 7620 || (id >= 8472 && id <= 8473) || (id >= 23373 && id <= 23375) || id == 35563 {
		return true
	}
	return false
}

// restoreOpenContainers recursively scans the player's inventory and depot
// to restore any containers that were left open by the client (attrOpenContainer).
func (g *GameProtocol) restoreOpenContainers() {
	if g.player == nil {
		return
	}

	var traverse func(item *game.Item)
	traverse = func(item *game.Item) {
		if item == nil {
			return
		}
		if item.Attr != nil && item.Attr.OpenContainer != nil {
			cid := *item.Attr.OpenContainer

			slog.Default().Info("Restoring open container", "cid", cid, "itemId", item.ID)

			g.player.OpenContainerAtWithPos(cid, item, game.Position{}, false)
			g.sendContainer(cid, item, item.Parent != nil)
			// Clear it so it gets wiped from the DB on next save, preventing it from
			// becoming a ghost if the client loses sync.
			item.Attr.OpenContainer = nil
		}
		// Also traverse inside this container
		for _, child := range item.Contents {
			traverse(child)
		}
	}

	for _, item := range g.player.Inventory {
		traverse(item)
	}
	traverse(g.player.StoreInbox)
}
