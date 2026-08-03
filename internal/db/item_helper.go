package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/omurilo/canary-go/internal/game"
)

type itemRow struct {
	pid  int
	sid  int
	item *game.Item
}

func (d *DB) loadItemsFromTable(ctx context.Context, playerID uint32, tableName string) (map[int]*game.Item, []itemRow, error) {
	q := fmt.Sprintf(`SELECT pid, sid, itemtype, count, attributes FROM %s WHERE player_id = ? ORDER BY sid DESC`, tableName)
	rows, err := d.SQL.QueryContext(ctx, q, playerID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	itemsBySID := make(map[int]*game.Item)
	var loadedRows []itemRow

	for rows.Next() {
		var pid, sid int
		var itemtype, count uint16
		var attrs []byte
		if err := rows.Scan(&pid, &sid, &itemtype, &count, &attrs); err != nil {
			return nil, nil, err
		}

		item := &game.Item{
			ID:    itemtype,
			Count: count,
		}

		if attr, subType, err := game.DecodeItemAttributes(attrs, count); err != nil {
			slog.Default().Warn("failed to decode item attributes; preserving raw blob",
				"player_id", playerID, "itemtype", itemtype, "table", tableName, "err", err)
			// Keep the undecodable blob so a later save round-trips it verbatim.
			item.Attr = &game.ItemAttributes{Raw: attrs}
		} else {
			// attr is nil when the blob is empty (the common case); leave Attr nil.
			if attr != nil {
				attr.Raw = attrs
				item.Attr = attr
			}
			item.Count = subType
		}
		if imbs := game.DecodeImbuementBlob(attrs); len(imbs) > 0 {
			item.Imbuements = imbs
		}
		itemsBySID[sid] = item
		loadedRows = append(loadedRows, itemRow{pid: pid, sid: sid, item: item})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return itemsBySID, loadedRows, nil
}
