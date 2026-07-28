# C++ → Go Migration Status

## 📊 Comparison

### ✅ COMPLETOS

| System | C++ | Go | Notes |
|--------|-----|----|-------|
| Login Protocol | ✅ | ✅ | |
| Game Protocol (1525) | ✅ | ✅ | |
| Multi-protocol (1100/860) | ✅ | ✅ | |
| Status Protocol | ✅ | ✅ | |
| Item Movement | ✅ | ✅ | |
| Containers | ✅ | ✅ | |
| Market | ✅ | ✅ | |
| Party | ✅ | ✅ | |
| Guild | ✅ | ✅ | |
| VIP | ✅ | ✅ | |
| Combat Engine | ✅ | ✅ | |
| Spells (Lua) | ✅ | ✅ | |
| A* Pathfinding | ✅ | ✅ | |
| Spawn Engine | ✅ | ✅ | |
| AI Engine | ✅ | ✅ | |
| Monster Spells AI | ✅ | ✅ | |
| Forge System | ✅ | ✅ | |
| Wheel of Destiny | ✅ | ✅ | |
| Gem Atelier | ✅ | ✅ | |
| Prey System | ✅ | ✅ | |
| Task Hunting | ✅ | ✅ | |
| Bosstiary | ✅ | ✅ | |
| Bestiary | ✅ | ✅ | |
| Charms | ✅ | ✅ | |
| Imbuements | ✅ | ✅ | |
| Quick Loot | ✅ | ✅ | |
| Loot Containers | ✅ | ✅ | |
| Stash | ✅ | ✅ | |
| Gold Pouch | ✅ | ✅ | |
| Reward Chest | ✅ | ✅ | |
| Blessings | ✅ | ✅ | |
| Death System | ✅ | ✅ | |
| Houses | ✅ | ✅ | |
| Bed System | ✅ | ✅ | |
| Mounts | ✅ | ✅ | |
| Outfits | ✅ | ✅ | |
| Familiars | ✅ | ✅ | |
| Achievements | ✅ | ✅ | |
| Badges | ✅ | ✅ | |
| Titles | ✅ | ✅ | |
| Depot / Inbox | ✅ | ✅ | |
| Mail System | ✅ | ✅ | |
| Chat Channels | ✅ | ✅ | |
| Modal Windows | ✅ | ✅ | |
| Cyclopedia Character | ✅ | ✅ | |
| Highscore | ✅ | ✅ | |
| Team Finder | ✅ | ✅ | |
| Wait List | ✅ | ✅ | |
| Ban / IP Block | ✅ | ✅ | |
| Attached Effects | ✅ | ✅ | |
| Rule Violation | ✅ | ✅ | |
| LuaScriptBytecodeCache | ✅ | ✅ | |
| Lua Script Debug | ✅ | ✅ | |
| Lua API (player stubs) | ✅ | ✅ | 136/136 |
| Game Store | ✅ | ✅ | (Lua) |
| NPC Dialog | ✅ | ✅ | (Lua) |
| Exercise Training | ✅ | ✅ | (Lua) |
| Offline Training | ✅ | ✅ | |
| Full Config (369 keys) | ✅ | ✅ | |
| Datapack Download | ✅ | ✅ | |
| Custom Map Loading | ✅ | ✅ | OTBM + spawns + NPCs + houses |
| Open Containers Persist | ✅ | ✅ | |
| Multi-protocol profiles | ✅ | ✅ | Current/1100/860 |
| **Protocol opcodes** | **~163** | **132** | **81% coverage** |

### 📈 Metrics (honest)

| Metric | C++ | Go | Coverage |
|--------|-----|----|----------|
| `protocolgame.cpp` / Go handlers | 12,332 lines | ~6,000 | ~50% |
| `player.cpp` / `player.go` | 15,213 lines | 3,597 | ~24% |
| Lua API functions | ~1,300 | ~500 | ~38% |
| Opcodes handled | 163 | 132 | ~81% |
| Core game systems | ~60 | ~58 | ~97% |
| Protocol depth (edge cases) | Complete | Minimal | ~30% |
| Lua files (scripts) | Large set | Large set | ~90% (shared) |

### 🔴 NON-EXISTENT

| System | C++ | Go | Notes |
|--------|-----|----|-------|
| **Livecast** (livestream) | ✅ | ❌ | Player spectate system |

### Key differences from C++

1. **Player depth (24%)**: Go player has core fields/methods but C++ has 5x more lines — many setter/getter/validation functions not yet ported
2. **Protocol depth (30%)**: Core opcodes work but edge cases, validations, and error handling are minimal in Go
3. **Lua API (38%)**: ~500 of ~1300 Lua functions bound. Missing: many item functions, creature functions, container functions
4. **Livecast**: Not ported (low priority spectate system)

### Files

| Category | C++ src/ | Go internal/ |
|----------|----------|-------------|
| Core server files | ~450 .cpp/.hpp | ~130 .go |
| Protocol | 16 files | ~40 files |
| Game model | ~30 files | ~55 files |
| Lua engine | ~100 files | ~40 files |
| Database | ~20 files | ~20 files |
