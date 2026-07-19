# Canary-Go — GM / God Commands

In-game commands typed in the chat, starting with `/`. They are handled natively in
Go (`internal/protocol/game_commands.go`) instead of Lua talkactions, and mirror the
behaviour of the reference C++ Canary talkactions where a matching subsystem exists.

A command is intercepted before chat broadcast: if the text starts with `/` and
matches a known command it is executed and **not** shown as speech.

---

## Implemented

| Command | Usage | Description |
|---|---|---|
| `/pos` | `/pos` | Shows your current position `[x, y, z]`. Alias: `/position`. |
| `/goto` | `/goto <x> <y> [z]` | Teleport to coordinates. Commas allowed (`/goto 5010, 4998, 7`). If `z` is omitted, keeps the current floor. Aliases: `/go`, `/cliport`. |
| `/up` | `/up` | Teleport one floor up (lower `z`). |
| `/down` | `/down` | Teleport one floor down (higher `z`). |
| `/town` | `/town <name>` | Teleport to a town's temple. With no name, lists the towns. Alias: `/t`. |
| `/i` | `/i <id> [count]` | Create an item on the tile under you. `id` is the client item id; `count` (default 1) sets the stack/subtype. Alias: `/create`. |
| `/addskill` | `/addskill <skill> [n]` | Raise a skill by `n` (default 1) and resend skills. Skills: `fist`, `club`, `sword`, `axe`, `dist`, `shield`, `fish`, `ml`. |
| `/save` | `/save` | Persist every online player to the database. |
| `/b` | `/b <text>` | Broadcast a status message to all online players. Alias: `/broadcast`. |
| `/commands` | `/commands` | List the available commands. Alias: `/help`. |

### Examples

```
/pos
/goto 5010 4998 7
/goto 5010, 4998, 7
/town montag
/t alexandria
/up
/i 2160 5          -- 5 crystal coins on the floor
/addskill sword 10
/save
/b Server restarting in 5 minutes
```

### Teleport behaviour

`/goto`, `/up`, `/down`, `/town` all go through the same teleport path, mirroring the
teleport branch of `ProtocolGame::sendMoveCreature`:

1. Remove the creature from the old tile (for other spectators and for you).
2. Move the player and send a full map description (`0x64`) at the destination.
3. Play the teleport magic effect (`CONST_ME_TELEPORT`).
4. Re-appear for spectators around the destination.

Any in-flight click-to-move (auto-walk) is cancelled first.

---

## Not migrated yet

The reference C++ Canary ships ~60 god talkactions. Most depend on gameplay
subsystems that the Go port has not built yet, so they currently reply
`Command /x is not migrated yet.` They will be enabled as each subsystem lands.

| Command(s) | Blocked on |
|---|---|
| `/m`, `/n`, `/s` (create monster / npc / summon), `/setmonstername` | Creatures & AI |
| `/attr`, `/createloot`, `/clearloot`, `/countloot`, `/addloot`, `/createtestshop` | Item attributes (OTBR blob), loot |
| `/getstorage`, `/setstorage`, `/getkv`, `/getallkv`, `/setkv` | Player storage / key-value store |
| `/addcharms`, `/addminorcharms`, `/resetcharms`, `/charmexpansion`, `/charmrunes` | Charms system |
| `/openforge`, `/adddusts`, `/setdusts`, `/adddustlevel`, `/getdusts`, `/removedusts` | Forge system |
| `/addbadge`, `/testicon`, `/playericon`, `/bakragoreicon` | Badges / icons |
| `/addtitle`, `/settitle` | Titles |
| `/setbestiary`, `/addbosskill`, `/addbosstiarykills`, `/fiendish`, `/influenced`, `/setfiendish` | Bestiary / bosstiary |
| `/addachievement`, `/removeachievement`, `/checkachievements` | Achievements |
| `/vip`, `/hireling`, `/clearhirelingstas`, `/hirelinglamp` | Account / hirelings |
| `/addmount`, `/addaddon`, `/rewardbag`, `/addreward`, `/inbox` | Inventory / rewards |
| `/gotohouse`, `/owner` | Houses |
| `/raid`, `/simraid`, `/listraid` | Raids |
| `/reload` | Script hot-reload |
| `/ipban` | Ban system |
| `/hasflag`, `/setflag`, `/removeflag`, `/flags` | Player flags |
| `/zones` | Zones |
| `/resetcd`, `/clearcooldown` | Spell cooldowns |
| `/addmoney`, `/adddusts` | Inventory items |
| `/areasound`, `/internalsound`, `/globalsound`, `/ambientsound`, `/musicsound` | Sound effects |
| `/proficiency` | Weapon proficiency |
| `/add_condition`, `/testtaintconditions` | Conditions |
| `/openserver`, `/closeserver` | Server login gating |

> Note: `/i` in the C++ version adds the item to the player's backpack. The Go port
> has no player inventory yet, so `/i` places the item on the ground instead.

---

## Related in-game actions (not commands)

These are client actions (not `/` commands) that also need subsystems before they work:

- **Open a chest / container** — client sends `0x82` (use-item). Needs the container
  subsystem (`sendContainer` `0x6E` + item contents). Example capture:
  `0x82` `pos(5006,5016,7)` itemId `3500` stackpos `2`.
- **Use a ladder / stairs (floor change)** — needs `MoveUpCreature` (`0xBE`) /
  `MoveDownCreature` (`0xBF`) plus floor-change tile detection.
- **Move / drop an item** — client sends `0x78` (throw). Needs item movement.
- **Look at** — client sends `0x8C`. Needs item/creature descriptions.
