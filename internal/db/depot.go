package db

import (
	"strings"
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
			parent.Contents = append([]*game.Item{row.item}, parent.Contents...)
			row.item.Parent = parent
		}
	}

	// Second pass: put top-level items into their respective depot chests
	for _, row := range loadedRows {
		if row.pid >= 0 && row.pid < 100 {
			// pid is the depotId (1-17)
			chest := p.DepotManager.GetDepotChest(uint16(row.pid), true)
			if chest != nil {
				if chest.Contents == nil {
					chest.Contents = make([]*game.Item, 0)
				}
				chest.Contents = append([]*game.Item{row.item}, chest.Contents...)
				row.item.Parent = chest
			}
		}
	}

	return nil
}

// SavePlayerDepot persists all depot items to the player_depotitems table.
func (d *DB) SavePlayerDepot(ctx context.Context, p *game.Player) error {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if p.DepotManager == nil || len(p.DepotManager.Chests) == 0 {
		_, err := tx.ExecContext(ctx, "DELETE FROM player_depotitems WHERE player_id = ?", p.DBID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	// Clear existing depot items
	if _, err := tx.ExecContext(ctx, "DELETE FROM player_depotitems WHERE player_id = ?", p.DBID); err != nil {
		return err
	}

	var args []interface{}
	var placeholders []string
	sidCounter := 100

	flush := func() error {
		if len(placeholders) == 0 {
			return nil
		}
		query := "INSERT INTO player_depotitems (player_id, pid, sid, itemtype, count, attributes) VALUES " + strings.Join(placeholders, ",")
		_, err := tx.ExecContext(ctx, query, args...)
		
		// Reset slices
		args = args[:0]
		placeholders = placeholders[:0]
		return err
	}

	// Helper function to save an item and its children recursively
	var saveItem func(item *game.Item, parentSID int) error
	saveItem = func(item *game.Item, parentSID int) error {
		if item == nil {
			return nil
		}

		sid := sidCounter
		sidCounter++

		attrs := make([]byte, 0)
		if item.Attr != nil {
			if b := item.Attr.Encode(item.Count); len(b) > 0 {
				attrs = b
			}
		} else if len(item.Attributes) > 0 {
			attrs = item.Attributes
		}
		if imbBlob := game.EncodeImbuementBlob(item.Imbuements); len(imbBlob) > 0 {
			attrs = append(attrs, imbBlob...)
		}

		count := item.Count
		if count == 0 {
			count = 1
		}

		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?)")
		args = append(args, p.DBID, parentSID, sid, item.ID, count, attrs)

		if len(placeholders) >= 500 {
			if err := flush(); err != nil {
				return fmt.Errorf("failed to save depot item sid=%d: %w", sid, err)
			}
		}

		// Save children recursively
		for _, child := range item.Contents {
			if err := saveItem(child, sid); err != nil {
				return err
			}
		}

		return nil
	}

	// Save all chests' contents
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

	if err := flush(); err != nil {
		return err
	}

	return tx.Commit()
}
