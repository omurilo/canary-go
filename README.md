# Canary-Go

A port of the [Canary](https://github.com/opentibiabr/canary) MMORPG server
(C++, Tibia 13.x protocol) to **Go**, with **MariaDB** persistence and an
embedded **Lua** scripting engine.

The C++ source in `../src` is the specification. Every divergence found so far
has turned out to be a silent bug, so the working rule is 1:1 with upstream, and
anything that cannot be 1:1 is written down in a comment next to the code rather
than left for someone to discover.

## How parity is measured

Not by reading a document — the previous write-up went stale without anyone
noticing, and three of its headline numbers were wrong by the time they were
acted on. Two scripts re-derive everything on each run.

```bash
scripts/parity.sh            # syntactic: does a counterpart exist?
scripts/semantic-parity.sh   # semantic: is it reachable, and does it decide as much?
```

`parity.sh` walks the C++ methods of a class and asks whether a Go function of
that name exists. That question can be satisfied by a one-line stub, so it is
only half the answer. Methods that will never have a counterpart are listed
explicitly with a reason — trivial accessors, and this fork's async scheduling
machinery — so the denominator stays honest.

`semantic-parity.sh` asks the two harder questions:

- **Is it reachable?** A method nothing calls is not parity, it is dead code
  with the right name. This port has shipped plenty of it.
- **Does it decide as much?** Branch points are a crude but hard-to-fake proxy
  for how much of the original logic survived. A 40-branch C++ function
  reimplemented in 3 branches did not survive.

Both are proxies, and the output says "go and look", not "this is wrong". Go
legitimately needs fewer branches than C++ for the same logic, so a ratio a
little under 100% is normal; near zero is the signal.

### Current state

| class | methods | decisions (C++ → Go) | reachable |
|---|---|---|---|
| `Monster` | 101/101 | 574 → 630 | 68/102 |
| `Npc` | 43/43 | 121 → 133 | 30/43 |
| `House` | 29/29 | 96 → 92 | 12/29 |
| `Decay` | 4/4 | 46 → 38 | 3/4 |
| `SpawnMonster` | 15/15 | 54 → 59 | 7/15 |

The decision ratios say the logic survived translation. The reachable column is
the open work: **73 ported methods still have no caller**, mostly in the house
flow and the NPC shop, because the protocol layer still reaches past them to the
older code paths. Porting the behaviour and wiring the callers are two jobs and
only the first is finished.

Broader counts from `parity.sh`: 152 → 140 inbound opcodes dispatched, 159 → 112
outbound, 1225 → 988 Lua class methods, 35 of 48 schema tables referenced from
code. Lua enum parity is closed and pinned by `TestRegisteredEnumsMatchUpstream`.

## What works end to end

- **Protocol core (byte-exact):** raw RSA-1024, XTEA (32 rounds), Adler-32 and
  monotonic-sequence checksums, and the Tibia 13.x transport framing with its
  padding-byte payload — see [`docs/PROTOCOL.md`](docs/PROTOCOL.md).
- **Login, both paths:** binary on 7171 for the OTClient family, and the
  HTTP/JSON login the official client uses. The binary path needs the world-name
  line and the sequence slot stripped; both are covered by tests.
- **Game server (7172):** enter world, movement, chat, trade, containers,
  cyclopedia, bestiary/bosstiary, imbuements, quick loot, market.
- **Monsters:** the full `onThink` pipeline (yell, defense spells, summons,
  target re-selection), `getDistanceStep`, target and friend lists with
  factions, defensive stats and immunities, the challenge timers.
- **NPCs:** spectator tracking and idling, the conversation lifecycle, the
  per-player shop registry.
- **Persistence:** MariaDB against the canonical `schema.sql`; item attribute
  TLV blobs round-trip with the C++ server.
- **Lua:** an embedded gopher-lua VM loading the otservbr datapack.
- **A headless Go client** (`cmd/canary-client`) that logs in and plays.

## Requirements

- Go 1.25+
- Docker (for MariaDB)
- `protoc` to compile `appearances.proto`

## Quick start

```bash
cd canary-go
make db-up     # MariaDB on localhost:3307
make smoke     # build → server → client → teardown
```

Running the pieces separately:

```bash
make db-up
make run       # applies the schema, seeds account god/god
make client    # in another shell: connects, logs in, walks, chats, logs out
```

## Configuration

`config.lua` is executed as Lua, the same idea as the C++ server.

| Key | Default | Meaning |
|-----|---------|---------|
| `loginProtocolPort` | 7171 | login server port |
| `gameProtocolPort` | 7172 | game server port |
| `mysqlHost`/`mysqlPort`/`mysqlUser`/`mysqlPass`/`mysqlDatabase` | localhost:3307, canary/canary | MariaDB connection |
| `rsaKeyFile` | `key.pem` | RSA private key (the standard OT key) |
| `motd` | — | message of the day |

Server flags: `-config`, `-schema`, `-scripts`, `-migrate`, `-seed`,
`-logLevel`. An explicit `-logLevel` beats `config.lua`; config only fills in
the default.

## Connecting a real client

The server implements `CLIENT_VERSION = 1525` (Tibia 13.x). Both the official
client (HTTP login) and the OTClient family (binary 7171) connect. Point either
at `127.0.0.1:7171` with this repository's `key.pem`.

## Project layout

```
cmd/canary/          server entrypoint
cmd/canary-client/   headless test client
internal/
  tibcrypto/         RSA-1024 (raw), XTEA, Adler-32
  netmsg/            NetworkMessage reader/writer
  transport/         framing: checksum + XTEA + modern padding + length
  network/           TCP listener + per-connection read loop
  config/            config.lua loader
  db/                MariaDB pool, schema apply, repositories, auth
  game/              world model, creatures, combat, spawns, houses, decay
  creatures/         MonsterType / NpcType and the datapack registry
  protocol/          login (7171) + game (7172) handlers
  luaengine/         embedded gopher-lua VM and the ~1000-method API bridge
  kv/                the kv_store abstraction
schema/schema.sql    canonical Canary schema
docs/PROTOCOL.md     reverse-engineered wire-protocol reference
deploy/              docker-compose (MariaDB, MyAAC, login-server)
scripts/             parity tooling + smoke.sh + Lua startup
```

## Tests

```bash
make test        # crypto and transport round-trips, no DB needed
go test ./...    # everything, including the datapack-backed tests
```

The datapack tests load real monster and NPC files from
`../data-otservbr-global` and skip when it is not present.

## What is next

1. **Wire the 73 unreachable methods.** The behaviour is ported and tested; the
   callers still take the old paths.
2. **Measure the classes nobody has looked at.** `Monster`, `Npc`, `House`,
   `Decay` and `SpawnMonster` are covered. Point `behaviour_coverage` at another
   class in `scripts/parity.sh` and its gap becomes visible.
3. **Concurrency model.** The dispatcher exists with its lanes transcribed, but
   the protocol layer still mutates players directly from the connection
   goroutine under an ad-hoc mutex rather than enqueueing onto a lane.
