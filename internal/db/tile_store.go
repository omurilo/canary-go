package db

import (
	"context"
	"fmt"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/io/propstream"
	"github.com/opentibiabr/canary-go/internal/items"
)

// The tile_store codec, a port of src/io/iomapserialize.cpp. Nothing was reading or
// writing this table, so every item a player left inside a house — furniture,
// containers and their contents, beds — was silently lost on restart.
//
// One row per house tile that has anything worth saving:
//
//	x u16 | y u16 | z u8 | count u32 | count items
//
// and one item is:
//
//	id u16 | attribute TLV | [ATTR_CONTAINER_ITEMS u8, size u32, children...] | 0x00
//
// The children are nested inline, which is why the attribute decoder had to grow a
// streaming entry point: a whole-blob decoder cannot say where one item's
// attributes stop and the next item begins.

// attrContainerItemsTag is ATTR_CONTAINER_ITEMS (items_definitions.hpp).
const attrContainerItemsTag = 23

// LoadHouseItems restores every house tile's items from tile_store, mirroring
// IOMapSerialize::loadHouseItems. Rows whose tile is no longer part of the map are
// skipped, as upstream does.
func (d *DB) LoadHouseItems(ctx context.Context, world *game.World) (int, error) {
	rows, err := d.SQL.QueryContext(ctx, "SELECT `data` FROM `tile_store`")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	restored := 0
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return restored, err
		}
		ps := propstream.NewPropStream(blob)
		x, err := ps.ReadUint16()
		if err != nil {
			continue
		}
		y, err := ps.ReadUint16()
		if err != nil {
			continue
		}
		z, err := ps.ReadUint8()
		if err != nil {
			continue
		}
		pos := game.Position{X: x, Y: y, Z: z}
		tile := world.Map.GetTile(pos)
		if tile == nil {
			// The map changed under the save: upstream skips the row rather than
			// resurrecting a tile that no longer exists.
			continue
		}
		count, err := ps.ReadUint32()
		if err != nil {
			continue
		}
		for i := uint32(0); i < count; i++ {
			it, err := readItem(ps)
			if err != nil {
				// A truncated or unknown tag makes the rest of the row meaningless;
				// stop on this tile instead of appending garbage.
				break
			}
			tile.Items = append(tile.Items, it)
			restored++
		}
	}
	return restored, rows.Err()
}

// readItem reads one item record, recursing into container contents.
func readItem(ps *propstream.PropStream) (*game.Item, error) {
	id, err := ps.ReadUint16()
	if err != nil {
		return nil, err
	}
	item := &game.Item{ID: id}

	attrs, subType, children, err := game.DecodeItemAttributesFrom(ps, 0)
	if err != nil {
		return nil, err
	}
	if attrs != nil {
		item.Attr = attrs
	}
	item.Count = subType

	if children >= 0 {
		for i := int64(0); i < children; i++ {
			child, err := readItem(ps)
			if err != nil {
				return nil, err
			}
			child.Parent = item
			item.Contents = append(item.Contents, child)
		}
		// The container tag is followed by the terminator that closed the attribute
		// list before we were handed control.
		if b, err := ps.ReadUint8(); err == nil && b != 0x00 {
			return nil, fmt.Errorf("tile_store: item %d children not followed by a terminator (got %d)", id, b)
		}
	}
	return item, nil
}

// SaveHouseItems rewrites tile_store from the live map, mirroring
// IOMapSerialize::SaveHouseItemsGuard: the table is cleared and repopulated inside
// one transaction, so a crash mid-save cannot leave houses half written.
func (d *DB) SaveHouseItems(ctx context.Context, world *game.World) (int, error) {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM `tile_store`"); err != nil {
		return 0, err
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO `tile_store` (`house_id`, `data`) VALUES (?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	saved := 0
	for _, house := range world.Houses {
		for _, pos := range house.HouseTiles {
			tile := world.Map.GetTile(pos)
			if tile == nil {
				continue
			}
			blob := encodeTile(tile, pos, world.Items)
			if blob == nil {
				continue
			}
			if _, err := stmt.ExecContext(ctx, house.ID, blob); err != nil {
				return saved, err
			}
			saved++
		}
	}
	if err := tx.Commit(); err != nil {
		return saved, err
	}
	return saved, nil
}

// encodeTile writes one tile's saveable items, or nil when it has none. Mirrors
// IOMapSerialize::saveTile, including the reversed order: C++ builds the list with
// push_front, so the tile's last item is written first.
func encodeTile(tile *game.Tile, pos game.Position, cat *items.Catalog) []byte {
	var saveable []*game.Item
	for _, it := range tile.Items {
		if it == nil || !savedToHouses(it, cat) {
			continue
		}
		saveable = append([]*game.Item{it}, saveable...)
	}
	if len(saveable) == 0 {
		return nil
	}

	ws := propstream.NewPropWriteStream()
	ws.WriteUint16(pos.X)
	ws.WriteUint16(pos.Y)
	ws.WriteUint8(pos.Z)
	ws.WriteUint32(uint32(len(saveable)))
	for _, it := range saveable {
		writeItem(ws, it)
	}
	return ws.GetStream()
}

// writeItem mirrors IOMapSerialize::saveItem.
func writeItem(ws *propstream.PropWriteStream, it *game.Item) {
	ws.WriteUint16(it.ID)
	if it.Attr != nil {
		ws.WriteBytes(it.Attr.Encode(it.Count))
	}
	if len(it.Contents) > 0 {
		ws.WriteUint8(attrContainerItemsTag)
		ws.WriteUint32(uint32(len(it.Contents)))
		// C++ iterates getReversedItems, so contents go out back to front too.
		for i := len(it.Contents) - 1; i >= 0; i-- {
			if it.Contents[i] != nil {
				writeItem(ws, it.Contents[i])
			}
		}
	}
	ws.WriteUint8(0x00) // attribute list terminator
}

// savedToHouses ports Item::isSavedToHouses (src/items/item.cpp): only furniture a
// player could have put there is persisted, not the map's own decoration.
func savedToHouses(it *game.Item, cat *items.Catalog) bool {
	// A non-empty container is kept regardless of its own type, so its contents
	// survive even inside fixed furniture.
	if len(it.Contents) > 0 {
		return true
	}
	if cat == nil {
		return false
	}
	t := cat.Get(it.ID)
	if t == nil {
		return false
	}
	return t.Movable || t.WrapKit || t.IsDoor || t.TypeName == "bed" ||
		t.Type == items.ItemTypeBed || t.Type == items.ItemTypeCarpet
}
