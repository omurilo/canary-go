package db

import (
	"context"
	"fmt"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayerDepot loads all depot items for a player from the player_depotitems table.
// It constructs the depot locker hierarchy (lockers -> chests -> items) and attaches
// it to the player's DepotManager.
func (d *DB) LoadPlayerDepot(ctx context.Context, p *game.Player) error {
	itemsBySID, loadedRows, err := d.loadItemsFromTable(ctx, p.DBID, "player_depotitems")
	if err != nil {
		return err
	}

	if len(loadedRows) == 0 {
		// No depot items, initialize empty depot manager
		p.DepotManager = game.NewPlayerDepotManager(p)
		return nil
	}

	// Build the item tree from parent-child relationships
	for _, row := range loadedRows {
		if row.pid == 0 {
			// Top-level item (depot locker or chest)
			continue
		}
		parent := itemsBySID[row.pid]
		if parent != nil {
			parent.Contents = append(parent.Contents, row.item)
		}
	}

	// Initialize depot manager
	p.DepotManager = game.NewPlayerDepotManager(p)

	// Reconstruct depot lockers from loaded items
	// SID 0-99 are reserved for depot lockers (one per town)
	for sid, item := range itemsBySID {
		if sid >= 0 && sid < 100 {
			// This is a depot locker
			if item.IsDepotLocker() {
				// Determine town ID from SID (SID 0 = town 1, SID 1 = town 2, etc.)
				townID := uint16(sid + 1)
				locker := game.NewDepotLocker(townID)
				locker.Item = item
				p.DepotManager.Lockers[townID] = locker
			}
		}
	}

	return nil
}

// SavePlayerDepot persists all depot items to the player_depotitems table.
// It flattens the depot hierarchy into the parent-child SID structure.
func (d *DB) SavePlayerDepot(ctx context.Context, p *game.Player) error {
	if p.DepotManager == nil || len(p.DepotManager.Lockers) == 0 {
		// No depot items, clear the table
		_, err := d.SQL.ExecContext(ctx, "DELETE FROM player_depotitems WHERE player_id = ?", p.DBID)
		return err
	}

	// Clear existing depot items
	if _, err := d.SQL.ExecContext(ctx, "DELETE FROM player_depotitems WHERE player_id = ?", p.DBID); err != nil {
		return err
	}

	// Flatten the depot hierarchy and assign SIDs
	sidCounter := 0
	sidMap := make(map[*game.Item]int)

	const insertQuery = `INSERT INTO player_depotitems
		(player_id, pid, sid, itemtype, count, attributes)
		VALUES (?, ?, ?, ?, ?, ?)`

	// Helper function to save an item and its children recursively
	var saveItem func(item *game.Item, parentSID int) error
	saveItem = func(item *game.Item, parentSID int) error {
		if item == nil {
			return nil
		}

		// Assign SID to this item
		sid := sidCounter
		sidCounter++
		sidMap[item] = sid

				// Use attributes blob (encoding handled elsewhere)
		attrs := item.Attributes

		// Insert this item
		count := item.Count
		if count == 0 {
			count = 1
		}

		_, err := d.SQL.ExecContext(ctx, insertQuery,
			p.DBID, parentSID, sid, item.ID, count, attrs)
		if err != nil {
			return fmt.Errorf("failed to save depot item sid=%d: %w", sid, err)
		}

		// Save children recursively
		for _, child := range item.Contents {
			if err := saveItem(child, sid); err != nil {
				return err
			}
		}

		return nil
	}

	// Save each depot locker (one per town)
	for townID, locker := range p.DepotManager.Lockers {
		if locker == nil || locker.Item == nil {
			continue
		}

		// SID for depot locker is based on town ID (town 1 = SID 0, town 2 = SID 1, etc.)
		lockerSID := int(townID - 1)
		if lockerSID < 0 {
			lockerSID = 0
		}

		sidCounter = lockerSID + 1 // Start children after the locker
		sidMap[locker.Item] = lockerSID

		// Save the locker itself
		attrs := locker.Item.Attributes

		_, err := d.SQL.ExecContext(ctx, insertQuery,
			p.DBID, 0, lockerSID, locker.Item.ID, 1, attrs)
		if err != nil {
			return fmt.Errorf("failed to save depot locker town=%d: %w", townID, err)
		}

		// Save all depot chests and their contents
		for _, chest := range locker.Item.Contents {
			if err := saveItem(chest, lockerSID); err != nil {
				return err
			}
		}

		// Reserve SID range for this town (0-99 are for lockers)
		// Next town starts at next 100 boundary to avoid collisions
		sidCounter = ((sidCounter / 100) + 1) * 100
	}

	return nil
}
