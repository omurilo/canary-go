
### Size

| what | C++ | Go | note |
|---|---|---|---|
| source files | 486 | 255 | 52% |
| lines of code | 177374 | 76419 | 43% |
| test lines | - | 13321 |  |

### Inbound opcodes (client -> server)

| what | C++ | Go | note |
|---|---|---|---|
| dispatched cases | 152 | 141 | 92% |
| parse* defined | - | 109 |  |
| parse* dispatched | - | 107 | 2 never reached |

### Outbound opcodes (server -> client)

| what | C++ | Go | note |
|---|---|---|---|
| distinct opcodes sent | 159 | 112 | 70% |

### Lua API

| what | C++ | Go | note |
|---|---|---|---|
| class methods | 1225 | 989 | names only, not behaviour |
| enum globals | 1312 | 0 | 0 missing, 0 wrong (snapshot) |

### Database tables referenced from code

| what | C++ | Go | note |
|---|---|---|---|
| tables in schema | - | 48 |  |
| referenced from Go | - | 35 | 72% |

Never referenced: account_ban_history account_bans account_sessions active_livestream_casters coins_transactions daily_reward_history forge_history global_storage guild_invites guildwar_kills ip_bans player_namelocks store_history

### Subsystem size ratios

| what | C++ | Go | note |
|---|---|---|---|
| monster AI | 4013 | 479 | 11% |
| npc | 1270 | 122 | 9% |
| house | 1073 | 628 | 58% |
| spawn | 511 | 385 | 75% |
| decay | 224 | 60 | 26% |
| map serialize | 468 | 298 | 63% |

