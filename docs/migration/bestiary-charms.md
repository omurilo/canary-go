# Migration Spec: Bestiary & Charms (Bestiário e Charms)

## 1. Overview
The Bestiary system tracks monster kills per player across three unlock tiers (First, Second, Complete). Unlocking a monster awards Charm Points, which are used to purchase and assign Charms (e.g., Wound, Freeze, Zap, Dodge, Gut) to specific completed monster entries.

## 2. C++ Source References
* `src/creatures/players/components/bestiary.hpp`
* `src/creatures/players/components/bestiary.cpp`
* `src/creatures/players/components/charms.cpp`

## 3. Go Target Packages
* `canary-go/internal/game/bestiary.go`
* `canary-go/internal/game/combat_engine.go` (Charm procs)
* `canary-go/internal/protocol/game_encode.go`

## 4. Database Schema
* Table: `player_kills` (`player_id`, `monster_id`, `count`)
* Table: `player_charms` (`player_id`, `charm_id`, `monster_id`)

## 5. Protocol Opcodes
* Client → Server: `0xEB` (Open Bestiary / Select Charm / Assign Charm)
* Server → Client: `0xD5` (Bestiary Data Payload)

## 6. Functional Requirements
1. **Kill Counter:** Increment monster kill counters upon death in `combat_engine.go`.
2. **Unlock Milestones:** Notify player when reaching unlock thresholds (e.g. 25, 250, 1000 kills) and grant Charm Points.
3. **Charm Execution:** On monster hit, calculate proc chance (10% base) and execute Charm elemental damage or status effect.
