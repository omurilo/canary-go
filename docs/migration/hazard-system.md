# Migration Spec: Hazard System (Sistema de Hazard)

## 1. Overview
The Hazard System increases dungeon difficulty dynamically (Hazard Levels 1 to 12+), scaling monster damage, speed, health, and spawn rates, while boosting player XP, Loot multipliers, and Plunder Patriarch spawns (e.g., Gnomprona / Timepiece areas).

## 2. C++ Source References
* `src/creatures/monsters/components/hazard.hpp`
* `src/creatures/monsters/components/hazard.cpp`

## 3. Go Target Packages
* `canary-go/internal/game/hazard.go`
* `canary-go/internal/game/combat_engine.go`

## 4. Functional Requirements
1. **Area Hazard State:** Track Hazard levels per zone/map area.
2. **Dynamic Multipliers:** Apply Hazard level scaling formulas to monster damage output and player incoming damage in `combat_engine.go`.
3. **Hazard Pods & Hazards Spawns:** Spawn environmental Pod hazards (e.g. lava/poison pools) and bonus boss encounters on high Hazard levels.
