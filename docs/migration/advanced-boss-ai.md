# Migration Spec: Advanced Boss AI (IA Avançada de Bosses)

## 1. Overview
Advanced Boss AI expands basic monster behavior with multi-phase mechanics, dynamic arena transformations, immunity toggles, environmental hazards, summons, and mechanics (e.g. Soul War, Secret Library, Rotten Blood bosses).

## 2. C++ Source References
* `src/creatures/monsters/monster.cpp`
* `data/scripts/bosses/`

## 3. Go Target Packages
* `canary-go/internal/game/ai_engine.go`
* `canary-go/internal/luaengine/monster.go`

## 4. Functional Requirements
1. **Health Threshold Phase Triggers:** Fire Lua events when boss health drops below phase thresholds (e.g., 75%, 50%, 25%).
2. **Dynamic Immunities & Resistance Shifts:** Support changing elemental damage multipliers mid-combat via Lua API (`monster:setElementResistance(...)`).
3. **Arena Teleportation & Mechanics:** Teleport players/bosses to sub-arenas and track phase timers accurately.
