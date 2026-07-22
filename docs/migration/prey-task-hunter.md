# Migration Spec: Prey & Task Hunter (Prey e Task Hunter)

## 1. Overview
The Prey System grants players 3 prey slots to select target monsters for 2-hour bonuses (Damage Boost, Damage Reduction, XP Bonus, or Improved Loot). Task Hunter allows selecting monster tasks for Hunting Task Points and rewards.

## 2. C++ Source References
* `src/creatures/players/components/prey.hpp`
* `src/creatures/players/components/prey.cpp`
* `src/creatures/players/components/task_hunter.cpp`

## 3. Go Target Packages
* `canary-go/internal/game/prey.go`
* `canary-go/internal/game/task_hunter.go`
* `canary-go/internal/protocol/game_actions.go`

## 4. Database Schema
* Table: `player_prey` (`player_id`, `slot`, `monster_name`, `bonus_type`, `bonus_value`, `time_left`)
* Table: `player_task_hunter` (`player_id`, `task_id`, `kills`, `points`)

## 5. Protocol Opcodes
* Client → Server: `0xEA` (Prey Action / Roll / Lock)
* Server → Client: `0xD4` (Prey Window Data)

## 6. Functional Requirements
1. **Reroll & Selection:** Support Free Reroll (20h cooldown) and Wildcard Rerolls (Tibia Coins).
2. **Timer Countdown:** Decrement prey time only while player is actively in combat with monsters.
3. **Bonus Calculation:** Multiply damage/XP/loot drops in `combat_engine.go` when attacking active prey targets.
