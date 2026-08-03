# Memory Optimization — Handoff

> Purpose: document the server's memory footprint, what has already been cut, and
> the two larger opportunities (a compact `Item`/`Container` split and a lazy
> `MapCache`) with the C++ reference implementation, so a later session can pick
> up either one.
>
> Status: first cut (`Item` attribute blob fold) **done** (`e7e9ed0`). Container
> split and MapCache **planned** — see the sequencing section.

## 1. Measured baseline

With **zero players online**, the `canary-go-server` container sits at:

```
docker stats
canary-go-server  6.867 GiB / 15.15 GiB   24.76%
```

The map alone is the dominant cost. Boot log:

```
loaded OTBM map  file=data-otservbr-global/world/otservbr.otbm  tiles=18102439 items=23619919
house items loaded items=491889
```

The 179 MB `.otbm` expands roughly 35× because every tile and item becomes a Go
struct with slice headers and pointers.

## 2. Memory breakdown (estimate, pre-fold)

| Component | Resident (est.) | Notes |
|---|---|---|
| Map tiles (`map[Position]*Tile`, `internal/game/map.go`) | ~2–3 GB | ~18.1M parsed; the flat map's per-entry overhead dominates, even with the load-time `TileCache` dedup |
| Items (23.6M `*game.Item`) | ~2.5 GB | before `e7e9ed0` each was ~104 bytes |
| Lua datapack (gopher-lua state) | ~1 GB | full OTServBR: 1666 monster types, 1029+ NPCs, achievements, quests |
| Spawns + living creatures | ~0.5–1 GB | 52,468 spawn blocks, ~84k monsters + ~2k NPCs resident after boot |
| Houses + item catalog | ~0.3 GB | 1086 houses, 42,107 item types |

## 3. Done: Item attribute blob fold (`e7e9ed0`)

The raw OTBR attribute blob lived on **every** `Item` as `Attributes []byte`, even
though the OTBM loader never populates it (it is only a round-trip fallback for
blobs `DecodeItemAttributes` cannot model). For ~23.6M attribute-less map items
that was a 24-byte slice header each — ~560 MB of empty headers.

Change: moved the blob into `ItemAttributes.Raw`, which is only allocated for
items that already have attributes.

- `internal/game/item.go` — removed `Attributes []byte` from `Item`; added
  `Raw []byte` to `ItemAttributes`; added `Item.RawAttributes()`.
- `internal/db/item.go` / `internal/db/depot.go` — save: encode from `Attr`,
  fall back to `Attr.Raw` when there are no decoded fields (preserves verbatim
  round-trip of undecodable blobs).
- `internal/db/item_helper.go` — load: stash the blob on `Attr.Raw`; guard the
  empty-blob case (`DecodeItemAttributes` returns a **nil** `Attr` with no
  error — this was a nil-pointer panic on player load).
- `internal/protocol/game_item_move.go` (stack split) and
  `internal/protocol/game_mail.go` (stamped mail) — carry the blob via
  `RawAttributes()`.

`Item` shrank from ~104 → **80 bytes**. Expected server saving ≈ **0.5 GB**.

## 4. Opportunity A: Container split (+ imbuements side-table) — ~1.5 GB

The C++ `Item` (`../canary/src/items/item.hpp`) is tiny: `id`, `count`, a
`weak_ptr` parent and four booleans. Container data lives in a **`Container`
subclass**; attributes are **lazily allocated** (`attributePtr` is a
`unique_ptr`, created on first `setAttribute`). The Go `Item` instead carries
everything on every instance:

| Field on every Go `Item` | Bytes | Only meaningful for |
|---|---|---|
| `Contents []*Item` | 24 | containers |
| `MaxSize`, `MaxItems` | 4 | containers |
| `Unlocked`, `Pagination`, `Actor` | 3 (+pad) | containers |
| `Parent *Item` | 8 | containers |
| `Imbuements map[uint8]...` | 8 | imbued items |
| `imbueMu sync.Mutex` | 8 | imbued items (concurrent access) |

Removing these from the base `Item` leaves `ID`+`Count`+`Attr` ≈ **24 bytes**,
saving ~56 bytes per non-container item. With ~22M non-container items on the
map that is **~1.3 GB**, and the imbuement fields alone (16 B × 23.6M) another
**~380 MB** — together approaching **~1.7 GB**.

### Suggested shape (mirrors C++ `Container : Item`)

```go
type Item struct {
    ID    uint16
    Count uint16
    Attr  *ItemAttributes
}

type Container struct {
    Item
    Contents   []*Item
    MaxSize    uint16
    MaxItems   uint16
    Unlocked   bool
    Pagination bool
    Parent     *Container
    Actor      bool
}
```

Imbuements can go either on `Container` or into a side table
(`map[*Item]map[uint8]ImbuementInfo` + a single package/world mutex — the
per-item `imbueMu` protects a rare map against the dispatcher ticking combat
durations while a connection goroutine equips; contention is negligible, so a
single lock is fine and drops 8 B/item).

### Files that construct containers today (must create `*Container` instead)

- `internal/game/depot.go` (locker, depot chest, inbox)
- `internal/game/forge.go` (exaltation chest)
- `internal/luaengine/player.go` (inbox)
- `internal/protocol/game_browse_field.go`, `internal/protocol/game_mail.go`,
  `internal/db/market_expiry.go`

Plus every reader of `.Contents` / `.MaxSize` / `.Pagination` / `.Parent`:
inventory, depot, container API (`internal/luaengine/container.go`,
`internal/protocol/game_containers.go`), item move, holding-count, etc. Most
call sites will do a `if c, ok := item.(*Container); ok` type switch.

### Risk

Medium. The container system is exercised by inventory, depot, browse field and
mail. `go test ./internal/game/... ./internal/luaengine/... ./internal/db/...`
plus a live login + open-a-backpack smoke test covers it.

## 5. Opportunity B: MapCache / lazy tile materialization — 3–4 GB+

The C++ loads the OTBM into a **compact cache and materializes full tiles on
demand**:

- `../canary/src/io/iomap.cpp:135,254` — the loader builds `BasicTile` records
  and calls `map.setBasicTile(...)`. It does **not** create `Tile`/`Item`
  objects during load.
- `../canary/src/map/mapcache.hpp` — `BasicTile`/`BasicItem` are lightweight
  hashed records (`id`, `charges`, `actionId`, `uniqueId`, teleport dest, text,
  sub-items); `isCacheShareable()` lets simple tiles be **shared** across
  positions; storage is sector-based (`mapSectors`).
- `../canary/src/map/map.cpp:275,329,366` — `Map::getOrCreateTile` →
  `getOrCreateTileFromCache(...)` materializes a full `Tile` only when a
  player/creature/script actually touches the tile.

The Go port instead materializes **everything** eagerly:
`internal/otbm/otbm.go:400-413` creates a `*game.Tile` (and a `*game.Item` per
item node) for every non-empty tile and stores them in `map[Position]*Tile`
(`internal/game/map.go`). That flat map + the 23.6M items is the ~4.5 GB chunk.

### Adopting the idea in Go

1. Keep the load-time `TileCache` dedup (`internal/otbm/mapcache.go`) but store
   a **compact tile record** keyed by position instead of a full `*Tile`:
   ground id, item ids+counts, flags, house id. Simple tiles (the vast
   majority) share one immutable record.
2. Replace the flat `map[Position]*Tile` with a **sector index**
   (`map[uint32]*sector` like C++ `mapSectors`, or a 2D grid), so a viewport
   query only touches the sectors a player can see.
3. Materialize a real `*Tile` on `Map.GetTile` for the rare tiles that need it
   (action/unique ids, teleports, doors, house tiles) or when something mutates
   the tile. Cache the materialized tiles by sector, evict on
   player/creature leaving.

This is the largest change and touches the hot path (`GetTile`, creature
vision, pathfinding). Sequencing it **after** the `Container` split keeps the two
refactors independent and each testable on its own.

### What it buys

Only the tiles players are near (and containers/imbued items) stay resident.
The ~4.5 GB of eager tiles+items drops toward the fraction actually in view —
**the single biggest lever** after the Container split.

## 6. Sequencing recommendation

1. ✅ **Done** — attribute blob fold (`e7e9ed0`, ~0.5 GB).
2. → **Container split + imbuements side-table** (~1.5–1.7 GB). Self-contained,
   medium risk, unlocks the numbers below.
3. → **MapCache / sector + lazy materialization** (3–4 GB+). Big architectural
   change; do it on its own after the Item layout is stable.

After 1+2 the server should idle near **~4 GB**; after 3, materially less.

## 7. Reference file map

Go (this repo):
- `internal/game/item.go` — `Item`, `ItemAttributes`, `RawAttributes()`
- `internal/game/item_attr.go` — `DecodeItemAttributes`, `Encode`
- `internal/game/map.go` — `Map`, `RangeRect`, `WalkableFor`
- `internal/otbm/otbm.go` — OTBM loader (eager `*Tile`/`*Item`)
- `internal/otbm/mapcache.go` — load-time tile dedup
- `internal/game/depot.go`, `internal/game/forge.go` — container construction

C++ reference (`../canary/src`, on disk):
- `items/item.hpp` — compact `Item`, lazy `ItemAttribute`
- `map/mapcache.hpp` / `map/mapcache.cpp` — `BasicTile`/`BasicItem`, sectors
- `map/map.cpp` — `getOrCreateTile` (lazy materialization)
- `io/iomap.cpp` — OTBM → `BasicTile` cache

## 8. Verification

- Tests: `go test ./internal/game/... ./internal/db/... ./internal/luaengine/...`
  (the two failing `internal/protocol` stackpos tests are pre-existing and
  unrelated).
- Live check: `docker stats` with zero players before/after, plus login +
  open-a-backpack + deposit-in-locker + send-mail smoke tests.
- The server panicked on player load once (nil `Attr` on an empty attribute
  blob); the load path in `internal/db/item_helper.go` now guards it.
