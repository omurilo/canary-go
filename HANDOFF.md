# Handoff

For whoever picks this up next — human or model. Read this before touching
anything; section 1 is the part that will save you the most time.

Last updated after the reachability work landed (`d5fa474`).

---

## 1. The rules that are not negotiable

**`../src` is the specification.** Never modify it. Every divergence found so
far has turned out to be a silent bug — not one has turned out to be an
improvement. When Go cannot express something 1:1, write the reason in a comment
next to the code rather than leaving it for the next person to rediscover.

**Do not trust prose, including this file.** The two parity scripts re-derive
every number on each run. If this document and `scripts/semantic-parity.sh`
disagree, the script is right and this file is stale. That rule exists because
the previous version of this document had three headline numbers that were wrong
by the time someone acted on them.

**Commit only inside `canary-go/`, and stage only files you touched.** The
repository owner runs parallel work in the same tree — `docs/MIGRATION-STATE.md`
is theirs. `git add -A` from the repo root will sweep it up. Push with
`GIT_SSH_COMMAND=ssh git push`.

**Do not stop or drop the MariaDB container on port 3307.** It is shared with
MyAAC and the login-server.

**No Python in this repo.** Tooling goes in bash, or as a Go test.

**The live datapack is `canary-go/data/`.** Monster and NPC definitions come
from `../data-otservbr-global/`. The OTBM in the working tree is not the one the
running server has loaded.

---

## 2. How to know where you are

```bash
bash scripts/parity.sh            # syntactic: does a counterpart exist?
bash scripts/semantic-parity.sh   # semantic: is it reachable, and does it decide as much?
go build ./... && go test ./...
```

`parity.sh` walks the C++ methods of a class and asks whether a Go function of
that name exists. That can be satisfied by a one-line stub, so it is half an
answer. Methods that will never have a counterpart are listed in `SKIP_METHODS`
with a reason, so the denominator stays honest.

`semantic-parity.sh` asks the two harder questions:

- **Is it reachable?** A method nothing calls is not parity, it is dead code
  with the right name.
- **Does it decide as much?** Branch-point counts are crude but hard to fake. A
  40-branch C++ function reimplemented in 3 branches did not survive.

Both are proxies. A ratio slightly under 100% is normal — Go needs fewer
branches than C++ for the same logic. Near zero is the signal.

### Current reading

```
Monster        compared 102  decisions C++ 574   Go 621   108%   thin 3   dead 3 (+3 dead upstream too)
Npc            compared 43   decisions C++ 121   Go 142   117%   thin 0   dead 0
House          compared 29   decisions C++ 96    Go 93     96%   thin 0   dead 1
Decay          compared 4    decisions C++ 46    Go 38     82%   thin 0   dead 0
SpawnMonster   compared 15   decisions C++ 54    Go 60    111%   thin 1   dead 0 (+3 dead upstream too)
```

`(+N dead upstream too)` counts methods with no caller in the **C++** source
either. Porting one and leaving it unreachable is 1:1, not a gap. They live in
`DEAD_OK` at the top of `semantic-parity.sh`, each with a reason, each
re-checkable with a grep. **Add to that list only after grepping `../src` and
finding nothing but the declaration and the definition.** It is the one place in
the tooling where a wrong entry hides real work.

---

## 3. What is actually left

### 3a. The four unreachable methods (each blocked on a missing subsystem)

These are not wiring jobs. Do not try to "reach" them by inventing a call site —
that is how you get a method that is technically called and behaviourally wrong.

| method | blocked on |
|---|---|
| `Monster::canWalkOnFieldType` | magic fields |
| `Monster::isIgnoringFieldDamage` | magic fields |
| `Monster::checkCanApplyCharm` | the composed damage message |
| `House::executeTransfer` | player-to-player trade |

**Magic fields.** No field item anywhere in the port applies damage or a
condition. `items.ItemTypeMagicField` exists as a type constant and nothing
reads it. What is needed: a field → `CombatType` mapping, per-tick field damage,
and the walk guard in `Tile::__queryAdd` (`src/items/tile.cpp:710`). Once that
exists, `Monster.CanWalkTo` gains the field branch and both methods are reached
from it. `ignoreFieldDamage` is already set correctly by `Monster.DrainHealth`;
it just has nobody asking.

**The composed damage message.** `Game::combatChangeHealth`
(`src/game/game.cpp:8890`) builds the "X loses N hitpoints due to your attack"
string and appends `" (low blow charm)"` when `checkCanApplyCharm` says so. The
port sends damage text from the combat engine without composing it. Port the
message builder and this falls out.

**Player-to-player trade.** `World.PlayerRequestTrade` and `PlayerAcceptTrade`
are empty function bodies (`internal/game/world.go`). The protocol handlers for
`0x7D`/`0x7E`/`0x7F`/`0x80` parse and discard. `house:startTrade` already runs
upstream's full validation chain and stops exactly where `internalStartTrade`
would be called, performing the reset that upstream performs on failure — so
once trade exists, that one call site completes the house transfer flow.

### 3b. The four thin methods

Thin means the Go body branches far less than the C++ one. Go and look; some are
legitimate, some are missing logic.

| method | Go/C++ branches | what to check |
|---|---|---|
| `Monster::death` | 6/17 | probably a real reduction — the port splits it across `Death`, `GetCorpse` and `DropLoot`. Confirm before "fixing". |
| `Monster::onCreatureAppear` | 2/6 | upstream's extra branches are summon/master bookkeeping. |
| `Monster::onThink` | 2/7 | the port moved the timers into the AI engine. Confirm nothing was dropped in the move. |
| `SpawnMonster::startup` | 4/11 | genuinely missing: the `RANDOM_MONSTER_SPAWN` block (`spawn_monster.cpp:241`) that merges monster types across spawn blocks. Config-gated, off by default. |

### 3c. Classes nobody has measured

Only five classes have been through `behaviour_coverage`. The big ones have not:

| class | C++ lines |
|---|---|
| `Player` | 13232 |
| `Game` | 13104 |
| `Item` | 3690 |
| `Combat` | 2730 |
| `Creature` | 2239 |
| `Tile` | 2030 |
| `Container` | 1387 |

Adding a row is one line at the bottom of `scripts/parity.sh`:

```bash
behaviour_coverage "creatures/players/player.cpp" "internal/game" "Player"
```

and the same in `semantic-parity.sh`:

```bash
report "creatures/players/player.cpp" "Player" "internal/game" "Player"
```

Expect the first run on `Player` to be ugly. That is the point.

### 3d. Everything else the scripts report

```
inbound opcodes dispatched     C++ 152   Go 140    92%
outbound opcodes sent          C++ 159   Go 112    70%
Lua class methods              C++ 1225  Go 990
schema tables referenced       48 in schema, 35 referenced
```

Two `parse*` handlers exist and are never dispatched: `parseBuyBlessing`,
`parseReportViolation`. Decide per handler whether to wire the opcode or delete
the dead function — do not wire one by guessing its opcode.

Thirteen tables are never touched from Go: `account_ban_history`,
`account_bans`, `account_sessions`, `active_livestream_casters`,
`coins_transactions`, `daily_reward_history`, `forge_history`,
`global_storage`, `guild_invites`, `guildwar_kills`, `ip_bans`,
`player_namelocks`, `store_history`. Each is a feature that silently does not
persist.

Subsystem size ratios (a hint, not a verdict — `decay` sits at 146% and is
correct):

```
monster AI     C++ 4013  Go 907   22%
npc (core)     C++ 1270  Go 736   57%
house          C++ 1073  Go 730   68%
spawn          C++  511  Go 381   74%
map serialize  C++  468  Go 298   63%
```

---

## 4. Open bugs from the last runtime log

`canary.log` in the repo root is a real session. Three things in it are unsolved
and **none has been diagnosed to the root** — treat the theories below as
starting points, not conclusions.

**1. Store hireling purchase fails.**
```
[parseBuyStoreOffer] - Purchase failed due to an unhandled script error.
data/libs/gamestore/store.lua:244: bad argument #1 to (anonymous) (userdata expected, got nil)
```
Line 244 is `for v in match do`, iterating a `string.gmatch` result inside
`canUseHirelingName`. "userdata expected" is gopher-lua's `CheckUserData`
message, which means a Go-registered function is being invoked as the iterator —
so something in the engine is shadowing `gmatch` or the string metatable.
Nothing in `internal/luaengine` or the datapack assigns `string.gmatch`, so the
next step is a focused Go test that runs that exact snippet through
`luaengine.Engine` and bisects from there.

**2. Modal windows are broken.**
```
data/libs/functions/modal_window_helper.lua:192:
attempt to index a non-table object(userdata) with key 'setPriority'
```
The `ModalWindow` userdata reaches Lua without a metatable carrying the methods
the datapack expects. Same class of failure as (1).

**3. NPCs behaving as bankers.** Reported by the repository owner, **not yet
investigated**. The suspects, in order, all come from the NPC batch (`598788d`):
`npc:isPlayerInteractingOnTopic` went from a hardcoded `false` to a real answer,
which changes which branch every topic-keyed dialogue takes; `npc:setSpeechBubble`
and `npc:setCurrency` went from no-ops to real writes; and the creature encoder
now sends the real speech-bubble byte instead of a hardcoded zero.
`SetSpeechBubble` writes to `n.Type`, shared by every NPC of that name — which
is what upstream does, but verify it is not being called with a stale type.
**Start by reverting `npcIsplayerinteractingontopic` to `false` locally and
seeing whether the behaviour goes away.** That isolates it in one run.

Also in the log, and almost certainly pre-existing: ~45 `[loadLuaMapAction] -
Wrong item id N found` errors. Map action/unique ids referencing item ids the
catalog does not have. Worth a pass, low priority.

---

## 5. How to do the work

### Find the C++ first, always

Before writing a line of Go, read the upstream function end to end. Then find
its **callers** — `grep -rn "\bmethodName(" ../src`. The caller tells you two
things the definition does not: where the behaviour belongs, and what the
arguments actually mean at that site. Half the bugs in this port came from
porting a function correctly and hanging it in the wrong place.

### Traps that have each cost real time here

- **`grep` on this machine respects `.gitignore`.** A module-wide rename
  silently skipped all of `cmd/canary`, and the verification grep came back
  clean while the build was broken. Use `find(1)` when completeness matters.
- **Bash 3.2 on macOS has no associative arrays.** `declare -A` fails.
- **awk dynamic regex survives two levels of escaping.** `\*` degrades to a bare
  `*` and awk rejects it. Locate the line with `grep -n` and hand awk a number.
- **The LSP diagnostics in this repo are frequently stale**, reporting missing
  methods and impossible type assertions that do not exist. `go build ./...` is
  the authority. Do not chase LSP errors.
- **`go vet` has pre-existing failures** (`House` contains a `sync.RWMutex` and
  is copied in `db/house_xml.go` and `cmd/canary/main.go`). Not yours; do not
  let them mask a new one.
- **Many files are already unformatted at HEAD.** Run `gofmt -w` on the specific
  files you touched, never on a directory — `gofmt -w internal/game/` reformats
  dozens of files that are not yours and pollutes the diff.

### The comment standard

Comments carry the C++ reference and the *reason*, not a restatement of the
code. The pattern that has worked:

```go
// GetAccessList is House::getAccessList (house.cpp:483).
//
// The bool is what stops sendHouseWindow opening a window for a list id that
// does not name a real door — so the check is door existence, NOT whether a
// list has been written. Keying it on the map (which is what this did) meant a
// door nobody had set a list on yet could never have one set, because the
// window refused to open.
```

Three parts: what it is upstream and where, what the non-obvious bit does, and —
when fixing something — what the old behaviour was and why it was wrong. That
last part is what stops the bug being reintroduced.

When something genuinely cannot be ported, say so in place:

```go
// Upstream also mails a per-item breakdown to the store inbox
// (sendSaleLetterIfNeeded). That needs the store inbox container, which the
// port does not model yet, so the letter half is missing and the summary
// line is not.
```

### The commit standard

Subject as `type(scope): imperative`. The body explains what was wrong before,
not what the code now does — the diff already says that. When wiring reveals
bugs, list them; that list is the most valuable part of the message. End with:

```
Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

### Verification before every commit

```bash
go build ./...
go test ./...                     # 16 packages should report ok
bash scripts/semantic-parity.sh   # the number should have moved the right way
```

Runtime, when the change touches gameplay: `make smoke`, or the full stack via
`deploy/docker-compose.yml`, then read `canary.log` for new `level=ERROR`.
Expect *more* errors after removing a stub, not fewer — a stub returning `true`
was hiding a failure. That is the desired outcome; triage what surfaces.

---

## 6. What reachability taught us, so you do not relearn it

Of the 73 methods that had no caller, almost none was a forgotten call. The
method was dead because **the behaviour it belongs to did not exist**. Finding
that out is the entire value of the measurement. A sample of what it surfaced:

- No monster in the game could be poisoned, burned, slowed or hasted. Only
  `Player` implemented the condition-holder interface, so the combat adapter's
  type assertion failed for every monster and the condition vanished with no
  error.
- Every monster walked at exactly one tile per second. Movement ran on the
  one-second think loop, so a rat and a dragon moved identically and ground
  speed was ignored.
- `getHealingCombatValue` read the resistance map. Had anything called it, every
  fire-*resistant* monster would have been *healed* by fire.
- The NPC shop priced every purchase off the type's list, because the per-player
  registry was never filled — which is the entire purpose of that registry.
- The house access-list window did not exist at all: two Lua bindings returned
  `true` without doing anything, and the `0x8A` handler read two bytes and
  discarded them, which is not even the right packet shape.

So: when you find a dead method, **do not look for somewhere to call it from.**
Look for the feature it belongs to and ask whether that feature is present. The
answer is usually no, and that is the finding.

The same applies to a stub that returns a plausible value. `return true`,
`return 0` and `L.Push(lua.LTrue)` are the three shapes that hid the most here.
Grepping for them is a productive afternoon:

```bash
grep -rn "func .*lua.LState) int {$" -A 1 internal/luaengine | grep -B 1 "return 0$"
```
