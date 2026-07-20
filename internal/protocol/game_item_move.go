package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseItemMove handles an item move/throw request (0x78)
func (g *GameProtocol) parseItemMove(r *netmsg.Reader) {
	fromPos := r.GetPosition()
	spriteID := r.GetU16()
	fromStack := r.GetByte()
	toPos := r.GetPosition()
	count := r.GetByte()

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
			if cont, ok := g.containers[cid]; ok {
				fromContainer = cont
				fromSlot = uint8(fromPos.Z) // Client sends slot index as Z
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

	if g.deps.Events != nil {
		if !g.deps.Events.ExecuteOnMoveItem(g.player, item, uint16(count), game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}, game.Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z}) {
			g.revertMove(fromPos, toPos, spriteID)
			return // Rejected by Lua
		}
	}

	it := g.deps.Items.Get(item.ID)

	// Validation
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
				if it.SlotPosition == "backpack" && toSlot == 3 { valid = true }
				if it.SlotPosition == "body" && toSlot == 4 { valid = true }
				if (toSlot == 5 || toSlot == 6) && (it.SlotPosition == "two-handed" || it.SlotPosition == "right-hand" || it.SlotPosition == "left-hand" || it.WeaponType != "") { valid = true }
				if it.SlotPosition == "legs" && toSlot == 7 { valid = true }
				if it.SlotPosition == "feet" && toSlot == 8 { valid = true }
				if it.SlotPosition == "ring" && toSlot == 9 { valid = true }
				if (it.SlotPosition == "ammo" || it.WeaponType == "ammunition" || it.WeaponType == "ammo") && toSlot == 10 { valid = true }

				if !valid {
					// Check if there is a container in the slot. If so, redirect to it.
					if existing := g.player.Inventory[toSlot]; existing != nil {
						if existingType := g.deps.Items.Get(existing.ID); existingType != nil && existingType.IsContainer() {
							// Find open container ID
							foundCid := -1
							for cid, cont := range g.containers {
								if cont == existing {
									foundCid = int(cid)
									break
								}
							}
							if foundCid != -1 {
								// Redirect to open container
								toPos = netmsg.Position{X: 0xFFFF, Y: uint16(0x40 + foundCid), Z: 0}
							} else {
								g.sendStatusText("You cannot dress this object there.")
								g.revertMove(fromPos, toPos, spriteID)
								return
							}
						} else {
							g.sendStatusText("You cannot dress this object there.")
							g.revertMove(fromPos, toPos, spriteID)
							return
						}
					} else {
						g.sendStatusText("You cannot dress this object there.")
						g.revertMove(fromPos, toPos, spriteID)
						return
					}
				}
			}
		}
	}

	// 2. Determine move count
	moveCount := uint16(count)
	if moveCount > item.Count {
		moveCount = item.Count
	}
	if moveCount == 0 {
		moveCount = 1
	}

	// Swapping logic
	var swapItem *game.Item
	if toPos.X == 0xFFFF && toPos.Y < 0x40 {
		toSlot := uint8(toPos.Y)
		if toSlot > 0 && toSlot <= 10 {
			if existing := g.player.Inventory[toSlot]; existing != nil {
				swapItem = existing
				// If swapping, we must move the full stack, or at least we treat moveItem as taking the slot.
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
			g.deps.World.Map.RemoveItemPtr(game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}, item)
			g.broadcastRemoveTileThing(game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}, fromStack)
		} else {
			if fromPos.Y >= 0x40 {
				fromContainer.Contents = append(fromContainer.Contents[:fromSlot], fromContainer.Contents[fromSlot+1:]...)
				g.sendRemoveContainerItem(uint8(fromPos.Y-0x40), fromSlot, nil)
			} else {
				g.player.Inventory[fromSlot] = nil
				g.sendInventoryEmpty(fromSlot)
			}
		}
	} else {
		// Just update the source count
		if fromPos.X != 0xFFFF {
			g.broadcastUpdateTileThing(game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}, fromStack, item)
		} else {
			if fromPos.Y >= 0x40 {
				g.sendUpdateContainerItem(uint8(fromPos.Y-0x40), fromSlot, item)
			} else {
				g.sendInventoryItem(fromSlot, item)
			}
		}
	}

	// 4. Add to destination
	if toPos.X != 0xFFFF {
		pos := game.Position{X: toPos.X, Y: toPos.Y, Z: toPos.Z}
		if !g.deps.World.AddItem(pos, moveItem) {
			// Create a new tile if none exists
			g.deps.World.Map.SetTile(pos, &game.Tile{Items: []*game.Item{moveItem}})
		}
		g.broadcastAddTileItem(pos, moveItem)
	} else {
		if toPos.Y >= 0x40 {
			cid := uint8(toPos.Y - 0x40)
			if toContainer, ok := g.containers[cid]; ok {
				// Insert at the beginning of the container (index 0)
				toContainer.Contents = append([]*game.Item{moveItem}, toContainer.Contents...)
				if len(toContainer.Contents) > 0xFF {
					toContainer.Contents = toContainer.Contents[:0xFF] // simple truncation
				}
				g.sendAddContainerItem(cid, 0, moveItem)
			}
		} else {
			toSlot := uint8(toPos.Y)
			if toSlot > 0 && toSlot <= 10 {
				g.player.Inventory[toSlot] = moveItem
				g.sendInventoryItem(toSlot, moveItem)
			}
		}
	}

	// 5. Handle swapItem placement back to fromPos
	if swapItem != nil {
		if fromPos.X != 0xFFFF {
			pos := game.Position{X: fromPos.X, Y: fromPos.Y, Z: fromPos.Z}
			if !g.deps.World.AddItem(pos, swapItem) {
				g.deps.World.Map.SetTile(pos, &game.Tile{Items: []*game.Item{swapItem}})
			}
			g.broadcastAddTileItem(pos, swapItem)
		} else {
			if fromPos.Y >= 0x40 {
				cid := uint8(fromPos.Y - 0x40)
				if fromContainer != nil {
					fromContainer.Contents = append([]*game.Item{swapItem}, fromContainer.Contents...)
					g.sendAddContainerItem(cid, 0, swapItem)
				}
			} else {
				fSlot := uint8(fromPos.Y)
				if fSlot > 0 && fSlot <= 10 {
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
			if cont, ok := g.containers[cid]; ok {
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
			if cont, ok := g.containers[cid]; ok {
				g.sendContainer(cid, cont, false)
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

			stack := gp.stackPosOfItem(pos, item)
			gp.sendAddTileItem(pos, stack, item)
		}
	}
}

func (g *GameProtocol) sendAddTileItem(pos game.Position, stack uint8, item *game.Item) {
	w := netmsg.NewWriter()
	w.AddByte(0x6A) // TileAddThing
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(stack)
	g.addItem(w, item)
	g.SendToClient(w)
}

func (g *GameProtocol) stackPosOfItem(pos game.Position, item *game.Item) uint8 {
	g.deps.World.RLock()
	defer g.deps.World.RUnlock()
	stack := 0
	tile := g.deps.World.Map.GetTile(pos)
	if tile != nil {
		if tile.Ground != nil {
			if tile.Ground == item {
				return uint8(stack)
			}
			stack++
		}
		for _, it := range tile.Items {
			if g.isTopItem(it) {
				if it == item {
					return uint8(stack)
				}
				stack++
			}
		}
	}
	if tile != nil {
		stack += len(tile.Creatures)
	}
	if tile != nil {
		for _, it := range tile.Items {
			if !g.isTopItem(it) {
				if it == item {
					return uint8(stack)
				}
				stack++
			}
		}
	}
	return uint8(stack)
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
