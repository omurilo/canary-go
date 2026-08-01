package protocol
import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseItemMove handles an item move/throw request (0x78)
func (g *GameProtocol) parseItemMove(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	fromPos := r.GetPosition()
	spriteID := r.GetU16()
	fromStack := r.GetByte()
	toPos := r.GetPosition()
	count := r.GetByte()

	// Reject moves where source and destination are the same position on the map
	if fromPos.X != 0xFFFF && toPos.X != 0xFFFF && fromPos.X == toPos.X && fromPos.Y == toPos.Y && fromPos.Z == toPos.Z {
		return
	}

	var item *game.Item
	var fromContainer *game.Item
	var fromSlot uint8

	// 1. Find the source item
	if fromPos.X != 0xFFFF {
		// Map position
		tile := g.deps.World.Map.GetTile(game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z})
		if tile != nil {
			item = g.findTileItemByStackPos(tile, spriteID, fromStack)
		}
	} else {
		if fromPos.Y >= 0x40 {
			// Container
			cid := uint8(fromPos.Y - 0x40)
			if cont, offset, ok := g.openContainerByCID(cid); ok {
				fromContainer = cont
				fromSlot = uint8(fromPos.Z + uint8(offset)) // Client sends slot index as Z
				if int(fromSlot) < len(cont.Contents) {
					item = cont.Contents[fromSlot]
				}
			}
		} else {
			// Inventory
			fromSlot = uint8(fromPos.Y)
			if fromSlot > 0 && fromSlot <= 10 {
				item = g.player.Inventory[fromSlot]
			}
		}
	}

	if item == nil || item.ID != spriteID {
		return // Invalid move (item not found or ID mismatch)
	}

	// Distance check: player must be within range of both source and destination
	if fromPos.X != 0xFFFF {
		fromGamePos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
		if dist := g.player.Pos.MaxDistance(fromGamePos); dist < 0 || dist > 8 {
			return
		}
	}
	if toPos.X != 0xFFFF {
		toGamePos := game.Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z}
		if dist := g.player.Pos.MaxDistance(toGamePos); dist < 0 || dist > 8 {
			return
		}
	}

	if g.deps.Events != nil {
		if !g.deps.Events.ExecuteOnMoveItem(g.player, item, uint16(count), game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}, game.Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z}) {
			g.revertMove(fromPos, toPos, spriteID)
			return // Rejected by Lua
		}
	}

	it := g.deps.Items.Get(item.ID)

	// Map item protection: non-pickupable items on the map can only be moved
	// by god-level accounts (AccountType >= 5). This prevents normal players
	// from dragging map decorations, walls, etc.
	// Similarly, items placed on the map with an ActionID or UniqueID cannot
	// be moved by regular players, as they are part of map mechanics/quests.
	if fromPos.X != 0xFFFF {
		if (it != nil && !it.Pickupable) || (item.Attr != nil && (item.Attr.ActionID != nil || item.Attr.UniqueID != nil)) {
			if g.player.AccountType < 5 {
				g.sendStatusText("You cannot move this object.")
				g.revertMove(fromPos, toPos, spriteID)
				return
			}
		}
	}

	// Validation: non-pickupable items can't be picked up to inventory even by gods
	// (use /i command instead). This is Tibia's standard behavior.
	if toPos.X == 0xFFFF {
		if fromPos.X != 0xFFFF && it != nil && !it.Pickupable {
			g.sendStatusText("You cannot take this object.")
			g.revertMove(fromPos, toPos, spriteID)
			return
		}

		if toPos.Y < 0x40 {
			toSlot := uint8(toPos.Y)
			if toSlot > 0 && toSlot <= 10 && it != nil {
				valid := false
				if it.SlotPosition == "head" && toSlot == 1 { valid = true }
				if it.SlotPosition == "necklace" && toSlot == 2 { valid = true }
				if (it.SlotPosition == "backpack" || it.Group == items.GroupContainer) && toSlot == 3 { valid = true }
				if it.SlotPosition == "armor" && toSlot == 4 { valid = true }
				if (toSlot == 5 || toSlot == 6) && (it.SlotPosition == "hand" || it.SlotPosition == "two-handed" || it.SlotPosition == "right-hand" || it.SlotPosition == "left-hand" || it.WeaponType != "" || it.IsQuiver) { valid = true }
				if it.SlotPosition == "legs" && toSlot == 7 { valid = true }
				if it.SlotPosition == "feet" && toSlot == 8 { valid = true }
				if it.SlotPosition == "ring" && toSlot == 9 { valid = true }
				if (it.SlotPosition == "ammo" || it.WeaponType == "ammunition" || it.WeaponType == "ammo") && toSlot == 10 { valid = true }

				if !valid {
					if existing := g.player.Inventory[toSlot]; existing != nil && existing.IsContainer(g.deps.Items) {
						// Drop target is the container sitting in the equipment slot
					} else {
						g.sendStatusText("You cannot dress this object there.")
						g.revertMove(fromPos, toPos, spriteID)
						return
					}
				}
			}
		}
	}

	// 1.5 Determine target container (e.g. equipment container, child container in window, or container on tile)
	var destContainer *game.Item
	if toPos.X == 0xFFFF {
		if toPos.Y < 0x40 {
			toSlot := uint8(toPos.Y)
			if toSlot > 0 && toSlot <= 10 {
				if existing := g.player.Inventory[toSlot]; existing != nil && existing.IsContainer(g.deps.Items) {
					valid := false
					if it != nil {
						if it.SlotPosition == "head" && toSlot == 1 { valid = true }
						if it.SlotPosition == "necklace" && toSlot == 2 { valid = true }
						if (it.SlotPosition == "backpack" || it.Group == items.GroupContainer) && toSlot == 3 { valid = true }
						if it.SlotPosition == "armor" && toSlot == 4 { valid = true }
						if (toSlot == 5 || toSlot == 6) && (it.SlotPosition == "hand" || it.SlotPosition == "two-handed" || it.SlotPosition == "right-hand" || it.SlotPosition == "left-hand" || it.WeaponType != "" || it.IsQuiver) { valid = true }
						if it.SlotPosition == "legs" && toSlot == 7 { valid = true }
						if it.SlotPosition == "feet" && toSlot == 8 { valid = true }
						if it.SlotPosition == "ring" && toSlot == 9 { valid = true }
						if (it.SlotPosition == "ammo" || it.WeaponType == "ammunition" || it.WeaponType == "ammo") && toSlot == 10 { valid = true }
					}
					if !valid {
						destContainer = existing
					}
				}
			}
		} else {
			cid := uint8(toPos.Y - 0x40)
			if openCont, offset, ok := g.openContainerByCID(cid); ok {
				// Browse field containers redirect to tile
				if openCont.ID == game.ItemBrowseField {
					for bfPos, bf := range g.deps.World.BrowseFields {
						if bf == openCont {
							toPos = netmsg.Position{X: bfPos.X, Y: bfPos.Y, Z: uint8(bfPos.Z)}
							break
						}
					}
				} else {
					slotIdx := int(toPos.Z) + offset
					if slotIdx >= 0 && slotIdx < len(openCont.Contents) {
						targetItem := openCont.Contents[slotIdx]
						if targetItem != nil && targetItem.IsContainer(g.deps.Items) && targetItem != item {
							destContainer = targetItem
						} else {
							destContainer = openCont
						}
					} else {
						destContainer = openCont
					}
				}
			}
		}
	}
	// Map positions always target the tile, not containers on it.
	// Only container-to-container moves (above) get a destContainer.
	// This matches C++ behavior where Tile::queryDestination returns
	// the tile itself for map-position moves — depot lockers, chests,
	// and other containers on the floor never receive thrown items.

	// 2. Determine move count
	moveCount := uint16(count)
	itType := g.deps.Items.Get(item.ID)
	if itType == nil || !itType.Stackable {
		moveCount = item.Count
	} else if moveCount > item.Count {
		moveCount = item.Count
	}
	if moveCount == 0 {
		moveCount = 1
	}

	// Swapping logic
	var swapItem *game.Item
	if destContainer == nil && toPos.X == 0xFFFF && toPos.Y < 0x40 {
		toSlot := uint8(toPos.Y)
		if toSlot > 0 && toSlot <= 10 {
			if existing := g.player.Inventory[toSlot]; existing != nil {
				swapItem = existing
			}
		}
	}

	// Splitting logic
	var moveItem *game.Item
	if moveCount >= item.Count {
		moveItem = item
	} else {
		moveItem = &game.Item{ID: item.ID, Count: moveCount, Attributes: item.Attributes}
		item.Count -= moveCount
	}

	// 2.5 Capacity check
	if toPos.X == 0xFFFF {
		moveWeight := g.getItemWeight(moveItem)
		swapWeight := g.getItemWeight(swapItem)
		weightDiff := moveWeight
		if swapWeight > 0 && weightDiff >= swapWeight {
			weightDiff -= swapWeight
		} else if swapWeight > weightDiff {
			weightDiff = 0 // Actually freeing up weight
		}
		
		// If moving from map to inventory, or moving inside inventory (which might involve swapping)
		if fromPos.X != 0xFFFF {
			totalWeight := g.getPlayerTotalWeight()
			if totalWeight + weightDiff > g.player.Capacity * 100 {
				g.sendStatusText("This object is too heavy for you to carry.")
				if moveItem != item {
					item.Count += moveCount // restore count
				}
				g.revertMove(fromPos, toPos, spriteID)
				return
			}
		}
	}

	// 3. Remove from source
	if moveItem == item {
		if fromPos.X != 0xFFFF {
			pos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
			// Captured before the removal, because the index is only knowable while
			// the item is still on the tile. -1 means no client holds it there.
			actualStack := g.stackPosOfItem(pos, item)
			g.deps.World.Map.RemoveItemPtr(pos, item)
			if actualStack != -1 {
				g.onRemoveTileItem(pos, uint8(actualStack), item)
			}
		} else {
			if fromPos.Y >= 0x40 {
				fromContainer.Contents = append(fromContainer.Contents[:fromSlot], fromContainer.Contents[fromSlot+1:]...)
				g.sendRemoveContainerItem(uint8(fromPos.Y-0x40), fromSlot, nil)
				// Browse field: also remove from tile (use tile stack pos)
				if fromContainer.ID == game.ItemBrowseField {
					for bfPos, bf := range g.deps.World.BrowseFields {
						if bf == fromContainer {
							tileStack := g.stackPosOfItem(bfPos, item)
							g.deps.World.Map.RemoveItemPtr(bfPos, item)
							if tileStack != -1 {
								g.broadcastRemoveTileThing(bfPos, uint8(tileStack))
							}
							break
						}
					}
				}
			} else {
				g.player.Inventory[fromSlot] = nil
				g.sendInventoryEmpty(fromSlot)
			}
		}
	} else {
		// Just update the source count
		if fromPos.X != 0xFFFF {
			pos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
			if actualStack := g.stackPosOfItem(pos, item); actualStack != -1 {
				g.broadcastUpdateTileThing(pos, uint8(actualStack), item)
			}
		} else {
			if fromPos.Y >= 0x40 {
				g.sendUpdateContainerItem(uint8(fromPos.Y-0x40), fromSlot, item)
			} else {
				g.sendInventoryItem(fromSlot, item)
			}
		}
	}

	// 4. Add to destination
	if destContainer != nil {
		if !game.AddItemToContainer(g.deps.Items, destContainer, moveItem) {
			// Step 3 already took the item off its source. revertMove only re-sends
			// packets describing the CURRENT state — it restores nothing — so bailing
			// out here used to destroy the item outright. Dropping things into a full
			// depot box made them vanish.
			//
			// C++ never gets here: internalMoveItem runs queryAdd on the destination
			// BEFORE removing from the source. Putting it back is the containable fix;
			// reordering playerMoveThing is the real one.
			g.restoreToSource(fromPos, fromContainer, fromSlot, moveItem)
			g.sendStatusText("There is not enough room.")
			g.revertMove(fromPos, toPos, spriteID)
			return
		}
		g.RefreshContainer(destContainer)
		if fromContainer != nil && fromContainer != destContainer {
			g.RefreshContainer(fromContainer)
		}
	} else if toPos.X != 0xFFFF {
		pos := game.Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z}
		moveItem.Parent = nil
		
		// Map merging logic
		tile := g.deps.World.Map.GetTile(pos)
		// Mailbox check: if destination has a mailbox, process mail send.
		if tile != nil && g.tileHasMailbox(tile) && g.processMailSend(moveItem) {
			// Mail sent - item consumed. No tile placement.
			if fromContainer != nil {
				g.RefreshContainer(fromContainer)
			}
			return
		}
		var merged bool
		if tile != nil && len(tile.Items) > 0 && it != nil && it.Stackable {
			topItem := tile.Items[len(tile.Items)-1]
			if topItem.ID == moveItem.ID && topItem.Count < 100 {
				room := 100 - topItem.Count
				take := moveItem.Count
				if take > room {
					take = room
				}
				topItem.Count += take
				moveItem.Count -= take
				g.broadcastUpdateTileThing(pos, uint8(len(tile.Items)-1), topItem)
				if moveItem.Count == 0 {
					merged = true
				}
			}
		}

		if !merged {
			// World.AddItem already tells the spectators through OnItemAppear, which is
			// how C++ notifies too (postAddNotification -> onAddTileItem). Broadcasting
			// again here sent a SECOND identical 0x6A for every move, and the client
			// drew the item twice — the "unusable copies left on the floor".
			//
			// The fallback path creates the tile directly and so notifies nobody; that
			// is the only branch that still has to broadcast.
			if !g.deps.World.AddItem(pos, moveItem) {
				g.deps.World.Map.SetTile(pos, &game.Tile{Items: []*game.Item{moveItem}})
				g.broadcastAddTileItem(pos, moveItem)
			}
		}
		if fromContainer != nil {
			g.RefreshContainer(fromContainer)
		}
	} else {
		if toPos.Y >= 0x40 {
			cid := uint8(toPos.Y - 0x40)
			if toContainer, _, ok := g.openContainerByCID(cid); ok {
				var merged bool
				if it != nil && it.Stackable {
					// Search container for any non-full matching stacks first
					for idx, targetItem := range toContainer.Contents {
						if targetItem != nil && targetItem.ID == moveItem.ID && targetItem.Count < 100 {
							room := 100 - targetItem.Count
							take := moveItem.Count
							if take > room {
								take = room
							}
							targetItem.Count += take
							moveItem.Count -= take
							g.sendUpdateContainerItem(cid, uint8(idx), targetItem)
							if moveItem.Count == 0 {
								merged = true
								break
							}
						}
					}
				}

				if !merged {
					moveItem.Parent = toContainer
					// Insert at the beginning (prepend) — C++ comportamento padrao
					toContainer.Contents = append([]*game.Item{moveItem}, toContainer.Contents...)
					if len(toContainer.Contents) > 0xFF {
						toContainer.Contents = toContainer.Contents[:0xFF] // simple truncation
					}
					// Skip sendAddContainerItem para containers paginados
					// (slot 0 na pagina atual nao e o indice absoluto 0)
					if !toContainer.Pagination {
						g.sendAddContainerItem(cid, 0, moveItem)
					}
				}
				g.RefreshContainer(toContainer)
			}
		} else {
			toSlot := uint8(toPos.Y)
			if toSlot > 0 && toSlot <= 10 {
				moveItem.Parent = nil
				var merged bool
				if it != nil && it.Stackable {
					if targetItem := g.player.Inventory[toSlot]; targetItem != nil {
						if targetItem.ID == moveItem.ID && targetItem.Count < 100 {
							room := 100 - targetItem.Count
							take := moveItem.Count
							if take > room {
								take = room
							}
							targetItem.Count += take
							moveItem.Count -= take
							g.sendInventoryItem(toSlot, targetItem)
							if moveItem.Count == 0 {
								merged = true
							}
						}
					}
				}

				if !merged {
					g.player.Inventory[toSlot] = moveItem
					g.sendInventoryItem(toSlot, moveItem)
				}
			}
		}
		if fromContainer != nil {
			g.RefreshContainer(fromContainer)
		}
	}

	// 5. Handle swapItem placement back to fromPos
	if swapItem != nil {
		if fromPos.X != 0xFFFF {
			swapItem.Parent = nil
			pos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
			// Same as above: only the tile-creating fallback needs an explicit
			// broadcast, because AddItem's OnItemAppear covers the normal path.
			if !g.deps.World.AddItem(pos, swapItem) {
				g.deps.World.Map.SetTile(pos, &game.Tile{Items: []*game.Item{swapItem}})
				g.broadcastAddTileItem(pos, swapItem)
			}
		} else {
			if fromPos.Y >= 0x40 {
				cid := uint8(fromPos.Y - 0x40)
				if fromContainer != nil {
					swapItem.Parent = fromContainer
					fromContainer.Contents = append([]*game.Item{swapItem}, fromContainer.Contents...)
					if !fromContainer.Pagination {
						g.sendAddContainerItem(cid, 0, swapItem)
					}
				}
			} else {
				fSlot := uint8(fromPos.Y)
				if fSlot > 0 && fSlot <= 10 {
					swapItem.Parent = nil
					g.player.Inventory[fSlot] = swapItem
					g.sendInventoryItem(fSlot, swapItem)
				}
			}
		}
	}

	// 6. Update capacity in client if inventory changed
	if fromPos.X == 0xFFFF || toPos.X == 0xFFFF {
		g.sendStats()
	}
}

// findTileItemByStackPos finds an item on a tile matching the client stackpos and sprite id.
func (g *GameProtocol) findTileItemByStackPos(tile *game.Tile, spriteID uint16, stackPos uint8) *game.Item {
	// Simple lookup for now: we just match by sprite ID
	if tile.Ground != nil && tile.Ground.ID == spriteID {
		return tile.Ground
	}
	for _, it := range tile.Items {
		if it.ID == spriteID {
			return it
		}
	}
	return nil
}

// revertMove forces the client to redraw the from and to positions.
func (g *GameProtocol) revertMove(fromPos netmsg.Position, toPos netmsg.Position, spriteID uint16) {
	if fromPos.X != 0xFFFF {
		pos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
		tile := g.deps.World.Map.GetTile(pos)
		if tile != nil {
			g.sendUpdateTile(pos, tile)
		}
	} else {
		if fromPos.Y >= 0x40 {
			cid := uint8(fromPos.Y - 0x40)
			fromSlot := uint8(fromPos.Z)
			if cont, _, ok := g.openContainerByCID(cid); ok {
				var oldItem *game.Item
				if int(fromSlot) < len(cont.Contents) {
					oldItem = cont.Contents[fromSlot]
				}
				g.sendUpdateContainerItem(cid, fromSlot, oldItem)
			}
		} else {
			slot := uint8(fromPos.Y)
			if slot > 0 && slot <= 10 {
				if oldItem := g.player.Inventory[slot]; oldItem != nil {
					g.sendInventoryItem(slot, oldItem)
				} else {
					g.sendInventoryEmpty(slot)
				}
			}
		}
	}

	if toPos.X != 0xFFFF {
		pos := game.Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z}
		tile := g.deps.World.Map.GetTile(pos)
		if tile != nil {
			g.sendUpdateTile(pos, tile)
		}
	} else {
		if toPos.Y >= 0x40 {
			cid := uint8(toPos.Y - 0x40)
			// Wait, the client doesn't specify a to-slot for containers (it goes to index 0).
			// If it was reverted, we just resend the container or it might be OK since we didn't add it yet.
			// Actually, sending the full container updates it.
			if cont, _, ok := g.openContainerByCID(cid); ok {
				g.sendContainer(cid, cont, cont.Parent != nil)
			}
		} else {
			slot := uint8(toPos.Y)
			if slot > 0 && slot <= 10 {
				if oldItem := g.player.Inventory[slot]; oldItem != nil {
					g.sendInventoryItem(slot, oldItem)
				} else {
					g.sendInventoryEmpty(slot)
				}
			}
		}
	}
}

// sendUpdateTile sends opUpdateTile to force a client to redraw a single tile.
func (g *GameProtocol) sendUpdateTile(pos game.Position, tile *game.Tile) {
	// canSee-gated in C++ (protocolgame.cpp sendUpdateTile).
	if !g.canSee(pos) {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(opUpdateTile)
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	
	g.deps.World.RLock()
	if tile != nil {
		g.addTileDescription(w, tile, pos)
		w.AddByte(0x00)
		w.AddByte(0xFF)
	} else {
		w.AddByte(0x01)
		w.AddByte(0xFF)
	}
	g.deps.World.RUnlock()
	g.SendToClient(w)
}

func (g *GameProtocol) broadcastRemoveTileThing(pos game.Position, stack uint8) {
	for _, s := range g.deps.World.Spectators(pos, 0) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.sendRemoveTileThing(pos, stack)
		}
	}
}

func (g *GameProtocol) sendRemoveTileThing(pos game.Position, stack uint8) {
	if stack >= 10 {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x6C) // TileRemoveThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.SendToClient(w)
}

func (g *GameProtocol) broadcastUpdateTileThing(pos game.Position, stack uint8, item *game.Item) {
	for _, s := range g.deps.World.Spectators(pos, 0) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.sendUpdateTileThing(pos, stack, item)
		}
	}
}

func (g *GameProtocol) sendUpdateTileThing(pos game.Position, stack uint8, item *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x6B) // TileUpdateThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.addItem(w, item)
	g.SendToClient(w)
}

func (g *GameProtocol) broadcastAddTileItem(pos game.Position, item *game.Item) {
	for _, s := range g.deps.World.Spectators(pos, 0) {
		if gp, ok := s.Session.(*GameProtocol); ok {

			// Per viewer, and skipped entirely for a viewer with no slot for it.
			if stack := gp.stackPosOfItem(pos, item); stack != -1 {
				gp.sendAddTileItem(pos, uint8(stack), item)
			}
		}
	}
}

func (g *GameProtocol) sendAddTileItem(pos game.Position, stack uint8, item *game.Item) {
	// canSee-gated in C++ (protocolgame.cpp sendAddTileItem).
	if !g.canSee(pos) {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x6A) // TileAddThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.addItem(w, item)
	g.SendToClient(w)
}

// stackPosOfItem is Tile::getStackposOfItem (src/items/tile.cpp:1496-1543): the
// index `item` occupies in THIS client's view of the tile, or -1 when the client
// does not hold it at that index.
//
// -1 is not an error code to paper over — it is the answer for two real cases the
// old version got wrong by returning a count instead:
//
//   - the item is not on the tile at all, where the count was a plausible-looking
//     index pointing at whatever else was there;
//   - the tile holds ten or more things, where the client simply stopped storing
//     and has no index to update.
//
// Callers must skip the packet on -1, exactly as C++ does. Sending an update for
// an index the client does not have is how the same door ended up reported at 2,
// then 3, then 9 while the client held it at 7 — and why opening it did nothing
// until a relog rebuilt the tile.
func (g *GameProtocol) stackPosOfItem(pos game.Position, item *game.Item) int {
	g.deps.World.RLock()
	defer g.deps.World.RUnlock()

	tile := g.deps.World.Map.GetTile(pos)
	if tile == nil || item == nil {
		return -1
	}

	n := 0
	if tile.Ground != nil {
		if tile.Ground == item {
			return n
		}
		n++
	}

	// The always-on-top items are searched one by one only when the item we are
	// looking for is itself always-on-top; otherwise C++ skips the whole group by
	// count, because a down item can never be found among them.
	if g.isTopItem(item) {
		for _, it := range tile.Items {
			if !g.isTopItem(it) {
				continue
			}
			if it == item {
				return n
			}
			n++
			if n == 10 {
				return -1
			}
		}
	} else {
		for _, it := range tile.Items {
			if g.isTopItem(it) {
				n++
			}
		}
		if n >= 10 {
			return -1
		}
	}

	// Creatures sit between the top and the down items, and only the ones THIS
	// client can see occupy a slot — an invisible creature or a ghost is not in the
	// stack the client holds.
	for _, c := range tile.Creatures {
		if g.canSeeCreature(c) {
			n++
			if n >= 10 {
				return -1
			}
		}
	}

	if !g.isTopItem(item) {
		for _, it := range tile.Items {
			if g.isTopItem(it) {
				continue
			}
			if it == item {
				return n
			}
			n++
			if n >= 10 {
				return -1
			}
		}
	}
	return -1
}

func (g *GameProtocol) getItemWeight(item *game.Item) uint32 {
	if item == nil {
		return 0
	}
	weight := uint32(0)
	if item.Attr != nil && item.Attr.Weight != nil {
		weight = *item.Attr.Weight
	} else if it := g.deps.Items.Get(item.ID); it != nil {
		weight = it.Weight
	}
	if it := g.deps.Items.Get(item.ID); it != nil && it.Stackable {
		weight *= uint32(item.Count)
	}
	for _, child := range item.Contents {
		weight += g.getItemWeight(child)
	}
	return weight
}

func (g *GameProtocol) getPlayerTotalWeight() uint32 {
	weight := uint32(0)
	for _, item := range g.player.Inventory {
		weight += g.getItemWeight(item)
	}
	return weight
}

func isChildOf(parent, child *game.Item) bool {
	if parent == nil || child == nil {
		return false
	}
	if parent == child {
		return true
	}
	for _, item := range parent.Contents {
		if item != nil {
			if isChildOf(item, child) {
				return true
			}
		}
	}
	return false
}

// onRemoveTileItem sends RemoveTileThing to each spectator with per-player stack position.
// Mirrors C++ Tile::onRemoveTileItem.
func (g *GameProtocol) onRemoveTileItem(pos game.Position, stackPos uint8, item *game.Item) {
	for _, s := range g.deps.World.Spectators(pos, 0) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.sendRemoveTileThing(pos, stackPos)
		}
	}
}

// restoreToSource puts an item back where a failed move took it from, so a
// rejected destination costs the player nothing.
//
// This exists because playerMoveThing removes before it knows the destination
// will accept. C++ asks first (Game::internalMoveItem runs queryAdd on the
// destination cylinder ahead of removeThing), which is the ordering this port
// should eventually adopt; until then, undoing the removal is what keeps a full
// container from eating items.
func (g *GameProtocol) restoreToSource(fromPos netmsg.Position, fromContainer *game.Item, fromSlot uint8, item *game.Item) {
	if item == nil {
		return
	}
	if fromPos.X != 0xFFFF {
		pos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
		g.deps.World.AddItem(pos, item)
		return
	}
	if fromPos.Y >= 0x40 {
		if fromContainer != nil {
			item.Parent = fromContainer
			at := int(fromSlot)
			if at > len(fromContainer.Contents) {
				at = len(fromContainer.Contents)
			}
			fromContainer.Contents = append(fromContainer.Contents, nil)
			copy(fromContainer.Contents[at+1:], fromContainer.Contents[at:])
			fromContainer.Contents[at] = item
			g.RefreshContainer(fromContainer)
		}
		return
	}
	if int(fromSlot) < len(g.player.Inventory) && g.player.Inventory[fromSlot] == nil {
		g.player.Inventory[fromSlot] = item
		g.sendInventoryItem(fromSlot, item)
	}
}
