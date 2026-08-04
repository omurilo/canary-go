package db

import (
	"context"
	"fmt"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/io/propstream"
	"github.com/omurilo/canary-go/internal/items"
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
			if placeHouseItem(world.Items, tile, it) {
				restored++
			}
		}
	}
	return restored, rows.Err()
}

// placeHouseItem puts one restored item on the tile, the two-branch placement of
// IOMapSerialize::loadItem (src/io/iomapserialize.cpp:158-225).
//
// The branch is the whole point, and it was missing: every row was appended.
// A door is saved to tile_store on purpose — isSavedToHouses includes getDoor()
// (item.cpp:2348) — because the row records whether it is open or shut. But the
// door itself already exists on the tile, from the map. Appending it added a
// SECOND door on every start:
//
//   - the tile kept a closed door however many times the server had booted, so it
//     stayed blocking and walking into it answered "Sorry, not possible";
//   - clicking transformed one of them, so the open state was real but invisible
//     behind the others, and only a relog appeared to fix it;
//   - the client's stack held all of the phantom copies, which is why it reported
//     the door at index 6 while the server computed 2.
//
// Only house tiles pass through tile_store, which is exactly why only house doors
// misbehaved.
//
// Returns false when the row describes something the map no longer has; C++ reads
// those attributes into a dummy and drops them.
func placeHouseItem(cat *items.Catalog, tile *game.Tile, it *game.Item) bool {
	if tile == nil || it == nil {
		return false
	}
	t := cat.Get(it.ID)

	// Things a player genuinely put there are new objects. Beds, carpets and trash
	// holders join them because C++ treats them as created rather than matched.
	if t == nil || t.Movable ||
		t.Type == items.ItemTypeBed ||
		t.Type == items.ItemTypeCarpet ||
		t.Type == items.ItemTypeTrashHolder {
		tile.Items = append(tile.Items, it)
		return true
	}

	// Stationary map furniture — doors, blackboards, bookcases. Find the one that
	// is already there and restore its state onto it.
	for _, existing := range tile.Items {
		if existing == nil {
			continue
		}
		et := cat.Get(existing.ID)
		match := existing.ID == it.ID ||
			(t.IsDoor && et != nil && et.IsDoor) ||
			// C++ keeps a bed arm here too, though the branch above already claimed
			// every bed; mirrored so the two read the same.
			(t.Type == items.ItemTypeBed && et != nil && et.Type == items.ItemTypeBed)
		if !match {
			continue
		}
		// g_game().transformItem(item, id) — the saved id wins, so a door saved open
		// comes back open.
		existing.ID = it.ID
		if it.Attr != nil {
			existing.Attr = it.Attr
		}
		existing.Count = it.Count
		if it.Container != nil && len(it.Container.Contents) > 0 {
			if existing.Container == nil {
				existing.Container = game.NewContainer(8)
			}
			existing.Container.Contents = it.Container.Contents
			for _, c := range existing.Container.Contents {
				if c.Container != nil {
					c.Container.Parent = existing
				}
			}
		}
		return true
	}

	// The map changed since the last save.
	return false
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
			if item.Container == nil {
				item.Container = game.NewContainer(8)
			}
			if child.Container != nil {
				child.Container.Parent = item
			}
			item.Container.Contents = append(item.Container.Contents, child)
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
	// Count what a save would actually write, and what is already stored, BEFORE
	// touching the table. SaveHouseItems used to unconditionally `DELETE FROM
	// tile_store` and repopulate; a save that ran while the world had no house
	// items in memory (a shutdown during a partial boot, or after a crash)
	// permanently wiped every player-placed item — with no binlog or backup,
	// unrecoverable. Refuse to wipe when there is nothing to write and rows exist.
	saveable := 0
	for _, house := range world.Houses {
		for _, pos := range house.HouseTiles {
			if tile := world.Map.GetTile(pos); tile != nil && encodeTile(tile, pos, world.Items) != nil {
				saveable++
			}
		}
	}
	var existing int
	if err := d.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM `tile_store`").Scan(&existing); err != nil {
		return 0, err
	}
	if saveable == 0 && existing > 0 {
		return 0, fmt.Errorf("refusing to wipe %d tile_store rows: world has no saveable house items (partial boot?)", existing)
	}

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
	if it.Container != nil && len(it.Container.Contents) > 0 {
		ws.WriteUint8(attrContainerItemsTag)
		ws.WriteUint32(uint32(len(it.Container.Contents)))
		// C++ iterates getReversedItems, so contents go out back to front too.
		for i := len(it.Container.Contents) - 1; i >= 0; i-- {
			if it.Container.Contents[i] != nil {
				writeItem(ws, it.Container.Contents[i])
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
	if it.Container != nil && len(it.Container.Contents) > 0 {
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
