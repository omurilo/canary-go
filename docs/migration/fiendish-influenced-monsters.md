# Migration Spec: Fiendish & Influenced Monsters (Monstros Fiendish e Influenced)

## 1. Overview
Influenced monsters are randomly empowered creatures across the map with increased HP, attack power, and Forge Dust drop rates (1 to 5 stacks). Fiendish monsters are rare server-wide targets marked by a red aura, granting high Forge Dust / Slivers and trackable via `Exaltation Forge` Find Fiendish Monster.

## 2. C++ Source References
* `src/creatures/monsters/monsters.cpp` (Fiendish/Influenced spawns)
* `src/items/forge/forge.cpp`

## 3. Go Target Packages
* `canary-go/internal/game/monster.go`
* `canary-go/internal/game/spawns/`
* `canary-go/internal/protocol/game_encode.go` (Aura rendering opcode)

## 4. Functional Requirements
1. **Periodic Fiendish Spawn Routine:** Every X minutes, select a eligible monster on the map to promote to Fiendish state.
2. **Influenced Level Scaling:** Randomly spawn Influenced monsters (Forge stack level 1-5) with scaled health and damage.
3. **Loot Drop:** On death, Fiendish/Influenced creatures drop Forge Dust and Slivers directly into player inventory or corpse.
