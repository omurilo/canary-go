# Migration Spec: Exaltation Forge (Forja de Exaltação)

## 1. Overview
The Exaltation Forge allows players to fuse identical weapons/armors/helmets to increase their Tier (up to Tier 10), unlocking special combat effects (Onslaught, Ruse, Momentum, Transcendence). It also supports Tier Transfer and Dust/Sliver conversion.

## 2. C++ Source References
* `src/items/forge/forge.hpp`
* `src/items/forge/forge.cpp`
* `src/server/network/protocol/protocolgame.cpp` (Opcodes `0xDC` Forge Actions)

## 3. Go Target Packages
* `canary-go/internal/game/forge.go`
* `canary-go/internal/protocol/game_actions.go`
* `canary-go/internal/luaengine/item.go`

## 4. Database Schema
* Table: `player_forge`
  * `player_id` (INT)
  * `dust` (INT)
  * `dust_limit` (INT)
  * `slivers` (INT)
  * `cores` (INT)

## 5. Protocol Opcodes
* Client → Server: `0xDC` (Forge Request: Fusion, Transfer, Dust Conversion)
* Server → Client: `0xDB` (Forge UI State, success/failure result, updated dust/sliver counts)

## 6. Functional Requirements
1. **Fusion Mechanics:** Verify same item ID, tier, required dust, cores, and gold fee.
2. **Success & Success Rate Modifiers:** Calculate base success rate (50%) + Exaltation Core bonus (+20%) + Tier Transfer reduction rules.
3. **Tier Bonuses in Combat Engine:** Trigger Onslaught (critical extra damage), Ruse (dodge damage), and Momentum (cooldown reduction) in `combat_engine.go`.
