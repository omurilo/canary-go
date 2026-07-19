package db

import (
	"context"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayerItems loads the items for a given player and populates the Inventory and Contents.
func (d *DB) LoadPlayerItems(ctx context.Context, p *game.Player) error {
	const q = `SELECT pid, sid, itemtype, count, attributes 
	           FROM player_items WHERE player_id = ? ORDER BY sid ASC`
	
	rows, err := d.SQL.QueryContext(ctx, q, p.DBID)
	if err != nil {
		return err
	}
	defer rows.Close()

	itemsBySID := make(map[int]*game.Item)
	type itemRow struct {
		pid  int
		item *game.Item
	}
	var loadedRows []itemRow

	for rows.Next() {
		var pid, sid int
		var itemtype, count uint16
		var attrs []byte
		if err := rows.Scan(&pid, &sid, &itemtype, &count, &attrs); err != nil {
			return err
		}

		item := &game.Item{
			ID:         itemtype,
			Count:      count,
			Attributes: attrs,
		}
		itemsBySID[sid] = item
		loadedRows = append(loadedRows, itemRow{pid: pid, item: item})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	for _, row := range loadedRows {
		if row.pid >= 1 && row.pid <= 10 {
			if row.pid < len(p.Inventory) {
				p.Inventory[row.pid] = row.item
			}
		} else {
			if parent, ok := itemsBySID[row.pid]; ok {
				parent.Contents = append(parent.Contents, row.item)
			}
		}
	}

	return nil
}

// SavePlayerItems saves the player's items into the database.
func (d *DB) SavePlayerItems(ctx context.Context, p *game.Player) error {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM player_items WHERE player_id = ?`, p.DBID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO player_items (player_id, pid, sid, itemtype, count, attributes) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	sidCounter := 100

	var saveItem func(item *game.Item, pid int) error
	saveItem = func(item *game.Item, pid int) error {
		if item == nil {
			return nil
		}
		sid := sidCounter
		sidCounter++

		var attrs []byte
		if len(item.Attributes) > 0 {
			attrs = item.Attributes
		}

		if _, err := stmt.ExecContext(ctx, p.DBID, pid, sid, item.ID, item.Count, attrs); err != nil {
			return err
		}

		for _, child := range item.Contents {
			if err := saveItem(child, sid); err != nil {
				return err
			}
		}
		return nil
	}

	for slot := 1; slot <= 10; slot++ {
		if p.Inventory[slot] != nil {
			if err := saveItem(p.Inventory[slot], slot); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
