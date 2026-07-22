# Migration Spec: Quick Loot / Auto-Loot Filtering (Coleta Rápida)

## 1. Overview
Quick Loot allows players to loot monster corpses instantly (Shift + Right-Click) into assigned Loot Backpacks based on configured categories (Gold, Equipment, Creature Products, Valuables) or explicit Skipped/Accepted item filters.

## 2. C++ Source References
* `src/creatures/players/player.cpp` (`Player::quickLoot`)
* `src/server/network/protocol/protocolgame.cpp` (Opcode `0x83` Quick Loot Settings)

## 3. Go Target Packages
* `canary-go/internal/game/quick_loot.go`
* `canary-go/internal/game/container.go`

## 4. Database Schema
* Table: `player_quick_loot` (`player_id`, `filter_type`, `item_id`)

## 5. Protocol Opcodes
* Client → Server: `0x83` (Quick Loot Config / Set Container Destination)

## 6. Functional Requirements
1. **Filter Matching:** Match corpse items against player's accepted/skipped loot list.
2. **Container Routing:** Move matching items directly into assigned designated containers (e.g. Gold Container, Product Container) or main backpack.
