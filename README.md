# Canary-Go

A migration of the [Canary](https://github.com/opentibiabr/canary) MMORPG server
(C++, Tibia 13.x protocol) to **Go**, with **MariaDB** persistence, an embedded
**Lua** scripting engine, and a robust Game Dispatcher.

> Status: **working vertical slice.** The wire protocol, account login, 
> enter-world, movement, chat, ping and logout with MariaDB persistence all work end-to-end.
> Additionally, the Combat Engine, A* Pathfinding, PropStream codecs, and the vast majority 
> of the ~1300-function Lua API are fully mapped and integrated.

## What works today

- **Protocol core (byte-exact):** raw RSA-1024, XTEA (32 rounds), Adler-32 and
  monotonic-sequence checksums, and the modern (Tibia 13.x) transport framing
  with the padding-byte payload — see [`docs/PROTOCOL.md`](docs/PROTOCOL.md).
- **Login server (7171):** account authentication (Argon2 + legacy SHA-1), MOTD,
  session key and the character-list packet.
- **Game server (7172):** login challenge, RSA/XTEA handshake, the full
  enter-world sequence (self-appear, map description, stats, skills, light,
  basic data), plus movement, turning, chat, ping/pong and logout.
- **MariaDB:** schema runs against the canonical `schema.sql`; account/player repositories; player position/vitals persisted on logout.
- **Item Persistence:** `PropStream` binary blobs seamlessly save/load inventories and nested containers.
- **Combat & AI:** Native `Dispatcher` loop, `A* Pathfinding`, and a complete `Combat Engine` (damage formulas, cooldowns, conditions).
- **Lua:** an embedded gopher-lua VM (Lua 5.1) loads `scripts/*.lua`; `onPlayerLogin`
  and `onPlayerSay` hooks fire from the game server, and `Game.getPlayerCount()`
  is bound from Go.
- **A headless Go client** (`cmd/canary-client`) that logs in and plays, proving
  server↔client interoperability.

## Requirements

- Go 1.25+
- Docker (for MariaDB)
- `protoc` (protobuf-compiler) to compile `appearances.proto`

## Quick start

```bash
cd canary-go

# 1. Start MariaDB (mapped to localhost:3307).
make db-up

# 2. Run the full end-to-end smoke test (build → server → client → teardown).
make smoke
```

Expected client output ends with:

```
✅ login OK — character selected: "Gm Test"
   entered world
   ← Gm Test says: "Hello from canary-client!"
   ← pong
✅ game session completed successfully
```

### Running the pieces separately

```bash
make db-up                       # MariaDB on :3307
make run                         # server: applies schema, seeds account god/god
# in another shell:
make client                      # connects, logs in, walks, chats, logs out
```

## Configuration

`config.lua` is executed as Lua (same idea as the C++ server). Key settings:

| Key | Default | Meaning |
|-----|---------|---------|
| `loginProtocolPort` | 7171 | login server port |
| `gameProtocolPort` | 7172 | game server port |
| `mysqlHost`/`mysqlPort`/`mysqlUser`/`mysqlPass`/`mysqlDatabase` | localhost:3307 canary/canary | MariaDB connection settings |
| `rsaKeyFile` | `key.pem` | RSA private key (the standard OT key; the client has the matching public key) |
| `motd` | — | message of the day |

Server flags: `-config`, `-schema`, `-scripts`, `-migrate` (apply schema),
`-seed` (create the `god`/`god` test account and a `Gm Test` character).

## Connecting a real client

The server implements the modern binary login + game protocol for
`CLIENT_VERSION = 1525` (Tibia 13.x). An **OTClient**-family client configured
for that protocol version, pointed at `127.0.0.1:7171`, using this repository's
`key.pem` public key, follows the same handshake the bundled Go client verifies.
Enter-world packet-shape parity for the official CipSoft client should be
validated against a live client (the protocol *mechanics* — RSA/XTEA/checksum/
framing/login — are byte-exact and covered by tests).

## Project layout

```
cmd/canary/          server entrypoint
cmd/canary-client/   headless test client (login + play)
internal/
  tibcrypto/         RSA-1024 (raw), XTEA, Adler-32
  netmsg/            NetworkMessage reader/writer (LE primitives, strings, positions)
  transport/         framing/codec: checksum + XTEA + modern padding + length
  network/           TCP listener + per-connection read loop
  config/            config.lua loader
  db/                pgx pool, schema apply, account/player repos, async jobs, auth
  game/              world model: Position, Outfit, Player, Tile, Map, World
  protocol/          login (7171) + game (7172) protocol handlers
  luaengine/         embedded gopher-lua VM + starter API bridge
schema/postgres.sql  PostgreSQL schema (converted from MySQL)
docs/PROTOCOL.md     reverse-engineered wire-protocol reference
deploy/              docker-compose (PostgreSQL)
scripts/             Lua startup scripts + smoke.sh
```

## Tests

```bash
make test    # crypto round-trips + transport codec round-trips (no DB needed)
```

## Roadmap

1. **Real-client validation** of `0xA0`/`0xA1`/enter-world byte layouts against a live Tibia/OTClient.
2. **Further Lua API Polish** testing complex scripts.
