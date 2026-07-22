# Migration Spec: Bosstiary & Boss Slot (Bosstiary e Slot de Boss)

## 1. Overview
Bosstiary tracks boss kills categorized by difficulty (Bane, Archfoe, Nemesis). Unlocking boss entries grants Boss Points and allows assigning a boss to the Boss Slot for increased equipment drop rates (+25% loot bonus).

## 2. C++ Source References
* `src/creatures/players/components/bosstiary.hpp`
* `src/creatures/players/components/bosstiary.cpp`

## 3. Go Target Packages
* `canary-go/internal/game/bosstiary.go`
* `canary-go/internal/protocol/game_encode.go`

## 4. Database Schema
* Table: `player_bosstiary` (`player_id`, `boss_id`, `kills`, `slot_assigned`)

## 5. Protocol Opcodes
* Client → Server: `0xEC` (Bosstiary Open / Assign Slot)
* Server → Client: `0xD7` (Bosstiary Data Window)

## 6. Functional Requirements
1. **Kill Tracking:** Differentiate normal monster kills from Archfoe/Nemesis boss kills upon death.
2. **Loot Bonus Proc:** Apply the Equipment Loot Bonus multiplier when generating boss corpse loot tables.
