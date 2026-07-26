package db

import (
	"context"
	"fmt"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayerDepot loads all depot items for a player from the player_depotitems table.
// It populates the individual depot chests (boxes 1-17+) in the DepotManager.
func (d *DB) LoadPlayerDepot(ctx context.Context, p *game.Player) error {
	itemsBySID, loadedRows, err := d.loadItemsFromTable(ctx, p.DBID, "player_depotitems")
	if err != nil {
		return err
	}

	if p.DepotManager == nil {
		p.DepotManager = game.NewPlayerDepotManager(p)
	}

	if len(loadedRows) == 0 {
		return nil
	}

	// First pass: link children to their parent containers (for sid > 100)
	for _, row := range loadedRows {
		if row.pid >= 0 && row.pid < 100 {
			// This item belongs directly to a depot chest (pid = chest index)
			continue
		}
		
		parent := itemsBySID[row.pid]
		if parent != nil {
			parent.Contents = append(parent.Contents, row.item)
			row.item.Parent = parent
		}
	}

	// Second pass: put top-level items into their respective depot chests
	for _, row := range loadedRows {
		if row.pid >= 0 && row.pid < 100 {
			// pid is the depotId (1-17)
			chest := p.DepotManager.GetDepotChest(uint16(row.pid), true)
			if chest != nil {
				chest.Contents = append(chest.Contents, row.item)
				row.item.Parent = chest
			}
		}
	}

	return nil
}

// SavePlayerDepot persists all depot items to the player_depotitems table.
func (d *DB) SavePlayerDepot(ctx context.Context, p *game.Player) error {
	if p.DepotManager == nil || len(p.DepotManager.Chests) == 0 {
		_, err := d.SQL.ExecContext(ctx, "DELETE FROM player_depotitems WHERE player_id = ?", p.DBID)
		return err
	}

	// Clear existing depot items
	if _, err := d.SQL.ExecContext(ctx, "DELETE FROM player_depotitems WHERE player_id = ?", p.DBID); err != nil {
		return err
	}

	sidCounter := 100 // nested SIDs start at 100 to avoid colliding with PIDs (0-99)
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

		sid := sidCounter
		sidCounter++
		sidMap[item] = sid

		attrs := item.Attributes
		if imbBlob := game.EncodeImbuementBlob(item.Imbuements); len(imbBlob) > 0 {
			attrs = append(attrs, imbBlob...)
		}

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

	// Save each depot chest's contents
	for pid, chest := range p.DepotManager.Chests {
		if chest == nil {
			continue
		}

		for _, item := range chest.Contents {
			if err := saveItem(item, int(pid)); err != nil {
				return err
			}
		}
	}

	return nil
}
