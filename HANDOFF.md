# canary-go — Handoff / Migration Status

A Go port of the **Canary** C++ Tibia 13.x MMORPG server. Goal: reach feature
parity with the C++ server (`../src`), one subsystem at a time, validated against
the official BattlEye client.

- **Repo:** `./canary-go` (nested git repo; module `github.com/opentibiabr/canary-go`, Go 1.25). Work branch `dudantas/item-mechanics`.
- **C++ reference:** `../src` (~130k LOC). Go so far: ~26k LOC (`internal/`). **The C++ is the spec** — when porting, find the function in `../src` and mirror it (rules AND wire bytes).
- **Datapacks:** core Lua in `../data`; live content in `../data-otservbr-global` (map/monsters/npcs/spells). Items: `../data/items/{items.xml,appearances.dat}`.
- **DB:** MariaDB (host port 3307). Schema = repo-root `schema.sql` (canonical Canary schema).

Build/test (must stay green): `cd canary-go && go build ./... && go test ./...`
Run (Docker): `docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build canary-go`. The user usually runs the server themselves — you mostly just keep build+tests green and they recompile.

---

## 1. Current state — the working vertical slice

A player can, **end-to-end against the real client**: log in → walk/turn/auto-walk →
change floors via stairs/ramps/holes/teleports → chat → talk to NPCs (dialogue,
shop buy/sell, bank, travel, become a citizen) → manage inventory & containers →
buy/sell moving real items + gold → deposit/withdraw → eat food (regen + "full") →
fight monsters (melee + basic spells, loot, xp) → die and respawn at their town
temple → form a party (invite/join/shields/shared-exp toggle). GM/GOD chars are
untargetable by monsters.

This is the "essential playability" milestone. Everything below the line is what
separates that from full parity.

**Scale check:** C++ `creatures/` ≈ 43k LOC, `lua/` ≈ 37k, `game/` ≈ 16k. The Lua
`player_functions.cpp` alone is 5,600 lines. The Go Lua layer has **414 stubbed
methods** (`grep -rn "not modelled yet\|safe default\|TODO: implement" internal/luaengine`).
So the port is roughly "core loop done, long tail of systems remaining."

---

## 2. C++ → Go migration matrix

Legend: ✅ done · 🟡 partial (works but incomplete/simplified) · ❌ not started

| Subsystem | C++ location | Go location | Status | Notes / gaps |
|---|---|---|---|---|
| Net framing / crypto (RSA, XTEA, Adler, seq) | `server/network` | `network`, `tibcrypto`, `transport`, `netmsg` | ✅ | Real client connects/authenticates. Bundled headless client's 7172 handshake still fails (pre-encryption challenge framing) — doesn't affect BattlEye. |
| Login (char list) | `server` | `protocol` (login), `db/auth` | ✅ | 7171 old-style + login-server (8088) by email. |
| OTBM map | `map`, `io` | `otbm`, `game/map.go` | ✅ | Parses 1.94M tiles, 8 towns, spawns. Stores uid/aid. |
| Item catalog | `items` | `items` | ✅ | appearances.dat + items.xml merged. Most flags parsed. |
| Player core (stats/skills/look) | `creatures/players` | `game/player.go` | 🟡 | Stats/level/exp/mana/cap/soul work. Skill **tries** not accumulated; maglevel percent simplified; varStats/wheel bonuses stubbed. |
| Movement (walk/turn/autowalk/floors) | `game`, `map` | `protocol/game_actions.go`, `game/world.go` | ✅ | Walk pacing (level-scaled speed), stairs (height ramps), floor-change tiles with offsets, teleports. |
| Inventory / equipment | `creatures/players` | `game/player_inventory.go` | ✅ | Slots, capacity/weight, add/remove/find, stack split. Equip stat bonuses NOT applied. |
| Containers | `items/containers` | `game/player_containers.go`, `protocol/game_containers.go` | ✅ | Open state unified on Player; real capacity; add/update/remove packets. Pagination (store inbox) minimal. |
| Money & bank | `game`, `account` | `game/player_money.go`, `luaengine/bank.go` | ✅ | Coins 3031/3035/3043; inventory-first + bank fallback; transfer (online only). |
| NPC shop / trade | `game`, `creatures/npcs` | `protocol/game_actions.go`, `luaengine/npc*` | ✅ | Buy/sell/close, currency, auto-close on range. Shopping-bags & gold-pouch sell-all not done. |
| NPC dialogue / travel / set-town | `creatures/npcs` | `luaengine`, datapack npclib | ✅ | keyword handler, delayed say, travel, citizen tiles. |
| Combat (melee + spell damage) | `creatures/combat` | `game/combat_engine.go`, `game/combat/*` | 🟡 | Melee + spell damage/heal via one hit/death path. **Missing:** PvP rules, damage-split across attackers, full element/absorb/block, crit/leech, condition DoT. |
| Monsters | `creatures/monsters` | `game/monster.go`, `game/ai_engine.go`, `creatures` | 🟡 | Spawn, aggro/pathfind, melee, loot roll, xp to killer. **Missing:** monster spells/abilities, summons, targeting strategies, threat, exp split. |
| Spells | `creatures/combat/spells` | `spells`, `luaengine/spell.go` | 🟡 | Instant-spell cast + combat damage. **Missing:** most support/condition spells, runes, conjure, haste (speed), cooldown edge cases. |
| Conditions | `creatures/combat/condition*` | `game/conditions.go`, `luaengine/condition.go` | 🟡 | Regeneration/food modelled; generic storage for others. **Missing:** DoT (poison/fire/energy), attribute conditions, haste/paralyze **speed**, drunk, light, most icons. |
| Death / respawn | `creatures` | `game/player_death.go`, `protocol/game_death.go` | 🟡 | Temple respawn + 0x28 relogin window + exp/level loss. **Missing:** per-vocation stat downgrade, skill/mana loss, black-skull vitals, blessings, corpse ownership. |
| Party | `creatures/players/grouping` | `game/party.go`, `game/world_party.go` | 🟡 | Invite/join/leave/lead/disband/shared-exp/shields. **Missing:** analyzer (loot/supply/dmg), shared-exp level/distance gating. |
| Vocations | `creatures/players/vocation` | `game/vocations` | 🟡 | Loaded (base speed/attack/gains). **Missing:** per-vocation formulas, skill multipliers, HP/mana/cap per-level gains applied on level-up. |
| Events: Action / MoveEvent / TalkAction | `lua`, `game` | `actions`, `moveevents`, `talkactions` | 🟡 | Registered & firing (by id/uid/aid). CreatureEvent/GlobalEvent/Zone largely **❌** → most of the ~560 load-time warns. |
| Persistence (DB) | `io`, `database` | `db` | 🟡 | players core + item blob + accounts/auth + async jobs. **Missing saves:** maglevel, skills+tries, manaspent, conditions, storages, learned spells, blessings, stamina, town; **missing tables:** depot/inbox/stash, player_storage, player_spells, VIP, etc. |
| Lua API surface | `lua/functions` (~1300 fns) | `luaengine` | 🟡 | Core Creature/Player/Item/Container/Npc/Position/Game/Condition/Combat/MoveEvent/Action/Town/Party done. **414 stubs remain.** |

---

## 3. What's left, roughly prioritized

**A. Deepen the core loop (highest gameplay value)**
1. **Conditions engine** — DoT (poison/fire/energy/bleed), attribute mods, **haste/paralyze speed** (needs `0x8F` sendChangeSpeed + `ConditionSpeed` formula `min=mina*(var-40)+minb` → `Player.SpeedBonus`, with expiry). Unlocks combat spells, food-buff icon, many scripts.
2. **Combat completeness** — element/absorb/block/defense, damage split across attackers, PvP rules, crit/leech, monster spells & summons.
3. **Persistence gaps** — save skills/tries/maglevel/manaspent/conditions/storages/spells; add depot/inbox/stash + player_storage tables. Without this, progression doesn't survive relog.
4. **Status icons** — `0xA2 sendIcons` (poison/haste/hungry/pz/etc.).

**B. Progression systems (each = model + DB table + protocol packets + Lua bindings)**
mounts/outfits, blessings, bestiary/bosstiary, imbuement, charms, prey, task hunting,
achievements/titles, wheel of destiny, familiars, forge, market, store/Tibia coins,
reward system, depot/stash, hazard, animus mastery, concoctions, houses.
Each is a full subsystem; the 414 Lua stubs map to these. Do them one at a time,
each validated in-client.

**C. Long-tail Lua/events** — CreatureEvent / GlobalEvent / Zone registration
(kills most of the ~560 load warns), quest catalogs, gamestore modules.

Suggested order: A1 (conditions/haste) → A2 (combat) → A3 (persistence) → A4 (icons)
→ then B systems by player demand → C cleanup.

---

## 4. Architecture (how the layers connect)

- **`game`** = authoritative world model (World, Map/Tile, Player/Monster/Npc, Item, combat/AI engines, conditions, inventory, party, death). **No wire code.** It notifies clients only through: (a) callback fields on `World` (`OnCreatureMove`, `OnPlayerDeath`, `OnShieldUpdate`, `OnChangeSpeed`, `OnIconsUpdate`, …) wired in `cmd/canary/main.go` to `protocol.*` broadcasters, and (b) the **`game.Session`** interface (implemented by `*protocol.GameProtocol`) for per-player pushes (SendStats, OpenContainer, SendChangeSpeed, SendIcons, …).
- **`protocol`** = `GameProtocol`, the per-connection codec: parse inbound opcodes (31 handled; see `OnPacket` in `game.go`), encode outbound (~22 opcodes). Implements `game.Session`.
- **`luaengine`** = gopher-lua VM + the ~1300-fn API as metatables/globals. Most remaining work lives here (bindings) + backing model in `game`.
- Support: `items` (catalog), `otbm` (map), `creatures` (types), `spells`/`actions`/`moveevents`/`talkactions`/`events` (script registries), `db` (MariaDB), `network`/`netmsg`/`tibcrypto`/`transport` (wire), `appproto` (generated protobuf).
- Boot: `cmd/canary/main.go` → catalog → OTBM → vocations → datapack Lua → wire `world.On*` callbacks → start services.

---

## 5. Critical invariants (READ before porting — these silently break the game)

1. **revscriptsys metatable contract.** `data/libs/functions/revscriptsys.lua` overwrites the `__index` of Player/Monster/Npc/Item/Container/Teleport with `getmetatable(self)[key]`. So Lua methods for those types MUST be **directly on the metatable** (`SetFuncs(mt, methods); SetField(mt,"__index",mt)`), never in a separate index table. Engine-bound methods (need `e.world`/catalog) go via `SetField(mt, name, e.L.NewFunction(e.method))`.
2. **Mirror the C++ for anything wire-facing.** Field order/width in packets, tile stack order (ground→on-top→creatures→normal), player-only vocation byte in AddCreature, 0xAA always carries a position (except PRIVATE_PN→NPCs only). Diagnose client crashes by base64-decoding the client's error packet (`echo <b64> | base64 -d | xxd`).
3. **Keep-alive:** server must send ping `0x1D` every 5 s or the client drops.
4. **Floor changes resolve BEFORE the walkability check** (target tile often has no ground): height ramps (`Game::internalMoveCreature`) + floor-change tiles with directional offsets (`Tile::queryDestination`). Far teleports must re-send the full map to the moved player's OWN client (`sendFullMapAt`), not just spectators.
5. **Container open-state** has one source of truth: `game.Player.openContainers` (cid→OpenContainer; `GetContainerID` returns -1 when closed, cid 0 is valid).
6. **OTBM:** store ATTR_UNIQUE_ID/ACTION_ID; `str()` must un-escape (read n chars via `u8()`).
7. **StepIn/Action dispatch:** wrap the creature with its concrete metatable (`metatableForCreature`) or `getPlayer()` returns nil; pass items as `luaItem{item,pos}` not raw `*game.Item`; movements are looked up by item id + uid + aid.
8. **Items:** never hand-encode — use the catalog-aware `protocol.addItem`. Coins are 3031/3035/3043.
9. **gopher-lua:** `dofile/loadfile` are resilient (log+swallow) and preprocess `\z`; chunks named by file path; `string.gsub` numeric-replacement shim in `registerLuaCompat`.

---

## 6. Testing methodology

- Test client = official **CipSoft/BattlEye 13.x** (not OTClient, but `../otclient` is a good protocol reference). It logs `Error while processing network packet ... at position N` + a base64 of the bad packet — decode it to find the exact byte.
- Prefer a **failing Go test first**: pure helpers were extracted for this (`stairDestination`, `floorChangeDestination`, `InternalAddItem`, `PartyShield`, `CannotBeAttacked`, …). Some behaviors only reproduce with the FULL datapack libs loaded (see `luaengine/food_repro_test.go` loading all of `data/libs`).
- Loop: `go build ./...` + `go test ./...` green → user recompiles → validate the specific behavior in-client → if "still broken", get the SPECIFIC symptom (state changed? server-log Lua error? just the display?).

---

## 7. Detailed change history

Living blow-by-blow (every fix, with C++ line references and the reasoning) is in the
assistant's project memory file `canary-go-migration.md` — ask the user to share it
when you need the full history. Recent focus (2026-07): the essential-playability
slice (inventory/containers/money/shop/death/party), then movement (speed, stairs,
floor-changes), food/regeneration, and the temple/citizen teleport chain.
