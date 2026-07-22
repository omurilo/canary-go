# Migration Spec: Raids & Invasions Engine (Engine de Raids / Invasões)

## 1. Overview
The Raids Engine triggers scheduled or randomized city invasions/events (e.g. Ferumbras, Ghazbaran, Orshabaal, Rats in Thais). It manages timed spawn waves, broadcast messages, and raid margin boundaries.

## 2. C++ Source References
* `src/creatures/monsters/raids/raids.hpp`
* `src/creatures/monsters/raids/raids.cpp`
* `data/raids/raids.xml`

## 3. Go Target Packages
* `canary-go/internal/game/raids.go`
* `canary-go/internal/game/spawns/`

## 4. Functional Requirements
1. **XML Raid Loader:** Parse `data/raids/raids.xml` defining raid intervals, probability, announce messages, and spawn positions.
2. **Raid Executor:** Schedule raid waves using Go timers/tickers and broadcast world messages (`w.BroadcastTextMessage`).
3. **Spawn Placement:** Verify tile walkability before spawning invasion monster waves.
