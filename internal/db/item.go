package db

import (
	"context"
	"fmt"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadPlayerItems loads the items for a given player and populates the Inventory, StoreInbox, Depot, Inbox, and RewardChest.
func (d *DB) LoadPlayerItems(ctx context.Context, p *game.Player) error {
	// 1. Load Inventory and StoreInbox from player_items
	itemsBySID, loadedRows, err := d.loadItemsFromTable(ctx, p.DBID, "player_items")
	if err != nil {
		return err
	}
	for _, row := range loadedRows {
		if row.pid >= 1 && row.pid <= 10 {
			if row.pid < len(p.Inventory) {
				p.Inventory[row.pid] = row.item
			}
		} else if row.pid == 11 { // CONST_SLOT_STORE_INBOX
			p.StoreInbox = row.item
		} else {
			if parent, ok := itemsBySID[row.pid]; ok {
				row.item.Parent = parent
				parent.Contents = append([]*game.Item{row.item}, parent.Contents...)
			}
		}
	}


	// 3. Load Inbox
	if p.Inbox == nil {
		p.Inbox = &game.Item{ID: 14404} // ITEM_INBOX
	}
	inboxItemsBySID, inboxRows, err := d.loadItemsFromTable(ctx, p.DBID, "player_inboxitems")
	if err != nil {
		return err
	}
	for _, row := range inboxRows {
		if row.pid == 0 {
			row.item.Parent = p.Inbox
			p.Inbox.Contents = append([]*game.Item{row.item}, p.Inbox.Contents...)
		} else {
			if parent, ok := inboxItemsBySID[row.pid]; ok {
				row.item.Parent = parent
				parent.Contents = append([]*game.Item{row.item}, parent.Contents...)
			}
		}
	}

	// 4. Load Reward Chest
	if p.RewardChest == nil {
		p.RewardChest = &game.Item{ID: 21557} // ITEM_REWARD_CHEST
	}
	rewardItemsBySID, rewardRows, err := d.loadItemsFromTable(ctx, p.DBID, "player_rewards")
	if err != nil {
		return err
	}
	for _, row := range rewardRows {
		if row.pid == 0 {
			row.item.Parent = p.RewardChest
			p.RewardChest.Contents = append([]*game.Item{row.item}, p.RewardChest.Contents...)
		} else {
			if parent, ok := rewardItemsBySID[row.pid]; ok {
				row.item.Parent = parent
				parent.Contents = append([]*game.Item{row.item}, parent.Contents...)
			}
		}
	}

	// 5. Load Stash
	if p.Stash == nil {
		p.Stash = make(map[uint16]uint32)
	}
	stashQuery := `SELECT item_id, item_count FROM player_stash WHERE player_id = ?`
	stashRows, err := d.SQL.QueryContext(ctx, stashQuery, p.DBID)
	if err != nil {
		return err
	}
	defer stashRows.Close()
	for stashRows.Next() {
		var itemID uint16
		var count uint32
		if err := stashRows.Scan(&itemID, &count); err == nil {
			p.Stash[itemID] = count
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

	// Prepare open containers state
	var traverse func(item *game.Item)
	traverse = func(item *game.Item) {
		if item == nil {
			return
		}
		if item.Attr != nil {
			item.Attr.OpenContainer = nil
		}
		for _, child := range item.Contents {
			traverse(child)
		}
	}
	for _, item := range p.Inventory {
		traverse(item)
	}
	traverse(p.StoreInbox)
	if p.DepotManager != nil {
		for _, locker := range p.DepotManager.Lockers {
			traverse(locker)
		}
	}
	traverse(p.Inbox)
	traverse(p.RewardChest)

	for cid, openCont := range p.GetOpenContainers() {
		if openCont.Container != nil {
			if openCont.Container.Attr == nil {
				openCont.Container.Attr = &game.ItemAttributes{}
			}
			c := cid
			openCont.Container.Attr.OpenContainer = &c
		}
	}

	// Helper to save a tree of items to a table
	saveTree := func(tableName string, rootItems []itemRow) error {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE player_id = ?", tableName), p.DBID); err != nil {
			return err
		}
		stmt, err := tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (player_id, pid, sid, itemtype, count, attributes) VALUES (?, ?, ?, ?, ?, ?)", tableName))
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

		for _, root := range rootItems {
			if root.item != nil {
				if err := saveItem(root.item, root.pid); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// 1. Save Inventory
	var invRoots []itemRow
	for slot := 1; slot <= 10; slot++ {
		if p.Inventory[slot] != nil {
			invRoots = append(invRoots, itemRow{pid: slot, item: p.Inventory[slot]})
		}
	}
	if p.StoreInbox != nil {
		invRoots = append(invRoots, itemRow{pid: 11, item: p.StoreInbox})
	}
	if err := saveTree("player_items", invRoots); err != nil {
		return err
	}


	// 3. Save Inbox
	var inboxRoots []itemRow
	if p.Inbox != nil {
		for _, item := range p.Inbox.Contents {
			inboxRoots = append(inboxRoots, itemRow{pid: 0, item: item})
		}
	}
	if err := saveTree("player_inboxitems", inboxRoots); err != nil {
		return err
	}

	// 4. Save Reward Chest
	var rewardRoots []itemRow
	if p.RewardChest != nil {
		for _, item := range p.RewardChest.Contents {
			rewardRoots = append(rewardRoots, itemRow{pid: 0, item: item})
		}
	}
	if err := saveTree("player_rewards", rewardRoots); err != nil {
		return err
	}

	// 5. Save Stash
	if _, err := tx.ExecContext(ctx, "DELETE FROM player_stash WHERE player_id = ?", p.DBID); err != nil {
		return err
	}
	if p.Stash != nil {
		for itemID, count := range p.Stash {
			if _, err := tx.ExecContext(ctx, "INSERT INTO player_stash (player_id, item_id, item_count) VALUES (?, ?, ?)", p.DBID, itemID, count); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
